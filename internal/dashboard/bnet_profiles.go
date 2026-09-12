package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/library/persist"
)

// bnetProfileTTL honours the 24h freshness Blizzard itself specifies on the
// aurora-profile endpoint via Cache-Control: private, max-age=86400. Callers
// following a live play session can request a tighter bound via max_age (the
// profile's game_results move with every game played), floored at
// bnetProfileMinMaxAge so polling can't turn the cache into a pass-through —
// the #319 rate limiter stays the hard backstop either way.
const (
	bnetProfileTTL       = 24 * time.Hour
	bnetProfileMinMaxAge = time.Minute
)

var errBnetBridgeUnavailable = errors.New("SC:R bridge is not connected")

type bnetProfileResult struct {
	Toon        string    `json:"toon"`
	Gateway     int64     `json:"gateway"`
	Found       bool      `json:"found"`
	AuroraID    int64     `json:"aurora_id,omitempty"`
	BattleTag   string    `json:"battle_tag,omitempty"`
	CountryCode string    `json:"country_code,omitempty"`
	FetchedAt   time.Time `json:"fetched_at"`
	Cached      bool      `json:"cached"`
	Stale       bool      `json:"stale,omitempty"`
}

// getOrFetchBnetProfile serves the cached profile when younger than maxAge
// (clamped to [1min, 24h]; pass 0 for the default 24h TTL), otherwise
// refetches through the rate-limited facade and upserts. A failed refetch
// (budget exhausted, cooldown, bridge gone) falls back to the freshest row we
// have, flagged Stale.
func (d *Dashboard) getOrFetchBnetProfile(ctx context.Context, toon string, gateway int64, prio bnetfacade.Priority, maxAge time.Duration) (*bnetProfileResult, error) {
	if maxAge <= 0 || maxAge > bnetProfileTTL {
		maxAge = bnetProfileTTL
	}
	if maxAge < bnetProfileMinMaxAge {
		maxAge = bnetProfileMinMaxAge
	}
	cached, err := d.dbStore.GetBnetProfile(ctx, toon, gateway)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if cached != nil && now.Sub(cached.FetchedAt) < maxAge {
		return bnetProfileResultFromRecord(cached, true, false), nil
	}

	fresh, err := d.fetchAndCacheBnetProfile(ctx, toon, gateway, prio, now)
	if err != nil {
		if cached != nil {
			return bnetProfileResultFromRecord(cached, true, true), nil
		}
		return nil, err
	}
	return bnetProfileResultFromRecord(fresh, false, false), nil
}

// fetchAndCacheBnetProfile is the one place the upstream payload exists: it is
// distilled into the typed profile record and the game archive's records, and
// the raw bytes are discarded.
func (d *Dashboard) fetchAndCacheBnetProfile(ctx context.Context, toon string, gateway int64, prio bnetfacade.Priority, now time.Time) (*persist.BnetProfile, error) {
	addr, _ := d.bnetAddr.Load().(string)
	if addr == "" || d.bnetDisabled.Load() {
		return nil, errBnetBridgeUnavailable
	}
	p, err := bnetfacade.FetchAuroraProfile(ctx, addr, toon, int(gateway), prio)
	if err != nil {
		return nil, err
	}
	profile, games := persist.DistillBnetProfile(toon, gateway, now, p.Raw)
	if err := d.dbStore.UpsertBnetProfile(ctx, profile); err != nil {
		return nil, err
	}
	if err := d.dbStore.UpsertBnetGames(ctx, games); err != nil {
		log.Printf("[bnet-profile] archiving games for %q: %v", toon, err)
	}
	return &profile, nil
}

func (d *Dashboard) countryCodesByPlayerKeys(playerKeys []string) (map[string]string, error) {
	if len(playerKeys) == 0 {
		return map[string]string{}, nil
	}
	return d.dbStore.GetBnetCountryCodesByPlayerKeys(d.ctx, playerKeys)
}

var defaultGatewayOrder = []int64{30, 20, 10, 45, 11}

// bnetProfileBackfillMaxPlayers bounds how many players one page view may put
// into the backfill queue. A player who is not in the cache costs up to one
// request per gateway, so an uncapped players page (25 rows x 5 gateways) could
// spend a fifth of the daily bridge budget in a single navigation. Callers pass
// their most significant players first, so the cap keeps the rows a user
// actually cares about and drops the tail; the tail fills in as they page or
// revisit.
const bnetProfileBackfillMaxPlayers = 20

func (d *Dashboard) triggerBnetProfileFetchesForPlayers(names []string, gameSource string) {
	if gameSource != "AssumedBattleNet" {
		return
	}
	d.backfillBnetProfiles(names)
}

// backfillBnetProfiles fetches, in the background, profiles that are missing
// or stale (older than bnetProfileTTL). Names must be ordered by how much the
// caller cares about them: the list is truncated to
// bnetProfileBackfillMaxPlayers.
func (d *Dashboard) backfillBnetProfiles(names []string) {
	if d.bnetDisabled.Load() {
		return
	}
	addr, _ := d.bnetAddr.Load().(string)
	if addr == "" {
		return
	}
	playerKeys := make([]string, 0, len(names))
	for _, n := range names {
		playerKeys = append(playerKeys, normalizePlayerKey(n))
	}
	fetchTimes, err := d.dbStore.GetBnetFetchedAtByPlayerKeys(d.ctx, playerKeys)
	if err != nil {
		return
	}
	now := time.Now()
	var stale []string
	seen := map[string]struct{}{}
	for _, n := range names {
		key := normalizePlayerKey(n)
		if fetchTime, ok := fetchTimes[key]; ok && now.Sub(fetchTime) < bnetProfileTTL {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		stale = append(stale, n)
	}
	if len(stale) == 0 {
		return
	}
	if len(stale) > bnetProfileBackfillMaxPlayers {
		stale = stale[:bnetProfileBackfillMaxPlayers]
	}
	gateways := defaultGatewayOrder
	if known := d.bnetGateway.Load(); known > 0 {
		gateways = append([]int64{known}, defaultGatewayOrder...)
	}
	d.bnetBackfillActive.Add(1)
	go func() {
		defer crashreport.GuardNonFatal(nil)
		defer d.bnetBackfillActive.Add(-1)
		for _, toon := range stale {
			for _, gw := range gateways {
				res, fetchErr := d.getOrFetchBnetProfile(d.ctx, toon, gw, bnetfacade.PriorityBackground, 0)
				if fetchErr != nil {
					continue
				}
				if res.Found {
					break
				}
			}
		}
	}()
}

func bnetProfileResultFromRecord(p *persist.BnetProfile, cached, stale bool) *bnetProfileResult {
	return &bnetProfileResult{
		Toon:        p.Toon,
		Gateway:     p.Gateway,
		Found:       p.Found,
		AuroraID:    p.AuroraID,
		BattleTag:   p.BattleTag,
		CountryCode: p.CountryCode,
		FetchedAt:   p.FetchedAt,
		Cached:      cached,
		Stale:       stale,
	}
}

func (d *Dashboard) handlerBnetProfile(w http.ResponseWriter, r *http.Request) {
	toon := r.URL.Query().Get("toon")
	gateway, err := strconv.ParseInt(r.URL.Query().Get("gateway"), 10, 64)
	if toon == "" || err != nil {
		http.Error(w, "toon and gateway query parameters are required", http.StatusBadRequest)
		return
	}
	if _, ok := bnetfacade.GatewayNames[int(gateway)]; !ok {
		http.Error(w, "unknown gateway", http.StatusBadRequest)
		return
	}
	var maxAge time.Duration
	if raw := r.URL.Query().Get("max_age_seconds"); raw != "" {
		secs, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || secs <= 0 {
			http.Error(w, "max_age_seconds must be a positive integer", http.StatusBadRequest)
			return
		}
		maxAge = time.Duration(secs) * time.Second
	}
	res, err := d.getOrFetchBnetProfile(r.Context(), toon, gateway, bnetfacade.PriorityUser, maxAge)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, errBnetBridgeUnavailable) {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

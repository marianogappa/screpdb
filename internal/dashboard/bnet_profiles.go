package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/crashreport"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
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
	Toon        string          `json:"toon"`
	Gateway     int64           `json:"gateway"`
	Found       bool            `json:"found"`
	AuroraID    int64           `json:"aurora_id,omitempty"`
	BattleTag   string          `json:"battle_tag,omitempty"`
	CountryCode string          `json:"country_code,omitempty"`
	Profile     json.RawMessage `json:"profile,omitempty"`
	FetchedAt   time.Time       `json:"fetched_at"`
	Cached      bool            `json:"cached"`
	Stale       bool            `json:"stale,omitempty"`
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
	row, err := d.dbStore.GetBnetProfile(ctx, toon, gateway)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if row != nil && now.Sub(row.FetchedAt) < maxAge {
		return bnetProfileResultFromRow(row, true, false), nil
	}

	fresh, err := d.fetchAndCacheBnetProfile(ctx, toon, gateway, prio, now)
	if err != nil {
		if row != nil {
			return bnetProfileResultFromRow(row, true, true), nil
		}
		return nil, err
	}
	return bnetProfileResultFromRow(fresh, false, false), nil
}

func (d *Dashboard) fetchAndCacheBnetProfile(ctx context.Context, toon string, gateway int64, prio bnetfacade.Priority, now time.Time) (*dashboarddb.BnetProfileRow, error) {
	addr, _ := d.bnetAddr.Load().(string)
	if addr == "" || d.bnetDisabled.Load() {
		return nil, errBnetBridgeUnavailable
	}
	p, err := bnetfacade.FetchAuroraProfile(ctx, addr, toon, int(gateway), prio)
	if err != nil {
		return nil, err
	}
	row := &dashboarddb.BnetProfileRow{
		Toon:        toon,
		Gateway:     gateway,
		Found:       p.Found(),
		AuroraID:    p.AuroraID,
		BattleTag:   p.BattleTag,
		CountryCode: p.CountryCode,
		Payload:     string(p.Raw),
		FetchedAt:   now.UTC(),
	}
	if err := upsertBnetProfileWithRetry(ctx, d.dbStore, *row); err != nil {
		return nil, err
	}
	return row, nil
}

// upsertBnetProfileWithRetry retries on "database is locked" with exponential
// backoff. Bridge responses are rate-limited (600/day), so losing a successful
// fetch because ingestion holds the write lock is wasteful — the retry cost is
// negligible compared to re-spending the budget.
func upsertBnetProfileWithRetry(ctx context.Context, store *dashboarddb.Store, row dashboarddb.BnetProfileRow) error {
	const maxAttempts = 10
	backoff := 100 * time.Millisecond
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := store.UpsertBnetProfile(ctx, row)
		if err == nil {
			return nil
		}
		if !strings.Contains(err.Error(), "database is locked") {
			return err
		}
		log.Printf("[bnet-profile] upsert retry %d/%d for %q: %v", attempt+1, maxAttempts, row.Toon, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 10*time.Second {
			backoff = 10 * time.Second
		}
	}
	return store.UpsertBnetProfile(ctx, row)
}

func (d *Dashboard) countryCodesByPlayerKeys(playerKeys []string) (map[string]string, error) {
	if len(playerKeys) == 0 {
		return map[string]string{}, nil
	}
	return d.dbStore.GetBnetCountryCodesByPlayerKeys(d.ctx, playerKeys)
}

var defaultGatewayOrder = []int64{30, 20, 10, 45, 11}

func (d *Dashboard) triggerBnetProfileFetchesForPlayers(names []string, gameSource string) {
	if d.bnetDisabled.Load() {
		log.Printf("[country-flags] skipping: bridge disabled")
		return
	}
	if gameSource != "AssumedBattleNet" {
		log.Printf("[country-flags] skipping: game_source=%q (not bnet)", gameSource)
		return
	}
	addr, _ := d.bnetAddr.Load().(string)
	if addr == "" {
		log.Printf("[country-flags] skipping: no bridge address")
		return
	}
	playerKeys := make([]string, 0, len(names))
	for _, n := range names {
		playerKeys = append(playerKeys, normalizePlayerKey(n))
	}
	cached, err := d.countryCodesByPlayerKeys(playerKeys)
	if err != nil {
		log.Printf("[country-flags] skipping: cache lookup error: %v", err)
		return
	}
	var uncached []string
	for _, n := range names {
		if _, ok := cached[normalizePlayerKey(n)]; !ok {
			uncached = append(uncached, n)
		}
	}
	if len(uncached) == 0 {
		log.Printf("[country-flags] all %d players already cached", len(names))
		return
	}
	gateways := defaultGatewayOrder
	if known := d.bnetGateway.Load(); known > 0 {
		gateways = append([]int64{known}, defaultGatewayOrder...)
	}
	log.Printf("[country-flags] fetching %d uncached players (gateways=%v)", len(uncached), gateways)
	go func() {
		defer crashreport.GuardNonFatal(nil)
		for _, toon := range uncached {
			for _, gw := range gateways {
				res, fetchErr := d.getOrFetchBnetProfile(d.ctx, toon, gw, bnetfacade.PriorityBackground, 0)
				if fetchErr != nil {
					log.Printf("[country-flags] fetch %q gw=%d: %v", toon, gw, fetchErr)
					continue
				}
				if res.Found {
					log.Printf("[country-flags] found %q on gw=%d country=%q", toon, gw, res.CountryCode)
					break
				}
			}
		}
	}()
}

func bnetProfileResultFromRow(row *dashboarddb.BnetProfileRow, cached, stale bool) *bnetProfileResult {
	res := &bnetProfileResult{
		Toon:        row.Toon,
		Gateway:     row.Gateway,
		Found:       row.Found,
		AuroraID:    row.AuroraID,
		BattleTag:   row.BattleTag,
		CountryCode: row.CountryCode,
		FetchedAt:   row.FetchedAt,
		Cached:      cached,
		Stale:       stale,
	}
	if row.Found {
		res.Profile = json.RawMessage(row.Payload)
	}
	return res
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

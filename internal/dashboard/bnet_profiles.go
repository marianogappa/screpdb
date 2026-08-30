package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
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
	if err := d.dbStore.UpsertBnetProfile(ctx, *row); err != nil {
		return nil, err
	}
	return row, nil
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

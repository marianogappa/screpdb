package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
)

func TestBnetProfileEndpoint_BadRequest(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()

	for _, path := range []string{
		"/api/custom/bnet/profile",
		"/api/custom/bnet/profile?toon=ByuN",
		"/api/custom/bnet/profile?gateway=30",
		"/api/custom/bnet/profile?toon=ByuN&gateway=abc",
		"/api/custom/bnet/profile?toon=ByuN&gateway=99",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", path, rec.Code)
		}
	}
}

func TestBnetProfileEndpoint_BridgeUnavailable(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/custom/bnet/profile?toon=ByuN&gateway=30", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503 when no bridge and no cache", rec.Code)
	}
}

func TestGetOrFetchBnetProfile_FreshCacheHitSkipsBridge(t *testing.T) {
	d := newTestDashboard(t)
	ctx := context.Background()

	seed := dashboarddb.BnetProfileRow{
		Toon: "ByuN", Gateway: 30, Found: true, AuroraID: 42,
		BattleTag: "ByuN#123", CountryCode: "KR",
		Payload: `{"aurora_id":42}`, FetchedAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := d.dbStore.UpsertBnetProfile(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// No bridge addr is set: a fetch attempt would fail, so success proves
	// the fresh row was served from cache.
	res, err := d.getOrFetchBnetProfile(ctx, "ByuN", 30, bnetfacade.PriorityUser, 0)
	if err != nil {
		t.Fatalf("getOrFetchBnetProfile: %v", err)
	}
	if !res.Cached || res.Stale || res.AuroraID != 42 {
		t.Errorf("got cached=%v stale=%v aurora_id=%d, want cached fresh 42", res.Cached, res.Stale, res.AuroraID)
	}
}

func TestGetOrFetchBnetProfile_StaleServedOnFetchFailure(t *testing.T) {
	d := newTestDashboard(t)
	ctx := context.Background()

	seed := dashboarddb.BnetProfileRow{
		Toon: "Stale", Gateway: 20, Found: true, AuroraID: 7,
		Payload: `{"aurora_id":7}`, FetchedAt: time.Now().UTC().Add(-25 * time.Hour),
	}
	if err := d.dbStore.UpsertBnetProfile(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := d.getOrFetchBnetProfile(ctx, "Stale", 20, bnetfacade.PriorityUser, 0)
	if err != nil {
		t.Fatalf("getOrFetchBnetProfile: %v", err)
	}
	if !res.Cached || !res.Stale || res.AuroraID != 7 {
		t.Errorf("got cached=%v stale=%v aurora_id=%d, want stale fallback 7", res.Cached, res.Stale, res.AuroraID)
	}
}

func TestGetOrFetchBnetProfile_FetchesAndCaches(t *testing.T) {
	d := newTestDashboard(t)
	ctx := context.Background()

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"aurora_id": 99, "battle_tag": "Fresh#1", "country_code": "DE"}`)
	}))
	defer srv.Close()
	d.bnetAddr.Store(srv.Listener.Addr().String())

	res, err := d.getOrFetchBnetProfile(ctx, "Fresh", 20, bnetfacade.PriorityUser, 0)
	if err != nil {
		t.Fatalf("getOrFetchBnetProfile: %v", err)
	}
	if res.Cached || res.AuroraID != 99 || res.BattleTag != "Fresh#1" || !res.Found {
		t.Errorf("fresh fetch: got %+v", res)
	}
	if calls != 1 {
		t.Fatalf("bridge calls: got %d, want 1", calls)
	}

	res, err = d.getOrFetchBnetProfile(ctx, "Fresh", 20, bnetfacade.PriorityUser, 0)
	if err != nil {
		t.Fatalf("second getOrFetchBnetProfile: %v", err)
	}
	if !res.Cached || calls != 1 {
		t.Errorf("second call: cached=%v bridge calls=%d, want cache hit with no new call", res.Cached, calls)
	}
	var payload struct {
		AuroraID int64 `json:"aurora_id"`
	}
	if err := json.Unmarshal(res.Profile, &payload); err != nil || payload.AuroraID != 99 {
		t.Errorf("cached payload: %v %+v", err, payload)
	}
}

func TestGetOrFetchBnetProfile_CachesUnknownToon(t *testing.T) {
	d := newTestDashboard(t)
	ctx := context.Background()

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"aurora_id": 0, "game_results": [], "replays": [], "toons": []}`)
	}))
	defer srv.Close()
	d.bnetAddr.Store(srv.Listener.Addr().String())

	for i := 0; i < 2; i++ {
		res, err := d.getOrFetchBnetProfile(ctx, "NoSuchToon", 10, bnetfacade.PriorityUser, 0)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if res.Found {
			t.Errorf("call %d: Found=true for unknown toon", i)
		}
	}
	if calls != 1 {
		t.Errorf("negative response not cached: %d bridge calls, want 1", calls)
	}
}

func TestGetOrFetchBnetProfile_MaxAgeForcesRefetch(t *testing.T) {
	d := newTestDashboard(t)
	ctx := context.Background()

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprintf(w, `{"aurora_id": %d}`, 100+calls)
	}))
	defer srv.Close()
	d.bnetAddr.Store(srv.Listener.Addr().String())

	seed := dashboarddb.BnetProfileRow{
		Toon: "Live", Gateway: 30, Found: true, AuroraID: 100,
		Payload: `{"aurora_id":100}`, FetchedAt: time.Now().UTC().Add(-10 * time.Minute),
	}
	if err := d.dbStore.UpsertBnetProfile(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := d.getOrFetchBnetProfile(ctx, "Live", 30, bnetfacade.PriorityUser, 0)
	if err != nil {
		t.Fatalf("default TTL call: %v", err)
	}
	if !res.Cached || calls != 0 {
		t.Fatalf("10min-old row should satisfy the default TTL: cached=%v calls=%d", res.Cached, calls)
	}

	res, err = d.getOrFetchBnetProfile(ctx, "Live", 30, bnetfacade.PriorityUser, 5*time.Minute)
	if err != nil {
		t.Fatalf("max_age call: %v", err)
	}
	if res.Cached || res.AuroraID != 101 || calls != 1 {
		t.Errorf("max_age=5m should refetch a 10min-old row: cached=%v aurora_id=%d calls=%d", res.Cached, res.AuroraID, calls)
	}

	res, err = d.getOrFetchBnetProfile(ctx, "Live", 30, bnetfacade.PriorityUser, time.Second)
	if err != nil {
		t.Fatalf("floored max_age call: %v", err)
	}
	if !res.Cached || calls != 1 {
		t.Errorf("max_age below the 1min floor must not force a refetch of a just-fetched row: cached=%v calls=%d", res.Cached, calls)
	}
}

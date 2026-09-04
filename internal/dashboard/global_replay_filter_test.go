package dashboard

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
)

// TestDefaultFilterMatchesTheLibraryDefault pins the two default definitions
// together: the dashboard's config defaults are what the settings screen
// renders, the library's are what an unconfigured corpus is filtered by, and a
// silent divergence would show the user one thing and count another.
func TestDefaultFilterMatchesTheLibraryDefault(t *testing.T) {
	config := defaultGlobalReplayFilterConfig()
	want := library.DefaultFilterConfig()

	if !reflect.DeepEqual(config.GameTypes, want.GameTypes) {
		t.Errorf("game types = %v, library default = %v", config.GameTypes, want.GameTypes)
	}
	if !reflect.DeepEqual(config.MapKinds, want.MapKinds) {
		t.Errorf("map kinds = %v, library default = %v", config.MapKinds, want.MapKinds)
	}
	if config.ExcludeShortGames != want.ExcludeShortGames || config.ExcludeComputers != want.ExcludeComputers {
		t.Errorf("exclusions = (%v, %v), library default = (%v, %v)",
			config.ExcludeShortGames, config.ExcludeComputers, want.ExcludeShortGames, want.ExcludeComputers)
	}
	if globalReplayFilterShortGameSeconds != library.ShortGameSeconds {
		t.Errorf("short game cutoff = %d, library = %d", globalReplayFilterShortGameSeconds, library.ShortGameSeconds)
	}
}

func TestDashboardAPI_GlobalReplayFilterGetAndUpdate(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/custom/global-replay-filter", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get config status %d: %s", rec.Code, rec.Body.String())
	}

	var initial globalReplayFilterConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &initial); err != nil {
		t.Fatalf("unmarshal get config: %v", err)
	}
	if !initial.ExcludeShortGames || !initial.ExcludeComputers {
		t.Fatalf("expected default booleans enabled, got %+v", initial)
	}

	updateBody := []byte(`{
		"game_types":["melee","free_for_all"],
		"exclude_short_games":false,
		"exclude_computers":false,
		"map_kinds":["money"]
	}`)
	rec = performDashboardRequest(router, http.MethodPut, "/api/custom/global-replay-filter", updateBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("update config status %d: %s", rec.Code, rec.Body.String())
	}

	var updated globalReplayFilterConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal updated config: %v", err)
	}
	if len(updated.GameTypes) != 2 || updated.GameTypes[0] != globalReplayFilterGameTypeFreeForAll || updated.GameTypes[1] != globalReplayFilterGameTypeMelee {
		t.Fatalf("expected melee + free_for_all game types, got %+v", updated)
	}
	if len(updated.MapKinds) != 1 || updated.MapKinds[0] != globalReplayFilterMapKindMoney {
		t.Fatalf("expected the money map kind, got %+v", updated)
	}
}

func TestDashboardAPI_GlobalReplayFilterAffectsWorkflowGames(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	total := func() int64 {
		t.Helper()
		rec := performDashboardRequest(router, http.MethodGet, "/api/games", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("workflow games status %d: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Total int64 `json:"total"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("workflow games json: %v", err)
		}
		return resp.Total
	}

	countWithMapKinds := func(kinds ...string) int64 {
		t.Helper()
		if _, err := dash.updateGlobalReplayFilterConfig(dash.ctx, globalReplayFilterConfig{
			ExcludeShortGames: false,
			ExcludeComputers:  false,
			MapKinds:          kinds,
		}); err != nil {
			t.Fatalf("updateGlobalReplayFilterConfig: %v", err)
		}
		return total()
	}

	both := countWithMapKinds(globalReplayFilterMapKindRegular, globalReplayFilterMapKindMoney)
	if both == 0 {
		t.Fatal("the corpus should have games under both map kinds")
	}
	regular := countWithMapKinds(globalReplayFilterMapKindRegular)
	money := countWithMapKinds(globalReplayFilterMapKindMoney)

	// The two kinds partition the corpus, so the filter provably reaches the
	// games list rather than the list ignoring it.
	if regular+money != both {
		t.Fatalf("regular (%d) plus money (%d) should account for every game (%d)", regular, money, both)
	}
	if regular == both || money == both {
		t.Fatalf("one map kind returned the whole corpus: regular %d, money %d, all %d", regular, money, both)
	}
}

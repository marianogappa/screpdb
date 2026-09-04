package dashboard

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestCompileGlobalReplayFilterSQLDefaults(t *testing.T) {
	config := defaultGlobalReplayFilterConfig()
	compiled, err := compileGlobalReplayFilterSQL(config)
	if err != nil {
		t.Fatalf("compileGlobalReplayFilterSQL: %v", err)
	}
	if !strings.Contains(compiled, "r.duration_seconds >= 120") {
		t.Fatalf("expected short game clause, got: %s", compiled)
	}
	if !strings.Contains(compiled, "computer controlled") {
		t.Fatalf("expected computer exclusion clause, got: %s", compiled)
	}
}

func TestCompileGlobalReplayFilterSQLWithOptions(t *testing.T) {
	config := globalReplayFilterConfig{
		GameTypes:         []string{globalReplayFilterGameTypeOneOnOne, globalReplayFilterGameTypeFreeForAll},
		ExcludeShortGames: false,
		ExcludeComputers:  false,
		MapKinds:          []string{globalReplayFilterMapKindMoney},
	}
	compiled, err := compileGlobalReplayFilterSQL(config)
	if err != nil {
		t.Fatalf("compileGlobalReplayFilterSQL: %v", err)
	}
	for _, fragment := range []string{
		"COUNT(DISTINCT p.team)",
		"lower(trim(coalesce(r.game_type, ''))) = 'free for all'",
		"r.map_kind = 'Money'",
		"r.map_kind != 'UseMapSettings'",
	} {
		if !strings.Contains(compiled, fragment) {
			t.Fatalf("expected fragment %q in compiled SQL: %s", fragment, compiled)
		}
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

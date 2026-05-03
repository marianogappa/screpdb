package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
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
	if updated.CompiledReplaysFilterSQL == nil || !strings.Contains(*updated.CompiledReplaysFilterSQL, "Money") {
		t.Fatalf("expected compiled SQL with map kind filter, got %+v", updated)
	}
}

func TestDashboardAPI_GlobalReplayFilterAffectsWorkflowGames(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	// Filter by map_kind = Regular and assert the workflow-games endpoint
	// returns the same total as the compiled global filter SQL.
	updated, err := dash.updateGlobalReplayFilterConfig(dash.ctx, globalReplayFilterConfig{
		GameTypes:         []string{},
		ExcludeShortGames: false,
		ExcludeComputers:  false,
		MapKinds:          []string{globalReplayFilterMapKindRegular},
	})
	if err != nil {
		t.Fatalf("updateGlobalReplayFilterConfig: %v", err)
	}
	if err := dash.refreshReplayScopedDB(); err != nil {
		t.Fatalf("refreshReplayScopedDB: %v", err)
	}

	var expected int64
	query := "SELECT COUNT(*) FROM (" + *updated.CompiledReplaysFilterSQL + ")"
	if err := dash.db.QueryRowContext(dash.ctx, query).Scan(&expected); err != nil {
		t.Fatalf("count compiled SQL: %v", err)
	}

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
	if resp.Total != expected {
		t.Fatalf("expected total %d, got %d", expected, resp.Total)
	}
}

func TestDashboardAPI_ReplayFilterAppliesToDetectedPatternViews(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()
	replayID, playerID := insertTestReplayPatternRows(t, dash)
	if _, err := dash.updateGlobalReplayFilterConfig(dash.ctx, globalReplayFilterConfig{
		GameTypes:         []string{},
		ExcludeShortGames: false,
		ExcludeComputers:  false,
		MapKinds:          []string{},
	}); err != nil {
		t.Fatalf("updateGlobalReplayFilterConfig: %v", err)
	}
	if err := dash.refreshReplayScopedDB(); err != nil {
		t.Fatalf("refreshReplayScopedDB: %v", err)
	}

	filterSQL := "SELECT * FROM replays WHERE id = " + int64ToString(replayID)
	if err := dash.updateDashboard(dash.ctx, "default", "Default Dashboard", ptrToString("The default dashboard"), &filterSQL); err != nil {
		t.Fatalf("updateDashboard: %v", err)
	}

	body := []byte(`{"query": "SELECT COUNT(*) AS c FROM replay_events WHERE event_kind = 'marker' AND event_type = 'test_replay_pattern' AND source_player_id IS NULL", "variable_values": {}, "dashboard_url": "default"}`)
	rec := performDashboardRequest(router, http.MethodPost, "/api/custom/query", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("execute query status %d: %s", rec.Code, rec.Body.String())
	}

	var replayResp struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &replayResp); err != nil {
		t.Fatalf("unmarshal replay query: %v", err)
	}
	if got := resultCountValue(t, replayResp.Results); got != 1 {
		t.Fatalf("expected replay pattern count 1, got %d", got)
	}

	playerBody := []byte(`{"query": "SELECT COUNT(*) AS c FROM replay_events WHERE event_kind = 'marker' AND source_player_id = ` + int64ToString(playerID) + ` AND event_type = 'test_player_pattern'", "variable_values": {}, "dashboard_url": "default"}`)
	rec = performDashboardRequest(router, http.MethodPost, "/api/custom/query", playerBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("execute player pattern query status %d: %s", rec.Code, rec.Body.String())
	}

	var playerResp struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &playerResp); err != nil {
		t.Fatalf("unmarshal player query: %v", err)
	}
	if got := resultCountValue(t, playerResp.Results); got != 1 {
		t.Fatalf("expected player pattern count 1, got %d", got)
	}
}

func insertTestReplayPatternRows(t *testing.T, dash *Dashboard) (int64, int64) {
	t.Helper()

	var replayID int64
	var playerID int64
	if err := dash.db.QueryRowContext(dash.ctx, `
		SELECT r.id, p.id
		FROM replays r
		JOIN players p ON p.replay_id = r.id
		WHERE p.is_observer = 0
		ORDER BY r.id ASC, p.id ASC
		LIMIT 1
	`).Scan(&replayID, &playerID); err != nil {
		t.Fatalf("query replay/player ids: %v", err)
	}

	// Post markers-migration: markers live in replay_events with event_kind='marker'.
	// Test fixtures inject directly into replay_events using the partial unique index
	// (replay_id, COALESCE(source_player_id,0), event_type) WHERE event_kind='marker'.
	if _, err := dash.db.ExecContext(dash.ctx, `
		INSERT INTO replay_events (replay_id, seconds_from_game_start, event_kind, event_type, source_player_id)
		VALUES (?, 0, 'marker', 'test_replay_pattern', NULL)
		ON CONFLICT (replay_id, COALESCE(source_player_id, 0), event_type) WHERE event_kind = 'marker'
		DO NOTHING
	`, replayID); err != nil {
		t.Fatalf("insert replay-level marker row: %v", err)
	}
	if _, err := dash.db.ExecContext(dash.ctx, `
		INSERT INTO replay_events (replay_id, seconds_from_game_start, event_kind, event_type, source_player_id)
		VALUES (?, 0, 'marker', 'test_player_pattern', ?)
		ON CONFLICT (replay_id, COALESCE(source_player_id, 0), event_type) WHERE event_kind = 'marker'
		DO NOTHING
	`, replayID, playerID); err != nil {
		t.Fatalf("insert player-level marker row: %v", err)
	}
	return replayID, playerID
}

func resultCountValue(t *testing.T, results []map[string]any) int64 {
	t.Helper()
	if len(results) != 1 {
		t.Fatalf("expected 1 result row, got %d", len(results))
	}
	value, ok := results[0]["c"]
	if !ok {
		t.Fatalf("missing count column")
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		t.Fatalf("unexpected count type %T", value)
		return 0
	}
}

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}

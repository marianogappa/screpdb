package dashboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		GameTypesMode:     globalReplayFilterModeOnlyThese,
		ExcludeShortGames: false,
		ExcludeComputers:  false,
		Maps:              []string{"Fighting Spirit"},
		MapFilterMode:     globalReplayFilterModeAllExceptThese,
		Players:           []string{"Soma"},
		PlayerFilterMode:  globalReplayFilterModeOnlyThese,
	}
	compiled, err := compileGlobalReplayFilterSQL(config)
	if err != nil {
		t.Fatalf("compileGlobalReplayFilterSQL: %v", err)
	}
	for _, fragment := range []string{
		"COUNT(DISTINCT p.team)",
		"lower(trim(coalesce(r.game_type, ''))) = 'free for all'",
		"NOT (lower(trim(coalesce(r.map_name, ''))) IN ('fighting spirit'))",
		"lower(trim(coalesce(p.name, ''))) IN ('soma')",
	} {
		if !strings.Contains(compiled, fragment) {
			t.Fatalf("expected fragment %q in compiled SQL: %s", fragment, compiled)
		}
	}
}

func TestDashboardAPI_GlobalReplayFilterGetAndUpdate(t *testing.T) {
	dash := newTestDashboard(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/custom/global-replay-filter", nil)
	dash.handlerGetGlobalReplayFilterConfig(rec, req)
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
		"game_types_mode":"only_these",
		"exclude_short_games":false,
		"exclude_computers":false,
		"maps":["fighting spirit"],
		"map_filter_mode":"all_except_these",
		"players":["soma"],
		"player_filter_mode":"only_these"
	}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/custom/global-replay-filter", bytes.NewReader(updateBody))
	dash.handlerUpdateGlobalReplayFilterConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update config status %d: %s", rec.Code, rec.Body.String())
	}

	var updated globalReplayFilterConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal updated config: %v", err)
	}
	if len(updated.GameTypes) != 2 || updated.GameTypes[0] != globalReplayFilterGameTypeFreeForAll || updated.GameTypes[1] != globalReplayFilterGameTypeMelee {
		t.Fatalf("expected melee game type, got %+v", updated)
	}
	if updated.CompiledReplaysFilterSQL == nil || !strings.Contains(*updated.CompiledReplaysFilterSQL, "fighting spirit") {
		t.Fatalf("expected compiled SQL with map filter, got %+v", updated)
	}
}

func TestDashboardAPI_GlobalReplayFilterAffectsWorkflowGames(t *testing.T) {
	dash := newTestDashboard(t)

	var mapName string
	if err := dash.db.QueryRowContext(dash.ctx, `
		SELECT lower(trim(map_name))
		FROM replays
		ORDER BY id ASC
		LIMIT 1
	`).Scan(&mapName); err != nil {
		t.Fatalf("query first map: %v", err)
	}

	updated, err := dash.updateGlobalReplayFilterConfig(dash.ctx, globalReplayFilterConfig{
		GameTypes:         []string{},
		GameTypesMode:     globalReplayFilterModeOnlyThese,
		ExcludeShortGames: false,
		ExcludeComputers:  false,
		Maps:              []string{mapName},
		MapFilterMode:     globalReplayFilterModeOnlyThese,
		Players:           []string{},
		PlayerFilterMode:  globalReplayFilterModeOnlyThese,
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

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/games", nil)
	dash.handlerGamesList(rec, req)
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
	replayID, playerID := insertTestReplayPatternRows(t, dash)
	if _, err := dash.updateGlobalReplayFilterConfig(dash.ctx, globalReplayFilterConfig{
		GameTypes:         []string{},
		GameTypesMode:     globalReplayFilterModeOnlyThese,
		ExcludeShortGames: false,
		ExcludeComputers:  false,
		Maps:              []string{},
		MapFilterMode:     globalReplayFilterModeOnlyThese,
		Players:           []string{},
		PlayerFilterMode:  globalReplayFilterModeOnlyThese,
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

	body := []byte(`{"query": "SELECT COUNT(*) AS c FROM detected_patterns_replay WHERE pattern_name = 'test_replay_pattern'", "variable_values": {}, "dashboard_url": "default"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/custom/query", bytes.NewReader(body))
	dash.handlerExecuteQuery(rec, req)
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

	playerBody := []byte(`{"query": "SELECT COUNT(*) AS c FROM detected_patterns_replay_player WHERE player_id = ` + int64ToString(playerID) + ` AND pattern_name = 'test_player_pattern'", "variable_values": {}, "dashboard_url": "default"}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/custom/query", bytes.NewReader(playerBody))
	dash.handlerExecuteQuery(rec, req)
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

func TestDashboardAPI_GlobalReplayFilterComposesWithDashboardFilter(t *testing.T) {
	dash := newTestDashboard(t)

	var replayOneID int64
	var replayOneMap string
	if err := dash.db.QueryRowContext(dash.ctx, `
		SELECT id, lower(trim(map_name))
		FROM replays
		ORDER BY id ASC
		LIMIT 1
	`).Scan(&replayOneID, &replayOneMap); err != nil {
		t.Fatalf("query first replay: %v", err)
	}

	var replayTwoID int64
	if err := dash.db.QueryRowContext(dash.ctx, `
		SELECT id
		FROM replays
		WHERE id <> ?
		ORDER BY id ASC
		LIMIT 1
	`, replayOneID).Scan(&replayTwoID); err != nil {
		t.Fatalf("query second replay: %v", err)
	}

	if _, err := dash.updateGlobalReplayFilterConfig(dash.ctx, globalReplayFilterConfig{
		GameTypes:         []string{},
		GameTypesMode:     globalReplayFilterModeOnlyThese,
		ExcludeShortGames: false,
		ExcludeComputers:  false,
		Maps:              []string{replayOneMap},
		MapFilterMode:     globalReplayFilterModeOnlyThese,
		Players:           []string{},
		PlayerFilterMode:  globalReplayFilterModeOnlyThese,
	}); err != nil {
		t.Fatalf("updateGlobalReplayFilterConfig: %v", err)
	}
	if err := dash.refreshReplayScopedDB(); err != nil {
		t.Fatalf("refreshReplayScopedDB: %v", err)
	}

	filterSQL := "SELECT * FROM replays WHERE id = " + int64ToString(replayTwoID)
	if err := dash.updateDashboard(dash.ctx, "default", "Default Dashboard", ptrToString("The default dashboard"), &filterSQL); err != nil {
		t.Fatalf("updateDashboard: %v", err)
	}

	body := []byte(`{"query": "SELECT COUNT(*) AS c FROM replays", "variable_values": {}, "dashboard_url": "default"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/custom/query", bytes.NewReader(body))
	dash.handlerExecuteQuery(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("execute query status %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal query: %v", err)
	}
	if got := resultCountValue(t, resp.Results); got != 0 {
		t.Fatalf("expected composed filter to return 0 rows, got %d", got)
	}
}

func TestCompileGlobalReplayFilterSQLPlayerModes(t *testing.T) {
	allowCompiled, err := compileGlobalReplayFilterSQL(globalReplayFilterConfig{
		GameTypes:         []string{},
		GameTypesMode:     globalReplayFilterModeOnlyThese,
		ExcludeShortGames: false,
		ExcludeComputers:  false,
		Maps:              []string{},
		MapFilterMode:     globalReplayFilterModeOnlyThese,
		Players:           []string{"soma"},
		PlayerFilterMode:  globalReplayFilterModeOnlyThese,
	})
	if err != nil {
		t.Fatalf("compile allowlist: %v", err)
	}
	if !strings.Contains(allowCompiled, "EXISTS (") {
		t.Fatalf("expected allowlist EXISTS predicate, got: %s", allowCompiled)
	}

	blockCompiled, err := compileGlobalReplayFilterSQL(globalReplayFilterConfig{
		GameTypes:         []string{},
		GameTypesMode:     globalReplayFilterModeOnlyThese,
		ExcludeShortGames: false,
		ExcludeComputers:  false,
		Maps:              []string{},
		MapFilterMode:     globalReplayFilterModeOnlyThese,
		Players:           []string{"soma"},
		PlayerFilterMode:  globalReplayFilterModeAllExceptThese,
	})
	if err != nil {
		t.Fatalf("compile blocklist: %v", err)
	}
	if !strings.Contains(blockCompiled, "NOT (EXISTS (") {
		t.Fatalf("expected blocklist NOT EXISTS predicate, got: %s", blockCompiled)
	}
}

func TestDashboardAPI_GlobalReplayFilterPlayerOnlyTheseMatchesByPresence(t *testing.T) {
	dash := newTestDashboard(t)
	replayID, allowedPlayer, _, err := replayPlayersForPresenceTest(dash)
	if err != nil {
		t.Fatalf("replayPlayersForPresenceTest: %v", err)
	}

	updated, err := dash.updateGlobalReplayFilterConfig(dash.ctx, globalReplayFilterConfig{
		GameTypes:         []string{},
		GameTypesMode:     globalReplayFilterModeOnlyThese,
		ExcludeShortGames: false,
		ExcludeComputers:  false,
		Maps:              []string{},
		MapFilterMode:     globalReplayFilterModeOnlyThese,
		Players:           []string{allowedPlayer},
		PlayerFilterMode:  globalReplayFilterModeOnlyThese,
	})
	if err != nil {
		t.Fatalf("updateGlobalReplayFilterConfig: %v", err)
	}
	if err := dash.refreshReplayScopedDB(); err != nil {
		t.Fatalf("refreshReplayScopedDB: %v", err)
	}

	var count int64
	query := "SELECT COUNT(*) FROM (" + *updated.CompiledReplaysFilterSQL + ") WHERE id = " + int64ToString(replayID)
	if err := dash.db.QueryRowContext(dash.ctx, query).Scan(&count); err != nil {
		t.Fatalf("count allowlisted replay: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected replay %d to remain visible for allowlisted player presence, got %d", replayID, count)
	}
}

func TestDashboardAPI_GlobalReplayFilterPlayerAllExceptExcludesAnyMatchingReplay(t *testing.T) {
	dash := newTestDashboard(t)
	replayID, blockedPlayer, _, err := replayPlayersForPresenceTest(dash)
	if err != nil {
		t.Fatalf("replayPlayersForPresenceTest: %v", err)
	}

	updated, err := dash.updateGlobalReplayFilterConfig(dash.ctx, globalReplayFilterConfig{
		GameTypes:         []string{},
		GameTypesMode:     globalReplayFilterModeOnlyThese,
		ExcludeShortGames: false,
		ExcludeComputers:  false,
		Maps:              []string{},
		MapFilterMode:     globalReplayFilterModeOnlyThese,
		Players:           []string{blockedPlayer},
		PlayerFilterMode:  globalReplayFilterModeAllExceptThese,
	})
	if err != nil {
		t.Fatalf("updateGlobalReplayFilterConfig: %v", err)
	}
	if err := dash.refreshReplayScopedDB(); err != nil {
		t.Fatalf("refreshReplayScopedDB: %v", err)
	}

	var count int64
	query := "SELECT COUNT(*) FROM (" + *updated.CompiledReplaysFilterSQL + ") WHERE id = " + int64ToString(replayID)
	if err := dash.db.QueryRowContext(dash.ctx, query).Scan(&count); err != nil {
		t.Fatalf("count blocked replay: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected replay %d to be excluded for blocklisted player, got %d", replayID, count)
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

	if _, err := dash.db.ExecContext(dash.ctx, `
		INSERT OR REPLACE INTO detected_patterns_replay (replay_id, algorithm_version, pattern_name, value_bool)
		VALUES (?, 1, 'test_replay_pattern', 1)
	`, replayID); err != nil {
		t.Fatalf("insert detected_patterns_replay: %v", err)
	}
	if _, err := dash.db.ExecContext(dash.ctx, `
		INSERT OR REPLACE INTO detected_patterns_replay_player (replay_id, player_id, pattern_name, value_bool)
		VALUES (?, ?, 'test_player_pattern', 1)
	`, replayID, playerID); err != nil {
		t.Fatalf("insert detected_patterns_replay_player: %v", err)
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

func replayPlayersForPresenceTest(dash *Dashboard) (int64, string, string, error) {
	var replayID int64
	var playerOne string
	var playerTwo string
	err := dash.db.QueryRowContext(dash.ctx, `
		SELECT replay_id, MIN(player_name), MAX(player_name)
		FROM (
			SELECT p.replay_id AS replay_id, lower(trim(p.name)) AS player_name
			FROM players p
			WHERE p.is_observer = 0
				AND lower(trim(coalesce(p.type, ''))) = 'human'
		)
		GROUP BY replay_id
		HAVING COUNT(*) >= 2 AND MIN(player_name) <> MAX(player_name)
		ORDER BY replay_id ASC
		LIMIT 1
	`).Scan(&replayID, &playerOne, &playerTwo)
	return replayID, playerOne, playerTwo, err
}

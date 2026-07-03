package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func firstPlayerKey(t *testing.T, dash *Dashboard) string {
	t.Helper()
	var name string
	if err := dash.dbStore.DefaultQueryRow(`SELECT name FROM players WHERE is_observer = 0 LIMIT 1`).Scan(&name); err != nil {
		t.Skip("no players in test DB")
	}
	return normalizePlayerKey(name)
}

func TestScrepColorsEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/screp-colors", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var colors map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &colors); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(colors) == 0 {
		t.Fatal("expected at least one screp color")
	}
	for key, hex := range colors {
		if !strings.HasPrefix(hex, "#") || len(hex) != 7 {
			t.Fatalf("color %q = %q, want #rrggbb", key, hex)
		}
		if key != strings.ToLower(key) || strings.Contains(key, " ") {
			t.Fatalf("color key %q should be lowercased and space-stripped", key)
		}
	}
}

func TestPlayerColorsEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/player-colors", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		PlayerColors map[string]string `json:"player_colors"`
		Palette      []string          `json:"palette"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Palette) == 0 {
		t.Fatal("expected a non-empty palette")
	}
	if len(resp.PlayerColors) > len(resp.Palette) {
		t.Fatalf("player_colors (%d) must not exceed palette size (%d)", len(resp.PlayerColors), len(resp.Palette))
	}
}

func TestHealthcheckEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OK           bool  `json:"ok"`
		TotalReplays int64 `json:"total_replays"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected ok true")
	}
	if resp.TotalReplays < 0 {
		t.Fatalf("negative total_replays: %d", resp.TotalReplays)
	}
}

func TestPlayerInsightEndpoint_TypesAndErrors(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()
	key := firstPlayerKey(t, dash)

	for _, insightType := range []string{"apm", "unit-production-cadence", "viewport-switch-rate"} {
		rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/insight?type="+insightType, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("type %q status %d: %s", insightType, rec.Code, rec.Body.String())
		}
		var resp struct {
			PlayerKey   string `json:"player_key"`
			InsightType string `json:"insight_type"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("type %q unmarshal: %v", insightType, err)
		}
		if resp.PlayerKey != key {
			t.Fatalf("type %q expected player_key %q, got %q", insightType, key, resp.PlayerKey)
		}
		if resp.InsightType != insightType {
			t.Fatalf("expected insight_type %q, got %q", insightType, resp.InsightType)
		}
	}

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/insight?type=nonsense", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsupported insight type expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/players/__nobody__/insight?type=apm", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown player insight expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPlayerApmHistogramEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()
	key := firstPlayerKey(t, dash)

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/insights/apm-histogram", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("invalid JSON: %s", rec.Body.String())
	}
}

func TestPlayerUnitCadenceEndpoint_FilterModes(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()
	key := firstPlayerKey(t, dash)

	for _, filter := range []string{"", "strict", "broad", "BROAD"} {
		path := "/api/players/" + key + "/insights/unit-production-cadence"
		if filter != "" {
			path += "?filter=" + filter
		}
		rec := performDashboardRequest(router, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("filter %q status %d: %s", filter, rec.Code, rec.Body.String())
		}
		if !json.Valid(rec.Body.Bytes()) {
			t.Fatalf("filter %q invalid JSON: %s", filter, rec.Body.String())
		}
	}

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/insights/unit-production-cadence?filter=garbage", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid filter expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPlayersUnitCadenceEndpoint_Params(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/insights/unit-production-cadence?filter=broad&min_games=1&limit=5", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("invalid JSON: %s", rec.Body.String())
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/players/insights/unit-production-cadence?limit=-1", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("negative limit expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/players/insights/unit-production-cadence?filter=garbage", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid filter expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPlayerOutliersEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()
	key := firstPlayerKey(t, dash)

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/outliers", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("invalid JSON: %s", rec.Body.String())
	}

	// Unknown player returns 404, consistent with PlayerDetail/PlayerRecentGames:
	// GetOutlierPlayerSummary now COALESCEs the NULL-name aggregate to an empty
	// string, so the builder yields sql.ErrNoRows instead of a scan error.
	rec = performDashboardRequest(router, http.MethodGet, "/api/players/__nobody__/outliers", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown player outliers expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPlayerSummaryOutliersEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()
	key := firstPlayerKey(t, dash)

	for _, category := range []string{"Order", "Build", "Train", "Morph", "Tech", "Upgrade"} {
		rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/summary/outliers?category="+category, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("category %q status %d: %s", category, rec.Code, rec.Body.String())
		}
		if !json.Valid(rec.Body.Bytes()) {
			t.Fatalf("category %q invalid JSON: %s", category, rec.Body.String())
		}
	}

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/summary/outliers?category=BogusCategory", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown category expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/summary/outliers", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing category expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPlayerRecentGamesEndpoint_ShapeAndUnknown(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()
	key := firstPlayerKey(t, dash)

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/recent-games", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		PlayerKey      string            `json:"player_key"`
		SummaryVersion string            `json:"summary_version"`
		RecentGames    []json.RawMessage `json:"recent_games"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.PlayerKey != key {
		t.Fatalf("expected player_key %q, got %q", key, resp.PlayerKey)
	}
	if resp.SummaryVersion == "" {
		t.Fatal("expected summary_version")
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/players/__nobody__/recent-games", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown player expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGamesListEndpoint_PaginationOffsetAndFilters(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	page1 := performDashboardRequest(router, http.MethodGet, "/api/games?limit=1&offset=0", nil)
	var r1 gamesListResponse
	if err := json.Unmarshal(page1.Body.Bytes(), &r1); err != nil {
		t.Fatalf("unmarshal page1: %v", err)
	}
	if r1.Total < 2 {
		t.Skip("need at least 2 games to exercise offset")
	}
	if r1.Offset != 0 || r1.Limit != 1 {
		t.Fatalf("page1 limit/offset = %d/%d", r1.Limit, r1.Offset)
	}

	page2 := performDashboardRequest(router, http.MethodGet, "/api/games?limit=1&offset=1", nil)
	var r2 gamesListResponse
	if err := json.Unmarshal(page2.Body.Bytes(), &r2); err != nil {
		t.Fatalf("unmarshal page2: %v", err)
	}
	if r2.Offset != 1 {
		t.Fatalf("page2 offset = %d, want 1", r2.Offset)
	}
	if len(r1.Items) == 1 && len(r2.Items) == 1 && r1.Items[0].ReplayID == r2.Items[0].ReplayID {
		t.Fatal("offset pagination returned the same replay on both pages")
	}

	over := performDashboardRequest(router, http.MethodGet, "/api/games?limit=99999", nil)
	var rOver gamesListResponse
	if err := json.Unmarshal(over.Body.Bytes(), &rOver); err != nil {
		t.Fatalf("unmarshal over: %v", err)
	}
	if rOver.Limit != 200 {
		t.Fatalf("limit should cap at 200, got %d", rOver.Limit)
	}
}

func TestMarkersDefinitionsEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/custom/markers/definitions", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AlgorithmVersion int                        `json:"algorithm_version"`
		Markers          map[string]json.RawMessage `json:"markers"`
		FeaturingOrder   []string                   `json:"featuring_order"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.AlgorithmVersion == 0 {
		t.Fatal("expected non-zero algorithm_version")
	}
	if len(resp.Markers) == 0 {
		t.Fatal("expected at least one marker definition")
	}
	if len(resp.FeaturingOrder) == 0 {
		t.Fatal("expected non-empty featuring_order")
	}
}

func TestDebugMapLayoutEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	var replayID int64
	if err := dash.dbStore.DefaultQueryRow(`SELECT id FROM replays WHERE trim(coalesce(file_path,'')) != '' ORDER BY id LIMIT 1`).Scan(&replayID); err != nil {
		t.Skip("no replay with file_path in test DB")
	}

	rec := performDashboardRequest(router, http.MethodGet, "/api/custom/debug/map-layout/"+strconv.FormatInt(replayID, 10), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ReplayID int64 `json:"replay_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ReplayID != replayID {
		t.Fatalf("expected replay_id %d, got %d", replayID, resp.ReplayID)
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/custom/debug/map-layout/abc", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-numeric replayID expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/custom/debug/map-layout/999999999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown replayID expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateStatusEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/custom/update/status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("invalid JSON: %s", rec.Body.String())
	}
}

func TestStaleReplaysCountEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/custom/replays/stale-count", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Count          int64 `json:"count"`
		CurrentVersion int   `json:"current_version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.CurrentVersion == 0 {
		t.Fatal("expected non-zero current_version")
	}
	if resp.Count < 0 {
		t.Fatalf("negative stale count: %d", resp.Count)
	}
}

func TestGlobalReplayFilterEndpoints(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	body := []byte(`{"game_types":["melee"],"exclude_short_games":true,"exclude_computers":true,"map_kinds":["regular"]}`)
	rec := performDashboardRequest(router, http.MethodPut, "/api/custom/global-replay-filter", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status %d: %s", rec.Code, rec.Body.String())
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/custom/global-replay-filter", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		GameTypes         []string `json:"game_types"`
		ExcludeShortGames bool     `json:"exclude_short_games"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !containsString(resp.GameTypes, "melee") {
		t.Fatalf("expected melee in stored game_types, got %v", resp.GameTypes)
	}
	if !resp.ExcludeShortGames {
		t.Fatal("expected exclude_short_games true to round-trip")
	}
}

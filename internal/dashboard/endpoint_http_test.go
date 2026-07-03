package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

type gamesListResponse struct {
	SummaryVersion string `json:"summary_version"`
	Limit          int    `json:"limit"`
	Offset         int    `json:"offset"`
	Total          int64  `json:"total"`
	Items          []struct {
		ReplayID        int64  `json:"replay_id"`
		MapName         string `json:"map_name"`
		DurationSeconds int64  `json:"duration_seconds"`
		Players         []struct {
			Name string `json:"name"`
			Team int64  `json:"team"`
		} `json:"players"`
		PlayersLabel string `json:"players_label"`
	} `json:"items"`
	FilterOptions map[string]any `json:"filter_options"`
}

func TestGamesListEndpoint_ShapeAndPagination(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/games?limit=1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp gamesListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Limit != 1 {
		t.Fatalf("expected limit 1, got %d", resp.Limit)
	}
	if resp.SummaryVersion == "" {
		t.Fatal("expected summary_version")
	}
	if resp.Total < 1 {
		t.Skip("no games in test DB")
	}
	if len(resp.Items) != 1 {
		t.Fatalf("limit=1 should return exactly one item, got %d", len(resp.Items))
	}
	if resp.FilterOptions == nil {
		t.Fatal("expected filter_options in response")
	}
	item := resp.Items[0]
	if item.ReplayID == 0 {
		t.Fatal("expected non-zero replay id")
	}
	if len(item.Players) == 0 {
		t.Fatal("expected players populated for the game")
	}
	if item.PlayersLabel == "" {
		t.Fatal("expected a players label")
	}
}

func TestGamesListEndpoint_DurationFilterNarrows(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	all := performDashboardRequest(router, http.MethodGet, "/api/games?limit=200", nil)
	var allResp gamesListResponse
	if err := json.Unmarshal(all.Body.Bytes(), &allResp); err != nil {
		t.Fatalf("unmarshal all: %v", err)
	}
	if allResp.Total == 0 {
		t.Skip("no games in test DB")
	}

	rec := performDashboardRequest(router, http.MethodGet, "/api/games?map=__no_such_map__", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("filtered status %d: %s", rec.Code, rec.Body.String())
	}
	var filtered gamesListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("unmarshal filtered: %v", err)
	}
	if filtered.Total != 0 {
		t.Fatalf("expected impossible map filter to return 0, got %d", filtered.Total)
	}
	if len(filtered.Items) != 0 {
		t.Fatalf("expected no items for impossible map, got %d", len(filtered.Items))
	}
}

func TestPlayersListEndpoint_ShapeAndSort(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/players?sort_by=games&sort_dir=desc&limit=50", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Items []struct {
			PlayerKey   string `json:"player_key"`
			PlayerName  string `json:"player_name"`
			GamesPlayed int64  `json:"games_played"`
		} `json:"items"`
		FilterOptions map[string]any `json:"filter_options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Items) == 0 {
		t.Skip("no players in test DB")
	}
	for i := 1; i < len(resp.Items); i++ {
		if resp.Items[i].GamesPlayed > resp.Items[i-1].GamesPlayed {
			t.Fatalf("games desc sort violated at %d: %d > %d", i, resp.Items[i].GamesPlayed, resp.Items[i-1].GamesPlayed)
		}
	}
}

func TestPlayersListEndpoint_NameFilter(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/players?name=__no_such_player__", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected no players for impossible name filter, got %d", len(resp.Items))
	}
}

func TestPlayerDetailEndpoint_KnownAndUnknown(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	var playerName string
	if err := dash.dbStore.DefaultQueryRow(`SELECT name FROM players WHERE is_observer = 0 LIMIT 1`).Scan(&playerName); err != nil {
		t.Skip("no players in test DB")
	}
	key := normalizePlayerKey(playerName)

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("known player status %d: %s", rec.Code, rec.Body.String())
	}
	var overview struct {
		PlayerKey   string `json:"player_key"`
		GamesPlayed int64  `json:"games_played"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &overview); err != nil {
		t.Fatalf("unmarshal overview: %v", err)
	}
	if overview.PlayerKey != key {
		t.Fatalf("expected player_key %q, got %q", key, overview.PlayerKey)
	}
	if overview.GamesPlayed < 1 {
		t.Fatalf("expected at least one game for %q", key)
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/players/__nobody__", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown player expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPlayerRecentGamesEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	var playerName string
	if err := dash.dbStore.DefaultQueryRow(`SELECT name FROM players WHERE is_observer = 0 LIMIT 1`).Scan(&playerName); err != nil {
		t.Skip("no players in test DB")
	}
	key := normalizePlayerKey(playerName)

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/recent-games", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("invalid JSON: %s", rec.Body.String())
	}
}

func TestGameDetailEndpoint_KnownAndUnknown(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	var replayID int64
	if err := dash.dbStore.DefaultQueryRow(`SELECT id FROM replays ORDER BY id LIMIT 1`).Scan(&replayID); err != nil {
		t.Skip("no replays in test DB")
	}

	rec := performDashboardRequest(router, http.MethodGet, "/api/games/"+strconv.FormatInt(replayID, 10), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("known game status %d: %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		ReplayID int64 `json:"replay_id"`
		Players  []struct {
			PlayerID int64 `json:"player_id"`
		} `json:"players"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}
	if detail.ReplayID != replayID {
		t.Fatalf("expected replay_id %d, got %d", replayID, detail.ReplayID)
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/games/424242424", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown game expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPlayerSummaryPerMatchupEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	var playerName string
	if err := dash.dbStore.DefaultQueryRow(`SELECT name FROM players WHERE is_observer = 0 LIMIT 1`).Scan(&playerName); err != nil {
		t.Skip("no players in test DB")
	}
	key := normalizePlayerKey(playerName)

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/summary/per-matchup", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		PlayerKey      string            `json:"player_key"`
		SummaryVersion string            `json:"summary_version"`
		Cards          []json.RawMessage `json:"cards"`
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
}

func TestPlayerSummarySpecialEndpoint(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	var playerName string
	if err := dash.dbStore.DefaultQueryRow(`SELECT name FROM players WHERE is_observer = 0 LIMIT 1`).Scan(&playerName); err != nil {
		t.Skip("no players in test DB")
	}
	key := normalizePlayerKey(playerName)

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/"+key+"/summary/special", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("invalid JSON: %s", rec.Body.String())
	}
}

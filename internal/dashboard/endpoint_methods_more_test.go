package dashboard

import (
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestParseWorkflowGamesListFilters(t *testing.T) {
	req := httptest.NewRequest("GET", "/?player=Bisu,Flash&map=Fighting+Spirit&duration=short&featuring=cheese&matchup=pvz&map_kind=regular", nil)
	got := parseWorkflowGamesListFilters(req)
	if !reflect.DeepEqual(got.PlayerKeys, []string{"bisu", "flash"}) {
		t.Errorf("PlayerKeys = %v", got.PlayerKeys)
	}
	if !reflect.DeepEqual(got.MapNames, []string{"Fighting Spirit"}) {
		t.Errorf("MapNames should preserve case = %v", got.MapNames)
	}
	if !reflect.DeepEqual(got.DurationBuckets, []string{"short"}) {
		t.Errorf("DurationBuckets = %v", got.DurationBuckets)
	}
	if !reflect.DeepEqual(got.MatchupKeys, []string{"pvz"}) {
		t.Errorf("MatchupKeys = %v", got.MatchupKeys)
	}
	if !reflect.DeepEqual(got.MapKindKeys, []string{"regular"}) {
		t.Errorf("MapKindKeys = %v", got.MapKindKeys)
	}

	empty := parseWorkflowGamesListFilters(httptest.NewRequest("GET", "/", nil))
	if len(empty.PlayerKeys) != 0 || len(empty.MapNames) != 0 {
		t.Errorf("empty request should yield empty filters, got %+v", empty)
	}
}

func TestExtractApmValues(t *testing.T) {
	points := []workflowPlayerApmHistogramPoint{
		{AverageAPM: 300},
		{AverageAPM: 100},
		{AverageAPM: 200},
	}
	got := extractApmValues(points)
	want := []float64{100, 200, 300}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractApmValues = %v, want sorted %v", got, want)
	}
	if got := extractApmValues(nil); len(got) != 0 {
		t.Fatalf("nil input should yield empty, got %v", got)
	}
}

func TestExtractCadenceValues(t *testing.T) {
	points := []workflowPlayerUnitCadencePoint{
		{AverageCadence: 5},
		{AverageCadence: 1},
		{AverageCadence: 3},
	}
	got := extractCadenceValues(points)
	want := []float64{1, 3, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractCadenceValues = %v, want sorted %v", got, want)
	}
}

func TestHasValidCenterBaseKind(t *testing.T) {
	for _, kind := range []string{"start", "starting", "natural", "expansion", "expa"} {
		if !hasValidCenterBaseKind(kind) {
			t.Errorf("expected %q to be a valid center base kind", kind)
		}
	}
	for _, kind := range []string{"", "mineral only", "unknown"} {
		if hasValidCenterBaseKind(kind) {
			t.Errorf("expected %q to be invalid", kind)
		}
	}
}

func TestDashboardPlayersByReplay(t *testing.T) {
	dash := newTestDashboard(t)
	byReplay, err := dash.playersByReplay()
	if err != nil {
		t.Fatalf("playersByReplay: %v", err)
	}
	if len(byReplay) == 0 {
		t.Skip("no replays in test DB")
	}
	for replayID, players := range byReplay {
		if replayID <= 0 {
			t.Fatalf("non-positive replay id %d", replayID)
		}
		for _, p := range players {
			if p.PlayerKey != normalizePlayerKey(p.Name) {
				t.Fatalf("player key %q not normalized from name %q", p.PlayerKey, p.Name)
			}
		}
	}
}

func TestDashboardTotalDistinctPlayers(t *testing.T) {
	dash := newTestDashboard(t)
	total, err := dash.totalDistinctPlayers()
	if err != nil {
		t.Fatalf("totalDistinctPlayers: %v", err)
	}
	if total <= 0 {
		t.Skip("no players in test DB")
	}

	byRace, err := dash.totalDistinctPlayersByRace("Protoss")
	if err != nil {
		t.Fatalf("totalDistinctPlayersByRace: %v", err)
	}
	if byRace < 0 || byRace > total {
		t.Fatalf("per-race distinct players (%v) out of range vs total (%v)", byRace, total)
	}
}

func TestDashboardPlayerGamesByRace(t *testing.T) {
	dash := newTestDashboard(t)
	key := firstPlayerKey(t, dash)
	byRace, err := dash.playerGamesByRace(key)
	if err != nil {
		t.Fatalf("playerGamesByRace: %v", err)
	}
	if len(byRace) == 0 {
		t.Fatalf("expected at least one race for player %q", key)
	}
	var totalGames int64
	for race, games := range byRace {
		if games <= 0 {
			t.Fatalf("race %q has non-positive game count %d", race, games)
		}
		totalGames += games
	}
	if totalGames <= 0 {
		t.Fatal("expected positive total games across races")
	}
}

func TestDashboardFirstExpansionAverageByPlayer(t *testing.T) {
	dash := newTestDashboard(t)
	averages, err := dash.firstExpansionAverageByPlayer()
	if err != nil {
		t.Fatalf("firstExpansionAverageByPlayer: %v", err)
	}
	for key, avg := range averages {
		if key == "" {
			t.Fatal("empty player key in averages")
		}
		if avg < 0 {
			t.Fatalf("negative expansion second average %v for %q", avg, key)
		}
	}
}

func TestDashboardCountCarrierGamesForPlayer(t *testing.T) {
	dash := newTestDashboard(t)
	key := firstPlayerKey(t, dash)
	count, err := dash.countCarrierGamesForPlayer(key)
	if err != nil {
		t.Fatalf("countCarrierGamesForPlayer: %v", err)
	}
	if count < 0 {
		t.Fatalf("negative carrier game count %d", count)
	}
}

func TestDashboardTopActionTypesForPlayer(t *testing.T) {
	dash := newTestDashboard(t)
	var playerID int64
	if err := dash.dbStore.DefaultQueryRow(`SELECT id FROM players WHERE is_observer = 0 LIMIT 1`).Scan(&playerID); err != nil {
		t.Skip("no players in test DB")
	}
	actions, err := dash.topActionTypesForPlayer(playerID, 5)
	if err != nil {
		t.Fatalf("topActionTypesForPlayer: %v", err)
	}
	if len(actions) > 5 {
		t.Fatalf("expected at most 5 action types, got %d", len(actions))
	}
	for _, a := range actions {
		if a == "" {
			t.Fatal("empty action type returned")
		}
	}
}

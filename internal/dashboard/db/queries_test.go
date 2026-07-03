package db

import (
	"context"
	"database/sql"
	"testing"
)

// fixtureBasic1v1 seeds a single 1v1 replay: BoxeR (Terran, winner) vs
// NaDa (Protoss, loser), plus one observer. Returns replay id and player ids.
func fixtureBasic1v1(t *testing.T, conn *sql.DB) (replayID, boxerID, nadaID int64) {
	t.Helper()
	replayID = seedReplay(t, conn, replayFixture{
		filePath: "/r/g1.rep", checksum: "chk1", fileName: "g1.rep",
		replayDate: "2024-06-01T10:00:00Z", mapName: "Fighting Spirit",
		durationSeconds: 900, gameType: "Melee", mapKind: "Regular",
		teamFormat: "1v1", matchup: "PvT",
	})
	boxerID = seedPlayer(t, conn, playerFixture{
		replayID: replayID, name: "BoxeR", race: "Terran", team: 1,
		apm: 300, eapm: 200, isWinner: true, slotID: 0,
	})
	nadaID = seedPlayer(t, conn, playerFixture{
		replayID: replayID, name: "NaDa", race: "Protoss", team: 2,
		apm: 250, eapm: 180, isWinner: false, slotID: 1,
	})
	seedPlayer(t, conn, playerFixture{
		replayID: replayID, name: "Obs", race: "Terran", team: 3,
		isObserver: true, slotID: 2,
	})
	return replayID, boxerID, nadaID
}

func TestGetReplaySummary(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()

	replayID := seedReplay(t, conn, replayFixture{
		filePath: "/r/summary.rep", checksum: "sumchk", fileName: "summary.rep",
		replayDate: "2024-06-01T10:00:00Z", mapName: "Python",
		durationSeconds: 1200, gameType: "Melee", mapKind: "Money",
		teamFormat: "1v1", matchup: "TvZ", teamStacking: true,
	})

	got, err := s.GetReplaySummary(ctx, replayID)
	if err != nil {
		t.Fatalf("GetReplaySummary: %v", err)
	}
	if got.MapName != "Python" || got.MapKind != "Money" || got.DurationSeconds != 1200 {
		t.Errorf("unexpected summary: %+v", got)
	}
	if !got.TeamStacking {
		t.Errorf("expected TeamStacking true")
	}
	if got.FileChecksum != "sumchk" {
		t.Errorf("checksum = %q", got.FileChecksum)
	}
}

func TestListReplayPlayersForDetailExcludesObservers(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, _, _ := fixtureBasic1v1(t, conn)

	rows, err := s.ListReplayPlayersForDetail(ctx, replayID)
	if err != nil {
		t.Fatalf("ListReplayPlayersForDetail: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 non-observer players, got %d", len(rows))
	}
	// Ordered by team ASC: BoxeR (team 1) first.
	if rows[0].Name != "BoxeR" || !rows[0].IsWinner || rows[0].APM != 300 {
		t.Errorf("row0 = %+v", rows[0])
	}
	if rows[1].Name != "NaDa" || rows[1].IsWinner {
		t.Errorf("row1 = %+v", rows[1])
	}
}

func TestListReplayPlayersForAllianceIncludesObservers(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, _, _ := fixtureBasic1v1(t, conn)

	rows, err := s.ListReplayPlayersForAlliance(ctx, replayID)
	if err != nil {
		t.Fatalf("ListReplayPlayersForAlliance: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 players (incl observer), got %d", len(rows))
	}
}

func TestGetPlayerOverviewSummary(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	fixtureBasic1v1(t, conn)

	got, err := s.GetPlayerOverviewSummary(ctx, "boxer")
	if err != nil {
		t.Fatalf("GetPlayerOverviewSummary: %v", err)
	}
	if got.PlayerName != "BoxeR" || got.GamesPlayed != 1 || got.Wins != 1 {
		t.Errorf("summary = %+v", got)
	}
	if got.AverageAPM != 300 {
		t.Errorf("avg apm = %v, want 300", got.AverageAPM)
	}
}

func TestListPlayerRecentGamesLabels(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	fixtureBasic1v1(t, conn)

	rows, err := s.ListPlayerRecentGames(ctx, "boxer")
	if err != nil {
		t.Fatalf("ListPlayerRecentGames: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 recent game, got %d", len(rows))
	}
	g := rows[0]
	if g.Matchup != "PvT" || g.MapName != "Fighting Spirit" {
		t.Errorf("game = %+v", g)
	}
	// players_label lists humans by team; winners_label only winners.
	if g.PlayersLabel != "BoxeR, NaDa" {
		t.Errorf("players label = %q", g.PlayersLabel)
	}
	if g.WinnersLabel != "BoxeR" {
		t.Errorf("winners label = %q", g.WinnersLabel)
	}
}

func TestListPlayerApmAggregatesMinGames(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()

	// Player with 2 games, opponent with 1.
	for i := 0; i < 2; i++ {
		rid := seedReplay(t, conn, replayFixture{
			filePath: "/r/apm" + string(rune('a'+i)) + ".rep",
			checksum: "apmchk" + string(rune('a'+i)),
			fileName: "apm.rep", replayDate: "2024-06-01T10:00:00Z",
			mapName: "Python", durationSeconds: 600, gameType: "Melee",
			mapKind: "Regular", teamFormat: "1v1", matchup: "PvT",
		})
		seedPlayer(t, conn, playerFixture{replayID: rid, name: "Flash", race: "Terran", team: 1, apm: 400, eapm: 300})
		seedPlayer(t, conn, playerFixture{replayID: rid, name: "Jaedong", race: "Zerg", team: 2, apm: 350, eapm: 280})
	}

	// minGames = 2 => only Flash qualifies (both players have 2 here actually).
	rows, err := s.ListPlayerApmAggregates(ctx, 2)
	if err != nil {
		t.Fatalf("ListPlayerApmAggregates: %v", err)
	}
	byKey := map[string]PlayerApmAggregateRow{}
	for _, r := range rows {
		byKey[r.PlayerKey] = r
	}
	if f, ok := byKey["flash"]; !ok || f.GamesPlayed != 2 || f.AverageAPM != 400 {
		t.Errorf("flash agg = %+v ok=%v", f, ok)
	}

	// minGames = 3 => nobody qualifies.
	rows, err = s.ListPlayerApmAggregates(ctx, 3)
	if err != nil {
		t.Fatalf("ListPlayerApmAggregates(3): %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for minGames=3, got %d", len(rows))
	}
}

func TestListReplayPatternsAndPlayerPatterns(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	// Replay-level marker (source_player_id NULL) and a player-level marker.
	seedMarker(t, conn, replayID, nil, "team_stacking", 0, nil)
	seedMarker(t, conn, replayID, &boxerID, "made_recalls", 300, ptrStr(`{"label":"2 recalls"}`))

	replayPatterns, err := s.ListReplayPatterns(ctx, replayID)
	if err != nil {
		t.Fatalf("ListReplayPatterns: %v", err)
	}
	if len(replayPatterns) != 1 || replayPatterns[0].PatternName != "team_stacking" {
		t.Fatalf("replay patterns = %+v", replayPatterns)
	}

	playerPatterns, err := s.ListPlayerPatterns(ctx, replayID)
	if err != nil {
		t.Fatalf("ListPlayerPatterns: %v", err)
	}
	if len(playerPatterns) != 1 || playerPatterns[0].PlayerID != boxerID {
		t.Fatalf("player patterns = %+v", playerPatterns)
	}
	if playerPatterns[0].PatternName != "made_recalls" {
		t.Errorf("pattern name = %q", playerPatterns[0].PatternName)
	}
}

func TestListReplayEventsJoinsPlayerNames(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, _, _ := fixtureBasic1v1(t, conn)

	seedGameEvent(t, conn, replayID, "cannon_rush", 120)
	// Markers should NOT appear in the game_event-only ListReplayEvents.
	seedMarker(t, conn, replayID, nil, "team_stacking", 0, nil)

	rows, err := s.ListReplayEvents(ctx, replayID)
	if err != nil {
		t.Fatalf("ListReplayEvents: %v", err)
	}
	if len(rows) != 1 || rows[0].EventType != "cannon_rush" || rows[0].Second != 120 {
		t.Fatalf("events = %+v", rows)
	}
}

func TestCountPlayerGamesAndRaceSections(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	fixtureBasic1v1(t, conn)

	n, err := s.CountPlayerGames(ctx, "boxer")
	if err != nil {
		t.Fatalf("CountPlayerGames: %v", err)
	}
	if n != 1 {
		t.Errorf("count = %d, want 1", n)
	}

	sections, err := s.ListRaceSections(ctx, "boxer")
	if err != nil {
		t.Fatalf("ListRaceSections: %v", err)
	}
	if len(sections) != 1 || sections[0].Race != "Terran" || sections[0].Wins != 1 {
		t.Errorf("sections = %+v", sections)
	}
}

func TestListPlayerMatchups(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	fixtureBasic1v1(t, conn)

	rows, err := s.ListPlayerMatchups(ctx, "boxer")
	if err != nil {
		t.Fatalf("ListPlayerMatchups: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("matchups = %+v", rows)
	}
	if rows[0].OwnRace != "Terran" || rows[0].OppRace != "Protoss" || rows[0].Wins != 1 {
		t.Errorf("matchup = %+v", rows[0])
	}
}

func TestGetPhaseBoundariesForReplay(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, _, _ := fixtureBasic1v1(t, conn)

	seedMarker(t, conn, replayID, nil, "mid_game_starts", 400, nil)
	seedMarker(t, conn, replayID, nil, "late_game_starts", 800, nil)

	pb, err := s.GetPhaseBoundariesForReplay(ctx, replayID)
	if err != nil {
		t.Fatalf("GetPhaseBoundariesForReplay: %v", err)
	}
	if pb.EarlyEndsAtSecond != 400 || pb.MidEndsAtSecond != 800 {
		t.Errorf("boundaries = %+v", pb)
	}
}

func TestListViewportGameRowsFiltersEmptyPayload(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, nadaID := fixtureBasic1v1(t, conn)

	seedMarker(t, conn, replayID, &boxerID, "viewport_multitasking", 100, ptrStr(`{"switches_per_minute":12}`))
	// Empty payload should be filtered out (distinct player: marker unique
	// index is per source_player_id).
	seedMarker(t, conn, replayID, &nadaID, "viewport_multitasking", 100, ptrStr("   "))

	rows, err := s.ListViewportGameRows(ctx, replayID, "viewport_multitasking")
	if err != nil {
		t.Fatalf("ListViewportGameRows: %v", err)
	}
	if len(rows) != 1 || rows[0].PlayerID != boxerID {
		t.Fatalf("viewport rows = %+v", rows)
	}
}

func TestCountReplays(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()

	n, err := s.CountReplays(ctx)
	if err != nil {
		t.Fatalf("CountReplays: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 replays initially, got %d", n)
	}
	fixtureBasic1v1(t, conn)
	n, err = s.CountReplays(ctx)
	if err != nil {
		t.Fatalf("CountReplays: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 replay, got %d", n)
	}
}

func TestGetReplayFilePathByID(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, _, _ := fixtureBasic1v1(t, conn)

	path, err := s.GetReplayFilePathByID(ctx, replayID)
	if err != nil {
		t.Fatalf("GetReplayFilePathByID: %v", err)
	}
	if path != "/r/g1.rep" {
		t.Errorf("path = %q", path)
	}
}

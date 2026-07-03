package db

import (
	"context"
	"testing"
)

func TestWorkflowGamesListQueries(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()

	rid1 := seedReplay(t, conn, replayFixture{
		filePath: "/r/wf1.rep", checksum: "wf1", fileName: "wf1.rep",
		replayDate: "2024-06-02T10:00:00Z", mapName: "Fighting Spirit", durationSeconds: 900,
		gameType: "Melee", mapKind: "Regular", teamFormat: "1v1", matchup: "TvP",
	})
	seedPlayer(t, conn, playerFixture{replayID: rid1, name: "BoxeR", race: "Terran", team: 1, apm: 300, isWinner: true})
	seedPlayer(t, conn, playerFixture{replayID: rid1, name: "NaDa", race: "Protoss", team: 2, apm: 250})

	rid2 := seedReplay(t, conn, replayFixture{
		filePath: "/r/wf2.rep", checksum: "wf2", fileName: "wf2.rep",
		replayDate: "2024-06-01T10:00:00Z", mapName: "Python", durationSeconds: 1300,
		gameType: "Melee", mapKind: "Money", teamFormat: "1v1", matchup: "TvZ",
	})
	seedPlayer(t, conn, playerFixture{replayID: rid2, name: "BoxeR", race: "Terran", team: 1, apm: 320})
	seedPlayer(t, conn, playerFixture{replayID: rid2, name: "Jaedong", race: "Zerg", team: 2, apm: 350})

	t.Run("CountGamesWithWhere", func(t *testing.T) {
		n, err := s.CountGamesWithWhere(ctx, "WHERE r.map_kind = ?", []any{"Money"})
		if err != nil {
			t.Fatalf("CountGamesWithWhere: %v", err)
		}
		if n != 1 {
			t.Errorf("count = %d, want 1", n)
		}
	})

	t.Run("ListGamesWithWhere ordering", func(t *testing.T) {
		rows, err := s.ListGamesWithWhere(ctx, "", nil, 10, 0)
		if err != nil {
			t.Fatalf("ListGamesWithWhere: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 games, got %d", len(rows))
		}
		// Ordered by replay_date DESC: wf1 (2024-06-02) first.
		if rows[0].ReplayID != rid1 || rows[0].MapName != "Fighting Spirit" {
			t.Errorf("row0 = %+v", rows[0])
		}
		if rows[1].ReplayID != rid2 {
			t.Errorf("row1 = %+v", rows[1])
		}
	})

	t.Run("ListReplayPlayers", func(t *testing.T) {
		rows, err := s.ListReplayPlayers(ctx, []int64{rid1})
		if err != nil {
			t.Fatalf("ListReplayPlayers: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 players, got %d", len(rows))
		}
		if rows[0].Name != "BoxeR" || !rows[0].IsWinner {
			t.Errorf("row0 = %+v", rows[0])
		}
	})

	t.Run("ListReplayPlayers empty", func(t *testing.T) {
		rows, err := s.ListReplayPlayers(ctx, nil)
		if err != nil {
			t.Fatalf("ListReplayPlayers empty: %v", err)
		}
		if len(rows) != 0 {
			t.Errorf("expected empty, got %d", len(rows))
		}
	})

	t.Run("ListCurrentPlayersForReplayIDs", func(t *testing.T) {
		rows, err := s.ListCurrentPlayersForReplayIDs(ctx, "boxer", []int64{rid1, rid2})
		if err != nil {
			t.Fatalf("ListCurrentPlayersForReplayIDs: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 boxer rows, got %d: %+v", len(rows), rows)
		}
		for _, r := range rows {
			if r.Name != "BoxeR" || r.Race != "Terran" {
				t.Errorf("row = %+v", r)
			}
		}
	})
}

func TestWorkflowPlayerAggregates(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 6; i++ {
		rid := seedReplay(t, conn, replayFixture{
			filePath: "/r/wp" + string(rune('a'+i)) + ".rep", checksum: "wp" + string(rune('a'+i)),
			fileName: "wp.rep", replayDate: "2024-06-01T10:00:00Z", mapName: "Python",
			durationSeconds: 900, gameType: "Melee", mapKind: "Regular", teamFormat: "1v1", matchup: "TvZ",
		})
		seedPlayer(t, conn, playerFixture{replayID: rid, name: "Flash", race: "Terran", team: 1, apm: 400})
		seedPlayer(t, conn, playerFixture{replayID: rid, name: "Jaedong", race: "Zerg", team: 2, apm: 350})
	}

	baseSQL, baseArgs := BuildWorkflowPlayersListBaseSQL("")
	whereSQL, whereArgs := BuildWorkflowPlayersListWhere(true, nil)
	allArgs := append(append([]any{}, baseArgs...), whereArgs...)

	t.Run("CountWorkflowPlayers", func(t *testing.T) {
		n, err := s.CountWorkflowPlayers(ctx, baseSQL, whereSQL, allArgs)
		if err != nil {
			t.Fatalf("CountWorkflowPlayers: %v", err)
		}
		// Both Flash and Jaedong have 6 games (>= 5).
		if n != 2 {
			t.Errorf("count = %d, want 2", n)
		}
	})

	t.Run("ListWorkflowPlayers", func(t *testing.T) {
		rows, err := s.ListWorkflowPlayers(ctx, baseSQL, whereSQL, "games_played", "DESC", allArgs, 10, 0)
		if err != nil {
			t.Fatalf("ListWorkflowPlayers: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 players, got %d: %+v", len(rows), rows)
		}
		byKey := map[string]WorkflowPlayersListRow{}
		for _, r := range rows {
			byKey[r.PlayerKey] = r
		}
		flash := byKey["flash"]
		if flash.GamesPlayed != 6 || flash.Race != "Terran" || flash.AverageAPM != 400 {
			t.Errorf("flash = %+v", flash)
		}
	})

	t.Run("CountWorkflowLastPlayedBuckets", func(t *testing.T) {
		// last_played is old (2024) so both windows should be 0 relative to now.
		c1m, c3m, err := s.CountWorkflowLastPlayedBuckets(ctx, baseSQL, whereSQL, allArgs)
		if err != nil {
			t.Fatalf("CountWorkflowLastPlayedBuckets: %v", err)
		}
		if c1m != 0 || c3m != 0 {
			t.Errorf("buckets = %d/%d, want 0/0 for old games", c1m, c3m)
		}
	})
}

func TestWorkflowFilterAndEventQueries(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()

	rid := seedReplay(t, conn, replayFixture{
		filePath: "/r/wfe.rep", checksum: "wfe", fileName: "wfe.rep",
		replayDate: "2024-06-01T10:00:00Z", mapName: "Fighting Spirit", durationSeconds: 500,
		gameType: "Melee", mapKind: "Regular", teamFormat: "1v1", matchup: "TvP",
	})
	boxerID := seedPlayer(t, conn, playerFixture{replayID: rid, name: "BoxeR", race: "Terran", team: 1, apm: 300, isWinner: true})
	nadaID := seedPlayer(t, conn, playerFixture{replayID: rid, name: "NaDa", race: "Protoss", team: 2, apm: 250})

	seedGameEvent(t, conn, rid, "cannon_rush", 120)
	seedGameEvent(t, conn, rid, "cliff_drop", 400)
	// Non-featuring game event ignored.
	seedGameEvent(t, conn, rid, "some_other_event", 200)

	seedMarker(t, conn, rid, &boxerID, "carriers", 500, ptrStr(`{"count":3}`))
	seedMarker(t, conn, rid, &boxerID, "bo_z_fuzzy", 100, ptrStr(`{"label":"~10 Hatch"}`))
	// Second bo_z_fuzzy goes on NaDa: the schema enforces one marker row per
	// (replay, player, event_type), and this also gives ListDistinctMarkerLabels
	// two distinct labels to sort.
	seedMarker(t, conn, rid, &nadaID, "bo_z_fuzzy", 110, ptrStr(`{"label":"~9 Pool"}`))

	t.Run("ListFeaturingReplayEventRows", func(t *testing.T) {
		rows, err := s.ListFeaturingReplayEventRows(ctx, []int64{rid})
		if err != nil {
			t.Fatalf("ListFeaturingReplayEventRows: %v", err)
		}
		types := map[string]bool{}
		for _, r := range rows {
			types[r.EventType] = true
		}
		if !types["cannon_rush"] || !types["cliff_drop"] || types["some_other_event"] {
			t.Errorf("featuring events = %+v", types)
		}
	})

	t.Run("ListFeaturingPlayerPatternRows", func(t *testing.T) {
		rows, err := s.ListFeaturingPlayerPatternRows(ctx, []int64{rid})
		if err != nil {
			t.Fatalf("ListFeaturingPlayerPatternRows: %v", err)
		}
		found := false
		for _, r := range rows {
			if r.PatternName == "carriers" && r.ReplayID == rid {
				found = true
			}
		}
		if !found {
			t.Errorf("expected carriers pattern row, got %+v", rows)
		}
	})

	t.Run("ListPatternValuesForPlayerIDs", func(t *testing.T) {
		rows, err := s.ListPatternValuesForPlayerIDs(ctx, []int64{boxerID})
		if err != nil {
			t.Fatalf("ListPatternValuesForPlayerIDs: %v", err)
		}
		// carriers + one bo_z_fuzzy belong to boxer (NaDa holds the other fuzzy).
		if len(rows) != 2 {
			t.Fatalf("expected 2 marker rows, got %d: %+v", len(rows), rows)
		}
		for _, r := range rows {
			if r.PlayerID != boxerID {
				t.Errorf("row player id = %d", r.PlayerID)
			}
		}
	})

	t.Run("ListDistinctMarkerLabels sorted by supply rung", func(t *testing.T) {
		labels, err := s.ListDistinctMarkerLabels(ctx, "bo_z_fuzzy")
		if err != nil {
			t.Fatalf("ListDistinctMarkerLabels: %v", err)
		}
		if len(labels) != 2 {
			t.Fatalf("expected 2 labels, got %d: %+v", len(labels), labels)
		}
		// "~9 Pool" (9) must sort before "~10 Hatch" (10).
		if labels[0] != "~9 Pool" || labels[1] != "~10 Hatch" {
			t.Errorf("labels = %+v", labels)
		}
	})

	t.Run("ListWorkflowFilterMaps", func(t *testing.T) {
		rows, err := s.ListWorkflowFilterMaps(ctx)
		if err != nil {
			t.Fatalf("ListWorkflowFilterMaps: %v", err)
		}
		if len(rows) != 1 || rows[0].Label != "Fighting Spirit" || rows[0].Games != 1 {
			t.Errorf("filter maps = %+v", rows)
		}
	})

	t.Run("CountWorkflowDurationBuckets", func(t *testing.T) {
		under10, m1020, m2030, m3045, m45, err := s.CountWorkflowDurationBuckets(ctx)
		if err != nil {
			t.Fatalf("CountWorkflowDurationBuckets: %v", err)
		}
		// 500s -> under_10m.
		if under10 != 1 || m1020 != 0 || m2030 != 0 || m3045 != 0 || m45 != 0 {
			t.Errorf("buckets = %d/%d/%d/%d/%d", under10, m1020, m2030, m3045, m45)
		}
	})
}

func TestListWorkflowFilterPlayersMinGames(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()

	// Player with 5 games qualifies; player with 1 does not (HAVING >= 5).
	for i := 0; i < 5; i++ {
		rid := seedReplay(t, conn, replayFixture{
			filePath: "/r/fp" + string(rune('a'+i)) + ".rep", checksum: "fp" + string(rune('a'+i)),
			fileName: "fp.rep", replayDate: "2024-06-01T10:00:00Z", mapName: "Python",
			durationSeconds: 900, gameType: "Melee", mapKind: "Regular", teamFormat: "1v1", matchup: "TvZ",
		})
		seedPlayer(t, conn, playerFixture{replayID: rid, name: "Flash", race: "Terran", team: 1, apm: 400})
		if i == 0 {
			seedPlayer(t, conn, playerFixture{replayID: rid, name: "Rare", race: "Zerg", team: 2, apm: 100})
		}
	}

	rows, err := s.ListWorkflowFilterPlayers(ctx)
	if err != nil {
		t.Fatalf("ListWorkflowFilterPlayers: %v", err)
	}
	if len(rows) != 1 || rows[0].Key != "flash" || rows[0].Games != 5 {
		t.Errorf("filter players = %+v", rows)
	}
}

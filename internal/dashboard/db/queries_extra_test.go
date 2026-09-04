package db

import (
	"context"
	"database/sql"
	"testing"
)

// seedRichCommand inserts a command carrying the optional text columns
// (tech_name, upgrade_name, chat_message, order_name) that the base
// seedCommand helper leaves nil.
func seedRichCommand(t *testing.T, conn *sql.DB, replayID, playerID, second int64, actionType string, cols richCommandCols) {
	t.Helper()
	mustExec(t, conn, `
		INSERT INTO commands (replay_id, player_id, frame, seconds_from_game_start, action_type, unit_type, tech_name, upgrade_name, chat_message, order_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		replayID, playerID, second*24, second, actionType,
		cols.unitType, cols.techName, cols.upgradeName, cols.chatMessage, cols.orderName)
}

type richCommandCols struct {
	unitType    *string
	techName    *string
	upgradeName *string
	chatMessage *string
	orderName   *string
}

func seedLowValueCommand(t *testing.T, conn *sql.DB, replayID, playerID, second int64, actionType string, alliancePlayerIDs *string) {
	t.Helper()
	mustExec(t, conn, `
		INSERT INTO commands_low_value (replay_id, player_id, frame, seconds_from_game_start, action_type, alliance_player_ids)
		VALUES (?, ?, ?, ?, ?, ?)`,
		replayID, playerID, second*24, second, actionType, alliancePlayerIDs)
}

func TestListRaceSectionsOrdering(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()

	// Boxer plays Terran twice (1 win) and Protoss once.
	for i, race := range []string{"Terran", "Terran", "Protoss"} {
		rid := seedReplay(t, conn, replayFixture{
			filePath: "/r/rs" + string(rune('a'+i)) + ".rep", checksum: "rs" + string(rune('a'+i)),
			fileName: "rs.rep", replayDate: "2024-06-01T10:00:00Z", mapName: "Python",
			durationSeconds: 600, gameType: "Melee", mapKind: "Regular", teamFormat: "1v1", matchup: "TvT",
		})
		seedPlayer(t, conn, playerFixture{replayID: rid, name: "BoxeR", race: race, team: 1, apm: 300, isWinner: i == 0})
	}

	sections, err := s.ListRaceSections(ctx, "boxer")
	if err != nil {
		t.Fatalf("ListRaceSections: %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("expected 2 race sections, got %d: %+v", len(sections), sections)
	}
	if sections[0].Race != "Terran" || sections[0].GameCount != 2 || sections[0].Wins != 1 {
		t.Errorf("section0 = %+v", sections[0])
	}
	if sections[1].Race != "Protoss" || sections[1].GameCount != 1 || sections[1].Wins != 0 {
		t.Errorf("section1 = %+v", sections[1])
	}
}

func TestListPlayerFirstExpansionTimings(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	// Two expansion game_events for BoxeR; earliest must be picked per (race,map_kind,replay).
	mustExec(t, conn, `
		INSERT INTO replay_events (replay_id, seconds_from_game_start, event_kind, event_type, source_player_id)
		VALUES (?, 300, 'game_event', 'expansion', ?), (?, 180, 'game_event', 'expansion', ?)`,
		replayID, boxerID, replayID, boxerID)

	rows, err := s.ListPlayerFirstExpansionTimings(ctx, "boxer")
	if err != nil {
		t.Fatalf("ListPlayerFirstExpansionTimings: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d: %+v", len(rows), rows)
	}
	if rows[0].Race != "Terran" || rows[0].MapKind != "Regular" || rows[0].FirstExpansionSecond != 180 {
		t.Errorf("row = %+v", rows[0])
	}
	if rows[0].ReplayID != replayID {
		t.Errorf("replay id = %d, want %d", rows[0].ReplayID, replayID)
	}
}

func TestListGameUnitProductionAndCasts(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedCommand(t, conn, replayID, boxerID, 60, "Train", ptrStr("Marine"))
	seedCommand(t, conn, replayID, boxerID, 120, "Unit Morph", ptrStr("Lurker"))
	seedRichCommand(t, conn, replayID, boxerID, 180, "Targeted Order", richCommandCols{orderName: ptrStr("CastRecall")})
	// Should be excluded (not one of the tracked action types).
	seedCommand(t, conn, replayID, boxerID, 200, "Right Click", nil)

	rows, err := s.ListGameUnitProductionAndCasts(ctx, replayID)
	if err != nil {
		t.Fatalf("ListGameUnitProductionAndCasts: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].ActionType != "Train" || rows[0].UnitType == nil || *rows[0].UnitType != "Marine" {
		t.Errorf("row0 = %+v", rows[0])
	}
	if rows[2].ActionType != "Targeted Order" || rows[2].OrderName == nil || *rows[2].OrderName != "CastRecall" {
		t.Errorf("row2 = %+v", rows[2])
	}
	if rows[0].SecondsFromGameStart != 60 || rows[2].SecondsFromGameStart != 180 {
		t.Errorf("ordering broken: %+v", rows)
	}
}

func TestListReplayAllianceCommands(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedLowValueCommand(t, conn, replayID, boxerID, 500, "Alliance", ptrStr("[1,2]"))
	seedLowValueCommand(t, conn, replayID, boxerID, 300, "Alliance", ptrStr("[3]"))
	// Non-alliance in the low-value table must be ignored.
	seedLowValueCommand(t, conn, replayID, boxerID, 100, "Right Click", nil)

	rows, err := s.ListReplayAllianceCommands(ctx, replayID)
	if err != nil {
		t.Fatalf("ListReplayAllianceCommands: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 alliance commands, got %d: %+v", len(rows), rows)
	}
	if rows[0].SecondsFromGameStart != 300 || rows[0].AlliancePlayerIDs != "[3]" {
		t.Errorf("row0 = %+v", rows[0])
	}
	if rows[1].SecondsFromGameStart != 500 {
		t.Errorf("ordering broken: %+v", rows)
	}
}

func TestPlayerInsightAuxQueries(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	fixtureBasic1v1(t, conn)

	t.Run("GetPlayerNameByKey", func(t *testing.T) {
		name, err := s.GetPlayerNameByKey(ctx, "boxer")
		if err != nil {
			t.Fatalf("GetPlayerNameByKey: %v", err)
		}
		if name != "BoxeR" {
			t.Errorf("name = %q, want BoxeR", name)
		}
	})

}

func TestListRaceOrderRowsAndMatchupOrderRows(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedRichCommand(t, conn, replayID, boxerID, 300, "Tech", richCommandCols{techName: ptrStr("Stim Packs")})
	seedRichCommand(t, conn, replayID, boxerID, 400, "Upgrade", richCommandCols{upgradeName: ptrStr("Terran Infantry Weapons")})
	// Tech with no name (NULL — the schema forbids an empty string) must be excluded.
	seedRichCommand(t, conn, replayID, boxerID, 500, "Tech", richCommandCols{techName: nil})

	raceRows, err := s.ListRaceOrderRows(ctx, "boxer")
	if err != nil {
		t.Fatalf("ListRaceOrderRows: %v", err)
	}
	if len(raceRows) != 2 {
		t.Fatalf("expected 2 race-order rows, got %d: %+v", len(raceRows), raceRows)
	}
	if raceRows[0].ActionType != "Tech" || raceRows[0].TechName == nil || *raceRows[0].TechName != "Stim Packs" {
		t.Errorf("raceRow0 = %+v", raceRows[0])
	}
	if raceRows[0].Race != "Terran" || raceRows[0].PlayerID != boxerID {
		t.Errorf("raceRow0 meta = %+v", raceRows[0])
	}

	matchupRows, err := s.ListMatchupOrderRows(ctx, "boxer")
	if err != nil {
		t.Fatalf("ListMatchupOrderRows: %v", err)
	}
	if len(matchupRows) != 2 {
		t.Fatalf("expected 2 matchup-order rows, got %d: %+v", len(matchupRows), matchupRows)
	}
	if matchupRows[0].OwnRace != "Terran" || matchupRows[0].OppRace != "Protoss" {
		t.Errorf("matchupRow0 = %+v", matchupRows[0])
	}
}

func TestListPlayerChatRows(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedRichCommand(t, conn, replayID, boxerID, 60, "Chat", richCommandCols{chatMessage: ptrStr("gl hf")})
	// Empty/whitespace chat must be excluded.
	seedRichCommand(t, conn, replayID, boxerID, 70, "Chat", richCommandCols{chatMessage: ptrStr("   ")})

	rows, err := s.ListPlayerChatRows(ctx, "boxer")
	if err != nil {
		t.Fatalf("ListPlayerChatRows: %v", err)
	}
	if len(rows) != 1 || rows[0].Message != "gl hf" || rows[0].ReplayID != replayID {
		t.Errorf("chat rows = %+v", rows)
	}
}

func TestTimingRowsQueries(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedCommand(t, conn, replayID, boxerID, 120, "Build", ptrStr("Refinery"))
	seedRichCommand(t, conn, replayID, boxerID, 300, "Upgrade", richCommandCols{upgradeName: ptrStr("Terran Infantry Weapons")})
	seedRichCommand(t, conn, replayID, boxerID, 400, "Tech", richCommandCols{techName: ptrStr("Stim Packs")})

	gas, err := s.ListGasTimingRows(ctx, replayID)
	if err != nil {
		t.Fatalf("ListGasTimingRows: %v", err)
	}
	if len(gas) != 1 || gas[0].Label != "Refinery" || gas[0].Second != 120 {
		t.Errorf("gas timings = %+v", gas)
	}

	ups, err := s.ListUpgradeTimingRows(ctx, replayID)
	if err != nil {
		t.Fatalf("ListUpgradeTimingRows: %v", err)
	}
	if len(ups) != 1 || ups[0].Label != "Terran Infantry Weapons" {
		t.Errorf("upgrade timings = %+v", ups)
	}

	techs, err := s.ListTechTimingRows(ctx, replayID)
	if err != nil {
		t.Fatalf("ListTechTimingRows: %v", err)
	}
	if len(techs) != 1 || techs[0].Label != "Stim Packs" || techs[0].PlayerID != boxerID {
		t.Errorf("tech timings = %+v", techs)
	}
}

func TestListHotkeyGamesRateByPlayer(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	_, boxerID, _ := fixtureBasic1v1(t, conn)

	// The rate reads players.hotkey_stream (the used_hotkey_groups marker was
	// retired); any non-NULL blob counts as a hotkey game.
	mustExec(t, conn, `UPDATE players SET hotkey_stream = X'FF02' WHERE id = ?`, boxerID)

	rates, err := s.ListHotkeyGamesRateByPlayer(ctx)
	if err != nil {
		t.Fatalf("ListHotkeyGamesRateByPlayer: %v", err)
	}
	if rates["boxer"] != 100 {
		t.Errorf("boxer hotkey rate = %v, want 100", rates["boxer"])
	}
	if rates["nada"] != 0 {
		t.Errorf("nada hotkey rate = %v, want 0", rates["nada"])
	}
}

// fixture2v2 seeds a single 2v2 melee-style game on a money map for the
// player-summary by-format queries. Returns the replay id.
func fixture2v2(t *testing.T, conn *sql.DB, mapKind string) int64 {
	t.Helper()
	rid := seedReplay(t, conn, replayFixture{
		filePath: "/r/2v2_" + mapKind + ".rep", checksum: "2v2" + mapKind, fileName: "2v2.rep",
		replayDate: "2024-06-01T10:00:00Z", mapName: "Python", durationSeconds: 1200,
		gameType: "Melee", mapKind: mapKind, teamFormat: "2v2", matchup: "",
	})
	seedPlayer(t, conn, playerFixture{replayID: rid, name: "BoxeR", race: "Terran", team: 1, apm: 300, eapm: 200, isWinner: true})
	seedPlayer(t, conn, playerFixture{replayID: rid, name: "Ally", race: "Zerg", team: 1, apm: 200, eapm: 150})
	seedPlayer(t, conn, playerFixture{replayID: rid, name: "Foe1", race: "Protoss", team: 2, apm: 250, eapm: 180})
	seedPlayer(t, conn, playerFixture{replayID: rid, name: "Foe2", race: "Protoss", team: 2, apm: 240, eapm: 170})
	return rid
}

func TestViewportAggregateRows(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, nadaID := fixtureBasic1v1(t, conn)

	seedMarker(t, conn, replayID, &boxerID, "viewport_multitasking", 100, ptrStr(`{"switches_per_minute":12}`))
	// Empty payload excluded.
	seedMarker(t, conn, replayID, &nadaID, "viewport_multitasking", 100, ptrStr("   "))

	rows, err := s.ListViewportAggregateRows(ctx, "viewport_multitasking")
	if err != nil {
		t.Fatalf("ListViewportAggregateRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 aggregate row, got %d: %+v", len(rows), rows)
	}
	if rows[0].PlayerKey != "boxer" || rows[0].PlayerName != "BoxeR" || rows[0].RawValue != `{"switches_per_minute":12}` {
		t.Errorf("aggregate row = %+v", rows[0])
	}
}

func TestFilterOptionQueries(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	fixtureBasic1v1(t, conn)

	t.Run("ListTopPlayerColorRows", func(t *testing.T) {
		rows, err := s.ListTopPlayerColorRows(ctx)
		if err != nil {
			t.Fatalf("ListTopPlayerColorRows: %v", err)
		}
		byKey := map[string]int64{}
		for _, r := range rows {
			byKey[r.PlayerKey] = r.Games
		}
		if byKey["boxer"] != 1 || byKey["nada"] != 1 {
			t.Errorf("color rows = %+v", byKey)
		}
	})

}

func TestUnitCadenceRowQueries(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedCommand(t, conn, replayID, boxerID, 60, "Train", ptrStr("Marine"))
	seedCommand(t, conn, replayID, boxerID, 90, "Build", ptrStr("Barracks"))
	// A Train with no unit_type (NULL — the schema forbids an empty string) is
	// excluded from the slice-command list but still counts as a raw command.
	seedCommand(t, conn, replayID, boxerID, 100, "Train", nil)

	slices, err := s.ListUnitSliceCommandRows(ctx, replayID)
	if err != nil {
		t.Fatalf("ListUnitSliceCommandRows: %v", err)
	}
	if len(slices) != 2 {
		t.Fatalf("expected 2 slice rows, got %d: %+v", len(slices), slices)
	}
	if slices[0].UnitType != "Marine" || slices[0].Second != 60 {
		t.Errorf("slice0 = %+v", slices[0])
	}

	firsts, err := s.ListFirstUnitCommandRows(ctx, replayID)
	if err != nil {
		t.Fatalf("ListFirstUnitCommandRows: %v", err)
	}
	if len(firsts) != 3 {
		t.Fatalf("expected 3 first-unit rows, got %d: %+v", len(firsts), firsts)
	}
	if firsts[0].ActionType != "Train" || firsts[0].UnitType == nil || *firsts[0].UnitType != "Marine" {
		t.Errorf("first0 = %+v", firsts[0])
	}
}

func TestListUnitCadenceReplayMetrics(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	for sec := int64(60); sec <= 600; sec += 30 {
		seedCommand(t, conn, replayID, boxerID, sec, "Train", ptrStr("Marine"))
	}

	rows, err := s.ListUnitCadenceReplayMetrics(ctx, []string{"Overlord"}, "boxer", 0, 1.0, 20, 3, 1)
	if err != nil {
		t.Fatalf("ListUnitCadenceReplayMetrics: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 metric row, got %d: %+v", len(rows), rows)
	}
	r := rows[0]
	if r.ReplayID != replayID || r.PlayerKey != "boxer" || r.PlayerName != "BoxeR" {
		t.Errorf("metric row meta = %+v", r)
	}
	if r.UnitsProduced == 0 {
		t.Errorf("expected some units produced, got %+v", r)
	}
}

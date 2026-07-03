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

func TestListRacePatterns(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedMarker(t, conn, replayID, &boxerID, "made_recalls", 300, nil)
	// Meta markers must be excluded.
	seedMarker(t, conn, replayID, &boxerID, "used_hotkey_groups", 10, nil)
	seedMarker(t, conn, replayID, &boxerID, "viewport_multitasking", 20, ptrStr(`{"x":1}`))

	rows, err := s.ListRacePatterns(ctx, "boxer")
	if err != nil {
		t.Fatalf("ListRacePatterns: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 race pattern, got %d: %+v", len(rows), rows)
	}
	if rows[0].Race != "Terran" || rows[0].PatternName != "made_recalls" || rows[0].ReplayCount != 1 {
		t.Errorf("row = %+v", rows[0])
	}
}

func TestListTopActionTypes(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedCommand(t, conn, replayID, boxerID, 10, "Select", nil)
	seedCommand(t, conn, replayID, boxerID, 11, "Select", nil)
	seedCommand(t, conn, replayID, boxerID, 12, "Select", nil)
	seedCommand(t, conn, replayID, boxerID, 13, "Train", ptrStr("Marine"))
	seedCommand(t, conn, replayID, boxerID, 14, "Targeted Order", nil)

	rows, err := s.ListTopActionTypes(ctx, boxerID, 2)
	if err != nil {
		t.Fatalf("ListTopActionTypes: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (limit), got %d: %+v", len(rows), rows)
	}
	if rows[0] != "Select" {
		t.Errorf("top action = %q, want Select", rows[0])
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
	replayID, boxerID, nadaID := fixtureBasic1v1(t, conn)

	t.Run("CountDistinctPlayers", func(t *testing.T) {
		n, err := s.CountDistinctPlayers(ctx)
		if err != nil {
			t.Fatalf("CountDistinctPlayers: %v", err)
		}
		// BoxeR, NaDa, Obs (observers still counted here: WHERE is_observer=0? Obs is observer -> excluded).
		if n != 2 {
			t.Errorf("distinct players = %v, want 2", n)
		}
	})

	t.Run("CountDistinctPlayersByRace", func(t *testing.T) {
		n, err := s.CountDistinctPlayersByRace(ctx, "Terran")
		if err != nil {
			t.Fatalf("CountDistinctPlayersByRace: %v", err)
		}
		if n != 1 {
			t.Errorf("terran players = %v, want 1", n)
		}
	})

	t.Run("ListPlayersByReplayRows", func(t *testing.T) {
		rows, err := s.ListPlayersByReplayRows(ctx)
		if err != nil {
			t.Fatalf("ListPlayersByReplayRows: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 non-observer rows, got %d", len(rows))
		}
		names := map[string]bool{}
		for _, r := range rows {
			names[r.Name] = true
			if r.ReplayID != replayID {
				t.Errorf("replay id = %d", r.ReplayID)
			}
		}
		if !names["BoxeR"] || !names["NaDa"] {
			t.Errorf("names = %+v", names)
		}
	})

	t.Run("GetPlayerNameByKey", func(t *testing.T) {
		name, err := s.GetPlayerNameByKey(ctx, "boxer")
		if err != nil {
			t.Fatalf("GetPlayerNameByKey: %v", err)
		}
		if name != "BoxeR" {
			t.Errorf("name = %q, want BoxeR", name)
		}
	})

	t.Run("ListExpansionEvents", func(t *testing.T) {
		mustExec(t, conn, `
			INSERT INTO replay_events (replay_id, seconds_from_game_start, event_kind, event_type, source_player_id)
			VALUES (?, 240, 'game_event', 'expansion', ?)`, replayID, boxerID)
		rows, err := s.ListExpansionEvents(ctx)
		if err != nil {
			t.Fatalf("ListExpansionEvents: %v", err)
		}
		if len(rows) != 1 || rows[0].Second != 240 || rows[0].PlayerID == nil || *rows[0].PlayerID != boxerID {
			t.Errorf("expansion events = %+v", rows)
		}
	})

	t.Run("CountCarrierGamesByPlayer", func(t *testing.T) {
		seedMarker(t, conn, replayID, &nadaID, "carriers", 600, nil)
		n, err := s.CountCarrierGamesByPlayer(ctx, "nada")
		if err != nil {
			t.Fatalf("CountCarrierGamesByPlayer: %v", err)
		}
		if n != 1 {
			t.Errorf("carrier games = %d, want 1", n)
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
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedMarker(t, conn, replayID, &boxerID, "used_hotkey_groups", 10, nil)

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

func TestPlayerInsightRaceQueries(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	t.Run("ListPlayerGamesByRace", func(t *testing.T) {
		rows, err := s.ListPlayerGamesByRace(ctx, "boxer")
		if err != nil {
			t.Fatalf("ListPlayerGamesByRace: %v", err)
		}
		if len(rows) != 1 || rows[0].Race != "Terran" || rows[0].Count != 1 {
			t.Errorf("rows = %+v", rows)
		}
	})

	t.Run("ListPopulationGamesByRace", func(t *testing.T) {
		rows, err := s.ListPopulationGamesByRace(ctx)
		if err != nil {
			t.Fatalf("ListPopulationGamesByRace: %v", err)
		}
		byRace := map[string]int64{}
		for _, r := range rows {
			byRace[r.Race] = r.Count
		}
		if byRace["Terran"] != 1 || byRace["Protoss"] != 1 {
			t.Errorf("population by race = %+v", byRace)
		}
	})

	t.Run("ListPlayerGamesByRaceMapKind", func(t *testing.T) {
		rows, err := s.ListPlayerGamesByRaceMapKind(ctx, "boxer")
		if err != nil {
			t.Fatalf("ListPlayerGamesByRaceMapKind: %v", err)
		}
		if len(rows) != 1 || rows[0].Race != "Terran" || rows[0].MapKind != "Regular" || rows[0].Count != 1 {
			t.Errorf("rows = %+v", rows)
		}
	})

	t.Run("ListPopulationGamesByRaceMapKind", func(t *testing.T) {
		rows, err := s.ListPopulationGamesByRaceMapKind(ctx)
		if err != nil {
			t.Fatalf("ListPopulationGamesByRaceMapKind: %v", err)
		}
		if len(rows) != 2 {
			t.Errorf("expected 2 (race,mapkind) rows, got %d: %+v", len(rows), rows)
		}
	})

	t.Run("ListPopulationDistinctPlayersByRace", func(t *testing.T) {
		rows, err := s.ListPopulationDistinctPlayersByRace(ctx)
		if err != nil {
			t.Fatalf("ListPopulationDistinctPlayersByRace: %v", err)
		}
		byRace := map[string]float64{}
		for _, r := range rows {
			byRace[r.Race] = r.Value
		}
		if byRace["Terran"] != 1 || byRace["Protoss"] != 1 {
			t.Errorf("distinct players by race = %+v", byRace)
		}
	})

	t.Run("ListPopulationDistinctPlayersByRaceMapKind", func(t *testing.T) {
		rows, err := s.ListPopulationDistinctPlayersByRaceMapKind(ctx)
		if err != nil {
			t.Fatalf("ListPopulationDistinctPlayersByRaceMapKind: %v", err)
		}
		if len(rows) != 2 {
			t.Errorf("expected 2 rows, got %d: %+v", len(rows), rows)
		}
	})

	t.Run("ListCommonBehaviours", func(t *testing.T) {
		seedMarker(t, conn, replayID, &boxerID, "made_recalls", 300, nil)
		seedMarker(t, conn, replayID, &boxerID, "used_hotkey_groups", 10, nil)
		rows, err := s.ListCommonBehaviours(ctx, "boxer")
		if err != nil {
			t.Fatalf("ListCommonBehaviours: %v", err)
		}
		if len(rows) != 1 || rows[0].PatternName != "made_recalls" || rows[0].ReplayCount != 1 {
			t.Errorf("common behaviours = %+v", rows)
		}
	})

	t.Run("GetOutlierPlayerSummary", func(t *testing.T) {
		row, err := s.GetOutlierPlayerSummary(ctx, "boxer")
		if err != nil {
			t.Fatalf("GetOutlierPlayerSummary: %v", err)
		}
		if row.Name == nil || *row.Name != "BoxeR" || row.Count != 1 {
			t.Errorf("summary = %+v", row)
		}
	})
}

func TestOutlierRawQueries(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedCommand(t, conn, replayID, boxerID, 60, "Train", ptrStr("Marine"))
	seedCommand(t, conn, replayID, boxerID, 61, "Train", ptrStr("Marine"))
	seedCommand(t, conn, replayID, boxerID, 62, "Train", ptrStr("Medic"))

	t.Run("ListOutlierPlayerCounts instance share", func(t *testing.T) {
		rows, err := s.ListOutlierPlayerCounts(ctx, "boxer", "Terran", "unit_type", true, []string{"Train"})
		if err != nil {
			t.Fatalf("ListOutlierPlayerCounts: %v", err)
		}
		byName := map[string]int64{}
		for _, r := range rows {
			byName[r.Name] = r.Count
			if r.Race != "Terran" {
				t.Errorf("race = %q", r.Race)
			}
		}
		if byName["Marine"] != 2 || byName["Medic"] != 1 {
			t.Errorf("instance-share counts = %+v", byName)
		}
	})

	t.Run("ListOutlierPlayerCounts distinct player", func(t *testing.T) {
		rows, err := s.ListOutlierPlayerCounts(ctx, "boxer", "Terran", "unit_type", false, []string{"Train"})
		if err != nil {
			t.Fatalf("ListOutlierPlayerCounts distinct: %v", err)
		}
		byName := map[string]int64{}
		for _, r := range rows {
			byName[r.Name] = r.Count
		}
		if byName["Marine"] != 1 {
			t.Errorf("distinct-player Marine count = %d, want 1", byName["Marine"])
		}
	})

	t.Run("ListOutlierGlobalRows", func(t *testing.T) {
		rows, err := s.ListOutlierGlobalRows(ctx, "Terran", "unit_type", true, []string{"Train"})
		if err != nil {
			t.Fatalf("ListOutlierGlobalRows: %v", err)
		}
		byName := map[string]OutlierGlobalRow{}
		for _, r := range rows {
			byName[r.Name] = r
		}
		if byName["Marine"].Games != 2 || byName["Marine"].Players != 1 {
			t.Errorf("global Marine = %+v", byName["Marine"])
		}
	})

	t.Run("ListOutlierPlayerCountsSegmented", func(t *testing.T) {
		rows, err := s.ListOutlierPlayerCountsSegmented(ctx, "boxer", "Terran", "unit_type", true, []string{"Train"})
		if err != nil {
			t.Fatalf("ListOutlierPlayerCountsSegmented: %v", err)
		}
		byName := map[string]SegmentedOutlierCountRow{}
		for _, r := range rows {
			byName[r.Name] = r
		}
		if byName["Marine"].GamesAll != 2 || byName["Marine"].GamesRegular != 2 || byName["Marine"].GamesMoney != 0 {
			t.Errorf("segmented Marine = %+v", byName["Marine"])
		}
	})

	t.Run("ListOutlierGlobalRowsSegmented", func(t *testing.T) {
		rows, err := s.ListOutlierGlobalRowsSegmented(ctx, "Terran", "unit_type", true, []string{"Train"})
		if err != nil {
			t.Fatalf("ListOutlierGlobalRowsSegmented: %v", err)
		}
		byName := map[string]SegmentedOutlierGlobalRow{}
		for _, r := range rows {
			byName[r.Name] = r
		}
		m := byName["Marine"]
		if m.GamesAll != 2 || m.GamesRegular != 2 || m.PlayersAll != 1 || m.PlayersRegular != 1 {
			t.Errorf("segmented global Marine = %+v", m)
		}
	})
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

func TestPlayerSummaryMatchupQueries(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	fixtureBasic1v1(t, conn)

	t.Run("ListPlayerMatchupAggregates", func(t *testing.T) {
		rows, err := s.ListPlayerMatchupAggregates(ctx, "boxer")
		if err != nil {
			t.Fatalf("ListPlayerMatchupAggregates: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("expected 1 matchup, got %d: %+v", len(rows), rows)
		}
		r := rows[0]
		if r.OwnRace != "Terran" || r.OppRace != "Protoss" || r.Games != 1 || r.Wins != 1 {
			t.Errorf("aggregate = %+v", r)
		}
		if r.AvgAPM != 300 || r.AvgEAPM != 200 {
			t.Errorf("aggregate apm/eapm = %+v", r)
		}
	})

	t.Run("ListPlayerMatchupMarkerCounts", func(t *testing.T) {
		replayID, boxerID, _ := func() (int64, int64, int64) {
			// reuse the seeded fixture's ids by re-querying
			var rid, pid int64
			row := conn.QueryRow(`SELECT r.id, p.id FROM replays r JOIN players p ON p.replay_id = r.id WHERE lower(trim(p.name)) = 'boxer'`)
			if err := row.Scan(&rid, &pid); err != nil {
				t.Fatalf("lookup: %v", err)
			}
			return rid, pid, 0
		}()
		seedMarker(t, conn, replayID, &boxerID, "made_recalls", 300, nil)
		seedMarker(t, conn, replayID, &boxerID, "used_hotkey_groups", 10, nil)

		rows, err := s.ListPlayerMatchupMarkerCounts(ctx, "boxer")
		if err != nil {
			t.Fatalf("ListPlayerMatchupMarkerCounts: %v", err)
		}
		if len(rows) != 1 || rows[0].PatternName != "made_recalls" || rows[0].ReplayCount != 1 {
			t.Errorf("marker counts = %+v", rows)
		}
	})
}

func TestPlayerSummaryByFormatQueries(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	fixture2v2(t, conn, "Money")

	t.Run("ListPlayerByFormatAggregates", func(t *testing.T) {
		rows, err := s.ListPlayerByFormatAggregates(ctx, "boxer")
		if err != nil {
			t.Fatalf("ListPlayerByFormatAggregates: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d: %+v", len(rows), rows)
		}
		r := rows[0]
		if r.OwnRace != "Terran" || r.TeamFormat != "2v2" || r.MapKind != "Money" || r.Games != 1 || r.Wins != 1 {
			t.Errorf("by-format aggregate = %+v", r)
		}
		if r.AvgAPM != 300 {
			t.Errorf("avg apm = %v", r.AvgAPM)
		}
	})

	t.Run("ListPlayerByFormatMarkerCounts", func(t *testing.T) {
		var rid, pid int64
		row := conn.QueryRow(`SELECT r.id, p.id FROM replays r JOIN players p ON p.replay_id = r.id WHERE lower(trim(p.name)) = 'boxer'`)
		if err := row.Scan(&rid, &pid); err != nil {
			t.Fatalf("lookup: %v", err)
		}
		seedMarker(t, conn, rid, &pid, "made_drops", 400, nil)

		rows, err := s.ListPlayerByFormatMarkerCounts(ctx, "boxer")
		if err != nil {
			t.Fatalf("ListPlayerByFormatMarkerCounts: %v", err)
		}
		if len(rows) != 1 || rows[0].PatternName != "made_drops" || rows[0].TeamFormat != "2v2" {
			t.Errorf("by-format marker counts = %+v", rows)
		}
	})
}

func TestMultiTeamMeleeQueries(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()

	rid := seedReplay(t, conn, replayFixture{
		filePath: "/r/ffa.rep", checksum: "ffa", fileName: "ffa.rep",
		replayDate: "2024-06-01T10:00:00Z", mapName: "Python", durationSeconds: 1500,
		gameType: "Free For All", mapKind: "Regular", teamFormat: "1v1v1", matchup: "",
	})
	p1 := seedPlayer(t, conn, playerFixture{replayID: rid, name: "BoxeR", race: "Terran", team: 1, apm: 300})
	seedPlayer(t, conn, playerFixture{replayID: rid, name: "Foe1", race: "Zerg", team: 2, apm: 200})
	seedPlayer(t, conn, playerFixture{replayID: rid, name: "Foe2", race: "Protoss", team: 3, apm: 250})

	games, err := s.CountPlayerMultiTeamMeleeGames(ctx, "boxer")
	if err != nil {
		t.Fatalf("CountPlayerMultiTeamMeleeGames: %v", err)
	}
	if games != 1 {
		t.Errorf("multi-team melee games = %d, want 1", games)
	}

	seedLowValueCommand(t, conn, rid, p1, 400, "Alliance", ptrStr("[2]"))
	seedLowValueCommand(t, conn, rid, p1, 500, "Alliance", ptrStr("[3]"))
	n, err := s.CountPlayerAllianceCommandsInMultiTeamMelee(ctx, "boxer")
	if err != nil {
		t.Fatalf("CountPlayerAllianceCommandsInMultiTeamMelee: %v", err)
	}
	if n != 2 {
		t.Errorf("alliance commands = %d, want 2", n)
	}
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

	t.Run("ListGlobalReplayFilterPlayerOptions", func(t *testing.T) {
		rows, err := s.ListGlobalReplayFilterPlayerOptions(ctx)
		if err != nil {
			t.Fatalf("ListGlobalReplayFilterPlayerOptions: %v", err)
		}
		if len(rows) != 2 {
			t.Errorf("expected 2 options, got %d: %+v", len(rows), rows)
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
	if firsts[0].ActionType != "Train" || !firsts[0].UnitType.Valid || firsts[0].UnitType.String != "Marine" {
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

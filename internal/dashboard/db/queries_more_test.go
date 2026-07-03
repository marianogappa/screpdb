package db

import (
	"context"
	"testing"
)

func TestSettingsIngestInputDirRoundTrip(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	// Seeded 'global' row defaults ingest_input_dir to ''.
	dir, err := s.GetIngestInputDir(ctx, "global")
	if err != nil {
		t.Fatalf("GetIngestInputDir: %v", err)
	}
	if dir != "" {
		t.Errorf("initial dir = %q, want empty", dir)
	}

	if err := s.SetIngestInputDir(ctx, "global", "  /replays/inbox  "); err != nil {
		t.Fatalf("SetIngestInputDir: %v", err)
	}
	dir, err = s.GetIngestInputDir(ctx, "global")
	if err != nil {
		t.Fatalf("GetIngestInputDir: %v", err)
	}
	// Getter trims whitespace.
	if dir != "/replays/inbox" {
		t.Errorf("dir = %q, want /replays/inbox", dir)
	}
}

func TestGetGlobalReplayFilterConfigRawDefaults(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	cfg, err := s.GetGlobalReplayFilterConfigRaw(ctx, "global")
	if err != nil {
		t.Fatalf("GetGlobalReplayFilterConfigRaw: %v", err)
	}
	if !cfg.ExcludeShortGames || !cfg.ExcludeComputers {
		t.Errorf("expected exclusions on by default, got %+v", cfg)
	}
	// No compiled SQL persisted yet.
	if cfg.CompiledReplaysFilterSQL != nil {
		t.Errorf("expected nil compiled SQL, got %v", *cfg.CompiledReplaysFilterSQL)
	}

	if err := s.UpdateGlobalReplayFilterConfigRaw(ctx, "global",
		"all", "only_these", `["melee"]`, false, false,
		"only_these", `["regular"]`, "only_these", `[]`,
		"SELECT r.* FROM replays r",
	); err != nil {
		t.Fatalf("UpdateGlobalReplayFilterConfigRaw: %v", err)
	}
	cfg, err = s.GetGlobalReplayFilterConfigRaw(ctx, "global")
	if err != nil {
		t.Fatalf("GetGlobalReplayFilterConfigRaw: %v", err)
	}
	if cfg.ExcludeShortGames || cfg.ExcludeComputers {
		t.Errorf("expected exclusions off after update, got %+v", cfg)
	}
	if cfg.CompiledReplaysFilterSQL == nil || *cfg.CompiledReplaysFilterSQL != "SELECT r.* FROM replays r" {
		t.Errorf("compiled SQL = %v", cfg.CompiledReplaysFilterSQL)
	}
	if cfg.GameTypesJSON != `["melee"]` {
		t.Errorf("game types json = %q", cfg.GameTypesJSON)
	}
}

func TestPlayerAliasesRoundTrip(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	if err := s.UpsertPlayerAlias(ctx, "BoxeR", "boxer#123", "boxer#123", ptrI64(7), "manual"); err != nil {
		t.Fatalf("UpsertPlayerAlias: %v", err)
	}
	rows, err := s.ListPlayerAliases(ctx)
	if err != nil {
		t.Fatalf("ListPlayerAliases: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(rows))
	}
	if rows[0].CanonicalAlias != "BoxeR" || rows[0].Source != "manual" {
		t.Errorf("alias = %+v", rows[0])
	}
	if rows[0].AuroraID == nil || *rows[0].AuroraID != 7 {
		t.Errorf("aurora id = %v", rows[0].AuroraID)
	}

	// Upsert on the same conflict key (source, normalized tag, canonical alias)
	// updates the raw tag / aurora id in place. Only battleTagRaw changes.
	if err := s.UpsertPlayerAlias(ctx, "BoxeR", "BoxeR#123", "boxer#123", ptrI64(9), "manual"); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	rows, err = s.ListPlayerAliases(ctx)
	if err != nil {
		t.Fatalf("ListPlayerAliases: %v", err)
	}
	if len(rows) != 1 || rows[0].BattleTagRaw != "BoxeR#123" || *rows[0].AuroraID != 9 {
		t.Fatalf("after upsert = %+v", rows)
	}

	if err := s.DeletePlayerAliasByID(ctx, rows[0].ID); err != nil {
		t.Fatalf("DeletePlayerAliasByID: %v", err)
	}
	rows, err = s.ListPlayerAliases(ctx)
	if err != nil {
		t.Fatalf("ListPlayerAliases: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 aliases after delete, got %d", len(rows))
	}
}

func TestCountDistinctPlayers(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	fixtureBasic1v1(t, conn)

	total, err := s.CountDistinctPlayers(ctx)
	if err != nil {
		t.Fatalf("CountDistinctPlayers: %v", err)
	}
	// BoxeR + NaDa (observer excluded).
	if total != 2 {
		t.Errorf("distinct players = %v, want 2", total)
	}

	terran, err := s.CountDistinctPlayersByRace(ctx, "Terran")
	if err != nil {
		t.Fatalf("CountDistinctPlayersByRace: %v", err)
	}
	if terran != 1 {
		t.Errorf("terran players = %v, want 1", terran)
	}
}

func TestGetPlayerNameByKey(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	fixtureBasic1v1(t, conn)

	name, err := s.GetPlayerNameByKey(ctx, "boxer")
	if err != nil {
		t.Fatalf("GetPlayerNameByKey: %v", err)
	}
	if name != "BoxeR" {
		t.Errorf("name = %q", name)
	}
}

func TestListExpansionEvents(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	mustExec(t, conn, `
		INSERT INTO replay_events (replay_id, seconds_from_game_start, event_kind, event_type, source_player_id)
		VALUES (?, 300, 'game_event', 'expansion', ?)`, replayID, boxerID)
	// A non-expansion event and a NULL-source expansion should be ignored.
	seedGameEvent(t, conn, replayID, "cannon_rush", 120)

	rows, err := s.ListExpansionEvents(ctx)
	if err != nil {
		t.Fatalf("ListExpansionEvents: %v", err)
	}
	if len(rows) != 1 || rows[0].Second != 300 || rows[0].PlayerID == nil || *rows[0].PlayerID != boxerID {
		t.Fatalf("expansion events = %+v", rows)
	}
}

func TestListGasTimingRows(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedCommand(t, conn, replayID, boxerID, 90, "Build", ptrStr("Refinery"))
	// A non-gas Build should be excluded.
	seedCommand(t, conn, replayID, boxerID, 30, "Build", ptrStr("Barracks"))

	rows, err := s.ListGasTimingRows(ctx, replayID)
	if err != nil {
		t.Fatalf("ListGasTimingRows: %v", err)
	}
	if len(rows) != 1 || rows[0].Second != 90 {
		t.Fatalf("gas rows = %+v", rows)
	}
}

func TestLoadEarlyZergTimings(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()

	replayID := seedReplay(t, conn, replayFixture{
		filePath: "/r/zvz.rep", checksum: "zvzchk", fileName: "zvz.rep",
		replayDate: "2024-06-01T10:00:00Z", mapName: "Python",
		durationSeconds: 900, gameType: "Melee", mapKind: "Regular",
		teamFormat: "1v1", matchup: "ZvZ",
	})
	zergID := seedPlayer(t, conn, playerFixture{
		replayID: replayID, name: "Jaedong", race: "Zerg", team: 1, apm: 350, eapm: 280,
	})

	seedCommand(t, conn, replayID, zergID, 20, "Unit Morph", ptrStr("Drone"))
	seedCommand(t, conn, replayID, zergID, 40, "Unit Morph", ptrStr("Drone"))
	seedCommand(t, conn, replayID, zergID, 60, "Unit Morph", ptrStr("Overlord"))
	seedCommand(t, conn, replayID, zergID, 100, "Build", ptrStr("Spawning Pool"))
	seedCommand(t, conn, replayID, zergID, 200, "Build", ptrStr("Hatchery"))
	// Beyond the 600s early window: must be excluded by the query.
	seedCommand(t, conn, replayID, zergID, 700, "Unit Morph", ptrStr("Drone"))

	rows, err := s.LoadEarlyZergTimings(ctx, replayID)
	if err != nil {
		t.Fatalf("LoadEarlyZergTimings: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 zerg row, got %d", len(rows))
	}
	r := rows[0]
	if len(r.DroneMorphSecs) != 2 || r.DroneMorphSecs[0] != 20 || r.DroneMorphSecs[1] != 40 {
		t.Errorf("drone morphs = %v", r.DroneMorphSecs)
	}
	if r.FirstOverlordSec == nil || *r.FirstOverlordSec != 60 {
		t.Errorf("first overlord = %v", r.FirstOverlordSec)
	}
	if r.FirstPoolSec == nil || *r.FirstPoolSec != 100 {
		t.Errorf("first pool = %v", r.FirstPoolSec)
	}
	if r.FirstHatcherySec == nil || *r.FirstHatcherySec != 200 {
		t.Errorf("first hatchery = %v", r.FirstHatcherySec)
	}
}

func TestListGameUnitCadenceRows(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()

	replayID := seedReplay(t, conn, replayFixture{
		filePath: "/r/cad.rep", checksum: "cadchk", fileName: "cad.rep",
		replayDate: "2024-06-01T10:00:00Z", mapName: "Python",
		durationSeconds: 600, gameType: "Melee", mapKind: "Regular",
		teamFormat: "1v1", matchup: "TvZ",
	})
	pID := seedPlayer(t, conn, playerFixture{replayID: replayID, name: "Flash", race: "Terran", team: 1, apm: 400, eapm: 300})

	// Evenly spaced Marine production across the full game.
	for sec := int64(30); sec <= 300; sec += 30 {
		seedCommand(t, conn, replayID, pID, sec, "Train", ptrStr("Marine"))
	}

	rows, err := s.ListGameUnitCadenceRows(ctx, replayID, 600, []string{"SCV"}, 0, 1.0, 20)
	if err != nil {
		t.Fatalf("ListGameUnitCadenceRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 cadence row, got %d", len(rows))
	}
	r := rows[0]
	if r.PlayerID != pID || r.UnitsProduced != 10 || r.WindowSeconds != 600 {
		t.Errorf("cadence row = %+v", r)
	}
	// Perfectly even cadence => very low CV / burstiness.
	if r.CVGap.Valid && r.CVGap.Float64 > 0.01 {
		t.Errorf("expected near-zero CV gap, got %v", r.CVGap.Float64)
	}
}

func TestListUnitSliceCommandRowsExcludesNonProduction(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, boxerID, _ := fixtureBasic1v1(t, conn)

	seedCommand(t, conn, replayID, boxerID, 50, "Train", ptrStr("Marine"))
	seedCommand(t, conn, replayID, boxerID, 60, "Build", ptrStr("Barracks"))
	// Select is not a production action; excluded.
	seedCommand(t, conn, replayID, boxerID, 70, "Select", nil)

	rows, err := s.ListUnitSliceCommandRows(ctx, replayID)
	if err != nil {
		t.Fatalf("ListUnitSliceCommandRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 production rows, got %d (%+v)", len(rows), rows)
	}
}

func TestWorkflowGamesListWithWhere(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	fixtureBasic1v1(t, conn)

	// Second replay: a Money-map game.
	rid2 := seedReplay(t, conn, replayFixture{
		filePath: "/r/g2.rep", checksum: "chk2", fileName: "g2.rep",
		replayDate: "2024-06-02T10:00:00Z", mapName: "Big Game Hunters",
		durationSeconds: 1800, gameType: "Melee", mapKind: "Money",
		teamFormat: "1v1", matchup: "TvZ",
	})
	seedPlayer(t, conn, playerFixture{replayID: rid2, name: "Flash", race: "Terran", team: 1, apm: 400, eapm: 300, isWinner: true})
	seedPlayer(t, conn, playerFixture{replayID: rid2, name: "Jaedong", race: "Zerg", team: 2, apm: 350, eapm: 280})

	whereSQL, args := BuildWorkflowGamesListWhere(nil, nil, nil, nil, nil, []string{"money"}, WorkflowDurationSQLByKey())

	total, err := s.CountGamesWithWhere(ctx, whereSQL, args)
	if err != nil {
		t.Fatalf("CountGamesWithWhere: %v", err)
	}
	if total != 1 {
		t.Errorf("money-map game count = %d, want 1", total)
	}

	games, err := s.ListGamesWithWhere(ctx, whereSQL, args, 10, 0)
	if err != nil {
		t.Fatalf("ListGamesWithWhere: %v", err)
	}
	if len(games) != 1 || games[0].MapName != "Big Game Hunters" || games[0].MapKind != "Money" {
		t.Fatalf("games = %+v", games)
	}
}

func TestListReplayPlayers(t *testing.T) {
	s, conn := newTestStore(t)
	ctx := context.Background()
	replayID, _, _ := fixtureBasic1v1(t, conn)

	players, err := s.ListReplayPlayers(ctx, []int64{replayID})
	if err != nil {
		t.Fatalf("ListReplayPlayers: %v", err)
	}
	if len(players) == 0 {
		t.Fatalf("expected players, got none")
	}
	names := map[string]bool{}
	for _, p := range players {
		names[p.Name] = true
	}
	if !names["BoxeR"] || !names["NaDa"] {
		t.Errorf("expected BoxeR and NaDa, got %v", names)
	}
}

func TestListReplayPlayersEmptyIDs(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	players, err := s.ListReplayPlayers(ctx, nil)
	if err != nil {
		t.Fatalf("ListReplayPlayers(nil): %v", err)
	}
	if len(players) != 0 {
		t.Errorf("expected no players for empty id list, got %d", len(players))
	}
}

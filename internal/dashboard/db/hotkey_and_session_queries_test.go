package db

import (
	"context"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/hotkeystream"
)

func seedHotkeyCorpus(t *testing.T) (*Store, int64, int64, int64) {
	t.Helper()
	store, conn := newTestStore(t)
	replayID := seedReplay(t, conn, replayFixture{
		filePath: "/replays/a.rep", checksum: "a", fileName: "a.rep",
		replayDate: "2026-09-01T10:00:00Z", mapName: "Fighting Spirit",
		durationSeconds: 900, gameType: "TopVsBottom", mapKind: "Regular",
		teamFormat: "1v1", matchup: "TvZ",
	})
	blob := hotkeystream.Encode([]hotkeystream.Event{
		{Sec: 5, Type: hotkeystream.TypeAssignBuilding, Group: 1, Building: hotkeystream.BuildingID("Command Center"), TileX: 60, TileY: 30},
		{Sec: 8, Type: hotkeystream.TypeSelect, Group: 1},
		{Sec: 20, Type: hotkeystream.TypeAssignUnits, Group: 2, Count: 6},
	})
	p1 := seedPlayer(t, conn, playerFixture{replayID: replayID, name: "KeyedPlayer", race: "Terran", team: 1, apm: 200, eapm: 150})
	p2 := seedPlayer(t, conn, playerFixture{replayID: replayID, name: "BareHands", race: "Zerg", team: 2, apm: 180, eapm: 140})
	mustExec(t, conn, `UPDATE players SET hotkey_stream = ? WHERE id = ?`, blob, p1)
	return store, replayID, p1, p2
}

func TestListReplayPlayerHotkeyStreams(t *testing.T) {
	store, replayID, p1, p2 := seedHotkeyCorpus(t)
	rows, err := store.ListReplayPlayerHotkeyStreams(context.Background(), replayID)
	if err != nil {
		t.Fatalf("ListReplayPlayerHotkeyStreams: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].PlayerID != p1 || rows[0].Name != "KeyedPlayer" || len(rows[0].HotkeyStream) == 0 {
		t.Fatalf("row 0 wrong: %+v", rows[0])
	}
	if rows[1].PlayerID != p2 || rows[1].HotkeyStream != nil {
		t.Fatalf("row 1 must have a NULL stream: %+v", rows[1])
	}
}

func TestListPlayerHotkeyStreamsByKey(t *testing.T) {
	store, _, _, _ := seedHotkeyCorpus(t)
	rows, err := store.ListPlayerHotkeyStreamsByKey(context.Background(), "keyedplayer")
	if err != nil {
		t.Fatalf("ListPlayerHotkeyStreamsByKey: %v", err)
	}
	if len(rows) != 1 || rows[0].Race != "Terran" || rows[0].Name != "KeyedPlayer" || rows[0].DurationSeconds != 900 {
		t.Fatalf("rows wrong: %+v", rows)
	}
	// NULL streams and unknown keys yield nothing.
	if rows, _ := store.ListPlayerHotkeyStreamsByKey(context.Background(), "barehands"); len(rows) != 0 {
		t.Fatalf("player without streams must yield no rows, got %+v", rows)
	}
}

func TestGetReplayPlayerHotkeyStream(t *testing.T) {
	store, replayID, p1, _ := seedHotkeyCorpus(t)
	row, err := store.GetReplayPlayerHotkeyStream(context.Background(), replayID, p1)
	if err != nil {
		t.Fatalf("GetReplayPlayerHotkeyStream: %v", err)
	}
	if row.Name != "KeyedPlayer" || len(row.HotkeyStream) == 0 {
		t.Fatalf("row wrong: %+v", row)
	}
	if _, err := store.GetReplayPlayerHotkeyStream(context.Background(), replayID, 99999); err == nil {
		t.Fatal("expected error for unknown player")
	}
}

func TestGamingSessionQueriesSeeded(t *testing.T) {
	store, conn := newTestStore(t)
	ctx := context.Background()
	r1 := seedReplay(t, conn, replayFixture{
		filePath: "/autosave/x.rep", checksum: "x", fileName: "x.rep",
		replayDate: "2026-09-02T20:00:00Z", mapName: "Polypoid",
		durationSeconds: 700, gameType: "Melee", mapKind: "Regular",
		teamFormat: "1v1", matchup: "PvT",
	})
	r2 := seedReplay(t, conn, replayFixture{
		filePath: "/autosave/y.rep", checksum: "y", fileName: "y.rep",
		replayDate: "2026-09-02T20:30:00Z", mapName: "Polypoid",
		durationSeconds: 800, gameType: "Melee", mapKind: "Regular",
		teamFormat: "1v1", matchup: "PvT",
	})
	seedPlayer(t, conn, playerFixture{replayID: r1, name: "SessionGuy", race: "Protoss", team: 1, apm: 150, eapm: 120})
	seedPlayer(t, conn, playerFixture{replayID: r2, name: "SessionGuy", race: "Protoss", team: 1, apm: 160, eapm: 130})
	seedPlayer(t, conn, playerFixture{replayID: r2, name: "OtherGuy", race: "Terran", team: 2, apm: 90, eapm: 70})

	recent, err := store.ListRecentAutosaveGamesForPlayers(ctx, []string{"sessionguy"}, 10)
	if err != nil {
		t.Fatalf("ListRecentAutosaveGamesForPlayers: %v", err)
	}
	if len(recent) == 0 {
		t.Fatal("expected recent autosave rows")
	}

	rows, err := store.ListReplaysByIDs(ctx, []int64{r1, r2})
	if err != nil {
		t.Fatalf("ListReplaysByIDs: %v", err)
	}
	if len(rows) != 2 || rows[0].MapName != "Polypoid" {
		t.Fatalf("session replay rows wrong: %+v", rows)
	}
	if empty, err := store.ListReplaysByIDs(ctx, nil); err != nil || len(empty) != 0 {
		t.Fatalf("nil ids: %v %v", empty, err)
	}

	apm, err := store.ListPlayerAPMByReplayIDs(ctx, []int64{r1, r2})
	if err != nil {
		t.Fatalf("ListPlayerAPMByReplayIDs: %v", err)
	}
	if len(apm) != 3 {
		t.Fatalf("got %d apm rows, want 3", len(apm))
	}
	if empty, err := store.ListPlayerAPMByReplayIDs(ctx, nil); err != nil || len(empty) != 0 {
		t.Fatalf("nil ids: %v %v", empty, err)
	}
}

func TestWorkflowFilterCounts(t *testing.T) {
	store, conn := newTestStore(t)
	ctx := context.Background()
	r1 := seedReplay(t, conn, replayFixture{
		filePath: "/r/1.rep", checksum: "1", fileName: "1.rep",
		replayDate: "2026-08-01T10:00:00Z", mapName: "FS",
		durationSeconds: 900, gameType: "Melee", mapKind: "Regular",
		teamFormat: "1v1", matchup: "TvZ",
	})
	r2 := seedReplay(t, conn, replayFixture{
		filePath: "/r/2.rep", checksum: "2", fileName: "2.rep",
		replayDate: "2026-08-02T10:00:00Z", mapName: "BGH",
		durationSeconds: 1200, gameType: "Melee", mapKind: "Money",
		teamFormat: "4v4", matchup: "", teamStacking: true,
	})
	p1 := seedPlayer(t, conn, playerFixture{replayID: r1, name: "A", race: "Terran", team: 1, apm: 100, eapm: 80})
	seedMarker(t, conn, r1, &p1, "never_used_hotkeys", 30, nil)
	seedGameEvent(t, conn, r1, "zergling_rush", 200)
	seedGameEvent(t, conn, r2, "drop", 400)

	matchups, err := store.CountWorkflowMatchupGames(ctx)
	if err != nil {
		t.Fatalf("CountWorkflowMatchupGames: %v", err)
	}
	if matchups["tvz"] != 1 {
		t.Fatalf("matchup counts wrong: %v", matchups)
	}

	mapKinds, err := store.CountWorkflowMapKindGames(ctx)
	if err != nil {
		t.Fatalf("CountWorkflowMapKindGames: %v", err)
	}
	if mapKinds["regular"] != 1 || mapKinds["money"] != 1 {
		t.Fatalf("map kind counts wrong: %v", mapKinds)
	}

	counts, err := store.CountWorkflowFeaturingGames(ctx, []string{
		"zergling_rush", "drop", "team_stacking", "never_used_hotkeys", "not_a_real_feature",
	})
	if err != nil {
		t.Fatalf("CountWorkflowFeaturingGames: %v", err)
	}
	if counts["zergling_rush"] != 1 {
		t.Fatalf("zergling_rush count wrong: %v", counts)
	}
	if counts["drop"] != 1 {
		t.Fatalf("drop count wrong: %v", counts)
	}
	if counts["team_stacking"] != 1 {
		t.Fatalf("team_stacking count wrong: %v", counts)
	}
	if counts["never_used_hotkeys"] != 1 {
		t.Fatalf("never_used_hotkeys count wrong: %v", counts)
	}
	if _, ok := counts["not_a_real_feature"]; ok {
		t.Fatalf("unknown feature must be skipped: %v", counts)
	}

	if empty, err := store.CountWorkflowFeaturingGames(ctx, nil); err != nil || len(empty) != 0 {
		t.Fatalf("empty keys: %v %v", empty, err)
	}
}

func TestBnetProfileKeyLookups(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if codes, err := store.GetBnetCountryCodesByPlayerKeys(ctx, nil); err != nil || len(codes) != 0 {
		t.Fatalf("empty keys: %v %v", codes, err)
	}
	if rows, err := store.ListBnetProfilePayloadsByPlayerKeys(ctx, nil); err != nil || len(rows) != 0 {
		t.Fatalf("empty keys: %v %v", rows, err)
	}

	seed := func(toon, country, payload string, found bool) {
		t.Helper()
		if err := store.UpsertBnetProfile(ctx, BnetProfileRow{
			Toon: toon, Gateway: 30, Found: found, AuroraID: 1,
			BattleTag: toon + "#1", CountryCode: country, Payload: payload,
			FetchedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("seed %s: %v", toon, err)
		}
	}
	seed("Keyed", "KR", `{"id":1}`, true)
	seed("Ghosted", "", `{"id":2}`, false)

	codes, err := store.GetBnetCountryCodesByPlayerKeys(ctx, []string{"keyed", "ghosted", "missing"})
	if err != nil {
		t.Fatalf("GetBnetCountryCodesByPlayerKeys: %v", err)
	}
	if codes["keyed"] != "KR" {
		t.Fatalf("country codes wrong: %v", codes)
	}

	rows, err := store.ListBnetProfilePayloadsByPlayerKeys(ctx, []string{"keyed", "ghosted"})
	if err != nil {
		t.Fatalf("ListBnetProfilePayloadsByPlayerKeys: %v", err)
	}
	if len(rows) != 1 || rows[0].Toon != "Keyed" || rows[0].Payload != `{"id":1}` {
		t.Fatalf("payload rows wrong (found=0 must be excluded): %+v", rows)
	}
}

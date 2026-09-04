package db

import (
	"context"
	"reflect"
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func TestLibStoreListRecentAutosaveGamesForPlayers(t *testing.T) {
	autosave := librarytest.Replay(
		librarytest.WithDate(librarytest.BaseDate),
		librarytest.WithPath("/sc/Maps/Replays/Autosave/LastReplay.rep", librarytest.BaseDate),
		librarytest.WithPlayer("Flash", librarytest.Team(1)),
		librarytest.WithPlayer("Watcher", librarytest.Team(2), librarytest.Observer()),
	)
	autosave.Flags |= library.FlagIsAutosave
	downloaded := melee("downloaded")
	s := newTestLibStore(t, autosave, downloaded)

	rows, err := s.ListRecentAutosaveGamesForPlayers(context.Background(), []string{"flash"}, 300)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (both games where Flash played): %+v", len(rows), rows)
	}
	if rows[0].ReplayID != autosave.ID || rows[0].FilePath != "/sc/Maps/Replays/Autosave/LastReplay.rep" {
		t.Fatalf("row 0 = %+v, want the newest game with its Autosave path", rows[0])
	}
	if rows[0].PlayerKey != "flash" || rows[0].PlayerName != "Flash" {
		t.Fatalf("row 0 identity = %+v", rows[0])
	}
	if rows[1].ReplayID != downloaded.ID {
		t.Fatalf("rows must be newest first: %+v", rows)
	}

	// Observers never count as having played.
	if rows, _ := s.ListRecentAutosaveGamesForPlayers(context.Background(), []string{"watcher"}, 300); len(rows) != 0 {
		t.Fatalf("observer rows = %+v", rows)
	}
	if rows, _ := s.ListRecentAutosaveGamesForPlayers(context.Background(), nil, 300); len(rows) != 0 {
		t.Fatalf("no keys = %+v", rows)
	}
	if rows, _ := s.ListRecentAutosaveGamesForPlayers(context.Background(), []string{"flash"}, 0); len(rows) != 0 {
		t.Fatalf("zero limit = %+v", rows)
	}
}

func TestLibStoreListRecentAutosaveGamesLimitsRows(t *testing.T) {
	replays := make([]*library.Replay, 0, 5)
	for i := 0; i < 5; i++ {
		replays = append(replays, melee("game"))
	}
	s := newTestLibStore(t, replays...)

	// The limit bounds joined player rows, not replays: both Flash and Bisu
	// match, so three rows stop inside the second game.
	rows, err := s.ListRecentAutosaveGamesForPlayers(context.Background(), []string{"flash", "BISU"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	if rows[0].PlayerKey != "flash" || rows[1].PlayerKey != "bisu" || rows[0].ReplayID != rows[1].ReplayID {
		t.Fatalf("rows = %+v, want both slots of the newest game first", rows)
	}
}

func TestLibStoreListReplaysByIDs(t *testing.T) {
	older := melee("older", librarytest.WithMap("Polypoid"), librarytest.WithMatchup("TvP"))
	newer := melee("newer", librarytest.WithDate(librarytest.BaseDate))
	short := melee("short", librarytest.WithDuration(30))
	s := newTestLibStore(t, older, newer, short)

	rows, err := s.ListReplaysByIDs(context.Background(), []int64{older.ID, short.ID, newer.ID, 999})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (filtered and unknown ids dropped): %+v", len(rows), rows)
	}
	if rows[0].ReplayID != newer.ID || rows[1].ReplayID != older.ID {
		t.Fatal("rows must be replay-date descending regardless of the id order asked for")
	}
	if rows[1].MapName != "Polypoid" || rows[1].Matchup != "TvP" || rows[1].DurationSeconds != 900 {
		t.Fatalf("row = %+v", rows[1])
	}
	if rows[1].MapKind != "Regular" || rows[1].GameType != "Melee" || rows[1].FileName == "" {
		t.Fatalf("row = %+v", rows[1])
	}
	if rows, _ := s.ListReplaysByIDs(context.Background(), nil); len(rows) != 0 {
		t.Fatalf("no ids = %+v", rows)
	}
}

func TestLibStoreListPlayerAPMByReplayIDs(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.APM(200, 150)),
		librarytest.WithPlayer("Bisu", librarytest.Team(2), librarytest.APM(180, 120)),
		librarytest.WithPlayer("Watcher", librarytest.Team(3), librarytest.Observer()),
	)
	s := newTestLibStore(t, r)

	rows, err := s.ListPlayerAPMByReplayIDs(context.Background(), []int64{r.ID, r.ID, 999})
	if err != nil {
		t.Fatal(err)
	}
	want := []SessionPlayerAPMRow{
		{ReplayID: r.ID, PlayerKey: "flash", APM: 200, EAPM: 150},
		{ReplayID: r.ID, PlayerKey: "bisu", APM: 180, EAPM: 120},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("apm rows = %+v, want %+v (observers and repeated ids dropped)", rows, want)
	}
	if rows, _ := s.ListPlayerAPMByReplayIDs(context.Background(), nil); len(rows) != 0 {
		t.Fatalf("no ids = %+v", rows)
	}
}

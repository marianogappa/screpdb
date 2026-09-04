package db

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func TestLibStoreListReplayPlayerHotkeyStreams(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Second", librarytest.Team(2), librarytest.Race(library.RaceZerg)),
		librarytest.WithPlayer("First", librarytest.Team(1), librarytest.Race(library.RaceTerran)),
		librarytest.WithPlayer("Watcher", librarytest.Team(1), librarytest.Observer()),
		librarytest.WithPlayer("Rescue", librarytest.Team(1), librarytest.Type(library.PlayerTypeRescuePassive)),
		librarytest.WithHotkeyStream(1, []byte{1, 2, 3}),
	)
	s := newTestLibStore(t, r)

	rows, err := s.ListReplayPlayerHotkeyStreams(context.Background(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (observer and non-human excluded): %+v", len(rows), rows)
	}
	if rows[0].Name != "First" || rows[0].Team != 1 || rows[1].Name != "Second" || rows[1].Team != 2 {
		t.Fatalf("rows must be team ascending: %+v", rows)
	}
	if rows[0].PlayerID != r.PlayerID(1) || rows[0].Race != "Terran" {
		t.Fatalf("row 0 = %+v", rows[0])
	}
	if !bytes.Equal(rows[0].HotkeyStream, []byte{1, 2, 3}) {
		t.Fatalf("stream = %v, want the blob shipped through unchanged", rows[0].HotkeyStream)
	}
	if rows[1].HotkeyStream != nil {
		t.Fatalf("a player with no stream must carry nil, got %v", rows[1].HotkeyStream)
	}

	// A missing or filtered-out replay is an empty list, as the SQL was.
	if rows, err := s.ListReplayPlayerHotkeyStreams(context.Background(), 999); err != nil || len(rows) != 0 {
		t.Fatalf("unknown replay = %+v, %v", rows, err)
	}
}

func TestLibStoreListPlayerHotkeyStreamsByKey(t *testing.T) {
	withStream := func(opts ...librarytest.Option) *library.Replay {
		base := []librarytest.Option{
			librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceTerran)),
			librarytest.WithPlayer("Bisu", librarytest.Team(2)),
			librarytest.WithHotkeyStream(0, []byte{9}),
		}
		return librarytest.Replay(append(base, opts...)...)
	}
	newest := withStream(librarytest.WithDate(librarytest.BaseDate), librarytest.WithDuration(600))
	older := withStream(librarytest.WithDuration(400))
	tooShort := withStream(librarytest.WithDuration(359))
	noStream := melee("no stream", librarytest.WithDuration(900))
	observed := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Observer()),
		librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		librarytest.WithHotkeyStream(0, []byte{9}),
	)
	s := newTestLibStore(t, newest, older, tooShort, noStream, observed)

	rows, err := s.ListPlayerHotkeyStreamsByKey(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (short game, streamless game and observer slot dropped): %+v", len(rows), rows)
	}
	if rows[0].DurationSeconds != 600 || rows[1].DurationSeconds != 400 {
		t.Fatalf("rows must be replay-date descending: %+v", rows)
	}
	if rows[0].Name != "Flash" || rows[0].Race != "Terran" || !bytes.Equal(rows[0].HotkeyStream, []byte{9}) {
		t.Fatalf("row 0 = %+v", rows[0])
	}
	if rows, _ := s.ListPlayerHotkeyStreamsByKey(context.Background(), "nobody"); len(rows) != 0 {
		t.Fatalf("unknown key = %+v", rows)
	}
}

func TestLibStoreListPlayerHotkeyStreamsByKeyCaps(t *testing.T) {
	replays := make([]*library.Replay, 0, 130)
	for i := 0; i < 130; i++ {
		replays = append(replays, librarytest.Replay(
			librarytest.WithPlayer("Flash", librarytest.Team(1)),
			librarytest.WithPlayer("Bisu", librarytest.Team(2)),
			librarytest.WithHotkeyStream(0, []byte{1}),
		))
	}
	s := newTestLibStore(t, replays...)

	rows, err := s.ListPlayerHotkeyStreamsByKey(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 120 {
		t.Fatalf("got %d rows, want the LIMIT 120", len(rows))
	}
}

func TestLibStoreGetReplayPlayerHotkeyStream(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceTerran)),
		librarytest.WithPlayer("Watcher", librarytest.Team(2), librarytest.Observer()),
		librarytest.WithHotkeyStream(0, []byte{4, 5}),
	)
	s := newTestLibStore(t, r)

	got, err := s.GetReplayPlayerHotkeyStream(context.Background(), r.ID, r.PlayerID(0))
	if err != nil {
		t.Fatal(err)
	}
	if got.PlayerID != r.PlayerID(0) || got.Name != "Flash" || got.Race != "Terran" || got.SlotID != 0 {
		t.Fatalf("row = %+v", got)
	}
	if !bytes.Equal(got.HotkeyStream, []byte{4, 5}) {
		t.Fatalf("stream = %v", got.HotkeyStream)
	}

	// No observer or human filter: the SQL matched on ids alone.
	observer, err := s.GetReplayPlayerHotkeyStream(context.Background(), r.ID, r.PlayerID(1))
	if err != nil || observer.Name != "Watcher" {
		t.Fatalf("observer lookup = %+v, %v", observer, err)
	}

	if _, err := s.GetReplayPlayerHotkeyStream(context.Background(), 999, r.PlayerID(0)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown replay err = %v", err)
	}
	if _, err := s.GetReplayPlayerHotkeyStream(context.Background(), r.ID, r.PlayerID(9)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown ordinal err = %v", err)
	}
	if _, err := s.GetReplayPlayerHotkeyStream(context.Background(), r.ID, library.PlayerID(r.ID+1, 0)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("player id from another replay err = %v", err)
	}
}

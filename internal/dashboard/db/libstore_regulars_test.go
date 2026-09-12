package db

import (
	"context"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func TestLibStoreListCoPlayerCounts(t *testing.T) {
	old := librarytest.BaseDate.Add(-365 * 24 * time.Hour)
	withFoe := func(title string, when time.Time, opts ...librarytest.Option) *library.Replay {
		base := []librarytest.Option{
			librarytest.WithTitle(title),
			librarytest.WithDate(when),
			librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Type(library.PlayerTypeHuman)),
			librarytest.WithPlayer("Bisu", librarytest.Team(2), librarytest.Type(library.PlayerTypeHuman)),
		}
		return librarytest.Replay(append(base, opts...)...)
	}
	inWindow := withFoe("recent", librarytest.BaseDate)
	alsoInWindow := withFoe("recent2", librarytest.BaseDate.Add(-time.Hour))
	tooOld := withFoe("old", old)
	// A game the user was not in says nothing about who they play with.
	notOurs := librarytest.Replay(
		librarytest.WithTitle("strangers"),
		librarytest.WithDate(librarytest.BaseDate),
		librarytest.WithPlayer("Stork", librarytest.Team(1), librarytest.Type(library.PlayerTypeHuman)),
		librarytest.WithPlayer("Jaedong", librarytest.Team(2), librarytest.Type(library.PlayerTypeHuman)),
	)
	withComputer := librarytest.Replay(
		librarytest.WithTitle("vs ai"),
		librarytest.WithDate(librarytest.BaseDate),
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Type(library.PlayerTypeHuman)),
		librarytest.WithPlayer("Bot", librarytest.Team(2), librarytest.Computer()),
		librarytest.WithPlayer("Watcher", librarytest.Team(3), librarytest.Observer()),
	)
	s := newTestLibStore(t, inWindow, alsoInWindow, tooOld, notOurs, withComputer)

	since := librarytest.BaseDate.Add(-30 * 24 * time.Hour)
	rows, err := s.ListCoPlayerCounts(context.Background(), []string{"flash"}, since)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]CoPlayerRow{}
	for _, row := range rows {
		byKey[row.PlayerKey] = row
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want only Bisu (computers, observers and games without us are excluded)", rows)
	}
	bisu := byKey["bisu"]
	if bisu.Games != 2 {
		t.Errorf("games = %d, want the 2 inside the window", bisu.Games)
	}
	if !bisu.LastPlayed.Equal(librarytest.BaseDate) {
		t.Errorf("lastPlayed = %v, want the newest game", bisu.LastPlayed)
	}
	// The user never appears in their own list.
	if _, ours := byKey["flash"]; ours {
		t.Error("the user must not be one of their own co-players")
	}

	if rows, _ := s.ListCoPlayerCounts(context.Background(), nil, since); len(rows) != 0 {
		t.Errorf("no keys = %+v", rows)
	}
}

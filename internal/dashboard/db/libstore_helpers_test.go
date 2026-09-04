package db

import (
	"errors"
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func newTestLibStore(t *testing.T, replays ...*library.Replay) *LibStore {
	t.Helper()
	lib := library.New(library.Options{})
	t.Cleanup(lib.Close)
	lib.Add(0, replays...)
	lib.Flush()
	return NewLibStore(lib, nil, nil)
}

func melee(name string, opts ...librarytest.Option) *library.Replay {
	base := []librarytest.Option{
		librarytest.WithTitle(name),
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceTerran), librarytest.Type(library.PlayerTypeHuman)),
		librarytest.WithPlayer("Bisu", librarytest.Team(2), librarytest.Race(library.RaceProtoss), librarytest.Type(library.PlayerTypeHuman)),
	}
	return librarytest.Replay(append(base, opts...)...)
}

func TestLibStoreReplayHonoursTheFilter(t *testing.T) {
	long := melee("long")
	short := melee("short", librarytest.WithDuration(30))
	s := newTestLibStore(t, long, short)

	got, err := s.replay(long.ID)
	if err != nil || got != long {
		t.Fatalf("replay(long) = %v, %v", got, err)
	}
	if _, err := s.replay(short.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("filtered-out replay must be ErrNotFound, got %v", err)
	}
	if _, err := s.replay(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown replay must be ErrNotFound, got %v", err)
	}
}

func TestLibStoreReplaysByIDsKeepsInputOrderAndSkipsMisses(t *testing.T) {
	first := melee("first")
	second := melee("second")
	short := melee("short", librarytest.WithDuration(30))
	s := newTestLibStore(t, first, second, short)

	got := s.replaysByIDs([]int64{second.ID, 12345, short.ID, first.ID})
	if len(got) != 2 || got[0] != second || got[1] != first {
		t.Fatalf("replaysByIDs = %v, want [second first]", got)
	}
	if got := s.replaysByIDs(nil); len(got) != 0 {
		t.Fatalf("replaysByIDs(nil) = %v", got)
	}
}

func TestLibStorePlayerGamesIsFilteredAndNewestFirst(t *testing.T) {
	older := melee("older")
	newer := melee("newer", librarytest.WithDate(librarytest.BaseDate))
	short := melee("short", librarytest.WithDuration(30))
	s := newTestLibStore(t, older, newer, short)

	games := s.playerGames("  FLASH ")
	if len(games) != 2 {
		t.Fatalf("playerGames = %d refs, want 2", len(games))
	}
	if games[0].Replay != newer || games[1].Replay != older {
		t.Fatal("playerGames must be newest first")
	}
	if games[0].Player().Name != "Flash" {
		t.Fatalf("ref resolves to %q", games[0].Player().Name)
	}
}

func TestLibStoreResolvePlayer(t *testing.T) {
	r := melee("game")
	short := melee("short", librarytest.WithDuration(30))
	s := newTestLibStore(t, r, short)

	got, ordinal, ok := s.resolvePlayer(rowPlayerID(r, 1))
	if !ok || got != r || ordinal != 1 {
		t.Fatalf("resolvePlayer = %v, %d, %v", got, ordinal, ok)
	}
	if _, _, ok := s.resolvePlayer(library.PlayerID(r.ID, 9)); ok {
		t.Fatal("out-of-range ordinal must not resolve")
	}
	if _, _, ok := s.resolvePlayer(rowPlayerID(short, 0)); ok {
		t.Fatal("filtered-out replay must not resolve")
	}
}

func TestOrdinalForPlayerID(t *testing.T) {
	r := melee("game")
	other := melee("other")

	if ordinal, ok := ordinalForPlayerID(r, rowPlayerID(r, 1)); !ok || ordinal != 1 {
		t.Fatalf("ordinalForPlayerID = %d, %v", ordinal, ok)
	}
	if _, ok := ordinalForPlayerID(r, rowPlayerID(other, 0)); ok {
		t.Fatal("a player id from another replay must not resolve")
	}
	if _, ok := ordinalForPlayerID(r, library.PlayerID(r.ID, 7)); ok {
		t.Fatal("out-of-range ordinal must not resolve")
	}
	if _, ok := ordinalForPlayerID(nil, 1); ok {
		t.Fatal("nil replay must not resolve")
	}
}

func TestMemoBuildsOncePerKeyAndKeepsTheType(t *testing.T) {
	s := newTestLibStore(t, melee("game"))
	view := s.view()

	builds := 0
	build := func() []string {
		builds++
		return []string{"a", "b"}
	}
	first := memo(view, "k", build)
	second := memo(view, "k", build)
	if builds != 1 {
		t.Fatalf("built %d times, want 1", builds)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("memo values = %v, %v", first, second)
	}
	if n := memo(view, "other", func() int { return 7 }); n != 7 {
		t.Fatalf("memo(other) = %d", n)
	}
	// A key already holding another type yields the zero value rather than
	// panicking, so one bad call cannot take a request down.
	if got := memo(view, "other", func() string { return "x" }); got != "" {
		t.Fatalf("type mismatch = %q, want zero", got)
	}
}

func TestNormalizeKeyAndHumanNonObserver(t *testing.T) {
	if got := normalizeKey("  FlaSh "); got != "flash" {
		t.Fatalf("normalizeKey = %q", got)
	}
	r := librarytest.Replay(
		librarytest.WithPlayer("Human", librarytest.Type(library.PlayerTypeHuman)),
		librarytest.WithPlayer("Obs", librarytest.Type(library.PlayerTypeHuman), librarytest.Observer()),
		librarytest.WithPlayer("Bot", librarytest.Computer()),
	)
	if !humanNonObserver(&r.Players[0]) {
		t.Fatal("human non-observer must pass")
	}
	if humanNonObserver(&r.Players[1]) {
		t.Fatal("observer must not pass")
	}
	if humanNonObserver(&r.Players[2]) {
		t.Fatal("computer must not pass")
	}
	if humanNonObserver(nil) {
		t.Fatal("nil player must not pass")
	}
}

func TestForEachProdFiltersByKindAndPlayer(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash"),
		librarytest.WithPlayer("Bisu"),
		librarytest.WithProdCount(0, 100, library.ProdTrain, "Terran Marine", 2),
		librarytest.WithProd(0, 50, library.ProdTrain, "Terran Marine"),
		librarytest.WithProd(1, 70, library.ProdTrain, "Protoss Zealot"),
		librarytest.WithProd(0, 60, library.ProdBuild, "Terran Barracks"),
	)

	type hit struct {
		sec     uint16
		player  uint8
		subject string
		count   uint8
	}
	var got []hit
	collect := func(sec uint16, player uint8, subject uint16, count uint8) {
		got = append(got, hit{sec, player, library.Units.Name(subject), count})
	}

	forEachProd(r, library.ProdTrain, 0, collect)
	if len(got) != 2 || got[0].sec != 50 || got[1].sec != 100 || got[1].count != 2 {
		t.Fatalf("player 0 trains = %+v", got)
	}

	got = nil
	forEachProd(r, library.ProdTrain, library.NoPlayer, collect)
	if len(got) != 3 || got[1].player != 1 || got[1].subject != "Protoss Zealot" {
		t.Fatalf("all trains = %+v", got)
	}

	got = nil
	forEachProd(r, library.ProdUpgrade, library.NoPlayer, collect)
	if len(got) != 0 {
		t.Fatalf("upgrades = %+v", got)
	}
	forEachProd(nil, library.ProdTrain, library.NoPlayer, collect)
}

func TestMarkerLabelPrefersTheInternedLabel(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash"),
		librarytest.WithFuzzyLabel(0, 120, "~9 Overpool"),
		librarytest.WithMarker("bo_z_nhatch_muta", 0, 200, `{"label":"3 Hatch Muta"}`),
		librarytest.WithMarker("carriers", 0, 300, ""),
		librarytest.WithMarker("nukes", 0, 400, `not json`),
	)

	if label, ok := markerLabel(&r.Markers[0]); !ok || label != "~9 Overpool" {
		t.Fatalf("interned label = %q, %v", label, ok)
	}
	if label, ok := markerLabel(&r.Markers[1]); !ok || label != "3 Hatch Muta" {
		t.Fatalf("payload label = %q, %v", label, ok)
	}
	if _, ok := markerLabel(&r.Markers[2]); ok {
		t.Fatal("a marker with no payload has no label")
	}
	if _, ok := markerLabel(&r.Markers[3]); ok {
		t.Fatal("an unparseable payload has no label")
	}
	if _, ok := markerLabel(nil); ok {
		t.Fatal("nil marker has no label")
	}
}

func TestLibStoreSnapshotIsUnfiltered(t *testing.T) {
	long := melee("long")
	short := melee("short", librarytest.WithDuration(30))
	s := newTestLibStore(t, long, short)

	if got := s.snapshot().Len(); got != 2 {
		t.Fatalf("snapshot has %d replays, want both", got)
	}
	if got := s.view().Len(); got != 1 {
		t.Fatalf("view has %d replays, want the long one only", got)
	}
}

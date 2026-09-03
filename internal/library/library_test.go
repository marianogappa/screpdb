package library_test

import (
	"crypto/sha256"
	"sort"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func newTestLibrary(t *testing.T, opts library.Options) *library.Library {
	t.Helper()
	lib := library.New(opts)
	t.Cleanup(lib.Close)
	return lib
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestCommitOrdersNewestFirstAndIndexes(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	d := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	older := librarytest.Replay(librarytest.WithDate(d.Add(-time.Hour)), librarytest.WithPlayer("Flash"), librarytest.WithPlayer("Bisu"))
	newer := librarytest.Replay(librarytest.WithDate(d), librarytest.WithPlayer("flash "), librarytest.WithPlayer("Jaedong"))
	sameDateLowID := librarytest.Replay(librarytest.WithDate(d), librarytest.WithID(10))
	sameDateHighID := librarytest.Replay(librarytest.WithDate(d), librarytest.WithID(20))

	lib.Add(0, older, newer, sameDateLowID, sameDateHighID)
	lib.Flush()

	snap := lib.Snapshot()
	if snap.Version != 1 || snap.Len() != 4 {
		t.Fatalf("version %d len %d", snap.Version, snap.Len())
	}
	if newer.ID <= 20 {
		t.Fatalf("checksum-derived id %d unexpectedly small", newer.ID)
	}
	wantOrder := []int64{newer.ID, sameDateHighID.ID, sameDateLowID.ID, older.ID}
	for i, r := range snap.Replays {
		if r.ID != wantOrder[i] {
			t.Fatalf("position %d: got %d want %d", i, r.ID, wantOrder[i])
		}
	}
	if r, ok := snap.ByID(older.ID); !ok || r != older {
		t.Fatal("ByID miss")
	}
	if r, ok := snap.ByChecksum(newer.Checksum); !ok || r != newer {
		t.Fatal("ByChecksum miss")
	}
	if r, ok := snap.ByPath(newer.Path()); !ok || r != newer {
		t.Fatal("ByPath miss")
	}
	if _, ok := snap.ByID(999_999); ok {
		t.Fatal("unexpected hit")
	}
	games := snap.PlayerGames("flash")
	if len(games) != 2 || games[0].Replay != newer || games[1].Replay != older {
		t.Fatalf("PlayerGames(flash) = %+v", games)
	}
	if games[0].ID() != library.PlayerID(newer.ID, 0) || games[0].Player().Name != "flash " {
		t.Fatal("PlayerRef accessors")
	}
	keys := snap.PlayerKeys()
	if !sort.StringsAreSorted(keys) || len(keys) != 3 {
		t.Fatalf("PlayerKeys = %v", keys)
	}
}

func TestDuplicateFilesCollapseIntoOneRecord(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	oldTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first := librarytest.Replay(librarytest.WithChecksum("same"), librarytest.WithPath("/r/a.rep", oldTime))
	second := librarytest.Replay(librarytest.WithChecksum("same"), librarytest.WithPath("/r/Autosave/b.rep", oldTime.Add(time.Hour)))

	_, events, cancel := lib.Subscribe()
	defer cancel()

	lib.Add(0, first)
	lib.Flush()
	lib.Add(0, second)
	lib.Flush()

	snap := lib.Snapshot()
	if snap.Len() != 1 {
		t.Fatalf("len = %d, want 1", snap.Len())
	}
	r := snap.Replays[0]
	if len(r.Paths) != 2 || r.Paths[0].Path != "/r/Autosave/b.rep" || r.Paths[1].Path != "/r/a.rep" {
		t.Fatalf("paths = %+v", r.Paths)
	}
	if !r.Flags.Has(library.FlagIsAutosave) {
		t.Fatal("autosave flag should be set once an autosave path is attached")
	}
	if first.Paths[0].Path != "/r/a.rep" || len(first.Paths) != 1 {
		t.Fatal("committed record was mutated instead of copied")
	}
	if _, ok := snap.ByPath("/r/a.rep"); !ok {
		t.Fatal("old path lost")
	}
	if _, ok := snap.ByPath("/r/Autosave/b.rep"); !ok {
		t.Fatal("new path missing")
	}

	var corpus []library.Event
	for len(corpus) < 2 {
		select {
		case ev := <-events:
			if ev.Kind == library.EventCorpus {
				corpus = append(corpus, ev)
			}
		case <-time.After(time.Second):
			t.Fatalf("expected two corpus events, got %d", len(corpus))
		}
	}
	if len(corpus[0].Added) != 1 || corpus[0].Added[0] != r.ID {
		t.Fatalf("first commit added %v", corpus[0].Added)
	}
	if len(corpus[1].Added) != 0 || len(corpus[1].Removed) != 0 {
		t.Fatalf("alias commit should not report id changes: %+v", corpus[1])
	}
}

func TestRemoveAndAlias(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r := librarytest.Replay(librarytest.WithChecksum("k"), librarytest.WithPath("/r/a.rep", ts))
	lib.Add(0, r)
	lib.Flush()

	lib.Alias(0, library.FileRef{Path: "/r/b.rep", ModTime: ts.Add(time.Minute)}, r.Checksum)
	lib.Alias(0, library.FileRef{Path: "/r/unknown.rep", ModTime: ts}, sha256.Sum256([]byte("nobody")))
	lib.Flush()
	snap := lib.Snapshot()
	if snap.Len() != 1 || len(snap.Replays[0].Paths) != 2 || snap.Replays[0].Path() != "/r/b.rep" {
		t.Fatalf("after alias: %+v", snap.Replays[0].Paths)
	}
	if _, ok := snap.ByPath("/r/unknown.rep"); ok {
		t.Fatal("alias for an unknown checksum must be ignored")
	}

	lib.Remove(0, "/r/b.rep")
	lib.Flush()
	snap = lib.Snapshot()
	if snap.Len() != 1 || snap.Replays[0].Path() != "/r/a.rep" {
		t.Fatalf("after partial remove: %+v", snap.Replays[0].Paths)
	}

	_, events, cancel := lib.Subscribe()
	defer cancel()
	lib.Remove(0, "/r/a.rep")
	lib.Remove(0, "/r/never-there.rep")
	lib.Flush()
	if lib.Snapshot().Len() != 0 {
		t.Fatal("record should vanish with its last path")
	}
	select {
	case ev := <-events:
		if ev.Kind != library.EventCorpus || len(ev.Removed) != 1 || ev.Removed[0] != r.ID {
			t.Fatalf("unexpected event %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no removal event")
	}
}

func TestReplacedFileMovesPathToNewRecord(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	old := librarytest.Replay(librarytest.WithChecksum("old"), librarytest.WithPath("/r/a.rep", ts))
	lib.Add(0, old)
	lib.Flush()
	replacement := librarytest.Replay(librarytest.WithChecksum("new"), librarytest.WithPath("/r/a.rep", ts.Add(time.Hour)))
	lib.Add(0, replacement)
	lib.Flush()
	snap := lib.Snapshot()
	if snap.Len() != 1 || snap.Replays[0] != replacement {
		t.Fatalf("expected only the replacement, got %d records", snap.Len())
	}
}

func TestResetDropsCorpusAndStaleGenerations(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	lib.Add(0, librarytest.Replay())
	lib.Flush()
	lib.Reset(7)
	lib.Add(0, librarytest.Replay())
	lib.Add(7, librarytest.Replay())
	lib.Flush()
	snap := lib.Snapshot()
	if snap.Generation != 7 || snap.Len() != 1 {
		t.Fatalf("generation %d len %d", snap.Generation, snap.Len())
	}
}

func TestStageAndPromote(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	live := librarytest.Replay()
	lib.Add(0, live)
	lib.Flush()

	lib.Stage(1)
	staged := librarytest.Replay()
	lib.Add(1, staged)
	lib.Flush()
	snap := lib.Snapshot()
	if snap.Generation != 0 || snap.Len() != 1 || snap.Replays[0] != live {
		t.Fatal("staged records must not be visible before Promote")
	}

	lib.Promote(2)
	lib.Flush()
	if lib.Snapshot().Generation != 0 {
		t.Fatal("promoting a generation that is not staged must be a no-op")
	}

	lib.Promote(1)
	lib.Flush()
	snap = lib.Snapshot()
	if snap.Generation != 1 || snap.Len() != 1 || snap.Replays[0] != staged {
		t.Fatalf("after promote: gen %d len %d", snap.Generation, snap.Len())
	}
}

func TestCoalescingByCountAndDelay(t *testing.T) {
	lib := newTestLibrary(t, library.Options{CoalesceRecords: 3, CoalesceDelay: 40 * time.Millisecond})
	lib.Add(0, librarytest.Replay())
	lib.Add(0, librarytest.Replay())
	time.Sleep(10 * time.Millisecond)
	if lib.Snapshot().Len() != 0 {
		t.Fatal("two records must not commit before the delay elapses")
	}
	waitFor(t, func() bool { return lib.Snapshot().Len() == 2 }, "delay-triggered commit")

	lib.Add(0, librarytest.Replay(), librarytest.Replay(), librarytest.Replay())
	waitFor(t, func() bool { return lib.Snapshot().Len() == 5 }, "count-triggered commit")
	if v := lib.Snapshot().Version; v != 2 {
		t.Fatalf("expected exactly two commits, got version %d", v)
	}
}

func TestChecksumCollisionProbesToNextID(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	var a, b [32]byte
	for i := range a {
		a[i] = byte(i)
		b[i] = byte(i)
	}
	b[31] ^= 0xFF
	first := librarytest.Replay(librarytest.WithRawChecksum(a))
	second := librarytest.Replay(librarytest.WithRawChecksum(b))
	if first.ID != second.ID {
		t.Fatal("fixture checksums should collide on their top 48 bits")
	}
	lib.Add(0, first, second)
	lib.Flush()
	if first.ID == second.ID || second.ID != first.ID+1 {
		t.Fatalf("expected linear probe: %d vs %d", first.ID, second.ID)
	}
	snap := lib.Snapshot()
	if snap.Len() != 2 {
		t.Fatal("both colliding records must be kept")
	}
	for _, r := range []*library.Replay{first, second} {
		if got, ok := snap.ByID(r.ID); !ok || got != r {
			t.Fatal("collision victim not retrievable by its probed id")
		}
	}
}

func TestSubscribeProgressAndCancel(t *testing.T) {
	lib := newTestLibrary(t, library.Options{EventBuffer: 1})
	initial, events, cancel := lib.Subscribe()
	if initial.Phase != library.PhaseIdle {
		t.Fatalf("initial phase %q", initial.Phase)
	}
	lib.SetProgress(library.ProgressState{Phase: library.PhaseScanning, Total: 10})
	lib.SetProgress(library.ProgressState{Phase: library.PhaseRecent, Total: 10, Loaded: 1})
	ev := <-events
	if ev.Kind != library.EventProgress || ev.Progress.Phase != library.PhaseScanning {
		t.Fatalf("first event %+v", ev)
	}
	select {
	case ev := <-events:
		t.Fatalf("second event should have been dropped by the full buffer: %+v", ev)
	default:
	}
	if lib.Progress().Phase != library.PhaseRecent {
		t.Fatal("Progress() must reflect the latest SetProgress even when the event was dropped")
	}
	cancel()
	cancel()
	if _, open := <-events; open {
		t.Fatal("cancel must close the channel")
	}

	lib.Close()
	lib.Close()
	_, closedEvents, _ := lib.Subscribe()
	if _, open := <-closedEvents; open {
		t.Fatal("Subscribe after Close must return a closed channel")
	}
	lib.Add(0, librarytest.Replay())
	lib.Flush()
}

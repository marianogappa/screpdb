package library_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func TestViewFiltersAndIsCachedPerSnapshotAndFilter(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	long := librarytest.Replay(librarytest.WithPlayer("Flash", librarytest.Team(1)), librarytest.WithPlayer("Bisu", librarytest.Team(2)))
	short := librarytest.Replay(librarytest.WithDuration(30), librarytest.WithPlayer("Flash", librarytest.Team(1)), librarytest.WithPlayer("Stork", librarytest.Team(2)))
	lib.Add(0, long, short)
	lib.Flush()

	view := lib.View()
	if view != lib.View() {
		t.Fatal("same snapshot and filter must return the cached view")
	}
	if view.Len() != 1 || view.Replays()[0] != long {
		t.Fatalf("default filter should drop the short game: %d", view.Len())
	}
	if view.Contains(short.ID) {
		t.Fatal("Contains must honour the filter")
	}
	if _, ok := view.ByID(short.ID); ok {
		t.Fatal("ByID must honour the filter")
	}
	if r, ok := view.ByID(long.ID); !ok || r != long {
		t.Fatal("ByID miss for a passing replay")
	}
	if games := view.PlayerGames("flash"); len(games) != 1 || games[0].Replay != long {
		t.Fatalf("PlayerGames = %+v", games)
	}
	if keys := view.PlayerKeys(); len(keys) != 2 || keys[0] != "bisu" || keys[1] != "flash" {
		t.Fatalf("PlayerKeys = %v", keys)
	}
	if view.Snapshot() != lib.Snapshot() || !view.Filter().Equal(library.DefaultFilterConfig()) {
		t.Fatal("view accessors")
	}

	if err := lib.SetFilter(library.FilterConfig{}); err != nil {
		t.Fatal(err)
	}
	unfiltered := lib.View()
	if unfiltered == view {
		t.Fatal("a new filter must build a new view")
	}
	if unfiltered.Len() != 2 || !unfiltered.Contains(short.ID) {
		t.Fatal("empty filter must include the short game")
	}
	if keys := unfiltered.PlayerKeys(); len(keys) != 3 {
		t.Fatalf("PlayerKeys = %v", keys)
	}
	if err := lib.SetFilter(library.FilterConfig{}); err != nil {
		t.Fatal(err)
	}
	if lib.View() != unfiltered {
		t.Fatal("setting an equal filter must keep the cached view")
	}
	if err := lib.SetFilter(library.FilterConfig{GameTypes: []string{"bogus"}}); err == nil {
		t.Fatal("invalid filter must be rejected")
	}
	if !lib.Filter().Equal(library.FilterConfig{GameTypes: []string{}, MapKinds: []string{}}) {
		t.Fatalf("rejected filter must not be installed: %+v", lib.Filter())
	}

	lib.Add(0, librarytest.Replay())
	lib.Flush()
	if lib.View() == unfiltered {
		t.Fatal("a new snapshot must build a new view")
	}
}

func TestViewCorpusStamp(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	lib.Add(0, librarytest.Replay(), librarytest.Replay(librarytest.WithDuration(10)))
	lib.Flush()

	stamp := lib.View().Corpus()
	if stamp.Loaded != 2 || stamp.Total != 2 || stamp.Complete || stamp.Version != 1 {
		t.Fatalf("stamp without progress = %+v", stamp)
	}
	lib.SetProgress(library.ProgressState{Generation: 0, Phase: library.PhaseBackfill, Total: 10, Loaded: 2})
	stamp = lib.View().Corpus()
	if stamp.Loaded != 2 || stamp.Total != 10 || stamp.Complete {
		t.Fatalf("stamp while loading = %+v", stamp)
	}
	lib.SetProgress(library.ProgressState{Phase: library.PhaseReady, Total: 10, Loaded: 10})
	if !lib.View().Corpus().Complete {
		t.Fatal("ready must mark the corpus complete")
	}
	lib.SetProgress(library.ProgressState{Phase: library.PhaseWatching, Total: 10, Loaded: 10})
	if !lib.View().Corpus().Complete {
		t.Fatal("watching must mark the corpus complete")
	}
}

func TestViewMemoBuildsOncePerKey(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	lib.Add(0, librarytest.Replay())
	lib.Flush()
	view := lib.View()

	var builds atomic.Int32
	var wg sync.WaitGroup
	results := make([]any, 32)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = view.Memo("players", func() any {
				builds.Add(1)
				return []string{"flash"}
			})
		}(i)
	}
	wg.Wait()
	if builds.Load() != 1 {
		t.Fatalf("build ran %d times", builds.Load())
	}
	for _, r := range results {
		if r.([]string)[0] != "flash" {
			t.Fatal("memo returned a different value")
		}
	}
	other := view.Memo("apm", func() any { return 42 })
	if other != 42 || builds.Load() != 1 {
		t.Fatal("keys must be independent")
	}
	if lib.View().Memo("players", func() any { return nil }) == nil {
		t.Fatal("same view must keep memo values")
	}
	lib.Add(0, librarytest.Replay())
	lib.Flush()
	if lib.View().Memo("players", func() any { return "rebuilt" }) != "rebuilt" {
		t.Fatal("a new view must not inherit memo values")
	}
}

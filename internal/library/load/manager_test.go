package load

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/watch"
)

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.After(15 * time.Second)
	for {
		if condition() {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func newTestManager(t *testing.T, lib *library.Library, dir string) *Manager {
	t.Helper()
	t.Cleanup(iofacade.Reset)
	manager := NewManager(lib, ManagerOptions{
		Folder: dir,
		Loader: Options{Workers: 2, RecentCount: 1, PublishRate: time.Millisecond},
		Watch: watch.Options{
			Settle:            5 * time.Millisecond,
			ReconcileInterval: 10 * time.Millisecond,
			Debounce:          time.Millisecond,
			NewNativeWatcher:  func() (iofacade.DirWatcher, error) { return nil, iofacade.ErrWatchUnsupported },
		},
	})
	t.Cleanup(manager.Close)
	return manager
}

func TestManagerLoadsTheFolderAndThenWatchesIt(t *testing.T) {
	dir := corpusDir(t, "SomaJyJ.rep")
	lib := newLibrary(t)
	manager := newTestManager(t, lib, dir)

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, "the first load to finish", func() bool {
		return lib.Progress().Phase == library.PhaseWatching && lib.Snapshot().Len() == 1
	})

	added := filepath.Join(dir, "bgh.rep")
	copyFile(t, filepath.Join("..", "..", "testdata", "replays", "bgh.rep"), added)
	waitFor(t, "the new replay to be picked up", func() bool { return lib.Snapshot().Len() == 2 })

	if err := os.Remove(added); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the deleted replay to disappear", func() bool { return lib.Snapshot().Len() == 1 })
	if lib.Snapshot().Replays[0].FileName() != "SomaJyJ.rep" {
		t.Fatalf("the wrong replay survived: %q", lib.Snapshot().Replays[0].FileName())
	}
}

func TestManagerSetFolderSwapsTheCorpus(t *testing.T) {
	first := corpusDir(t, "SomaJyJ.rep")
	second := corpusDir(t, "bgh.rep")
	lib := newLibrary(t)
	manager := newTestManager(t, lib, first)

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, "the first folder to load", func() bool { return lib.Snapshot().Len() == 1 })

	if err := manager.SetFolder(context.Background(), second); err != nil {
		t.Fatalf("SetFolder: %v", err)
	}
	waitFor(t, "the second folder to load", func() bool {
		snap := lib.Snapshot()
		return snap.Len() == 1 && snap.Replays[0].FileName() == "bgh.rep"
	})
	if got := manager.Folder(); got != second {
		t.Fatalf("folder = %q, want %q", got, second)
	}
	if got := lib.Snapshot().Generation; got != 2 {
		t.Fatalf("generation = %d, want 2", got)
	}
}

func TestManagerRejectsAFolderWithoutReplays(t *testing.T) {
	dir := corpusDir(t, "SomaJyJ.rep")
	lib := newLibrary(t)
	manager := newTestManager(t, lib, dir)

	if err := manager.SetFolder(context.Background(), t.TempDir()); err == nil {
		t.Fatal("a folder with no replays should be rejected")
	}
	if got := manager.Folder(); got != dir {
		t.Fatalf("a rejected folder changed the setting to %q", got)
	}
}

func TestManagerKnowsWhatTheCorpusHolds(t *testing.T) {
	dir := corpusDir(t, "SomaJyJ.rep")
	lib := newLibrary(t)
	manager := newTestManager(t, lib, dir)
	run(t, lib, dir, Options{})

	paths := manager.Paths()
	if len(paths) != 1 {
		t.Fatalf("paths = %v", paths)
	}
	info, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if !manager.Known(paths[0], info.Size(), info.ModTime()) {
		t.Fatal("a loaded file should be known")
	}
	if manager.Known(paths[0], info.Size()+1, info.ModTime()) {
		t.Fatal("a file whose size changed should not be known")
	}
	if manager.Known(filepath.Join(dir, "absent.rep"), 1, time.Now()) {
		t.Fatal("an unknown path should not be known")
	}
}

package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

type fakeCorpus struct {
	mu    sync.Mutex
	files map[string]stamp
}

func newFakeCorpus() *fakeCorpus { return &fakeCorpus{files: map[string]stamp{}} }

func (c *fakeCorpus) Known(path string, size int64, modTime time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	known, ok := c.files[path]
	return ok && known.size == size && known.modTime.Equal(modTime)
}

func (c *fakeCorpus) Paths() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	paths := make([]string, 0, len(c.files))
	for path := range c.files {
		paths = append(paths, path)
	}
	return paths
}

func (c *fakeCorpus) add(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files[path] = stamp{size: info.Size(), modTime: info.ModTime()}
}

func newTestWatcher(t *testing.T, folder string, corpus Corpus) (*Watcher, *[]Change, *time.Time) {
	t.Helper()
	var changes []Change
	clock := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	w := New(Options{
		Folder:           folder,
		Settle:           time.Second,
		NewNativeWatcher: func() (iofacade.DirWatcher, error) { return nil, iofacade.ErrWatchUnsupported },
	}, corpus, func(change Change) { changes = append(changes, change) })
	w.now = func() time.Time { return clock }
	return w, &changes, &clock
}

func writeReplay(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileWaitsForAFileToStopChanging(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.rep")
	writeReplay(t, path, "first")
	w, changes, clock := newTestWatcher(t, dir, newFakeCorpus())

	w.Reconcile()
	if len(*changes) != 0 {
		t.Fatalf("a file seen once should not be reported yet: %+v", *changes)
	}

	*clock = clock.Add(2 * time.Second)
	w.Reconcile()
	if len(*changes) != 1 || len((*changes)[0].Changed) != 1 || (*changes)[0].Changed[0].Path != path {
		t.Fatalf("changes = %+v", *changes)
	}
}

func TestReconcileRestartsTheWaitWhenTheFileGrows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.rep")
	writeReplay(t, path, "first")
	w, changes, clock := newTestWatcher(t, dir, newFakeCorpus())

	w.Reconcile()
	*clock = clock.Add(2 * time.Second)
	writeReplay(t, path, "still being written")
	w.Reconcile()
	if len(*changes) != 0 {
		t.Fatalf("a growing file should not be reported: %+v", *changes)
	}

	*clock = clock.Add(2 * time.Second)
	w.Reconcile()
	if len(*changes) != 1 {
		t.Fatalf("a settled file should be reported once: %+v", *changes)
	}
}

func TestReconcileReportsEachFileOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	writeReplay(t, filepath.Join(dir, "game.rep"), "content")
	w, changes, clock := newTestWatcher(t, dir, newFakeCorpus())

	w.Reconcile()
	*clock = clock.Add(2 * time.Second)
	w.Reconcile()
	w.Reconcile()
	*clock = clock.Add(2 * time.Second)
	w.Reconcile()
	if len(*changes) != 1 {
		t.Fatalf("a file the loader could not read should not be retried forever: %+v", *changes)
	}
}

func TestReconcileReportsRemovals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.rep")
	writeReplay(t, path, "content")
	corpus := newFakeCorpus()
	corpus.add(t, path)
	w, changes, _ := newTestWatcher(t, dir, corpus)

	w.Reconcile()
	if len(*changes) != 0 {
		t.Fatalf("a file already in the corpus is not a change: %+v", *changes)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	w.Reconcile()
	if len(*changes) != 1 || len((*changes)[0].Removed) != 1 || (*changes)[0].Removed[0] != path {
		t.Fatalf("changes = %+v", *changes)
	}
}

func TestReconcileIgnoresStagingAndHiddenPaths(t *testing.T) {
	dir := t.TempDir()
	writeReplay(t, filepath.Join(dir, "000_screpdb_watch_me", "watch_me.rep"), "content")
	writeReplay(t, filepath.Join(dir, ".trash", "old.rep"), "content")
	writeReplay(t, filepath.Join(dir, "LastReplay.rep"), "content")
	writeReplay(t, filepath.Join(dir, "notes.txt"), "content")
	w, changes, clock := newTestWatcher(t, dir, newFakeCorpus())

	w.Reconcile()
	*clock = clock.Add(2 * time.Second)
	w.Reconcile()
	if len(*changes) != 0 {
		t.Fatalf("changes = %+v", *changes)
	}
}

func TestReconcileLeavesPathsOutsideTheFolderAlone(t *testing.T) {
	dir := t.TempDir()
	corpus := newFakeCorpus()
	corpus.files[filepath.Join(t.TempDir(), "elsewhere.rep")] = stamp{size: 1}
	w, changes, _ := newTestWatcher(t, dir, corpus)

	w.Reconcile()
	if len(*changes) != 0 {
		t.Fatalf("a path from another folder must not be removed: %+v", *changes)
	}
}

func TestRunReconcilesOnItsOwnAndStopsWithTheContext(t *testing.T) {
	dir := t.TempDir()
	writeReplay(t, filepath.Join(dir, "game.rep"), "content")
	var mu sync.Mutex
	var seen int
	w := New(Options{
		Folder:            dir,
		Settle:            time.Millisecond,
		ReconcileInterval: 5 * time.Millisecond,
		Debounce:          time.Millisecond,
		NewNativeWatcher:  func() (iofacade.DirWatcher, error) { return nil, errors.New("no watcher here") },
	}, newFakeCorpus(), func(change Change) {
		mu.Lock()
		defer mu.Unlock()
		seen += len(change.Changed)
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := w.Run(ctx); err != nil {
			t.Errorf("Run: %v", err)
		}
	}()

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		got := seen
		mu.Unlock()
		if got > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("the watcher never reported the new file")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the watcher did not stop with its context")
	}
}

func TestReconcileLeavesOutWhatTheCapExcludedButTakesNewGames(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	stamps := map[string]time.Time{}
	for i, name := range []string{"oldest.rep", "middle.rep", "newest.rep"} {
		path := filepath.Join(dir, name)
		writeReplay(t, path, name)
		stamp := base.Add(time.Duration(i) * time.Minute)
		stamps[name] = stamp
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	corpus := newFakeCorpus()
	var changes []Change
	// The initial read kept the two newest, so middle.rep is the cutoff.
	w := New(Options{
		Folder:           dir,
		Settle:           time.Second,
		Since:            stamps["middle.rep"],
		NewNativeWatcher: func() (iofacade.DirWatcher, error) { return nil, iofacade.ErrWatchUnsupported },
	}, corpus, func(change Change) { changes = append(changes, change) })
	clock := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	w.now = func() time.Time { return clock }

	w.Reconcile()
	clock = clock.Add(2 * time.Second)
	w.Reconcile()
	var reported []string
	for _, change := range changes {
		for _, file := range change.Changed {
			reported = append(reported, filepath.Base(file.Path))
		}
	}
	sort.Strings(reported)
	if !reflect.DeepEqual(reported, []string{"middle.rep", "newest.rep"}) {
		t.Fatalf("reported %v, want the cutoff and everything after it", reported)
	}

	corpus.add(t, filepath.Join(dir, "middle.rep"))
	corpus.add(t, filepath.Join(dir, "newest.rep"))
	changes = nil
	w.Reconcile()
	if len(changes) != 0 {
		t.Fatalf("a settled folder should be quiet, got %+v", changes)
	}

	// A game played now always loads, cap or no cap, and nothing is evicted.
	newer := filepath.Join(dir, "newer.rep")
	writeReplay(t, newer, "newer")
	stamp := base.Add(time.Hour)
	if err := os.Chtimes(newer, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	changes = nil
	w.Reconcile()
	clock = clock.Add(2 * time.Second)
	w.Reconcile()
	var added, removed []string
	for _, change := range changes {
		for _, file := range change.Changed {
			added = append(added, filepath.Base(file.Path))
		}
		removed = append(removed, change.Removed...)
	}
	if !reflect.DeepEqual(added, []string{"newer.rep"}) {
		t.Fatalf("added %v, want the new game", added)
	}
	if len(removed) != 0 {
		t.Fatalf("nothing should be evicted to make room, got %v", removed)
	}
}

func TestReconcileOnlyReportsFilesThatReallyLeftTheDisk(t *testing.T) {
	dir := t.TempDir()
	kept := filepath.Join(dir, "kept.rep")
	gone := filepath.Join(dir, "gone.rep")
	writeReplay(t, kept, "kept")
	writeReplay(t, gone, "gone")
	corpus := newFakeCorpus()
	corpus.add(t, kept)
	corpus.add(t, gone)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}

	var changes []Change
	// A cutoff in the future excludes kept.rep from the walk, so it is absent
	// for a reason that is not deletion.
	w := New(Options{
		Folder:           dir,
		Settle:           time.Second,
		Since:            time.Now().Add(time.Hour),
		NewNativeWatcher: func() (iofacade.DirWatcher, error) { return nil, iofacade.ErrWatchUnsupported },
	}, corpus, func(change Change) { changes = append(changes, change) })
	w.Reconcile()

	var removed []string
	for _, change := range changes {
		for _, path := range change.Removed {
			removed = append(removed, filepath.Base(path))
		}
	}
	if !reflect.DeepEqual(removed, []string{"gone.rep"}) {
		t.Fatalf("removed %v, want only the deleted file", removed)
	}
}

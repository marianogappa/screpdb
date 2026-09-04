package load

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/library"
)

func corpusDir(t *testing.T, names ...string) string {
	t.Helper()
	source := filepath.Join("..", "..", "testdata", "replays")
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Skipf("no replay corpus: %v", err)
	}
	dir := t.TempDir()
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[name] = true
	}
	copied := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".rep" {
			continue
		}
		if len(wanted) > 0 && !wanted[entry.Name()] {
			continue
		}
		copyFile(t, filepath.Join(source, entry.Name()), filepath.Join(dir, entry.Name()))
		copied++
	}
	if copied == 0 {
		t.Skip("no replay files in corpus")
	}
	return dir
}

func copyFile(t *testing.T, source, target string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", target, err)
	}
}

func newLibrary(t *testing.T) *library.Library {
	t.Helper()
	lib := library.New(library.Options{CoalesceRecords: 1, CoalesceDelay: time.Millisecond})
	t.Cleanup(lib.Close)
	return lib
}

func run(t *testing.T, lib *library.Library, dir string, opts Options) *Loader {
	t.Helper()
	opts.Folder = dir
	if opts.Generation == 0 {
		opts.Generation = 1
	}
	if opts.Workers == 0 {
		opts.Workers = 2
	}
	if opts.PublishRate == 0 {
		opts.PublishRate = time.Millisecond
	}
	loader := New(lib, opts)
	if err := loader.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return loader
}

func TestRunLoadsTheWholeFolderNewestFirst(t *testing.T) {
	dir := corpusDir(t)
	lib := newLibrary(t)
	loader := run(t, lib, dir, Options{RecentCount: 2})

	snap := lib.Snapshot()
	if snap.Len() != 4 {
		t.Fatalf("loaded %d replays, want 4", snap.Len())
	}
	for i := 1; i < snap.Len(); i++ {
		if snap.Replays[i-1].Date.Before(snap.Replays[i].Date) {
			t.Fatalf("replays are not newest first: %v then %v", snap.Replays[i-1].Date, snap.Replays[i].Date)
		}
	}
	progress := loader.Progress()
	if progress.Phase != library.PhaseReady {
		t.Errorf("phase = %q, want ready", progress.Phase)
	}
	if progress.Total != 4 || progress.Loaded != 4 || progress.Failed != 0 || progress.Skipped != 0 {
		t.Errorf("progress = %+v", progress)
	}
	for _, record := range snap.Replays {
		if record.ID <= 0 || record.ID > library.MaxReplayID {
			t.Errorf("replay id %d out of range", record.ID)
		}
		if len(record.Players) == 0 || record.Prod.Len() == 0 {
			t.Errorf("replay %s compacted to nothing", record.FileName())
		}
	}
}

func TestRunSkipsFilesAlreadyInTheCorpus(t *testing.T) {
	dir := corpusDir(t)
	lib := newLibrary(t)
	run(t, lib, dir, Options{})
	version := lib.Snapshot().Version

	loader := run(t, lib, dir, Options{})
	if got := lib.Snapshot().Len(); got != 4 {
		t.Fatalf("rescan changed the corpus size to %d", got)
	}
	if progress := loader.Progress(); progress.Loaded != 4 || progress.Skipped != 4 {
		t.Fatalf("rescan progress = %+v, want everything skipped", progress)
	}
	if lib.Snapshot().Version != version {
		t.Error("rescan of an unchanged folder should not commit a new snapshot")
	}
}

func TestRunCollapsesDuplicateContentIntoOneReplay(t *testing.T) {
	dir := corpusDir(t, "SomaJyJ.rep")
	copyFile(t, filepath.Join(dir, "SomaJyJ.rep"), filepath.Join(dir, "copy.rep"))
	lib := newLibrary(t)
	loader := run(t, lib, dir, Options{})

	snap := lib.Snapshot()
	if snap.Len() != 1 {
		t.Fatalf("loaded %d replays, want 1", snap.Len())
	}
	if len(snap.Replays[0].Paths) != 2 {
		t.Fatalf("paths = %+v, want both copies", snap.Replays[0].Paths)
	}
	if progress := loader.Progress(); progress.Loaded != 2 || progress.Skipped != 1 {
		t.Fatalf("progress = %+v", progress)
	}
}

func TestRunIgnoresStagingAndHiddenFolders(t *testing.T) {
	dir := corpusDir(t, "SomaJyJ.rep")
	source := filepath.Join(dir, "SomaJyJ.rep")
	copyFile(t, source, filepath.Join(dir, "000_screpdb_watch_me", "watch_me.rep"))
	copyFile(t, source, filepath.Join(dir, ".trash", "old.rep"))
	copyFile(t, source, filepath.Join(dir, "half.rep.tmp"))

	lib := newLibrary(t)
	loader := run(t, lib, dir, Options{})
	if got := lib.Snapshot().Len(); got != 1 {
		t.Fatalf("loaded %d replays, want only the real one", got)
	}
	if progress := loader.Progress(); progress.Total != 1 {
		t.Fatalf("ignored files were counted: %+v", progress)
	}
}

func TestRunKeepsGoingWhenAFileCannotBeRead(t *testing.T) {
	dir := corpusDir(t)
	if err := os.WriteFile(filepath.Join(dir, "broken.rep"), []byte("not a replay"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := newLibrary(t)
	loader := run(t, lib, dir, Options{})

	if got := lib.Snapshot().Len(); got != 4 {
		t.Fatalf("loaded %d replays, want the 4 good ones", got)
	}
	progress := loader.Progress()
	if progress.Failed != 1 || progress.Total != 5 || progress.Loaded != 4 {
		t.Fatalf("progress = %+v", progress)
	}
}

func TestRunStagedReplacesTheCorpus(t *testing.T) {
	first := corpusDir(t, "SomaJyJ.rep")
	second := corpusDir(t, "bgh.rep")
	lib := newLibrary(t)
	run(t, lib, first, Options{Generation: 1})
	if got := lib.Snapshot().Len(); got != 1 {
		t.Fatalf("first load = %d", got)
	}

	run(t, lib, second, Options{Generation: 2, Staged: true})
	snap := lib.Snapshot()
	if snap.Len() != 1 {
		t.Fatalf("second load = %d replays", snap.Len())
	}
	if snap.Replays[0].FileName() != "bgh.rep" {
		t.Fatalf("corpus still holds %q", snap.Replays[0].FileName())
	}
	if snap.Generation != 2 {
		t.Fatalf("generation = %d, want 2", snap.Generation)
	}
}

func TestRunStopsWhenCancelled(t *testing.T) {
	dir := corpusDir(t)
	lib := newLibrary(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	loader := New(lib, Options{Folder: dir, Generation: 1, Workers: 2, PublishRate: time.Millisecond})
	if err := loader.Run(ctx); err == nil {
		t.Fatal("cancelled run should report the cancellation")
	}
	if lib.Snapshot().Len() != 0 {
		t.Fatalf("cancelled run loaded %d replays", lib.Snapshot().Len())
	}
}

func TestProcessAddsFilesToALoadedCorpus(t *testing.T) {
	dir := corpusDir(t, "SomaJyJ.rep")
	lib := newLibrary(t)
	loader := run(t, lib, dir, Options{})

	added := filepath.Join(dir, "bgh.rep")
	copyFile(t, filepath.Join("..", "..", "testdata", "replays", "bgh.rep"), added)
	info, err := fileops.NewFileInfoFromPath(added)
	if err != nil {
		t.Fatal(err)
	}

	loader.Process(context.Background(), []fileops.FileInfo{*info})
	if got := lib.Snapshot().Len(); got != 2 {
		t.Fatalf("corpus = %d replays, want 2", got)
	}
	if progress := loader.Progress(); progress.Total != 2 || progress.Loaded != 2 {
		t.Fatalf("progress = %+v", progress)
	}
}

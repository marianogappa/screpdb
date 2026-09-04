package load

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
)

// BenchmarkLibraryLoadCorpus measures what a user actually waits for on launch:
// the dashboard reading a replay folder into the in-memory library. It runs the
// production loader with production defaults (worker count, commit coalescing)
// and no replay cap, so the reported rate is comparable across corpus sizes.
func BenchmarkLibraryLoadCorpus(b *testing.B) {
	folder := os.Getenv("SCREPDB_BENCH_CORPUS")
	if folder == "" {
		var err error
		folder, err = benchCorpusDir()
		if err != nil {
			b.Fatalf("benchCorpusDir: %v", err)
		}
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		b.Fatalf("corpus %s is not a directory: %v", folder, err)
	}

	var (
		totalDur   time.Duration
		corpusSize int
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lib := library.New(library.Options{})
		// MaxReplays -1 reads the whole folder: the default 500 cap would make
		// the figure depend on the corpus size rather than on load speed.
		loader := New(lib, Options{Folder: folder, Generation: 1, MaxReplays: -1})

		start := time.Now()
		if err := loader.Run(context.Background()); err != nil {
			lib.Close()
			b.Fatalf("Run: %v", err)
		}
		totalDur += time.Since(start)

		loaded := lib.Snapshot().Len()
		lib.Close()
		if loaded == 0 {
			b.Fatalf("loaded no replays from %s", folder)
		}
		if corpusSize == 0 {
			corpusSize = loaded
		} else if loaded != corpusSize {
			b.Fatalf("iteration %d loaded %d replays, first loaded %d", i, loaded, corpusSize)
		}
	}

	ops := float64(b.N)
	b.ReportMetric(totalDur.Seconds()/ops, fmt.Sprintf("s/load_%dreplays", corpusSize))
	b.ReportMetric(totalDur.Seconds()/ops/float64(corpusSize), "s/replay")
}

func benchCorpusDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "replays")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", os.ErrNotExist
	}
	return dir, nil
}

package memprobe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/load"
)

// TestMarginalCostPerReplay loads two folders of different sizes and reports
// the slope, so the one-time cost of the marker registry and the embedded map
// data does not get charged to the replays.
func TestMarginalCostPerReplay(t *testing.T) {
	source := os.Getenv("PROBE_FOLDER")
	if source == "" {
		t.Skip("set PROBE_FOLDER")
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".rep" {
			names = append(names, e.Name())
		}
	}
	if len(names) < 80 {
		t.Skipf("need at least 80 replays, have %d", len(names))
	}

	measure := func(count int) (int, uint64) {
		dir := t.TempDir()
		for _, name := range names[:count] {
			data, err := os.ReadFile(filepath.Join(source, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := iofacade.AllowDir(dir); err != nil {
			t.Fatal(err)
		}
		lib := library.New(library.Options{})
		defer lib.Close()
		if err := load.New(lib, load.Options{Folder: dir, Generation: 1}).Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		runtime.GC()
		debug.FreeOSMemory()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		loaded := lib.Snapshot().Len()
		runtime.KeepAlive(lib)
		return loaded, m.HeapAlloc
	}

	smallN, smallHeap := measure(40)
	bigN, bigHeap := measure(len(names))
	slope := float64(bigHeap-smallHeap) / float64(bigN-smallN) / 1024
	fmt.Printf("\nPROBE-SLOPE small=%d (%.0f KB heap) big=%d (%.0f KB heap) marginal=%.1f KB/replay fixed=%.0f KB\n",
		smallN, float64(smallHeap)/1024, bigN, float64(bigHeap)/1024, slope,
		float64(smallHeap)/1024-slope*float64(smallN))
}

package library_test

import (
	"slices"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func TestSetEngineDataUpgradesInPlace(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	r := librarytest.Replay(librarytest.WithPath("/r/a.rep", time.Now()))
	other := librarytest.Replay(librarytest.WithPath("/r/b.rep", time.Now()))
	lib.Add(0, r, other)
	lib.Flush()
	before := lib.Snapshot()
	_, events, cancel := lib.Subscribe()
	defer cancel()

	data := &library.EngineData{EngineVersion: 1, TrustedUntil: 9000}
	lib.SetEngineData(r.Checksum, data)
	lib.SetEngineData([32]byte{9}, data)
	lib.Flush()

	got, ok := lib.Snapshot().ByChecksum(r.Checksum)
	if !ok || got.EngineData != data {
		t.Fatalf("engine data not attached: %+v", got)
	}
	if old, _ := before.ByChecksum(r.Checksum); old.EngineData != nil {
		t.Fatal("the previous snapshot changed")
	}
	if o, _ := lib.Snapshot().ByChecksum(other.Checksum); o.EngineData != nil {
		t.Fatal("another replay was upgraded")
	}
	ev := <-events
	if ev.Kind != library.EventCorpus || !slices.Equal(ev.Upgraded, []int64{got.ID}) || len(ev.Added) != 0 {
		t.Fatalf("event %+v", ev)
	}

	lib.Alias(0, library.FileRef{Path: "/r/copy.rep"}, r.Checksum)
	lib.Flush()
	if kept, _ := lib.Snapshot().ByChecksum(r.Checksum); kept.EngineData != data || len(kept.Paths) != 2 {
		t.Fatal("a new path dropped the engine data")
	}
}

func TestSetEngineDataReachesStagedCorpus(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	r := librarytest.Replay(librarytest.WithPath("/r/a.rep", time.Now()))
	lib.Stage(1)
	lib.Add(1, r)
	lib.SetEngineData(r.Checksum, &library.EngineData{EngineVersion: 1})
	lib.Promote(1)
	lib.Flush()
	if got, ok := lib.Snapshot().ByChecksum(r.Checksum); !ok || got.EngineData == nil {
		t.Fatal("staged replay lost its engine data on promote")
	}
}

package enginepass

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/gamedata"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func TestQueueOrder(t *testing.T) {
	q := newQueue()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		q.push(&job{checksum: [32]byte{byte(i)}, id: int64(i), date: base.Add(time.Duration(i) * time.Hour)})
	}
	q.push(&job{checksum: [32]byte{1}, id: 1, date: base.Add(10 * time.Hour)})
	q.remove([32]byte{3})
	q.setFocus([32]byte{0})
	var got []byte
	for j := q.pop(); j != nil; j = q.pop() {
		got = append(got, j.checksum[0])
	}
	if want := []byte{0, 1, 4, 2}; string(got) != string(want) {
		t.Fatalf("order %v, want %v", got, want)
	}
	if q.len() != 0 {
		t.Fatal("queue not empty")
	}
}

func TestNoInstallTurnsTheEnginePassOff(t *testing.T) {
	lib := library.New(library.Options{})
	defer lib.Close()
	r := New(Options{
		Library:   lib,
		CachePath: filepath.Join(t.TempDir(), "digests.bin"),
		OpenData:  func() (*engine.Data, error) { return nil, gamedata.ErrNoInstall },
		Logf:      t.Logf,
	})
	r.Start()
	r.Close()
	if s := r.Status(); s.State != StateOff {
		t.Fatalf("status %+v", s)
	}
}

func testReplay(t *testing.T, name string) *library.Replay {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "testdata", "replays", name))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return librarytest.Replay(librarytest.WithPath(path, time.Now()), librarytest.WithRawChecksum(sha256.Sum256(b)))
}

func runUntilUpgraded(t *testing.T, cachePath string, data *engine.Data, records ...*library.Replay) (*library.Library, time.Duration) {
	t.Helper()
	lib := library.New(library.Options{})
	t.Cleanup(lib.Close)
	lib.Add(0, records...)
	lib.Flush()
	r := New(Options{Library: lib, CachePath: cachePath, Workers: 2, OpenData: func() (*engine.Data, error) { return data, nil }, Logf: t.Logf})
	start := time.Now()
	r.Start()
	defer r.Close()
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		done := 0
		for _, rec := range lib.Snapshot().Replays {
			if rec.EngineData != nil {
				done++
			}
		}
		if done == len(records) {
			return lib, time.Since(start)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("not upgraded: %+v", r.Status())
	return nil, 0
}

func TestUpgradeAndWarmStart(t *testing.T) {
	dir := os.Getenv("OPENBW_DATA")
	if dir == "" {
		t.Skip("OPENBW_DATA is not set; the engine needs the StarCraft game data files")
	}
	data, err := engine.LoadData(dir)
	if err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(t.TempDir(), "digests.bin")
	jyj := testReplay(t, "SomaJyJ.rep")
	lib, cold := runUntilUpgraded(t, cachePath, data, jyj)
	got, _ := lib.Snapshot().ByChecksum(jyj.Checksum)
	if got.EngineData.EngineVersion != engine.EngineVersion || got.EngineData.LowConfidence || got.EngineData.TrustedUntil < 14000 {
		t.Fatalf("engine data %+v", got.EngineData)
	}

	_, warm := runUntilUpgraded(t, cachePath, data, testReplay(t, "SomaJyJ.rep"))
	if warm > cold/4 {
		t.Fatalf("warm start took %v after a %v cold start; the cache was not used", warm, cold)
	}
	t.Logf("cold %v, warm %v", cold, warm)
}

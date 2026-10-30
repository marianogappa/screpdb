package enginepass

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/digestcache"
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
	lib.SetProgress(library.ProgressState{Phase: library.PhaseReady})
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
	lib.SetProgress(library.ProgressState{Phase: library.PhaseReady})
	r := New(Options{Library: lib, CachePath: cachePath, SnapshotPath: cachePath + ".snapshots", Workers: func() int { return 2 }, OpenData: func() (*engine.Data, error) { return data, nil }, Logf: t.Logf})
	start := time.Now()
	r.Start()
	t.Cleanup(r.Close)
	lastRunner = r
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

var lastRunner *Runner

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

	r := lastRunner
	start := time.Now()
	snaps, err := r.Snapshots(context.Background(), jyj.Checksum, jyj.Path(), []int{60, 360})
	if err != nil || len(snaps) != 2 || len(snaps[360].Units) < 20 {
		t.Fatalf("snapshots: %v, %d seconds", err, len(snaps))
	}
	first := time.Since(start)
	start = time.Now()
	again, err := r.Snapshots(context.Background(), jyj.Checksum, jyj.Path(), []int{360})
	if err != nil || len(again[360].Units) != len(snaps[360].Units) {
		t.Fatalf("cached snapshot: %v", err)
	}
	if cached := time.Since(start); cached > first/4 {
		t.Fatalf("a cached snapshot took %v after a %v pass", cached, first)
	}
}

func TestNothingStartsBeforeTheLibraryHasLoaded(t *testing.T) {
	lib := library.New(library.Options{})
	defer lib.Close()
	lib.SetProgress(library.ProgressState{Phase: library.PhaseScanning})
	opened := make(chan struct{}, 1)
	r := New(Options{
		Library:   lib,
		CachePath: filepath.Join(t.TempDir(), "digests.bin"),
		OpenData: func() (*engine.Data, error) {
			opened <- struct{}{}
			return nil, gamedata.ErrNoInstall
		},
		Logf: t.Logf,
	})
	r.Start()
	defer r.Close()
	select {
	case <-opened:
		t.Fatal("the engine opened its game data while the library was still loading")
	case <-time.After(3 * libraryPoll):
	}
	lib.SetProgress(library.ProgressState{Phase: library.PhaseReady})
	select {
	case <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("the engine did not start once the library had loaded")
	}
}

func TestHiddenOwnCopiesAreNotQueued(t *testing.T) {
	lib := library.New(library.Options{})
	defer lib.Close()
	cache, err := digestcache.Open(filepath.Join(t.TempDir(), "digests.bin"), func(digestcache.Key) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	r := New(Options{Library: lib, CachePath: filepath.Join(t.TempDir(), "unused.bin")})
	r.cache = cache
	dir := t.TempDir()

	own := librarytest.Replay(librarytest.WithPath(filepath.Join(dir, "a.rep"), time.Now()))
	lib.Add(0, own)
	lib.Flush()
	r.enqueue(lib.Snapshot().Replays)
	complete := librarytest.Replay(librarytest.WithPath(filepath.Join(dir, "a-complete.rep"), time.Now()))
	lib.Add(0, complete)
	lib.Flush()
	got, _ := lib.Snapshot().ByPath(complete.Path())
	r.onEvent(library.Event{Kind: library.EventCorpus, Added: []int64{got.ID}})
	if _, queued := r.queue.byChecksum[own.Checksum]; queued {
		t.Fatal("the own copy stayed queued after its complete copy arrived")
	}
	if _, queued := r.queue.byChecksum[complete.Checksum]; !queued {
		t.Fatal("the complete copy was not queued")
	}

	r.queue = newQueue()
	r.enqueue(lib.Snapshot().Replays)
	if _, queued := r.queue.byChecksum[own.Checksum]; queued || r.queue.len() != 1 {
		t.Fatalf("a restart queued %d passes, the hidden own copy among them: %v", r.queue.len(), queued)
	}
}

func TestSummarizeNamesResultsByReplayPlayerAndTrustsOnlyAPassInStep(t *testing.T) {
	d := &engine.Digest{
		Players: []engine.Player{{Slot: 2, PlayerID: 7}, {Slot: 7, PlayerID: 0}, {Slot: 5, PlayerID: -1}},
		Outcomes: []engine.Outcome{
			{Frame: 25112, Owner: 2, Kind: engine.OutcomeDefeated},
			{Frame: 25143, Owner: 7, Kind: engine.OutcomeWon},
			{Frame: 100, Owner: 5, Kind: engine.OutcomeLeft},
		},
		Drift: engine.Drift{ErrorFrame: engine.NoFrame, DesyncFrame: engine.NoFrame},
	}
	data := Summarize(d, false)
	want := []library.EngineResult{
		{ReplayPlayerID: 7, Kind: library.EngineDefeated, Frame: 25112},
		{ReplayPlayerID: 0, Kind: library.EngineWon, Frame: 25143},
	}
	if !data.ResultsKnown || len(data.Results) != 2 || data.Results[0] != want[0] || data.Results[1] != want[1] {
		t.Fatalf("results %+v known %v", data.Results, data.ResultsKnown)
	}

	d.Drift.DesyncFrame = 20000
	if Summarize(d, false).ResultsKnown {
		t.Fatal("a desynced pass settled the winner")
	}
}

package dashboard

import (
	"slices"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func nextEngineMessage(t *testing.T, ch chan libraryEventMessage, wait time.Duration) *libraryEngine {
	t.Helper()
	deadline := time.After(wait)
	for {
		select {
		case msg := <-ch:
			if msg.Type == engineEventType {
				return msg.Engine
			}
		case <-deadline:
			return nil
		}
	}
}

func TestEngineUpgradesArriveInBatches(t *testing.T) {
	hub := newLibraryHub()
	hub.engineWindow = 30 * time.Millisecond
	hub.setEngineState(func() string { return engineStateRunning })
	_, ch, unsubscribe := hub.subscribe()
	defer unsubscribe()

	hub.publishEngine([]int64{1})
	first := nextEngineMessage(t, ch, time.Second)
	if first == nil || !slices.Equal(first.Upgraded, []int64{1}) || first.State != engineStateRunning {
		t.Fatalf("the first upgrade must go out at once: %+v", first)
	}
	for _, id := range []int64{5, 3, 5, 4} {
		hub.publishEngine([]int64{id})
	}
	if early := nextEngineMessage(t, ch, hub.engineWindow/3); early != nil {
		t.Fatalf("upgrades inside the window went out early: %+v", early)
	}
	trailing := nextEngineMessage(t, ch, time.Second)
	if trailing == nil || !slices.Equal(trailing.Upgraded, []int64{3, 4, 5}) {
		t.Fatalf("trailing batch %+v, want [3 4 5]", trailing)
	}
	if quiet := nextEngineMessage(t, ch, 5*hub.engineWindow); quiet != nil {
		t.Fatalf("broadcast after the burst: %+v", quiet)
	}
}

func TestUpgradeOnlyCommitDoesNotRefetchLists(t *testing.T) {
	hub := newLibraryHub()
	hub.engineWindow = 10 * time.Millisecond
	lib := library.New(library.Options{})
	defer lib.Close()
	r := librarytest.Replay(librarytest.WithPath("/r/a.rep", time.Now()))
	lib.Add(0, r)
	lib.Flush()
	hub.Watch(lib)
	_, ch, unsubscribe := hub.subscribe()
	defer unsubscribe()

	lib.SetEngineData(r.Checksum, &library.EngineData{EngineVersion: 1})
	lib.Flush()
	deadline := time.After(time.Second)
	for {
		select {
		case msg := <-ch:
			if msg.Type == "corpus" {
				t.Fatal("an engine upgrade broadcast a corpus change")
			}
			if msg.Type == engineEventType {
				if msg.Engine.Games != 1 || msg.Engine.Total != 1 {
					t.Fatalf("coverage %+v", msg.Engine.engineCoverage)
				}
				return
			}
		case <-deadline:
			t.Fatal("no engine message")
		}
	}
}

func TestEngineFieldsKeepOneShape(t *testing.T) {
	fast := engineFieldsOf(&library.Replay{})
	if fast.Mode != modeFast || fast.EngineConfidence != nil || fast.EngineTrustedUntilSeconds != nil {
		t.Fatalf("fast %+v", fast)
	}
	high := engineFieldsOf(&library.Replay{EngineData: &library.EngineData{TrustedUntil: 9000}})
	if high.Mode != modeEngine || *high.EngineConfidence != engineConfidenceHigh || high.EngineTrustedUntilSeconds != nil {
		t.Fatalf("high %+v", high)
	}
	low := engineFieldsOf(&library.Replay{EngineData: &library.EngineData{LowConfidence: true, TrustedUntil: 2381}})
	if *low.EngineConfidence != engineConfidenceLow || *low.EngineTrustedUntilSeconds != 100 {
		t.Fatalf("low %+v until %d", low, *low.EngineTrustedUntilSeconds)
	}
}

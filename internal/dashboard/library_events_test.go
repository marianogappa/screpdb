package dashboard

import (
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/load"
)

func TestStatusForPhase(t *testing.T) {
	for phase, want := range map[library.Phase]string{
		library.PhaseIdle:     libraryStatusIdle,
		library.PhaseScanning: libraryStatusLoading,
		library.PhaseRecent:   libraryStatusLoading,
		library.PhaseBackfill: libraryStatusLoading,
		library.PhaseReady:    libraryStatusWatching,
		library.PhaseWatching: libraryStatusWatching,
		library.PhaseFailed:   libraryStatusFailed,
	} {
		if got := statusForPhase(phase); got != want {
			t.Errorf("statusForPhase(%q) = %q, want %q", phase, got, want)
		}
	}
}

func TestProgressToWireNamesTheFolderAsTheBrowserDoes(t *testing.T) {
	wire := progressToWire(library.ProgressState{
		Generation: 3, Version: 7, Folder: "/replays", Phase: library.PhaseBackfill,
		Total: 100, Loaded: 40, Failed: 1, Skipped: 2,
	})
	if wire.ReplayDir != "/replays" || wire.Phase != "backfill" || wire.Loaded != 40 || wire.Complete {
		t.Fatalf("wire = %+v", wire)
	}
	if done := progressToWire(library.ProgressState{Phase: library.PhaseWatching}); !done.Complete {
		t.Fatal("a watching library should report complete")
	}
}

func TestLibraryHubSnapshotThenStream(t *testing.T) {
	lib := library.New(library.Options{})
	defer lib.Close()
	hub := newLibraryHub()
	hub.Watch(lib)

	snapshot, events, unsubscribe := hub.subscribe()
	defer unsubscribe()
	if snapshot.Type != "snapshot" || snapshot.Progress == nil {
		t.Fatalf("snapshot = %+v", snapshot)
	}

	hub.Log(load.LogEvent{Level: load.LogLevelInfo, Message: "found 2 replays"})
	lib.SetProgress(library.ProgressState{Generation: 1, Folder: "/replays", Phase: library.PhaseRecent, Total: 2, Loaded: 1})

	seen := map[string]libraryEventMessage{}
	deadline := time.After(3 * time.Second)
	for len(seen) < 3 {
		select {
		case message := <-events:
			seen[message.Type] = message
		case <-deadline:
			t.Fatalf("only saw %v", seen)
		}
	}
	if got := seen["log"]; got.Log == nil || got.Log.Message != "found 2 replays" {
		t.Fatalf("log message = %+v", got.Log)
	}
	if got := seen["progress"]; got.Progress == nil || got.Progress.Loaded != 1 || got.Status != libraryStatusLoading {
		t.Fatalf("progress message = %+v", got)
	}
	if got := seen["status"]; got.Status != libraryStatusLoading {
		t.Fatalf("status message = %+v", got)
	}

	later, _, cancel := hub.subscribe()
	defer cancel()
	if len(later.Logs) != 1 || later.Progress.Loaded != 1 || later.ReplayDir != "/replays" {
		t.Fatalf("a page connecting mid-load should see the tail: %+v", later)
	}
}

func TestLibraryHubKeepsTheLogTailBounded(t *testing.T) {
	hub := newLibraryHub()
	for i := 0; i < maxLibraryLogEvents+10; i++ {
		hub.Log(load.LogEvent{Level: load.LogLevelInfo, Message: "line"})
	}
	if got := len(hub.snapshotMessage().Logs); got != maxLibraryLogEvents {
		t.Fatalf("log tail = %d, want %d", got, maxLibraryLogEvents)
	}
}

func TestLibraryHubDropsForASlowSubscriber(t *testing.T) {
	hub := newLibraryHub()
	_, events, unsubscribe := hub.subscribe()
	defer unsubscribe()
	for i := 0; i < 1000; i++ {
		hub.Log(load.LogEvent{Level: load.LogLevelInfo, Message: "line"})
	}
	if len(events) != cap(events) {
		t.Fatalf("expected the buffer to fill and further messages to drop, got %d of %d", len(events), cap(events))
	}
}

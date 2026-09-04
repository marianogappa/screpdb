package dashboard

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
)

// TestHubStaysQuietWhileTheFolderIsRead is the fix for a page that twitched
// for as long as a read took: the corpus commits a batch every few hundred
// milliseconds, and forwarding each one made the browser refetch and
// re-render over and over.
func TestHubStaysQuietWhileTheFolderIsRead(t *testing.T) {
	hub := newLibraryHub()
	_, events, unsubscribe := hub.subscribe()
	defer unsubscribe()

	drain := func() []libraryEventMessage {
		var out []libraryEventMessage
		for {
			select {
			case message := <-events:
				out = append(out, message)
			default:
				return out
			}
		}
	}

	hub.publishProgress(library.ProgressState{Phase: library.PhaseScanning, Folder: "/replays"})
	started := drain()
	if len(started) != 2 || started[0].Type != "progress" || started[1].Type != "status" {
		t.Fatalf("starting a read should say so once, got %+v", started)
	}
	if started[1].Status != libraryStatusLoading {
		t.Fatalf("status = %q, want loading", started[1].Status)
	}

	// Everything a read does in between.
	for loaded := 100; loaded <= 500; loaded += 100 {
		hub.publishProgress(library.ProgressState{Phase: library.PhaseBackfill, Folder: "/replays", Loaded: loaded, Total: 500})
		hub.publishCorpus(library.Event{Kind: library.EventCorpus, Added: []int64{int64(loaded)}})
	}
	if quiet := drain(); len(quiet) != 0 {
		t.Fatalf("a read in progress should be quiet, got %d messages: %+v", len(quiet), quiet)
	}
	if got := hub.Progress().Loaded; got != 500 {
		t.Fatalf("the hub still has to track progress for the health endpoint, got %d", got)
	}

	hub.publishProgress(library.ProgressState{Phase: library.PhaseWatching, Folder: "/replays", Loaded: 500, Total: 500})
	finished := drain()
	if len(finished) != 3 {
		t.Fatalf("finishing should reveal once, got %+v", finished)
	}
	if finished[0].Type != "progress" || finished[1].Type != "status" || finished[2].Type != "corpus" {
		t.Fatalf("unexpected reveal: %+v", finished)
	}
	if finished[1].Status != libraryStatusWatching {
		t.Fatalf("status = %q, want watching", finished[1].Status)
	}

	// Once the read is done the watcher's own changes flow again.
	hub.publishCorpus(library.Event{Kind: library.EventCorpus, Added: []int64{7}})
	if live := drain(); len(live) != 1 || live[0].Type != "corpus" {
		t.Fatalf("a change after the read should reach the page, got %+v", live)
	}
}

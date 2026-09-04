package dashboard

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/load"
)

// Library status values as the browser sees them. The loader's phases are
// finer grained than the UI needs, so several collapse into one status.
const (
	libraryStatusIdle     = "idle"
	libraryStatusLoading  = "loading"
	libraryStatusWatching = "watching"
	libraryStatusFailed   = "failed"
)

// libraryProgress is the wire form of library.ProgressState. It is a separate
// type because the browser names the folder replay_dir everywhere else.
type libraryProgress struct {
	Generation uint64 `json:"generation"`
	Version    uint64 `json:"version"`
	Phase      string `json:"phase"`
	Total      int    `json:"total"`
	Loaded     int    `json:"loaded"`
	Failed     int    `json:"failed"`
	Skipped    int    `json:"skipped"`
	ReplayDir  string `json:"replay_dir"`
	Complete   bool   `json:"complete"`
}

type libraryCorpus struct {
	Generation uint64  `json:"generation"`
	Version    uint64  `json:"version"`
	Added      []int64 `json:"added,omitempty"`
	Removed    []int64 `json:"removed,omitempty"`
}

type libraryEventMessage struct {
	Type      string           `json:"type"`
	Status    string           `json:"status,omitempty"`
	ReplayDir string           `json:"replay_dir,omitempty"`
	Error     string           `json:"error,omitempty"`
	Progress  *libraryProgress `json:"progress,omitempty"`
	Corpus    *libraryCorpus   `json:"corpus,omitempty"`
}

func progressToWire(p library.ProgressState) libraryProgress {
	return libraryProgress{
		Generation: p.Generation,
		Version:    p.Version,
		Phase:      string(p.Phase),
		Total:      p.Total,
		Loaded:     p.Loaded,
		Failed:     p.Failed,
		Skipped:    p.Skipped,
		ReplayDir:  p.Folder,
		Complete:   p.Complete(),
	}
}

func statusForPhase(phase library.Phase) string {
	switch phase {
	case library.PhaseScanning, library.PhaseRecent, library.PhaseBackfill:
		return libraryStatusLoading
	case library.PhaseReady, library.PhaseWatching:
		return libraryStatusWatching
	case library.PhaseFailed:
		return libraryStatusFailed
	default:
		return libraryStatusIdle
	}
}

var libraryEventUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

// libraryHub turns the library's progress and corpus events into the browser's
// event stream.
type libraryHub struct {
	mu          sync.Mutex
	progress    library.ProgressState
	lastError   string
	subscribers map[chan libraryEventMessage]struct{}
}

func newLibraryHub() *libraryHub {
	return &libraryHub{subscribers: map[chan libraryEventMessage]struct{}{}}
}

// Watch consumes the library's event channel until it closes.
func (h *libraryHub) Watch(lib *library.Library) {
	progress, events, cancel := lib.Subscribe()
	h.mu.Lock()
	h.progress = progress
	h.mu.Unlock()
	go func() {
		defer crashreport.GuardNonFatal(nil)
		defer cancel()
		for event := range events {
			switch event.Kind {
			case library.EventProgress:
				h.publishProgress(event.Progress)
			case library.EventCorpus:
				h.publishCorpus(event)
			}
		}
	}()
}

// Log writes one loader line to the server log and keeps the last failure, so
// the screen can say something went wrong without listing every line.
func (h *libraryHub) Log(event load.LogEvent) {
	log.Printf("library: %s", event.Message)
	if event.Level != load.LogLevelError {
		return
	}
	h.mu.Lock()
	h.lastError = event.Message
	h.mu.Unlock()
}

// publishProgress keeps the hub's own copy current for /api/health, but tells
// the browser only when the state it renders actually changes.
//
// Reading a folder commits a batch every few hundred milliseconds. Forwarding
// each one made the page refetch its lists and re-render for as long as the
// read took, which on a large folder is a minute of a twitching screen. The
// browser needs to know that a read started, and that it finished.
func (h *libraryHub) publishProgress(p library.ProgressState) {
	h.mu.Lock()
	previous := h.progress
	h.progress = p
	h.mu.Unlock()

	status := statusForPhase(p.Phase)
	if statusForPhase(previous.Phase) == status {
		return
	}
	wire := progressToWire(p)
	h.broadcast(libraryEventMessage{Type: "progress", Status: status, ReplayDir: p.Folder, Progress: &wire})
	h.broadcast(libraryEventMessage{Type: "status", Status: status, ReplayDir: p.Folder, Error: h.Error()})
	if status == libraryStatusWatching {
		// One reveal for everything the read added.
		h.broadcast(libraryEventMessage{Type: "corpus", Corpus: &libraryCorpus{
			Generation: p.Generation,
			Version:    p.Version,
		}})
	}
}

// publishCorpus forwards the watcher's changes. Batches committed while a
// folder is still being read are covered by the single reveal above.
func (h *libraryHub) publishCorpus(event library.Event) {
	if statusForPhase(h.Progress().Phase) == libraryStatusLoading {
		return
	}
	h.broadcast(libraryEventMessage{Type: "corpus", Corpus: &libraryCorpus{
		Generation: event.Generation,
		Version:    event.Version,
		Added:      event.Added,
		Removed:    event.Removed,
	}})
}

func (h *libraryHub) Progress() library.ProgressState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.progress
}

func (h *libraryHub) Error() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastError
}

func (h *libraryHub) snapshotMessage() libraryEventMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	wire := progressToWire(h.progress)
	return libraryEventMessage{
		Type:      "snapshot",
		Status:    statusForPhase(h.progress.Phase),
		ReplayDir: h.progress.Folder,
		Error:     h.lastError,
		Progress:  &wire,
	}
}

func (h *libraryHub) subscribe() (libraryEventMessage, chan libraryEventMessage, func()) {
	ch := make(chan libraryEventMessage, 256)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if _, ok := h.subscribers[ch]; ok {
				delete(h.subscribers, ch)
				close(ch)
			}
		})
	}
	return h.snapshotMessage(), ch, unsubscribe
}

// broadcast drops messages for a subscriber that has fallen behind rather than
// stalling the loader; the next snapshot brings a reconnecting page up to date.
func (h *libraryHub) broadcast(message libraryEventMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subscribers {
		select {
		case ch <- message:
		default:
		}
	}
}

func (d *Dashboard) handlerLibraryEvents(w http.ResponseWriter, r *http.Request) {
	conn, err := libraryEventUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	snapshot, events, unsubscribe := d.libraryHub.subscribe()
	defer unsubscribe()

	if err := conn.WriteJSON(snapshot); err != nil {
		return
	}

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case message, ok := <-events:
			if !ok {
				return
			}
			if err := conn.WriteJSON(message); err != nil {
				return
			}
		case <-closed:
			return
		case <-d.ctx.Done():
			return
		}
	}
}

// corpusStamp tells a page how much of the folder the numbers it just received
// were computed from. Counts that look like a dead end while the library is
// still loading are only "nothing yet", which the browser renders differently.
func (d *Dashboard) corpusStamp() map[string]any {
	progress := d.libraryHub.Progress()
	loaded, total := progress.Loaded, progress.Total
	if total < loaded {
		total = loaded
	}
	return map[string]any{
		"generation": progress.Generation,
		"version":    progress.Version,
		"loaded":     loaded,
		"total":      total,
		"complete":   progress.Complete(),
	}
}

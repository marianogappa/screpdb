package dashboard

import (
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/marianogappa/screpdb/internal/ingest"
)

const maxIngestLogEvents = 4000

type ingestStreamMessage struct {
	Type     string            `json:"type"`
	Status   string            `json:"status,omitempty"`
	InputDir string            `json:"input_dir,omitempty"`
	Error    string            `json:"error,omitempty"`
	Log      *ingest.LogEvent  `json:"log,omitempty"`
	Logs     []ingest.LogEvent `json:"logs,omitempty"`
}

var ingestLogUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

func (d *Dashboard) newIngestLogger() *ingest.Logger {
	return ingest.NewLogger(os.Stderr, false, d.appendIngestLog)
}

func (d *Dashboard) tryStartIngest(inputDir string) bool {
	d.ingestMu.Lock()
	defer d.ingestMu.Unlock()
	if d.ingestRunning {
		return false
	}
	d.ingestRunning = true
	d.ingestStatus = "running"
	d.ingestError = ""
	d.ingestInputDir = inputDir
	d.ingestEvents = nil
	d.ingestSessionID++
	d.broadcastIngestLocked(ingestStreamMessage{
		Type:     "status",
		Status:   d.ingestStatus,
		InputDir: d.ingestInputDir,
	})
	return true
}

func (d *Dashboard) finishIngest(err error) {
	d.ingestMu.Lock()
	defer d.ingestMu.Unlock()
	d.ingestRunning = false
	if err != nil {
		d.ingestStatus = "failed"
		d.ingestError = err.Error()
	} else {
		d.ingestStatus = "completed"
		d.ingestError = ""
	}
	d.broadcastIngestLocked(ingestStreamMessage{
		Type:     "status",
		Status:   d.ingestStatus,
		InputDir: d.ingestInputDir,
		Error:    d.ingestError,
	})
}

func (d *Dashboard) appendIngestLog(event ingest.LogEvent) {
	d.ingestMu.Lock()
	defer d.ingestMu.Unlock()

	d.ingestEvents = append(d.ingestEvents, event)
	if len(d.ingestEvents) > maxIngestLogEvents {
		d.ingestEvents = append([]ingest.LogEvent(nil), d.ingestEvents[len(d.ingestEvents)-maxIngestLogEvents:]...)
	}

	eventCopy := event
	d.broadcastIngestLocked(ingestStreamMessage{
		Type: "log",
		Log:  &eventCopy,
	})
}

func (d *Dashboard) subscribeIngest() (ingestStreamMessage, chan ingestStreamMessage, func()) {
	ch := make(chan ingestStreamMessage, 256)

	d.ingestMu.Lock()
	if d.ingestSubscribers == nil {
		d.ingestSubscribers = map[chan ingestStreamMessage]struct{}{}
	}
	d.ingestSubscribers[ch] = struct{}{}
	snapshot := ingestStreamMessage{
		Type:     "snapshot",
		Status:   d.ingestStatus,
		InputDir: d.ingestInputDir,
		Error:    d.ingestError,
		Logs:     append([]ingest.LogEvent(nil), d.ingestEvents...),
	}
	d.ingestMu.Unlock()

	unsubscribe := func() {
		d.ingestMu.Lock()
		if _, ok := d.ingestSubscribers[ch]; ok {
			delete(d.ingestSubscribers, ch)
			close(ch)
		}
		d.ingestMu.Unlock()
	}

	return snapshot, ch, unsubscribe
}

func (d *Dashboard) broadcastIngestLocked(message ingestStreamMessage) {
	for subscriber := range d.ingestSubscribers {
		select {
		case subscriber <- message:
		default:
		}
	}
}

func (d *Dashboard) handlerIngestLogs(w http.ResponseWriter, r *http.Request) {
	conn, err := ingestLogUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	snapshot, events, unsubscribe := d.subscribeIngest()
	defer unsubscribe()

	if err := conn.WriteJSON(snapshot); err != nil {
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
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
		case <-done:
			return
		case <-r.Context().Done():
			return
		}
	}
}

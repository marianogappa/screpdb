package dashboard

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/marianogappa/screpdb/internal/crashreport"
)

// handlerQuit shuts screpdb down at the user's request (the dashboard "Quit"
// button). It flushes a success response before tearing the process down so the
// UI can show a clean "stopped" screen, mirroring the self-update apply handler.
// The shutdown is deferred to a goroutine so the response reaches the client
// first; it prefers the registered shutdown func (root context cancel, which
// lets the Windows launcher drop its tray and exit) and falls back to os.Exit.
func (d *Dashboard) handlerQuit(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "shutting_down": true})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		defer crashreport.GuardNonFatal(nil)
		time.Sleep(500 * time.Millisecond)
		log.Printf("quit requested from dashboard; shutting down")
		if d.shutdown != nil {
			d.shutdown()
			return
		}
		os.Exit(0)
	}()
}

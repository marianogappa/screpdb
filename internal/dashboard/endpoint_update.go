package dashboard

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/selfupdate"
)

// handlerUpdateStatus performs the read-only launch-time update check and
// reports the version, urgency tier, and whether this install can self-update.
// Registered as a manual route (not in the OpenAPI spec). Network failures are
// not surfaced as HTTP errors: the UI just shows no notice, matching the
// previous offline-tolerant behavior.
func (d *Dashboard) handlerUpdateStatus(w http.ResponseWriter, r *http.Request) {
	status, err := selfupdate.CheckStatus(r.Context())
	if err != nil {
		log.Printf("update check failed (offline or rate-limited): %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	if encErr := json.NewEncoder(w).Encode(status); encErr != nil {
		http.Error(w, encErr.Error(), http.StatusInternalServerError)
	}
}

// handlerUpdateApply downloads, verifies, and swaps the binary, then relaunches
// the new version. The swap completes before the response is written so download
// or verification failures are reported to the UI; the re-exec is deferred until
// after the response flushes. The detached context protects the critical swap
// from a client disconnect.
func (d *Dashboard) handlerUpdateApply(w http.ResponseWriter, _ *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	newVersion, err := selfupdate.Apply(ctx)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":     true,
		"new_version": newVersion,
		"restarting":  true,
	})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Relaunch after the response reaches the client. On Unix this replaces the
	// process image; on Windows it spawns the new binary and exits.
	go func() {
		defer crashreport.GuardNonFatal(nil)
		time.Sleep(750 * time.Millisecond)
		log.Printf("self-update applied (%s); relaunching", newVersion)
		if restartErr := selfupdate.Restart(); restartErr != nil {
			log.Printf("self-update relaunch failed; please restart manually: %v", restartErr)
		}
	}()
}

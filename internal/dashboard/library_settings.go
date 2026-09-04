package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	dashboardservice "github.com/marianogappa/screpdb/internal/dashboard/service"
	"github.com/marianogappa/screpdb/internal/fileops"
)

// librarySettingsResponse is what the Replay library screen renders.
type librarySettingsResponse struct {
	ReplayDir         string `json:"replay_dir"`
	IsSampleSet       bool   `json:"is_sample_set"`
	SampleAutoLoaded  bool   `json:"sample_auto_loaded"`
	DetectedReplayDir string `json:"detected_replay_dir"`
}

func (d *Dashboard) librarySettings(_ context.Context) librarySettingsResponse {
	detected, err := fileops.ResolveDefaultReplayDir()
	if err != nil {
		detected = ""
	}
	// sample_auto_loaded is read once: it exists so the first page load after a
	// first run can explain why it is showing example replays, and it must not
	// keep saying so on every later load.
	autoLoaded := d.sampleSetAutoLoaded
	d.sampleSetAutoLoaded = false
	return librarySettingsResponse{
		ReplayDir:         d.library.Folder(),
		IsSampleSet:       d.library.IsSampleSetActive(),
		SampleAutoLoaded:  autoLoaded,
		DetectedReplayDir: detected,
	}
}

func (d *Dashboard) setReplayDir(ctx context.Context, dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return dashboardservice.WithStatus(http.StatusBadRequest, errors.New("a replay folder is required"))
	}
	if err := d.library.SetFolder(ctx, dir); err != nil {
		return dashboardservice.WithStatus(http.StatusBadRequest, err)
	}
	d.invalidateFingerprintCache()
	return nil
}

func (d *Dashboard) handlerLibraryRescan(w http.ResponseWriter, _ *http.Request) {
	d.library.Rescan()
	writeJSON(w, map[string]any{"ok": true})
}

func (d *Dashboard) handlerLoadSampleSet(w http.ResponseWriter, r *http.Request) {
	dir, err := d.library.UseSampleSet(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d.invalidateFingerprintCache()
	writeJSON(w, map[string]any{"ok": true, "replay_dir": dir})
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

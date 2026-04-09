package dashboard

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/marianogappa/screpdb/internal/ingest"
)

type ingestRequest struct {
	InputDir         string `json:"input_dir"`
	SQLitePath       string `json:"sqlite_path"`
	Watch            bool   `json:"watch"`
	StoreRightClicks bool   `json:"store_right_clicks"`
	SkipHotkeys      bool   `json:"skip_hotkeys"`
	StopAfterN       int    `json:"stop_after_n_reps"`
	UpToDate         string `json:"up_to_yyyy_mm_dd"`
	UpToMonths       int    `json:"up_to_n_months"`
	Clean            bool   `json:"clean"`
	CleanDashboard   bool   `json:"clean_dashboard"`
}

func (d *Dashboard) handlerIngest(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	inputDir := strings.TrimSpace(req.InputDir)
	if inputDir != "" {
		if err := d.setIngestInputDir(r.Context(), inputDir); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		var err error
		inputDir, err = d.getIngestInputDir(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if inputDir == "" {
			http.Error(w, "replay folder is not configured", http.StatusBadRequest)
			return
		}
	}

	cfg := ingest.Config{
		InputDir:         inputDir,
		SQLitePath:       strings.TrimSpace(req.SQLitePath),
		Watch:            req.Watch,
		StoreRightClicks: req.StoreRightClicks,
		SkipHotkeys:      req.SkipHotkeys,
		StopAfterN:       req.StopAfterN,
		UpToDate:         strings.TrimSpace(req.UpToDate),
		UpToMonths:       req.UpToMonths,
		Clean:            req.Clean,
		CleanDashboard:   req.CleanDashboard,
		HandleSignals:    false,
		UseColor:         false,
		Logger:           d.newIngestLogger(),
	}
	if cfg.SQLitePath == "" {
		cfg.SQLitePath = d.sqlitePath
	}

	if !d.tryStartIngest(cfg.InputDir) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":          true,
			"started":     false,
			"in_progress": true,
			"input_dir":   inputDir,
			"sqlitePath":  cfg.SQLitePath,
		})
		return
	}

	go func() {
		err := ingest.Run(d.ctx, cfg)
		if err != nil {
			cfg.Logger.Errorf("Ingestion failed: %v", err)
		}
		d.finishIngest(err)
	}()

	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":         true,
		"started":    true,
		"input_dir":  cfg.InputDir,
		"sqlitePath": cfg.SQLitePath,
	})
}

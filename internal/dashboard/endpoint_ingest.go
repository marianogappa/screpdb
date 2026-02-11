package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/ingest"
)

type ingestRequest struct {
	InputDir       string `json:"input_dir"`
	SQLitePath     string `json:"sqlite_path"`
	Watch          bool   `json:"watch"`
	StopAfterN     int    `json:"stop_after_n_reps"`
	UpToDate       string `json:"up_to_yyyy_mm_dd"`
	UpToMonths     int    `json:"up_to_n_months"`
	Clean          bool   `json:"clean"`
	CleanDashboard bool   `json:"clean_dashboard"`
}

func (d *Dashboard) handlerIngest(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	cfg := ingest.Config{
		InputDir:       strings.TrimSpace(req.InputDir),
		SQLitePath:     strings.TrimSpace(req.SQLitePath),
		Watch:          req.Watch,
		StopAfterN:     req.StopAfterN,
		UpToDate:       strings.TrimSpace(req.UpToDate),
		UpToMonths:     req.UpToMonths,
		Clean:          req.Clean,
		CleanDashboard: req.CleanDashboard,
		HandleSignals:  false,
		UseColor:       false,
	}

	if cfg.InputDir == "" {
		cfg.InputDir = fileops.GetDefaultReplayDir()
	}
	if cfg.SQLitePath == "" {
		cfg.SQLitePath = d.sqlitePath
	}

	go func() {
		if err := ingest.Run(d.ctx, cfg); err != nil {
			log.Printf("ingestion failed: %v", err)
		}
	}()

	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":        true,
		"started":   true,
		"sqlitePath": cfg.SQLitePath,
	})
}

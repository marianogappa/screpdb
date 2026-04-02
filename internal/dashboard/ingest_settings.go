package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/marianogappa/screpdb/internal/fileops"
)

type ingestSettingsResponse struct {
	InputDir string `json:"input_dir"`
}

type updateIngestSettingsRequest struct {
	InputDir string `json:"input_dir"`
}

func (d *Dashboard) initializeIngestSettings(ctx context.Context) error {
	inputDir, err := d.getIngestInputDir(ctx)
	if err != nil {
		return err
	}
	if inputDir != "" {
		return nil
	}

	defaultDir, err := fileops.ResolveDefaultReplayDir()
	if err != nil {
		return nil
	}
	if err := d.setIngestInputDir(ctx, defaultDir); err != nil {
		return err
	}
	log.Printf("Resolved ingest replay folder to %s", defaultDir)
	return nil
}

func (d *Dashboard) getIngestInputDir(ctx context.Context) (string, error) {
	var inputDir string
	err := d.db.QueryRowContext(ctx, `
		SELECT ingest_input_dir
		FROM settings
		WHERE config_key = ?
	`, globalReplayFilterConfigKey).Scan(&inputDir)
	if err != nil {
		return "", fmt.Errorf("failed to load ingest replay folder: %w", err)
	}
	return strings.TrimSpace(inputDir), nil
}

func (d *Dashboard) setIngestInputDir(ctx context.Context, inputDir string) error {
	inputDir = strings.TrimSpace(inputDir)
	if err := fileops.ValidateReplayDir(inputDir); err != nil {
		return err
	}

	if _, err := d.db.ExecContext(ctx, `
		UPDATE settings
		SET ingest_input_dir = ?, updated_at = CURRENT_TIMESTAMP
		WHERE config_key = ?
	`, inputDir, globalReplayFilterConfigKey); err != nil {
		return fmt.Errorf("failed to save ingest replay folder: %w", err)
	}
	return nil
}

func (d *Dashboard) handlerGetIngestSettings(w http.ResponseWriter, r *http.Request) {
	inputDir, err := d.getIngestInputDir(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(ingestSettingsResponse{InputDir: inputDir})
}

func (d *Dashboard) handlerUpdateIngestSettings(w http.ResponseWriter, r *http.Request) {
	var req updateIngestSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if err := d.setIngestInputDir(r.Context(), req.InputDir); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_ = json.NewEncoder(w).Encode(ingestSettingsResponse{
		InputDir: strings.TrimSpace(req.InputDir),
	})
}

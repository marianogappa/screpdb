package dashboard

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/iofacade"
)

type ingestSettingsResponse struct {
	InputDir string `json:"input_dir"`
}

func (d *Dashboard) initializeIngestSettings(ctx context.Context) error {
	inputDir, err := d.getIngestInputDir(ctx)
	if err != nil {
		return err
	}
	if inputDir != "" {
		d.refreshYouAliasesBestEffort(ctx)
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
	d.refreshYouAliasesBestEffort(ctx)
	return nil
}

func (d *Dashboard) getIngestInputDir(ctx context.Context) (string, error) {
	inputDir, err := d.dbStore.GetIngestInputDir(ctx, globalReplayFilterConfigKey)
	if err != nil {
		return "", fmt.Errorf("failed to load ingest replay folder: %w", err)
	}
	// The replays folder is a permitted I/O root; register it as soon as we
	// learn it (it is user-configured and stored in the DB, not known until now).
	_ = iofacade.AllowDir(inputDir)
	return inputDir, nil
}

func (d *Dashboard) setIngestInputDir(ctx context.Context, inputDir string) error {
	inputDir = strings.TrimSpace(inputDir)
	// Allow before validating: ValidateReplayDir stats/walks the folder through
	// the facade, which would otherwise reject a not-yet-permitted root.
	_ = iofacade.AllowDir(inputDir)
	if err := fileops.ValidateReplayDir(inputDir); err != nil {
		return err
	}

	if err := d.dbStore.SetIngestInputDir(ctx, globalReplayFilterConfigKey, inputDir); err != nil {
		return fmt.Errorf("failed to save ingest replay folder: %w", err)
	}
	d.refreshYouAliasesBestEffort(ctx)
	return nil
}

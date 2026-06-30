package dashboard

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/ingest"
	"github.com/marianogappa/screpdb/internal/sampledata"
)

const sampleSetDirName = "sample_replays"

// sampleSetDir is the deterministic folder the embedded sample replays are
// extracted to: alongside the SQLite database. Recognising "the sample set is
// active" is then just comparing this path to the configured ingest folder.
func (d *Dashboard) sampleSetDir() string {
	dir := filepath.Join(filepath.Dir(d.sqlitePath), sampleSetDirName)
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(strings.TrimSpace(a))
	absB, errB := filepath.Abs(strings.TrimSpace(b))
	if errA != nil || errB != nil {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return filepath.Clean(absA) == filepath.Clean(absB)
}

// isSampleSetActive reports whether the configured ingest folder is the
// extracted sample-set directory.
func (d *Dashboard) isSampleSetActive(ctx context.Context) (bool, error) {
	inputDir, err := d.getIngestInputDir(ctx)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(inputDir) == "" {
		return false, nil
	}
	return samePath(inputDir, d.sampleSetDir()), nil
}

// prepareSampleSet extracts the embedded replays to the sample-set directory
// and points ingest at it, without starting an ingest. Safe to call during
// dashboard construction (it does not open a second DB connection).
func (d *Dashboard) prepareSampleSet(ctx context.Context) error {
	dir := d.sampleSetDir()
	if err := sampledata.Extract(dir); err != nil {
		return fmt.Errorf("extract sample set: %w", err)
	}
	if err := d.setIngestInputDir(ctx, dir); err != nil {
		return fmt.Errorf("point ingest at sample set: %w", err)
	}
	return nil
}

// loadSampleSet prepares the sample set and starts an asynchronous ingest that
// erases existing data first (Clean) so the example set is a clean, disjoint
// exploration database rather than a few games mixed into the user's library.
// Call only after the dashboard is fully constructed (an ingest opens its own
// DB connection, which races DB setup if done during New).
func (d *Dashboard) loadSampleSet(ctx context.Context) error {
	if err := d.prepareSampleSet(ctx); err != nil {
		return err
	}
	d.startIngestAsync(ingest.Config{InputDir: d.sampleSetDir(), Clean: true})
	return nil
}

// StartPendingSampleIngest runs the deferred sample-set ingest queued at
// startup (when the sample set was auto-loaded because the user's replay folder
// could not be resolved). It is a no-op otherwise. Call once the HTTP server is
// up and the database is quiescent.
func (d *Dashboard) StartPendingSampleIngest() {
	if !d.pendingSampleIngest {
		return
	}
	d.pendingSampleIngest = false
	d.startIngestAsync(ingest.Config{InputDir: d.sampleSetDir(), Clean: true})
}

// startIngestAsync runs ingest.Run in the background using the dashboard's
// ingest session machinery. It returns false if an ingest is already running.
func (d *Dashboard) startIngestAsync(cfg ingest.Config) bool {
	if strings.TrimSpace(cfg.SQLitePath) == "" {
		cfg.SQLitePath = d.sqlitePath
	}
	if cfg.Logger == nil {
		cfg.Logger = d.newIngestLogger()
	}
	if !d.tryStartIngest(cfg.InputDir) {
		return false
	}
	go func() {
		defer crashreport.Guard()
		runErr := ingest.Run(d.ctx, cfg)
		if runErr != nil {
			cfg.Logger.Errorf("Ingestion failed: %v", runErr)
		}
		d.finishIngest(runErr)
	}()
	return true
}

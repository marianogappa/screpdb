package ingest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/profile"
	"github.com/marianogappa/screpdb/internal/storage"
	"golang.org/x/sync/errgroup"
)

// errSkippedUMS is returned by processFileToChannel when a replay parses
// successfully but is excluded by the global UMS auto-discard policy. Callers
// use errors.Is to count these as skips, not errors.
var errSkippedUMS = errors.New("replay skipped: UMS not supported")

type Config struct {
	InputDir         string
	SQLitePath       string
	StoreRightClicks bool
	SkipHotkeys      bool
	StopAfterN       int
	UpToDate         string
	UpToMonths       int
	Clean            bool
	CleanDashboard   bool
	UseColor         bool
	Logger           *Logger

	// EarlyFilterDebugDir, when non-empty, makes the early-game spam
	// filter dump a per-replay JSON trace into this directory. Sourced
	// from the SCREPDB_EARLY_FILTER_DEBUG_DIR env var by cmd/ingest.go;
	// not user-facing.
	EarlyFilterDebugDir string

	// ProfileMode controls the SCREPDB_INGEST_PROFILE behavior. When
	// non-Off, per-replay phase timings are emitted to stderr and an
	// aggregate p50/p95 is printed at end of run.
	ProfileMode profile.Mode

	// CPUProfilePath, when non-empty, enables a runtime/pprof CPU profile
	// for the duration of Run. Sourced from SCREPDB_INGEST_PPROF by
	// cmd/ingest.go. Inspect with `go tool pprof <path>`.
	CPUProfilePath string
}

func Run(ctx context.Context, cfg Config) error {
	cfg = withDefaults(cfg)
	logger := cfg.Logger

	// The replays folder is a permitted I/O root (issue #135). Register it so
	// the facade allows discovering/reading replays under it.
	if err := iofacade.AllowDir(cfg.InputDir); err != nil {
		return fmt.Errorf("failed to register replay folder: %w", err)
	}

	if cfg.CPUProfilePath != "" {
		stop, err := startCPUProfile(cfg.CPUProfilePath)
		if err != nil {
			return fmt.Errorf("failed to start CPU profile: %w", err)
		}
		logger.Infof("CPU profile writing to %s", cfg.CPUProfilePath)
		defer stop()
	}

	// Initialize storage
	logger.Infof("Using SQLite storage at %s", cfg.SQLitePath)
	store, err := storage.NewSQLiteStorage(cfg.SQLitePath)
	if err != nil {
		return fmt.Errorf("failed to create SQLite storage: %w", err)
	}
	defer store.Close()
	store.SetCommandStorageOptions(cfg.StoreRightClicks, cfg.SkipHotkeys)

	if err := store.Initialize(ctx, cfg.Clean, cfg.CleanDashboard); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	sink := profile.NewSink(cfg.ProfileMode)
	defer sink.Aggregate()

	return runBatchMode(ctx, store, cfg, logger, sink)
}

// parserOptions translates ingest.Config into parser.Options.
func parserOptions(cfg Config) parser.Options {
	return parser.Options{EarlyFilterDebugDir: cfg.EarlyFilterDebugDir}
}

func withDefaults(cfg Config) Config {
	if cfg.InputDir == "" {
		cfg.InputDir = fileops.GetDefaultReplayDir()
	}
	if cfg.SQLitePath == "" {
		cfg.SQLitePath = "screp.db"
	}
	if cfg.Logger == nil {
		cfg.Logger = NewLogger(os.Stderr, cfg.UseColor, nil)
	}
	return cfg
}

func runBatchMode(ctx context.Context, store storage.Storage, cfg Config, logger *Logger, sink *profile.Sink) error {
	logger.Infof("Scanning directory: %s", cfg.InputDir)
	logger.Infof("Using %d CPU cores for parsing", runtime.GOMAXPROCS(0))

	// Start the ingestion process
	dataChan, errChan := store.StartIngestion(ctx, storage.IngestionHooks{
		OnReplayStored: logger.Progress,
		OnDuplicateReplay: func(err error) {
			// Silently swallow duplicate-replay errors. The pre-check at
			// batchCheckExistingReplays already filters known duplicates;
			// a race-condition straggler reaching the storage layer
			// shouldn't pollute the log. Aggregate "skipped" counter still
			// surfaces them.
			_ = err
		},
		OnStoreError: func(err error) {
			logger.Errorf("Error storing replay: %v", err)
		},
	})

	// Walk the directory without hashing yet — checksums get computed lazily
	// only for files that survive the cheap path-based dedup pass below.
	files, err := fileops.WalkReplayFiles(cfg.InputDir)
	if err != nil {
		return fmt.Errorf("failed to get replay files: %w", err)
	}

	logger.Successf("Found %d replay files", len(files))

	// Sort by modification time (newest first)
	fileops.SortFilesByModTime(files)

	// Apply date filters
	var upToDatePtr *time.Time
	if cfg.UpToDate != "" {
		parsed, err := time.Parse("2006-01-02", cfg.UpToDate)
		if err != nil {
			return fmt.Errorf("invalid date format: %w", err)
		}
		upToDatePtr = &parsed
	}

	var upToMonthsPtr *int
	if cfg.UpToMonths > 0 {
		upToMonthsPtr = &cfg.UpToMonths
	}

	filteredFiles := fileops.FilterFilesByDate(files, upToDatePtr, upToMonthsPtr)
	logger.Successf("After date filtering: %d files", len(filteredFiles))

	// Apply count limit
	if cfg.StopAfterN > 0 {
		filteredFiles = fileops.LimitFiles(filteredFiles, cfg.StopAfterN)
		logger.Successf("Limited to %d files", len(filteredFiles))
	}

	// Phase 1: cheap path-based dedup. Cuts I/O dramatically on incremental
	// re-scans of the same replay folder by skipping the SHA256 read of
	// every file that's already in the DB by file_path.
	logger.Warnf("Checking existing replays by path...")
	pathSurvivors, err := batchCheckExistingReplaysByPath(ctx, store, filteredFiles, logger)
	if err != nil {
		return fmt.Errorf("failed to check existing replays by path: %w", err)
	}
	pathSkipped := len(filteredFiles) - len(pathSurvivors)
	logger.Warnf("Skipped %d files already known by path", pathSkipped)

	// Phase 2: hash the survivors in parallel, then run checksum-based dedup
	// to catch the rename/move case (same content, new path).
	logger.Warnf("Hashing %d candidate files...", len(pathSurvivors))
	hashed, err := fileops.HashFiles(ctx, pathSurvivors)
	if err != nil {
		return fmt.Errorf("failed to hash candidate files: %w", err)
	}

	logger.Warnf("Checking existing replays by checksum...")
	filesToProcess, err := batchCheckExistingReplays(ctx, store, hashed, logger)
	if err != nil {
		return fmt.Errorf("failed to check existing replays: %w", err)
	}

	skippedCount := len(filteredFiles) - len(filesToProcess)
	logger.Warnf("Skipping %d existing replays", skippedCount)
	logger.Successf("Processing %d new replays", len(filesToProcess))

	// Process files in batches of 100
	const batchSize = 100
	var processed, errCount, skippedUMS int64
	var mu sync.Mutex

	for i := 0; i < len(filesToProcess); i += batchSize {
		end := min(i+batchSize, len(filesToProcess))

		batch := filesToProcess[i:end]
		logger.Infof("Processing batch %d-%d of %d", i+1, end, len(filesToProcess))

		// Create errgroup with context for this batch
		g, gCtx := errgroup.WithContext(ctx)

		for _, fileInfo := range batch {
			fileInfo := fileInfo // capture loop variable

			g.Go(func() error {
				if err := processFileToChannel(gCtx, dataChan, &fileInfo, parserOptions(cfg), sink); err != nil {
					if errors.Is(err, errSkippedUMS) {
						mu.Lock()
						skippedUMS++
						mu.Unlock()
						logger.Warnf("Skipping UMS replay: %s", fileInfo.Name)
						return nil
					}
					logger.Errorf("Error processing file %s: %v", fileInfo.Name, err)
					mu.Lock()
					errCount++
					mu.Unlock()
					return nil // Don't stop processing on errors
				}

				mu.Lock()
				processed++
				mu.Unlock()
				return nil
			})
		}

		// Wait for this batch to complete
		if err := g.Wait(); err != nil {
			logger.Errorf("Error during batch processing: %v", err)
		}
	}

	// Close the data channel to signal completion
	close(dataChan)

	// Wait for storage to finish
	if err := <-errChan; err != nil {
		return fmt.Errorf("storage error: %w", err)
	}

	logger.Successf("Processing complete: processed=%d skipped_existing=%d skipped_ums=%d errors=%d", processed, skippedCount, skippedUMS, errCount)

	return nil
}

// batchCheckExistingReplays checks for existing replays in batches of 100
// Returns only the FileInfo objects for replays that don't exist yet
func batchCheckExistingReplays(ctx context.Context, store storage.Storage, files []fileops.FileInfo, logger *Logger) ([]fileops.FileInfo, error) {
	const batchSize = 100
	var allFiltered []fileops.FileInfo

	for i := 0; i < len(files); i += batchSize {
		end := min(i+batchSize, len(files))

		batch := files[i:end]
		batchFiltered, err := store.FilterOutExistingReplays(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to check batch %d-%d: %w", i+1, end, err)
		}

		// Append filtered results
		allFiltered = append(allFiltered, batchFiltered...)

		skippedInBatch := len(batch) - len(batchFiltered)
		logger.Infof("Checked batch %d-%d: %d existing replays found", i+1, end, skippedInBatch)
	}

	return allFiltered, nil
}

// batchCheckExistingReplaysByPath runs the cheap path-only dedup pass against the
// DB. Same batching shape as batchCheckExistingReplays but does not require
// Checksum to be populated, so it can run before the SHA256 hashing step.
func batchCheckExistingReplaysByPath(ctx context.Context, store storage.Storage, files []fileops.FileInfo, logger *Logger) ([]fileops.FileInfo, error) {
	const batchSize = 100
	var allFiltered []fileops.FileInfo

	for i := 0; i < len(files); i += batchSize {
		end := min(i+batchSize, len(files))

		batch := files[i:end]
		batchFiltered, err := store.FilterOutExistingReplaysByPath(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to check batch %d-%d: %w", i+1, end, err)
		}

		allFiltered = append(allFiltered, batchFiltered...)

		skippedInBatch := len(batch) - len(batchFiltered)
		logger.Infof("Path-checked batch %d-%d: %d existing paths found", i+1, end, skippedInBatch)
	}

	return allFiltered, nil
}

// RunForFiles drives the standard ingest pipeline for an explicit list of files,
// skipping the directory walk and date/count filters used by Run. Callers that already
// know which .rep files to ingest (e.g. the bulk re-analyze flow) use this entry point
// so they don't pay for re-scanning the entire replay folder. The shared
// StartIngestion machinery handles dedup, batching, and pattern detection identically
// to a full ingest run.
func RunForFiles(ctx context.Context, cfg Config, files []fileops.FileInfo) error {
	cfg = withDefaults(cfg)
	logger := cfg.Logger

	if len(files) == 0 {
		return nil
	}

	store, err := storage.NewSQLiteStorage(cfg.SQLitePath)
	if err != nil {
		return fmt.Errorf("failed to create SQLite storage: %w", err)
	}
	defer store.Close()
	store.SetCommandStorageOptions(cfg.StoreRightClicks, cfg.SkipHotkeys)

	if err := store.Initialize(ctx, false, false); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	sink := profile.NewSink(cfg.ProfileMode)
	defer sink.Aggregate()

	dataChan, errChan := store.StartIngestion(ctx, storage.IngestionHooks{
		OnReplayStored: logger.Progress,
		OnDuplicateReplay: func(err error) {
			_ = err
		},
		OnStoreError: func(err error) {
			logger.Errorf("Error storing replay: %v", err)
		},
	})

	const batchSize = 100
	var processed, errCount, skippedUMS int64
	var mu sync.Mutex

	for i := 0; i < len(files); i += batchSize {
		end := min(i+batchSize, len(files))
		batch := files[i:end]
		logger.Infof("Re-analyzing batch %d-%d of %d", i+1, end, len(files))

		g, gCtx := errgroup.WithContext(ctx)
		for _, fileInfo := range batch {
			fileInfo := fileInfo
			g.Go(func() error {
				if err := processFileToChannel(gCtx, dataChan, &fileInfo, parserOptions(cfg), sink); err != nil {
					if errors.Is(err, errSkippedUMS) {
						mu.Lock()
						skippedUMS++
						mu.Unlock()
						logger.Warnf("Skipping UMS replay: %s", fileInfo.Name)
						return nil
					}
					logger.Errorf("Error re-analyzing file %s: %v", fileInfo.Name, err)
					mu.Lock()
					errCount++
					mu.Unlock()
					return nil
				}
				mu.Lock()
				processed++
				mu.Unlock()
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			logger.Errorf("Error during re-analyze batch: %v", err)
		}
	}

	close(dataChan)
	if err := <-errChan; err != nil {
		return fmt.Errorf("storage error: %w", err)
	}

	logger.Successf("Re-analyze complete: processed=%d skipped_ums=%d errors=%d", processed, skippedUMS, errCount)
	return nil
}

func processFileToChannel(ctx context.Context, dataChan storage.ReplayDataChannel, fileInfo *fileops.FileInfo, opts parser.Options, sink *profile.Sink) error {
	return runGuarded(func() error {
		// Create replay model from file info
		replay := parser.CreateReplayFromFileInfo(fileInfo.Path, fileInfo.Name, fileInfo.Size, fileInfo.Checksum)

		// Begin per-replay profile run; nil-safe when sink is disabled.
		run := sink.Replay(fileInfo.Name)

		// Parse the replay
		stop := run.Phase("parse")
		data, err := parser.ParseReplayWithOptions(fileInfo.Path, replay, opts)
		stop()
		if err != nil {
			return fmt.Errorf("failed to parse replay: %w", err)
		}

		// UMS replays are unsupported — drop them before they reach storage.
		// Existing UMS rows in older DBs are filtered out at query time
		// (see global filter compiler); this prevents new ones from landing.
		if data.Replay != nil && data.Replay.MapKind == "UseMapSettings" {
			return errSkippedUMS
		}

		data.Profile = run

		// Send to storage channel
		select {
		case dataChan <- data:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}

// runGuarded runs fn under a panic guard. Parsing + detection runs a lot of
// data-dependent code (screp, scmapanalyzer, the pattern detectors) against
// replays that can be old, truncated, or from unusual game modes. Without this,
// a single replay that trips a nil-deref or out-of-range index in any of that
// code would crash the whole ingest goroutine — and the entire run. Recover it,
// turn it into a normal per-file error (the caller logs it and bumps the error
// counter), and keep ingesting the rest. The stack is included so a tester can
// paste it into a bug report (issue #165).
//
// NOTE: recover cannot catch a runtime fatal such as "concurrent map writes";
// the parse/detect path was audited to be free of shared mutable state so that
// the only crash mode left here is an ordinary, recoverable panic.
func runGuarded(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while processing replay (this is a bug — please report it at "+
				"https://github.com/marianogappa/screpdb/issues): %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}

// startCPUProfile opens path for writing and begins a runtime/pprof CPU profile.
// The returned stop function ends the profile and closes the file. Caller must
// invoke stop (e.g. via defer) so the profile is flushed.
func startCPUProfile(path string) (func(), error) {
	f, err := iofacade.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create profile file: %w", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("start CPU profile: %w", err)
	}
	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}, nil
}

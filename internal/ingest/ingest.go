package ingest

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/storage"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	InputDir         string
	SQLitePath       string
	Watch            bool
	StoreRightClicks bool
	SkipHotkeys      bool
	StopAfterN       int
	UpToDate         string
	UpToMonths       int
	Clean            bool
	CleanDashboard   bool
	HandleSignals    bool
	UseColor         bool
	Logger           *Logger
}

func Run(ctx context.Context, cfg Config) error {
	cfg = withDefaults(cfg)
	logger := cfg.Logger

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

	if cfg.Watch {
		return runWatchMode(ctx, store, cfg, logger)
	}

	return runBatchMode(ctx, store, cfg, logger)
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

func runWatchMode(ctx context.Context, store storage.Storage, cfg Config, logger *Logger) error {
	logger.Infof("Watching directory: %s", cfg.InputDir)
	logger.Infof("Using %d CPU cores for parsing", runtime.GOMAXPROCS(0))
	logger.Warnf("Press Ctrl+C to stop...")

	// Start the ingestion process
	dataChan, errChan := store.StartIngestion(ctx, storage.IngestionHooks{
		OnReplayStored: logger.Progress,
		OnDuplicateReplay: func(err error) {
			logger.Warnf("Skipping duplicate replay: %v", err)
		},
		OnStoreError: func(err error) {
			logger.Errorf("Error storing replay: %v", err)
		},
	})

	watcher, err := fileops.NewFileWatcher(cfg.InputDir)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Create errgroup with context for concurrent processing
	g, gCtx := errgroup.WithContext(ctx)

	// Handle graceful shutdown (optional)
	var sigChan chan os.Signal
	if cfg.HandleSignals {
		sigChan = make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	}

	for {
		select {
		case fileInfo := <-watcher.Events():
			logger.Successf("New file detected: %s", fileInfo.Name)

			// Process file concurrently
			g.Go(func() error {
				if err := processFileToChannel(gCtx, dataChan, &fileInfo); err != nil {
					logger.Errorf("Error processing file %s: %v", fileInfo.Name, err)
					return nil // Don't stop processing on errors
				}

				logger.Successf("Successfully processed: %s", fileInfo.Name)
				return nil
			})

		case err := <-watcher.Errors():
			logger.Errorf("Watcher error: %v", err)

		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("storage error: %w", err)
			}

		case <-ctx.Done():
			close(dataChan)
			return ctx.Err()

		case <-signalChan(sigChan):
			logger.Warnf("Shutting down...")
			close(dataChan)
			return nil
		}
	}
}

func runBatchMode(ctx context.Context, store storage.Storage, cfg Config, logger *Logger) error {
	logger.Infof("Scanning directory: %s", cfg.InputDir)
	logger.Infof("Using %d CPU cores for parsing", runtime.GOMAXPROCS(0))

	// Start the ingestion process
	dataChan, errChan := store.StartIngestion(ctx, storage.IngestionHooks{
		OnReplayStored: logger.Progress,
		OnDuplicateReplay: func(err error) {
			logger.Warnf("Skipping duplicate replay: %v", err)
		},
		OnStoreError: func(err error) {
			logger.Errorf("Error storing replay: %v", err)
		},
	})

	// Get all replay files
	files, err := fileops.GetReplayFiles(cfg.InputDir)
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

	// Batch check for existing replays before processing
	logger.Warnf("Checking for existing replays...")
	filesToProcess, err := batchCheckExistingReplays(ctx, store, filteredFiles, logger)
	if err != nil {
		return fmt.Errorf("failed to check existing replays: %w", err)
	}

	skippedCount := len(filteredFiles) - len(filesToProcess)
	logger.Warnf("Skipping %d existing replays", skippedCount)
	logger.Successf("Processing %d new replays", len(filesToProcess))

	// Process files in batches of 100
	const batchSize = 100
	var processed, errors int64
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
				if err := processFileToChannel(gCtx, dataChan, &fileInfo); err != nil {
					logger.Errorf("Error processing file %s: %v", fileInfo.Name, err)
					mu.Lock()
					errors++
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

	logger.Successf("Processing complete: processed=%d skipped=%d errors=%d", processed, skippedCount, errors)

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

func processFileToChannel(ctx context.Context, dataChan storage.ReplayDataChannel, fileInfo *fileops.FileInfo) error {
	// Create replay model from file info
	replay := parser.CreateReplayFromFileInfo(fileInfo.Path, fileInfo.Name, fileInfo.Size, fileInfo.Checksum)

	// Parse the replay
	data, err := parser.ParseReplay(fileInfo.Path, replay)
	if err != nil {
		return fmt.Errorf("failed to parse replay: %w", err)
	}

	// Send to storage channel
	select {
	case dataChan <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func signalChan(sigChan chan os.Signal) <-chan os.Signal {
	if sigChan == nil {
		return nil
	}
	return sigChan
}

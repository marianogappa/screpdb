package ingest

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/storage"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	InputDir       string
	SQLitePath     string
	Watch          bool
	StopAfterN     int
	UpToDate       string
	UpToMonths     int
	Clean          bool
	CleanDashboard bool
	HandleSignals  bool
	UseColor       bool
}

func Run(ctx context.Context, cfg Config) error {
	cfg = withDefaults(cfg)

	// Initialize storage
	logInfo(cfg.UseColor, "Using SQLite storage at %s", cfg.SQLitePath)
	store, err := storage.NewSQLiteStorage(cfg.SQLitePath)
	if err != nil {
		return fmt.Errorf("failed to create SQLite storage: %w", err)
	}
	defer store.Close()

	if err := store.Initialize(ctx, cfg.Clean, cfg.CleanDashboard); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	if cfg.Watch {
		return runWatchMode(ctx, store, cfg)
	}

	return runBatchMode(ctx, store, cfg)
}

func withDefaults(cfg Config) Config {
	if cfg.InputDir == "" {
		cfg.InputDir = fileops.GetDefaultReplayDir()
	}
	if cfg.SQLitePath == "" {
		cfg.SQLitePath = "screp.db"
	}
	color.NoColor = !cfg.UseColor
	return cfg
}

func runWatchMode(ctx context.Context, store storage.Storage, cfg Config) error {
	logInfo(cfg.UseColor, "Watching directory: %s", cfg.InputDir)
	logInfo(cfg.UseColor, "Using %d CPU cores for parsing", runtime.GOMAXPROCS(0))
	logWarn(cfg.UseColor, "Press Ctrl+C to stop...")

	// Start the ingestion process
	dataChan, errChan := store.StartIngestion(ctx)

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
			logSuccess(cfg.UseColor, "New file detected: %s", fileInfo.Name)

			// Process file concurrently
			g.Go(func() error {
				if err := processFileToChannel(gCtx, dataChan, &fileInfo); err != nil {
					log.Printf("Error processing file %s: %v", fileInfo.Name, err)
					return nil // Don't stop processing on errors
				}

				logSuccess(cfg.UseColor, "Successfully processed: %s", fileInfo.Name)
				return nil
			})

		case err := <-watcher.Errors():
			logError(cfg.UseColor, "Watcher error: %v", err)

		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("storage error: %w", err)
			}

		case <-ctx.Done():
			close(dataChan)
			return ctx.Err()

		case <-signalChan(sigChan):
			logWarn(cfg.UseColor, "\nShutting down...")
			close(dataChan)
			return nil
		}
	}
}

func runBatchMode(ctx context.Context, store storage.Storage, cfg Config) error {
	logInfo(cfg.UseColor, "Scanning directory: %s", cfg.InputDir)
	logInfo(cfg.UseColor, "Using %d CPU cores for parsing", runtime.GOMAXPROCS(0))

	// Start the ingestion process
	dataChan, errChan := store.StartIngestion(ctx)

	// Get all replay files
	files, err := fileops.GetReplayFiles(cfg.InputDir)
	if err != nil {
		return fmt.Errorf("failed to get replay files: %w", err)
	}

	logSuccess(cfg.UseColor, "Found %d replay files", len(files))

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
	logSuccess(cfg.UseColor, "After date filtering: %d files", len(filteredFiles))

	// Apply count limit
	if cfg.StopAfterN > 0 {
		filteredFiles = fileops.LimitFiles(filteredFiles, cfg.StopAfterN)
		logSuccess(cfg.UseColor, "Limited to %d files", len(filteredFiles))
	}

	// Batch check for existing replays before processing
	logWarn(cfg.UseColor, "Checking for existing replays...")
	filesToProcess, err := batchCheckExistingReplays(ctx, store, filteredFiles)
	if err != nil {
		return fmt.Errorf("failed to check existing replays: %w", err)
	}

	skippedCount := len(filteredFiles) - len(filesToProcess)
	logWarn(cfg.UseColor, "Skipping %d existing replays", skippedCount)
	logSuccess(cfg.UseColor, "Processing %d new replays", len(filesToProcess))

	// Process files in batches of 100
	const batchSize = 100
	var processed, errors int64
	var mu sync.Mutex

	for i := 0; i < len(filesToProcess); i += batchSize {
		end := min(i+batchSize, len(filesToProcess))

		batch := filesToProcess[i:end]
		logInfo(cfg.UseColor, "Processing batch %d-%d of %d", i+1, end, len(filesToProcess))

		// Create errgroup with context for this batch
		g, gCtx := errgroup.WithContext(ctx)

		for j, fileInfo := range batch {
			j, fileInfo := j, fileInfo // capture loop variables

			g.Go(func() error {
				log.Printf("Processing %d/%d: %s", i+j+1, len(filesToProcess), fileInfo.Name)

				if err := processFileToChannel(gCtx, dataChan, &fileInfo); err != nil {
					log.Printf("Error processing file %s: %v", fileInfo.Name, err)
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
			log.Printf("Error during batch processing: %v", err)
		}
	}

	// Close the data channel to signal completion
	close(dataChan)

	// Wait for storage to finish
	if err := <-errChan; err != nil {
		return fmt.Errorf("storage error: %w", err)
	}

	logSuccess(cfg.UseColor, "\nProcessing complete:")
	logSuccess(cfg.UseColor, "  Processed: %d", processed)
	logWarn(cfg.UseColor, "  Skipped: %d", skippedCount)
	logError(cfg.UseColor, "  Errors: %d", errors)

	return nil
}

// batchCheckExistingReplays checks for existing replays in batches of 100
// Returns only the FileInfo objects for replays that don't exist yet
func batchCheckExistingReplays(ctx context.Context, store storage.Storage, files []fileops.FileInfo) ([]fileops.FileInfo, error) {
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
		log.Printf("Checked batch %d-%d: %d existing replays found", i+1, end, skippedInBatch)
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

func logInfo(useColor bool, format string, args ...any) {
	if useColor {
		color.Cyan(format, args...)
		return
	}
	log.Printf(format, args...)
}

func logSuccess(useColor bool, format string, args ...any) {
	if useColor {
		color.Green(format, args...)
		return
	}
	log.Printf(format, args...)
}

func logWarn(useColor bool, format string, args ...any) {
	if useColor {
		color.Yellow(format, args...)
		return
	}
	log.Printf(format, args...)
}

func logError(useColor bool, format string, args ...any) {
	if useColor {
		color.Red(format, args...)
		return
	}
	log.Printf(format, args...)
}

func signalChan(sigChan chan os.Signal) <-chan os.Signal {
	if sigChan == nil {
		return nil
	}
	return sigChan
}

package cmd

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
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest replay files into the database",
	Long:  `Ingest StarCraft: Brood War replay files from a directory into a database (SQLite or PostgreSQL).`,
	RunE:  runIngest,
}

var (
	inputDir           string
	sqliteOutput       string
	postgresConnString string
	watch              bool
	stopAfterN         int
	upToDate           string
	upToMonths         int
	clean              bool
)

func init() {
	ingestCmd.Flags().StringVarP(&inputDir, "input-dir", "i", fileops.GetDefaultReplayDir(), "Input directory containing replay files")
	ingestCmd.Flags().StringVarP(&sqliteOutput, "sqlite-output-file", "o", "screp.db", "Output SQLite database file")
	ingestCmd.Flags().StringVarP(&postgresConnString, "postgres-connection-string", "p", "", "PostgreSQL connection string (e.g., 'host=localhost port=5432 user=marianol dbname=screpdb sslmode=disable')")
	ingestCmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for new files and ingest them as they appear")
	ingestCmd.Flags().IntVarP(&stopAfterN, "stop-after-n-reps", "n", 0, "Stop after processing N replay files (0 = no limit)")
	ingestCmd.Flags().StringVarP(&upToDate, "up-to-yyyy-mm-dd", "d", "", "Only process files up to this date (YYYY-MM-DD format)")
	ingestCmd.Flags().IntVarP(&upToMonths, "up-to-n-months", "m", 0, "Only process files from the last N months (0 = no limit)")
	ingestCmd.Flags().BoolVar(&clean, "clean", false, "Drop all tables before ingesting to start over (useful for migrations)")
}

func runIngest(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate that only one storage option is specified
	if postgresConnString != "" && sqliteOutput != "screp.db" {
		return fmt.Errorf("cannot specify both --postgres-connection-string and --sqlite-output-file")
	}

	// Initialize storage
	var store storage.Storage
	var err error

	if postgresConnString != "" {
		color.Cyan("Using PostgreSQL storage")
		store, err = storage.NewPostgresStorage(postgresConnString)
		if err != nil {
			return fmt.Errorf("failed to create PostgreSQL storage: %w", err)
		}
	} else {
		color.Cyan("Using SQLite storage: %s", sqliteOutput)
		store, err = storage.NewSQLiteStorage(sqliteOutput)
		if err != nil {
			return fmt.Errorf("failed to create SQLite storage: %w", err)
		}
	}
	defer store.Close()

	if err := store.Initialize(ctx, clean); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	if watch {
		return runWatchMode(ctx, store)
	}

	return runBatchMode(ctx, store)
}

func runWatchMode(ctx context.Context, store storage.Storage) error {
	color.Cyan("Watching directory: %s", inputDir)
	color.Cyan("Using %d CPU cores for parsing", runtime.GOMAXPROCS(0))
	color.Yellow("Press Ctrl+C to stop...")

	// Start the ingestion process
	dataChan, errChan := store.StartIngestion(ctx)

	watcher, err := fileops.NewFileWatcher(inputDir)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Create errgroup with context for concurrent processing
	g, gCtx := errgroup.WithContext(ctx)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case fileInfo := <-watcher.Events():
			color.Green("New file detected: %s", fileInfo.Name)

			// Process file concurrently
			g.Go(func() error {
				if err := processFileToChannel(gCtx, dataChan, &fileInfo); err != nil {
					log.Printf("Error processing file %s: %v", fileInfo.Name, err)
					return nil // Don't stop processing on errors
				}

				color.Green("Successfully processed: %s", fileInfo.Name)
				return nil
			})

		case err := <-watcher.Errors():
			color.Red("Watcher error: %v", err)

		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("storage error: %w", err)
			}

		case <-sigChan:
			color.Yellow("\nShutting down...")
			close(dataChan)
			return nil
		}
	}
}

func runBatchMode(ctx context.Context, store storage.Storage) error {
	color.Cyan("Scanning directory: %s", inputDir)
	color.Cyan("Using %d CPU cores for parsing", runtime.GOMAXPROCS(0))

	// Start the ingestion process
	dataChan, errChan := store.StartIngestion(ctx)

	// Get all replay files
	files, err := fileops.GetReplayFiles(inputDir)
	if err != nil {
		return fmt.Errorf("failed to get replay files: %w", err)
	}

	color.Green("Found %d replay files", len(files))

	// Sort by modification time (newest first)
	fileops.SortFilesByModTime(files)

	// Apply date filters
	var upToDatePtr *time.Time
	if upToDate != "" {
		parsed, err := time.Parse("2006-01-02", upToDate)
		if err != nil {
			return fmt.Errorf("invalid date format: %w", err)
		}
		upToDatePtr = &parsed
	}

	var upToMonthsPtr *int
	if upToMonths > 0 {
		upToMonthsPtr = &upToMonths
	}

	filteredFiles := fileops.FilterFilesByDate(files, upToDatePtr, upToMonthsPtr)
	color.Green("After date filtering: %d files", len(filteredFiles))

	// Apply count limit
	if stopAfterN > 0 {
		filteredFiles = fileops.LimitFiles(filteredFiles, stopAfterN)
		color.Green("Limited to %d files", len(filteredFiles))
	}

	// Batch check for existing replays before processing
	color.Yellow("Checking for existing replays...")
	filesToProcess, err := batchCheckExistingReplays(ctx, store, filteredFiles)
	if err != nil {
		return fmt.Errorf("failed to check existing replays: %w", err)
	}

	skippedCount := len(filteredFiles) - len(filesToProcess)
	color.Yellow("Skipping %d existing replays", skippedCount)
	color.Green("Processing %d new replays", len(filesToProcess))

	// Process files in batches of 100
	const batchSize = 100
	var processed, errors int64
	var mu sync.Mutex

	for i := 0; i < len(filesToProcess); i += batchSize {
		end := min(i+batchSize, len(filesToProcess))

		batch := filesToProcess[i:end]
		color.Cyan("Processing batch %d-%d of %d", i+1, end, len(filesToProcess))

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

	color.Green("\nProcessing complete:")
	color.Green("  Processed: %d", processed)
	color.Yellow("  Skipped: %d", skippedCount)
	color.Red("  Errors: %d", errors)

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

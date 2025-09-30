package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var rootCmd = &cobra.Command{
	Use:   "screpdb",
	Short: "StarCraft Replay Database - CLI tool for ingesting and querying Brood War replays",
	Long:  `A CLI tool for ingesting StarCraft: Brood War replay files into a database and providing MCP server functionality for querying.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(mcpCmd)
}

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
	maxConcurrency     int
)

func init() {
	ingestCmd.Flags().StringVarP(&inputDir, "input-dir", "i", fileops.GetDefaultReplayDir(), "Input directory containing replay files")
	ingestCmd.Flags().StringVarP(&sqliteOutput, "sqlite-output-file", "o", "screp.db", "Output SQLite database file")
	ingestCmd.Flags().StringVarP(&postgresConnString, "postgres-connection-string", "p", "", "PostgreSQL connection string (e.g., 'host=localhost port=5432 user=marianol dbname=screpdb sslmode=disable')")
	ingestCmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for new files and ingest them as they appear")
	ingestCmd.Flags().IntVarP(&stopAfterN, "stop-after-n-reps", "n", 0, "Stop after processing N replay files (0 = no limit)")
	ingestCmd.Flags().StringVarP(&upToDate, "up-to-yyyy-mm-dd", "d", "", "Only process files up to this date (YYYY-MM-DD format)")
	ingestCmd.Flags().IntVarP(&upToMonths, "up-to-n-months", "m", 0, "Only process files from the last N months (0 = no limit)")
	ingestCmd.Flags().IntVarP(&maxConcurrency, "max-concurrency", "c", 4, "Maximum number of concurrent goroutines for processing replays")
}

func runIngest(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate that only one storage option is specified
	if postgresConnString != "" && sqliteOutput != "screp.db" {
		return fmt.Errorf("cannot specify both --postgres-connection-string and --sqlite-output-file")
	}

	// Validate max concurrency
	if maxConcurrency <= 0 {
		return fmt.Errorf("max concurrency must be greater than 0")
	}
	if maxConcurrency > 100 {
		return fmt.Errorf("max concurrency should not exceed 100 to avoid overwhelming the system")
	}

	// Initialize storage
	var store storage.Storage
	var err error

	if postgresConnString != "" {
		fmt.Printf("Using PostgreSQL storage\n")
		store, err = storage.NewPostgresStorage(postgresConnString)
		if err != nil {
			return fmt.Errorf("failed to create PostgreSQL storage: %w", err)
		}
	} else {
		fmt.Printf("Using SQLite storage: %s\n", sqliteOutput)
		store, err = storage.NewSQLiteStorage(sqliteOutput)
		if err != nil {
			return fmt.Errorf("failed to create SQLite storage: %w", err)
		}
	}
	defer store.Close()

	if err := store.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	if watch {
		return runWatchMode(ctx, store)
	}

	return runBatchMode(ctx, store)
}

func runWatchMode(ctx context.Context, store storage.Storage) error {
	fmt.Printf("Watching directory: %s\n", inputDir)
	fmt.Printf("Max concurrency: %d\n", maxConcurrency)
	fmt.Println("Press Ctrl+C to stop...")

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

	// Create a semaphore to limit concurrency
	semaphore := make(chan struct{}, maxConcurrency)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case fileInfo := <-watcher.Events():
			fmt.Printf("New file detected: %s\n", fileInfo.Name)

			// Process file concurrently
			g.Go(func() error {
				// Acquire semaphore
				select {
				case semaphore <- struct{}{}:
					defer func() { <-semaphore }()
				case <-gCtx.Done():
					return gCtx.Err()
				}

				if err := processFile(gCtx, store, &fileInfo); err != nil {
					log.Printf("Error processing file %s: %v", fileInfo.Name, err)
					return nil // Don't stop processing on errors
				}

				fmt.Printf("Successfully processed: %s\n", fileInfo.Name)
				return nil
			})

		case err := <-watcher.Errors():
			fmt.Printf("Watcher error: %v\n", err)

		case <-sigChan:
			fmt.Println("\nShutting down...")
			return nil
		}
	}
}

func runBatchMode(ctx context.Context, store storage.Storage) error {
	fmt.Printf("Scanning directory: %s\n", inputDir)
	fmt.Printf("Max concurrency: %d\n", maxConcurrency)

	// Get all replay files
	files, err := fileops.GetReplayFiles(inputDir)
	if err != nil {
		return fmt.Errorf("failed to get replay files: %w", err)
	}

	fmt.Printf("Found %d replay files\n", len(files))

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
	fmt.Printf("After date filtering: %d files\n", len(filteredFiles))

	// Apply count limit
	if stopAfterN > 0 {
		filteredFiles = fileops.LimitFiles(filteredFiles, stopAfterN)
		fmt.Printf("Limited to %d files\n", len(filteredFiles))
	}

	// Process files concurrently
	var processed, skipped, errors int64
	var mu sync.Mutex

	// Create a semaphore to limit concurrency
	semaphore := make(chan struct{}, maxConcurrency)

	// Create errgroup with context
	g, gCtx := errgroup.WithContext(ctx)

	for i, fileInfo := range filteredFiles {
		i, fileInfo := i, fileInfo // capture loop variables

		g.Go(func() error {
			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-gCtx.Done():
				return gCtx.Err()
			}

			fmt.Printf("Processing %d/%d: %s\n", i+1, len(filteredFiles), fileInfo.Name)

			exists, err := store.ReplayExists(gCtx, fileInfo.Path, fileInfo.Checksum)
			if err != nil {
				log.Printf("Error checking if replay exists for %s: %v", fileInfo.Name, err)
				mu.Lock()
				errors++
				mu.Unlock()
				return nil // Don't stop processing on errors
			}

			if exists {
				fmt.Printf("Skipping (already exists): %s\n", fileInfo.Name)
				mu.Lock()
				skipped++
				mu.Unlock()
				return nil
			}

			if err := processFile(gCtx, store, &fileInfo); err != nil {
				log.Printf("Error processing file %s: %v", fileInfo.Name, err)
				mu.Lock()
				errors++
				mu.Unlock()
				return nil // Don't stop processing on errors
			}

			fmt.Printf("Successfully processed: %s\n", fileInfo.Name)
			mu.Lock()
			processed++
			mu.Unlock()
			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		fmt.Printf("Error during concurrent processing: %v\n", err)
	}

	fmt.Printf("\nProcessing complete:\n")
	fmt.Printf("  Processed: %d\n", processed)
	fmt.Printf("  Skipped: %d\n", skipped)
	fmt.Printf("  Errors: %d\n", errors)

	return nil
}

func processFile(ctx context.Context, store storage.Storage, fileInfo *fileops.FileInfo) error {
	// Create replay model from file info
	replay := parser.CreateReplayFromFileInfo(fileInfo.Path, fileInfo.Name, fileInfo.Size, fileInfo.Checksum)

	// Parse the replay
	data, err := parser.ParseReplay(fileInfo.Path, replay)
	if err != nil {
		return fmt.Errorf("failed to parse replay: %w", err)
	}

	// Store in database
	if err := store.StoreReplay(ctx, data); err != nil {
		return fmt.Errorf("failed to store replay: %w", err)
	}

	return nil
}

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/fatih/color"
	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/marianogappa/screpdb/ui"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Watch directory for new replays and output analysis in browser tab",
	Long:  `...TODO`,
	RunE:  runUI,
}

var (
	uiInputDir string
	// sqliteOutput       string
	uiPostgresConnString string
	// watch              bool
	// stopAfterN         int
	// upToDate           string
	// upToMonths         int
	// clean              bool
)

func init() {
	uiCmd.Flags().StringVarP(&uiInputDir, "input-dir", "i", fileops.GetDefaultReplayDir(), "Input directory containing replay files")
	// uiCmd.Flags().StringVarP(&sqliteOutput, "sqlite-output-file", "o", "screp.db", "Output SQLite database file")
	uiCmd.Flags().StringVarP(&uiPostgresConnString, "postgres-connection-string", "p", "", "PostgreSQL connection string (e.g., 'host=localhost port=5432 user=marianol dbname=screpdb sslmode=disable')")
	// uiCmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for new files and ingest them as they appear")
	// uiCmd.Flags().IntVarP(&stopAfterN, "stop-after-n-reps", "n", 0, "Stop after processing N replay files (0 = no limit)")
	// uiCmd.Flags().StringVarP(&upToDate, "up-to-yyyy-mm-dd", "d", "", "Only process files up to this date (YYYY-MM-DD format)")
	// uiCmd.Flags().IntVarP(&upToMonths, "up-to-n-months", "m", 0, "Only process files from the last N months (0 = no limit)")
	// uiCmd.Flags().BoolVar(&clean, "clean", false, "Drop all tables before ingesting to start over (useful for migrations)")
}

func runUI(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate that only one storage option is specified
	if uiPostgresConnString != "" && sqliteOutput != "screp.db" {
		return fmt.Errorf("cannot specify both --postgres-connection-string and --sqlite-output-file")
	}

	// Initialize storage
	var store storage.Storage
	var err error

	if uiPostgresConnString != "" {
		color.Cyan("Using PostgreSQL storage")
		store, err = storage.NewPostgresStorage(uiPostgresConnString)
		if err != nil {
			return fmt.Errorf("failed to create PostgreSQL storage: %w", err)
		}
	}
	defer store.Close()

	if err := store.Initialize(ctx, clean); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	return uiRunWatchMode(ctx, store)
}

func uiRunWatchMode(ctx context.Context, store storage.Storage) error {
	color.Cyan("Watching directory: %s", uiInputDir)
	color.Cyan("Using %d CPU cores for parsing", runtime.GOMAXPROCS(0))
	color.Yellow("Press Ctrl+C to stop...")

	// Start the ingestion process
	dataChan, errChan := store.StartIngestion(ctx)
	var (
		repChan = make(chan *models.ReplayData)
		uiTool  = ui.NewUI(repChan)
	)
	go uiTool.Start()

	watcher, err := fileops.NewFileWatcher(uiInputDir)
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
				if err := uiProcessFileToChannel(gCtx, dataChan, repChan, &fileInfo); err != nil {
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

func uiProcessFileToChannel(ctx context.Context, dataChan storage.ReplayDataChannel, repChan chan *models.ReplayData, fileInfo *fileops.FileInfo) error {
	// Create replay model from file info
	replay := parser.CreateReplayFromFileInfo(fileInfo.Path, fileInfo.Name, fileInfo.Size, fileInfo.Checksum)

	// Parse the replay
	data, err := parser.ParseReplay(fileInfo.Path, replay)
	if err != nil {
		return fmt.Errorf("failed to parse replay: %w", err)
	}

	repChan <- data

	// Send to storage channel
	select {
	case dataChan <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

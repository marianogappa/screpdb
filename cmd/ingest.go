package cmd

import (
	"context"
	"fmt"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/ingest"
	"github.com/spf13/cobra"
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest replay files into the database",
	Long:  `Ingest StarCraft: Brood War replay files from a directory into a SQLite database.`,
	RunE:  runIngest,
}

var (
	inputDir       string
	sqlitePath     string
	watch          bool
	stopAfterN     int
	upToDate       string
	upToMonths     int
	clean          bool
	cleanDashboard bool
)

func init() {
	ingestCmd.Flags().StringVarP(&inputDir, "input-dir", "i", fileops.GetDefaultReplayDir(), "Input directory containing replay files")
	ingestCmd.Flags().StringVarP(&sqlitePath, "sqlite-path", "s", "screp.db", "SQLite database file path")
	ingestCmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for new files and ingest them as they appear")
	ingestCmd.Flags().IntVarP(&stopAfterN, "stop-after-n-reps", "n", 0, "Stop after processing N replay files (0 = no limit)")
	ingestCmd.Flags().StringVarP(&upToDate, "up-to-yyyy-mm-dd", "d", "", "Only process files up to this date (YYYY-MM-DD format)")
	ingestCmd.Flags().IntVarP(&upToMonths, "up-to-n-months", "m", 0, "Only process files from the last N months (0 = no limit)")
	ingestCmd.Flags().BoolVar(&clean, "clean", false, "Drop all non-dashboard tables before ingesting to start over (useful for migrations).")
	ingestCmd.Flags().BoolVar(&cleanDashboard, "clean-dashboard", false, "Drop all dashboard tables")
}

func runIngest(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	cfg := ingest.Config{
		InputDir:       inputDir,
		SQLitePath:     sqlitePath,
		Watch:          watch,
		StopAfterN:     stopAfterN,
		UpToDate:       upToDate,
		UpToMonths:     upToMonths,
		Clean:          clean,
		CleanDashboard: cleanDashboard,
		HandleSignals:  true,
		UseColor:       true,
	}

	if err := ingest.Run(ctx, cfg); err != nil {
		return fmt.Errorf("ingestion failed: %w", err)
	}

	return nil
}

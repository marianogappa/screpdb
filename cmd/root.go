package cmd

import (
	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "screpdb",
	Short: "StarCraft Replay Database - CLI tool for ingesting and querying Brood War replays",
	Long:  `A CLI tool for ingesting StarCraft: Brood War replay files into a database and providing MCP server functionality for querying.`,
	// PersistentPreRunE runs before every subcommand. It creates and registers
	// the single app-data root as a permitted I/O root (issue #237): the SQLite
	// database, cache, logs, crash reports, and sample replays all live under it.
	// Commands register the read-only replays folder as they resolve it.
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		_, err := appdata.Dir()
		return err
	},
	RunE: runDashboard,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(dashboardCmd)
	addDashboardFlags(rootCmd)
}

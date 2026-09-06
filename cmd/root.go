package cmd

import (
	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "screpdb",
	Short: "StarCraft Replay Database - dashboard, JSON API and MCP server for Brood War replays",
	Long:  `Reads a folder of StarCraft: Brood War replays into memory and serves them as a dashboard, a JSON API and an MCP server.`,
	// PersistentPreRunE runs before every subcommand. It creates and registers
	// the single app-data root as a permitted I/O root (issue #237): settings,
	// cache, logs, crash reports, and sample replays all live under it.
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
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(dashboardCmd)
	addDashboardFlags(rootCmd)
}

package cmd

import (
	"os"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "screpdb",
	Short: "StarCraft Replay Database - CLI tool for ingesting and querying Brood War replays",
	Long:  `A CLI tool for ingesting StarCraft: Brood War replay files into a database and providing MCP server functionality for querying.`,
	// PersistentPreRunE runs before every subcommand. It registers the current
	// working directory as a permitted I/O root (issue #135): the SQLite
	// database and opt-in debug artifacts live in pwd. Commands register
	// additional roots (replays folder, OS cache dir) as they resolve them.
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return iofacade.AllowDir(cwd)
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

package cmd

import "github.com/spf13/cobra"

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
	rootCmd.AddCommand(dashboardCmd)
}

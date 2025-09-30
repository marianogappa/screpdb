package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/marianogappa/screpdb/internal/mcp"
	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/spf13/cobra"
)

var (
	sqliteInput string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for querying the database",
	Long:  `Start a Model Context Protocol (MCP) server that provides SQL query capabilities for the replay database.`,
	RunE:  runMCP,
}

func init() {
	mcpCmd.Flags().StringVarP(&sqliteInput, "sqlite-input-file", "i", "screp.db", "Input SQLite database file")
}

func runMCP(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if database file exists
	if _, err := os.Stat(sqliteInput); os.IsNotExist(err) {
		return fmt.Errorf("database file does not exist: %s", sqliteInput)
	}

	// Initialize storage
	store, err := storage.NewSQLiteStorage(sqliteInput)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer store.Close()

	// Create MCP server
	server := mcp.NewServer(store)

	// Start the server
	fmt.Printf("Starting MCP server with database: %s\n", sqliteInput)
	fmt.Println("Server is running...")

	return server.Start(ctx)
}

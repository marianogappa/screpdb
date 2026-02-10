package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/marianogappa/screpdb/internal/mcp"
	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/spf13/cobra"
)

var (
	mcpSQLitePath string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for querying the database",
	Long:  `Start a Model Context Protocol (MCP) server that provides SQL query capabilities for the replay database.`,
	RunE:  runMCP,
}

func init() {
	mcpCmd.Flags().StringVarP(&mcpSQLitePath, "sqlite-path", "s", "screp.db", "SQLite database file path")
}

func runMCP(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize SQLite storage
	store, err := storage.NewSQLiteStorage(mcpSQLitePath)
	if err != nil {
		return fmt.Errorf("failed to create SQLite storage: %w", err)
	}
	log.Printf("Starting MCP server with SQLite database")
	defer store.Close()

	// Create MCP server
	server := mcp.NewServer(store)

	// Start the server
	log.Println("Server is running...")

	return server.Start(ctx)
}

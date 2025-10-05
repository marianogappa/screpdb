package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/marianogappa/screpdb/internal/mcp"
	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/spf13/cobra"
)

var (
	mcpSqliteInput        string
	mcpPostgresConnString string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for querying the database",
	Long:  `Start a Model Context Protocol (MCP) server that provides SQL query capabilities for the replay database.`,
	RunE:  runMCP,
}

func init() {
	mcpCmd.Flags().StringVarP(&mcpSqliteInput, "sqlite-input-file", "i", "screp.db", "Input SQLite database file")
	mcpCmd.Flags().StringVarP(&mcpPostgresConnString, "postgres-connection-string", "p", "", "PostgreSQL connection string (e.g., 'host=localhost port=5432 user=postgres password=secret dbname=screpdb sslmode=disable')")
}

func runMCP(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	var store storage.Storage
	var err error

	// Detect which database type was specified (matching ingest command logic)
	if mcpPostgresConnString != "" {
		// Initialize PostgreSQL storage
		store, err = storage.NewPostgresStorage(mcpPostgresConnString)
		if err != nil {
			return fmt.Errorf("failed to create PostgreSQL storage: %w", err)
		}
		log.Printf("Starting MCP server with PostgreSQL database")
	} else {
		// Check if SQLite database file exists
		if _, err := os.Stat(mcpSqliteInput); os.IsNotExist(err) {
			return fmt.Errorf("SQLite database file does not exist: %s", mcpSqliteInput)
		}

		// Initialize SQLite storage
		store, err = storage.NewSQLiteStorage(mcpSqliteInput)
		if err != nil {
			return fmt.Errorf("failed to create SQLite storage: %w", err)
		}
		log.Printf("Starting MCP server with SQLite database: %s", mcpSqliteInput)
	}
	defer store.Close()

	// Create MCP server
	server := mcp.NewServer(store)

	// Start the server
	log.Println("Server is running...")

	return server.Start(ctx)
}

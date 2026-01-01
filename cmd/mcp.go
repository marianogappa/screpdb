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
	mcpPostgresConnString string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for querying the database",
	Long:  `Start a Model Context Protocol (MCP) server that provides SQL query capabilities for the replay database.`,
	RunE:  runMCP,
}

func init() {
	mcpCmd.Flags().StringVarP(&mcpPostgresConnString, "postgres-connection-string", "p", "", "PostgreSQL connection string (e.g., 'host=localhost port=5432 user=postgres password=secret dbname=screpdb sslmode=disable')")
}

func runMCP(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate that postgres connection string is provided
	if mcpPostgresConnString == "" {
		return fmt.Errorf("--postgres-connection-string is required")
	}

	// Initialize PostgreSQL storage
	store, err := storage.NewPostgresStorage(mcpPostgresConnString)
	if err != nil {
		return fmt.Errorf("failed to create PostgreSQL storage: %w", err)
	}
	log.Printf("Starting MCP server with PostgreSQL database")
	defer store.Close()

	// Create MCP server
	server := mcp.NewServer(store)

	// Start the server
	log.Println("Server is running...")

	return server.Start(ctx)
}

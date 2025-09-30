package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server implements the MCP server using mcp-go library
type Server struct {
	storage   storage.Storage
	mcpServer *server.MCPServer
}

// NewServer creates a new MCP server
func NewServer(storage storage.Storage) *Server {
	// Create MCP server with tool capabilities
	mcpServer := server.NewMCPServer(
		"screpdb-mcp-server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	s := &Server{
		storage:   storage,
		mcpServer: mcpServer,
	}

	// Register the SQL query tool
	sqlTool := mcp.NewTool("query_database",
		mcp.WithDescription("Execute SQL queries against the StarCraft replay SQLite database. The database contains tables: replays (metadata), players (player info), actions (game events), units (unit data), buildings (building data). Use this tool to analyze replay statistics, player performance, unit usage, and game patterns."),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("SQL query to execute against the StarCraft replay SQLite database"),
		),
	)

	mcpServer.AddTool(sqlTool, s.handleSQLQuery)

	// Register a schema information tool
	schemaTool := mcp.NewTool("get_schema",
		mcp.WithDescription("Get detailed information about the StarCraft replay SQLite database schema including table structures, relationships, and example queries."),
	)

	mcpServer.AddTool(schemaTool, s.handleGetSchema)

	return s
}

// Start starts the MCP server
func (s *Server) Start(ctx context.Context) error {
	// Start the server using stdio transport
	return server.ServeStdio(s.mcpServer)
}

// handleSQLQuery handles SQL query execution
func (s *Server) handleSQLQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("sql")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid sql parameter: %v", err)), nil
	}

	// Execute the query
	results, err := s.storage.Query(ctx, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Query execution failed: %v", err)), nil
	}

	// Format results
	resultText := s.formatQueryResults(results)

	return mcp.NewToolResultText(resultText), nil
}

// handleGetSchema handles schema information requests
func (s *Server) handleGetSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema := GetDatabaseSchema()
	return mcp.NewToolResultText(schema), nil
}

// formatQueryResults formats query results for display
func (s *Server) formatQueryResults(results []map[string]any) string {
	if len(results) == 0 {
		return "No results found."
	}

	// Get column names from first row
	var columns []string
	for col := range results[0] {
		columns = append(columns, col)
	}

	// Create table format
	var output strings.Builder
	output.WriteString("Query Results:\n\n")

	// Header
	for i, col := range columns {
		if i > 0 {
			output.WriteString(" | ")
		}
		output.WriteString(col)
	}
	output.WriteString("\n")

	// Separator
	for i, col := range columns {
		if i > 0 {
			output.WriteString(" | ")
		}
		for j := 0; j < len(col); j++ {
			output.WriteString("-")
		}
	}
	output.WriteString("\n")

	// Data rows
	for _, row := range results {
		for i, col := range columns {
			if i > 0 {
				output.WriteString(" | ")
			}
			value := fmt.Sprintf("%v", row[col])
			output.WriteString(value)
		}
		output.WriteString("\n")
	}

	output.WriteString(fmt.Sprintf("\nTotal rows: %d", len(results)))

	return output.String()
}

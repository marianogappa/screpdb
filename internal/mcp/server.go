package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/marianogappa/screpdb/internal/storage"
)

// Server implements the MCP server
type Server struct {
	storage storage.Storage
}

// NewServer creates a new MCP server
func NewServer(storage storage.Storage) *Server {
	return &Server{
		storage: storage,
	}
}

// Start starts the MCP server
func (s *Server) Start(ctx context.Context) error {
	// For now, we'll implement a simple JSON-RPC over stdio
	// This is a basic implementation that can be extended

	logger := log.New(os.Stderr, "[MCP] ", log.LstdFlags)
	logger.Println("MCP Server started")

	// Handle incoming requests
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var request map[string]any
		if err := decoder.Decode(&request); err != nil {
			if err.Error() == "EOF" {
				break
			}
			logger.Printf("Error decoding request: %v", err)
			continue
		}

		response := s.handleRequest(ctx, request)
		if err := encoder.Encode(response); err != nil {
			logger.Printf("Error encoding response: %v", err)
		}
	}

	return nil
}

// handleRequest handles incoming MCP requests
func (s *Server) handleRequest(ctx context.Context, request map[string]any) map[string]any {
	method, ok := request["method"].(string)
	if !ok {
		return s.errorResponse(request["id"], -32600, "Invalid Request", nil)
	}

	id := request["id"]

	switch method {
	case "initialize":
		return s.handleInitialize(id)
	case "tools/list":
		return s.handleToolsList(id)
	case "tools/call":
		return s.handleToolsCall(ctx, id, request)
	default:
		return s.errorResponse(id, -32601, "Method not found", nil)
	}
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize(id any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "screpdb-mcp-server",
				"version": "1.0.0",
			},
		},
	}
}

// handleToolsList handles the tools/list request
func (s *Server) handleToolsList(id any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"tools": []map[string]any{
				{
					"name":        "sql-query",
					"description": "Execute SQL queries against the StarCraft replay database. " + GetDatabaseSchema(),
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"query": map[string]any{
								"type":        "string",
								"description": "SQL query to execute against the replay database",
							},
						},
						"required": []string{"query"},
					},
				},
			},
		},
	}
}

// handleToolsCall handles the tools/call request
func (s *Server) handleToolsCall(ctx context.Context, id any, request map[string]any) map[string]any {
	params, ok := request["params"].(map[string]any)
	if !ok {
		return s.errorResponse(id, -32602, "Invalid params", nil)
	}

	toolName, ok := params["name"].(string)
	if !ok {
		return s.errorResponse(id, -32602, "Invalid tool name", nil)
	}

	switch toolName {
	case "sql-query":
		return s.handleSQLQuery(ctx, id, params)
	default:
		return s.errorResponse(id, -32601, "Tool not found", nil)
	}
}

// handleSQLQuery handles SQL query execution
func (s *Server) handleSQLQuery(ctx context.Context, id any, params map[string]any) map[string]any {
	query, ok := params["arguments"].(map[string]any)
	if !ok {
		return s.errorResponse(id, -32602, "Invalid arguments", nil)
	}

	sqlQuery, ok := query["query"].(string)
	if !ok {
		return s.errorResponse(id, -32602, "Query parameter required", nil)
	}

	// Execute the query
	results, err := s.storage.Query(ctx, sqlQuery)
	if err != nil {
		return s.errorResponse(id, -32603, fmt.Sprintf("Query execution failed: %v", err), nil)
	}

	// Format results
	resultText := s.formatQueryResults(results)

	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": resultText,
				},
			},
		},
	}
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
	output := "Query Results:\n\n"

	// Header
	for i, col := range columns {
		if i > 0 {
			output += " | "
		}
		output += col
	}
	output += "\n"

	// Separator
	for i, col := range columns {
		if i > 0 {
			output += " | "
		}
		for j := 0; j < len(col); j++ {
			output += "-"
		}
	}
	output += "\n"

	// Data rows
	for _, row := range results {
		for i, col := range columns {
			if i > 0 {
				output += " | "
			}
			value := fmt.Sprintf("%v", row[col])
			output += value
		}
		output += "\n"
	}

	output += fmt.Sprintf("\nTotal rows: %d", len(results))

	return output
}

// errorResponse creates an error response
func (s *Server) errorResponse(id any, code int, message string, data any) map[string]any {
	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}

	if data != nil {
		response["error"].(map[string]any)["data"] = data
	}

	return response
}

package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"unicode"

	"github.com/marianogappa/screpdb/internal/dashboard/variables"
)

func (d *Dashboard) handlerExecuteQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("method not allowed"))
		return
	}

	bs, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("error reading request body"))
		return
	}

	type QueryRequest struct {
		Query          string         `json:"query"`
		VariableValues map[string]any `json:"variable_values"`
	}

	var req QueryRequest
	if err := json.Unmarshal(bs, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json"))
		return
	}
	if err := variables.ValidateReceivedVariableValues(req.VariableValues); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid variable values supplied: " + err.Error()))
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("query is required"))
		return
	}

	// Validate that the query is a SELECT statement
	if !isSelectQuery(query) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("only SELECT queries are allowed"))
		return
	}

	// Execute the query
	usedVariables := variables.FindVariables(query, req.VariableValues)
	results, columns, err := d.executeQuery(query, usedVariables)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("error executing query: " + err.Error()))
		return
	}

	type QueryResponse struct {
		Results []map[string]any `json:"results"`
		Columns []string         `json:"columns,omitempty"`
	}

	response := QueryResponse{
		Results: results,
		Columns: columns,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// isSelectQuery checks if a SQL query is a SELECT statement
// It trims whitespace and checks if the query starts with SELECT (case-insensitive)
func isSelectQuery(query string) bool {
	// Trim leading whitespace
	trimmed := strings.TrimLeftFunc(query, unicode.IsSpace)

	// Convert to uppercase for case-insensitive comparison
	upper := strings.ToUpper(trimmed)

	// Remove comments first
	// Handle multi-line comments /* ... */
	for strings.Contains(upper, "/*") {
		start := strings.Index(upper, "/*")
		end := strings.Index(upper[start+2:], "*/")
		if end == -1 {
			break
		}
		upper = upper[:start] + upper[start+end+4:]
	}

	// Handle single-line comments -- ...
	lines := strings.Split(upper, "\n")
	var cleanedLines []string
	for _, line := range lines {
		if idx := strings.Index(line, "--"); idx != -1 {
			line = line[:idx]
		}
		cleanedLines = append(cleanedLines, line)
	}
	upper = strings.Join(cleanedLines, "\n")

	// Trim again after removing comments
	upper = strings.TrimLeftFunc(upper, unicode.IsSpace)

	// Check if it starts with SELECT
	return strings.HasPrefix(upper, "SELECT")
}

package dashboard

import (
	"strings"
	"unicode"
)

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

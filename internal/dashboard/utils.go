package dashboard

import (
	"fmt"
	"strings"
)

// formatQueryResults formats query results for display
func formatQueryResults(results []map[string]any) string {
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

package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

// VariableType represents the type of a dashboard variable
type VariableType string

const (
	VariableTypeString  VariableType = "string"
	VariableTypeNumeric VariableType = "numeric"
)

// DashboardVariable represents a single dashboard variable
type DashboardVariable struct {
	Name  string       `json:"name"`
	Type  VariableType `json:"type"`
	Query string       `json:"query"` // SQL query to fill possible values
}

// ValidateVariable validates a dashboard variable
func (v *DashboardVariable) Validate() error {
	if v.Name == "" {
		return errors.New("variable name is required")
	}
	if v.Type != VariableTypeString && v.Type != VariableTypeNumeric {
		return errors.New("variable type must be 'string' or 'numeric'")
	}
	if v.Query == "" {
		return errors.New("variable query is required")
	}
	// Validate that the query is a SELECT statement
	trimmed := strings.TrimSpace(v.Query)
	if !strings.HasPrefix(strings.ToUpper(trimmed), "SELECT") {
		return errors.New("variable query must be a SELECT statement")
	}
	return nil
}

// GetZeroValue returns the zero value for the variable type
func (v *DashboardVariable) GetZeroValue() any {
	switch v.Type {
	case VariableTypeString:
		return ""
	case VariableTypeNumeric:
		return 0
	default:
		return ""
	}
}

// VariableValue represents a variable with its selected value
type VariableValue struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// ExtractNamedArgs extracts all named arguments (e.g., @variable_name) from a SQL query
func ExtractNamedArgs(query string) []string {
	// Match @ followed by alphanumeric characters and underscores
	re := regexp.MustCompile(`@([a-zA-Z_][a-zA-Z0-9_]*)`)
	matches := re.FindAllStringSubmatch(query, -1)

	seen := make(map[string]bool)
	var args []string
	for _, match := range matches {
		if len(match) > 1 {
			argName := match[1]
			if !seen[argName] {
				seen[argName] = true
				args = append(args, argName)
			}
		}
	}
	return args
}

// ValidateQueryVariables validates that all named args in a query exist in the provided variables
func ValidateQueryVariables(query string, variables []DashboardVariable) error {
	namedArgs := ExtractNamedArgs(query)
	if len(namedArgs) == 0 {
		return nil
	}

	// Create a map of available variable names
	availableVars := make(map[string]bool)
	for _, v := range variables {
		availableVars[v.Name] = true
	}

	// Check that all named args exist
	for _, arg := range namedArgs {
		if !availableVars[arg] {
			return fmt.Errorf("query uses undefined variable: @%s", arg)
		}
	}

	return nil
}

// InterpolateVariables interpolates variables into a SQL query using pgx.NamedArgs
// It returns the query with named args replaced and the arguments map for pgx
func InterpolateVariables(query string, variableValues []VariableValue) (string, map[string]any, error) {
	namedArgs := ExtractNamedArgs(query)
	if len(namedArgs) == 0 {
		return query, nil, nil
	}

	// Create a map of variable values
	valueMap := make(map[string]any)
	for _, vv := range variableValues {
		valueMap[vv.Name] = vv.Value
	}

	// Check that all named args have values
	args := make(map[string]any)
	for _, argName := range namedArgs {
		value, ok := valueMap[argName]
		if !ok {
			return "", nil, fmt.Errorf("variable @%s has no value", argName)
		}
		args[argName] = value
	}

	// pgx.NamedArgs will handle the interpolation, so we just return the query and args
	return query, args, nil
}

// ExecuteVariableQuery executes a variable's query to get possible values
// It can use previously defined variables for interpolation
func (d *Dashboard) ExecuteVariableQuery(ctx context.Context, variable DashboardVariable, previousValues []VariableValue) ([]any, error) {
	// Interpolate previous variables into the query
	query, args, err := InterpolateVariables(variable.Query, previousValues)
	if err != nil {
		return nil, fmt.Errorf("failed to interpolate variable query: %w", err)
	}

	var rows *sql.Rows
	if args != nil && len(args) > 0 {
		// Use pgx.NamedArgs for named parameter interpolation
		namedArgs := pgx.NamedArgs(args)
		rows, err = d.db.QueryContext(ctx, query, namedArgs)
	} else {
		rows, err = d.db.QueryContext(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to execute variable query: %w", err)
	}
	defer rows.Close()

	var results []any
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	if len(columns) == 0 {
		return nil, errors.New("variable query must return at least one column")
	}

	// Get the first column's values
	for rows.Next() {
		var value any
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		results = append(results, value)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// GetVariableValues gets all possible values for all variables in order
func (d *Dashboard) GetVariableValues(ctx context.Context, variables []DashboardVariable) (map[string][]any, error) {
	result := make(map[string][]any)
	var previousValues []VariableValue

	for _, variable := range variables {
		values, err := d.ExecuteVariableQuery(ctx, variable, previousValues)
		if err != nil {
			return nil, fmt.Errorf("failed to get values for variable %s: %w", variable.Name, err)
		}
		result[variable.Name] = values

		// Set the first value as the default for next variable interpolation
		if len(values) > 0 {
			previousValues = append(previousValues, VariableValue{
				Name:  variable.Name,
				Value: values[0],
			})
		} else {
			// Use zero value if no results
			previousValues = append(previousValues, VariableValue{
				Name:  variable.Name,
				Value: variable.GetZeroValue(),
			})
		}
	}

	return result, nil
}

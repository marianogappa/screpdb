package variables

import (
	"database/sql"
	"fmt"
	"strings"
)

type Variable struct {
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	variableType string
	defaultValue string
	SQL          string `json:"sql"`

	UsedValue      any   `json:"used_value"`
	PossibleValues []any `json:"possible_values"`
}

const (
	VariableTypeString  = "string"
	VariableTypeNumeric = "numeric"
)

var (
	variables = map[string]Variable{
		"all_players_name": {
			Name:         "all_players_name",
			DisplayName:  "Player Name",
			Description:  "All players in the database (the player name)",
			variableType: "string",
			defaultValue: "",
			SQL:          "SELECT DISTINCT name FROM players ORDER BY name",
		},
		"last_replay_players_name": {
			Name:         "last_replay_players_name",
			DisplayName:  "Player Name",
			Description:  "All players in the last replay (the player name)",
			variableType: "string",
			defaultValue: "",
			SQL:          "SELECT name FROM players p JOIN replays r ON p.replay_id = r.id WHERE r.replay_date = (SELECT MAX(replay_date) FROM replays) ORDER BY name",
		},
		"last_50_players_name": {
			Name:         "last_50_players_name",
			DisplayName:  "Player Name",
			Description:  "All players in the last 50 replays (the player name)",
			variableType: "string",
			defaultValue: "",
			SQL:          "SELECT DISTINCT name FROM ( SELECT p.name, r.replay_date FROM players p JOIN replays r ON p.replay_id = r.id ORDER BY r.replay_date DESC ) t LIMIT 50",
		},
		"races": {
			Name:         "races",
			DisplayName:  "Race",
			Description:  "The three races in the game (Protoss, Terran, Zerg)",
			variableType: "string",
			defaultValue: "",
			SQL:          "SELECT race FROM (SELECT 'Protoss' race UNION ALL SELECT 'Terran' race UNION ALL SELECT 'Zerg' race) t",
		},
	}

	buildInterpolation = func(v Variable) string { return fmt.Sprintf("@%s", v.Name) }
)

// GetAllVariables returns all available variables
func GetAllVariables() map[string]Variable {
	return variables
}

// TODO: this will cause an error if an interpolation exists inside a string comment or string literal
// ValidateReceivedVariableValues must be called first (otherwise a wrong variable type can be used)
func FindVariables(query string, receivedVariableValues map[string]any) map[string]Variable {
	if receivedVariableValues == nil {
		receivedVariableValues = map[string]any{}
	}
	fvs := map[string]Variable{}
	for _, v := range variables {
		if strings.Contains(query, buildInterpolation(v)) {
			usedVar := v
			usedVar.UsedValue = usedVar.defaultValue
			if rvv, ok := receivedVariableValues[v.Name]; ok {
				usedVar.UsedValue = rvv
			}
			fvs[v.Name] = usedVar
		}
	}
	return fvs
}

func ValidateReceivedVariableValues(vvs map[string]any) error {
	for vvk, vvv := range vvs {
		v, ok := variables[vvk]
		if !ok {
			return fmt.Errorf("the supplied variable name does not exist: %v", vvk)
		}
		switch v.variableType {
		case VariableTypeString:
			if _, ok := vvv.(string); !ok {
				return fmt.Errorf("the supplied variable [%v]'s value has invalid type (should be string)", vvk)
			}
		case VariableTypeNumeric:
			if _, ok := vvv.(float64); !ok {
				return fmt.Errorf("the supplied variable [%v]'s value has invalid type (should be float64)", vvk)
			}
		}
	}
	return nil
}

// RunAllUsedVariableQueries runs all queries in allUsedVariables and returns the results
// (each one keyed by the variable name and returning the single column values of the given types)
func RunAllUsedVariableQueries(db *sql.DB, allUsedVariables map[string]Variable) (map[string][]any, error) {
	result := make(map[string][]any)

	for varName, variable := range allUsedVariables {
		rows, err := db.Query(variable.SQL)
		if err != nil {
			return nil, fmt.Errorf("failed to run query for variable %s: %w", varName, err)
		}
		defer rows.Close()

		var values []any
		for rows.Next() {
			var value any
			if err := rows.Scan(&value); err != nil {
				rows.Close()
				return nil, fmt.Errorf("failed to scan result for variable %s: %w", varName, err)
			}
			// Convert []byte to string for JSON serialization
			if b, ok := value.([]byte); ok {
				value = string(b)
			}
			values = append(values, value)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating results for variable %s: %w", varName, err)
		}

		result[varName] = values
	}

	return result, nil
}

package variables

import (
	"database/sql"
	"fmt"
	"strings"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
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
			SQL:          dashboarddb.VariableQueryAllPlayersName,
		},
		"last_replay_players_name": {
			Name:         "last_replay_players_name",
			DisplayName:  "Player Name",
			Description:  "All players in the last replay (the player name)",
			variableType: "string",
			defaultValue: "",
			SQL:          dashboarddb.VariableQueryLastReplayPlayersName,
		},
		"last_50_players_name": {
			Name:         "last_50_players_name",
			DisplayName:  "Player Name",
			Description:  "All players in the last 50 replays (the player name)",
			variableType: "string",
			defaultValue: "",
			SQL:          dashboarddb.VariableQueryLast50PlayersName,
		},
		"races": {
			Name:         "races",
			DisplayName:  "Race",
			Description:  "The three races in the game (Protoss, Terran, Zerg)",
			variableType: "string",
			defaultValue: "",
			SQL:          dashboarddb.VariableQueryRaces,
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
		rows, err := dashboarddb.QueryOnDB(db, variable.SQL)
		if err != nil {
			return nil, fmt.Errorf("failed to run query for variable %s: %w", varName, err)
		}
		defer rows.Close()
		values, err := dashboarddb.ScanFirstColumn(rows)
		if err != nil {
			return nil, fmt.Errorf("error iterating results for variable %s: %w", varName, err)
		}

		result[varName] = values
	}

	return result, nil
}

package dashboard

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/marianogappa/screpdb/internal/dashboard/variables"
)

func (d *Dashboard) handlerGetQueryVariables(w http.ResponseWriter, r *http.Request) {
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

	type QueryVariablesRequest struct {
		Query string `json:"query"`
	}

	var req QueryVariablesRequest
	if err := json.Unmarshal(bs, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json"))
		return
	}

	// Find variables in the query
	usedVariables := variables.FindVariables(req.Query, nil)

	// Get possible values for all used variables
	variableOptions, err := variables.RunAllUsedVariableQueries(d.db, usedVariables)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed to run variable queries: " + err.Error()))
		return
	}

	// Build response with variable information
	type VariableResponse struct {
		Name           string `json:"name"`
		DisplayName    string `json:"display_name"`
		Description    string `json:"description"`
		PossibleValues []any  `json:"possible_values"`
	}
	variablesResponse := make(map[string]VariableResponse)
	for varName, variable := range usedVariables {
		variablesResponse[varName] = VariableResponse{
			Name:           variable.Name,
			DisplayName:    variable.DisplayName,
			Description:    variable.Description,
			PossibleValues: variableOptions[varName],
		}
	}

	type QueryVariablesResponse struct {
		Variables map[string]VariableResponse `json:"variables"`
	}

	response := QueryVariablesResponse{
		Variables: variablesResponse,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

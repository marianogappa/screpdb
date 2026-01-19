package dashboard

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"

	"database/sql"

	"github.com/gorilla/mux"
	"github.com/marianogappa/screpdb/internal/dashboard/variables"
)

func (d *Dashboard) handlerGetDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardURL := vars["url"]
	if dashboardURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("dashboard url missing"))
		return
	}

	type reqParams struct {
		VariableValues map[string]any `json:"variable_values"`
	}

	params := reqParams{}
	// Only read body for POST requests
	if r.Method == http.MethodPost {
		bs, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("error reading request POST payload"))
			return
		}
		if len(bs) > 0 {
			if err := json.Unmarshal(bs, &params); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("invalid json: " + err.Error()))
				return
			}
		}
	}
	if err := variables.ValidateReceivedVariableValues(params.VariableValues); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid variable values supplied: " + err.Error()))
		return
	}

	dash, err := d.getDashboardByURL(d.ctx, dashboardURL)
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("unknown dashboard url"))
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error getting dashboard url from db: " + err.Error()))
		return
	}

	widgets, err := d.getDashboardWidgets(d.ctx, dashboardURL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error listing dashboard widgets: " + err.Error()))
		return
	}

	type WidgetWithResults struct {
		ID          int64            `json:"id"`
		WidgetOrder *int64           `json:"widget_order"`
		Name        string           `json:"name"`
		Description *string          `json:"description"`
		Config      WidgetConfig     `json:"config"`
		Query       string           `json:"query"`
		Results     []map[string]any `json:"results"`
		Columns     []string         `json:"columns,omitempty"`
		CreatedAt   *string          `json:"created_at"`
		UpdatedAt   *string          `json:"updated_at"`
	}

	widgetsWithResults := make([]WidgetWithResults, 0, len(widgets))
	allUsedVariables := map[string]variables.Variable{}
	for _, widget := range widgets {
		var results []map[string]any
		var columns []string
		if widget.Query != "" {
			usedVariables := variables.FindVariables(widget.Query, params.VariableValues)
			maps.Copy(allUsedVariables, usedVariables)
			queryResults, queryColumns, err := d.executeQuery(widget.Query, usedVariables)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(fmt.Sprintf("error executing query for widget %d: %v", widget.ID, err.Error())))
				return
			}
			results = queryResults
			columns = queryColumns
		}

		config, err := bytesToWidgetConfig([]byte(widget.Config))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, "error parsing config for widget %d: %v", widget.ID, err.Error())
			return
		}

		var createdAt *string
		if widget.CreatedAt != nil {
			ts := widget.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
			createdAt = &ts
		}
		var updatedAt *string
		if widget.UpdatedAt != nil {
			ts := widget.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
			updatedAt = &ts
		}

		widgetsWithResults = append(widgetsWithResults, WidgetWithResults{
			ID:          widget.ID,
			WidgetOrder: widget.WidgetOrder,
			Name:        widget.Name,
			Description: widget.Description,
			Config:      config,
			Query:       widget.Query,
			Results:     results,
			Columns:     columns,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
	}

	variableOptions, err := variables.RunAllUsedVariableQueries(d.db, allUsedVariables)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "failed to run all used variable queries: %v", err.Error())
		return
	}

	// Build variables with possible_values for the response
	type VariableResponse struct {
		Name           string `json:"name"`
		DisplayName    string `json:"display_name"`
		Description    string `json:"description"`
		PossibleValues []any  `json:"possible_values"`
	}
	variablesResponse := make(map[string]VariableResponse)
	for varName, variable := range allUsedVariables {
		variablesResponse[varName] = VariableResponse{
			Name:           variable.Name,
			DisplayName:    variable.DisplayName,
			Description:    variable.Description,
			PossibleValues: variableOptions[varName],
		}
	}

	type DashboardResponse struct {
		URL         string                      `json:"url"`
		Name        string                      `json:"name"`
		Description *string                     `json:"description"`
		CreatedAt   *string                     `json:"created_at"`
		Widgets     []WidgetWithResults         `json:"widgets"`
		Variables   map[string]VariableResponse `json:"variables"`
	}

	var dashDescription *string
	if dash.Description != nil {
		dashDescription = dash.Description
	}
	var dashCreatedAt *string
	if dash.CreatedAt != nil {
		ts := dash.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
		dashCreatedAt = &ts
	}

	response := DashboardResponse{
		URL:         dash.URL,
		Name:        dash.Name,
		Description: dashDescription,
		CreatedAt:   dashCreatedAt,
		Widgets:     widgetsWithResults,
		Variables:   variablesResponse,
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

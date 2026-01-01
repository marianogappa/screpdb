package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"

	"database/sql"

	"github.com/gorilla/mux"
)

func (d *Dashboard) handlerGetDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardURL := vars["url"]
	if dashboardURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("dashboard url missing"))
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
		CreatedAt   *string          `json:"created_at"`
		UpdatedAt   *string          `json:"updated_at"`
	}

	widgetsWithResults := make([]WidgetWithResults, 0, len(widgets))
	for _, widget := range widgets {
		var results []map[string]any
		if widget.Query != "" {
			queryResults, err := d.executeQuery(widget.Query)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(fmt.Sprintf("error executing query for widget %d: %v", widget.ID, err.Error())))
				return
			}
			results = queryResults
		}

		config, err := bytesToWidgetConfig([]byte(widget.Config))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(fmt.Sprintf("error parsing config for widget %d: %v", widget.ID, err.Error())))
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
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
	}

	type DashboardResponse struct {
		URL         string              `json:"url"`
		Name        string              `json:"name"`
		Description *string             `json:"description"`
		CreatedAt   *string             `json:"created_at"`
		Widgets     []WidgetWithResults `json:"widgets"`
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
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

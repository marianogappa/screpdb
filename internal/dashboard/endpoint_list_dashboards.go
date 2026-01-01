package dashboard

import (
	"encoding/json"
	"net/http"
)

func (d *Dashboard) handlerListDashboards(w http.ResponseWriter, _ *http.Request) {
	dashboards, err := d.listDashboards(d.ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error listing dashboards from db: " + err.Error()))
		return
	}

	type DashboardResponse struct {
		URL         string  `json:"url"`
		Name        string  `json:"name"`
		Description *string `json:"description"`
		CreatedAt   *string `json:"created_at"`
	}
	response := make([]DashboardResponse, len(dashboards))
	for i, dash := range dashboards {
		var desc *string
		if dash.Description != nil {
			desc = dash.Description
		}
		var createdAt *string
		if dash.CreatedAt != nil {
			ts := dash.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
			createdAt = &ts
		}
		response[i] = DashboardResponse{
			URL:         dash.URL,
			Name:        dash.Name,
			Description: desc,
			CreatedAt:   createdAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}


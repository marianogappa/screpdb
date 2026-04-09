package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

func (d *Dashboard) handlerListDashboardWidgets(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardURL := vars["url"]
	if dashboardURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("dashboard url missing"))
		return
	}

	widgets, err := d.getDashboardWidgets(d.ctx, dashboardURL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error listing dashboards from db: " + err.Error()))
		return
	}

	type WidgetResponse struct {
		ID          int64   `json:"id"`
		DashboardID *string `json:"dashboard_id"`
		WidgetOrder *int64  `json:"widget_order"`
		Name        string  `json:"name"`
		Description *string `json:"description"`
		Config      []byte  `json:"config"`
		Query       string  `json:"query"`
		CreatedAt   *string `json:"created_at"`
		UpdatedAt   *string `json:"updated_at"`
	}
	response := make([]WidgetResponse, len(widgets))
	for i, w := range widgets {
		var createdAt *string
		if w.CreatedAt != nil {
			ts := w.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
			createdAt = &ts
		}
		var updatedAt *string
		if w.UpdatedAt != nil {
			ts := w.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
			updatedAt = &ts
		}
		response[i] = WidgetResponse{
			ID:          w.ID,
			DashboardID: w.DashboardID,
			WidgetOrder: w.WidgetOrder,
			Name:        w.Name,
			Description: w.Description,
			Config:      []byte(w.Config),
			Query:       w.Query,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}


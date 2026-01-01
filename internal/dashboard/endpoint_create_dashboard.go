package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
)

func (d *Dashboard) handlerCreateDashboard(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("error reading request POST payload"))
		return
	}
	type CreateDashboardRequest struct {
		URL         string  `json:"url"`
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	var req CreateDashboardRequest
	if err := json.Unmarshal(bs, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json: " + err.Error()))
		return
	}

	if req.URL == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("url cannot be empty"))
		return
	}

	if req.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("name cannot be empty"))
		return
	}

	dash, err := d.createDashboard(d.ctx, req.URL, req.Name, req.Description)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("error creating dashboard: " + err.Error()))
		return
	}

	type DashboardResponse struct {
		URL         string  `json:"url"`
		Name        string  `json:"name"`
		Description *string `json:"description"`
		CreatedAt   *string `json:"created_at"`
	}
	var dashDesc *string
	if dash.Description != nil {
		dashDesc = dash.Description
	}
	var dashCreatedAt *string
	if dash.CreatedAt != nil {
		ts := dash.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
		dashCreatedAt = &ts
	}
	response := DashboardResponse{
		URL:         dash.URL,
		Name:        dash.Name,
		Description: dashDesc,
		CreatedAt:   dashCreatedAt,
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

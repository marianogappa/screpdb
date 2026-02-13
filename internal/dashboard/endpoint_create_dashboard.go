package dashboard

import (
	"encoding/json"
	"io"
	"log"
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
		URL              string  `json:"url"`
		Name             string  `json:"name"`
		Description      *string `json:"description"`
		ReplaysFilterSQL *string `json:"replays_filter_sql"`
	}
	var req CreateDashboardRequest
	if err := json.Unmarshal(bs, &req); err != nil {
		log.Printf("dashboard create: invalid json err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json: " + err.Error()))
		return
	}

	if req.URL == "" {
		log.Printf("dashboard create: missing url")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("url cannot be empty"))
		return
	}

	if req.Name == "" {
		log.Printf("dashboard create: missing name url=%s", req.URL)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("name cannot be empty"))
		return
	}

	var replaysFilterSQL *string
	if req.ReplaysFilterSQL != nil {
		normalized, err := d.validateReplayFilterSQL(req.ReplaysFilterSQL)
		if err != nil {
			log.Printf("dashboard create: invalid replays_filter_sql url=%s err=%v", req.URL, err)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid replays_filter_sql: " + err.Error()))
			return
		}
		if normalized != "" {
			replaysFilterSQL = &normalized
		}
	}

	dash, err := d.createDashboard(d.ctx, req.URL, req.Name, req.Description, replaysFilterSQL)
	if err != nil {
		log.Printf("dashboard create: failed url=%s err=%v", req.URL, err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("error creating dashboard: " + err.Error()))
		return
	}

	type DashboardResponse struct {
		URL              string  `json:"url"`
		Name             string  `json:"name"`
		Description      *string `json:"description"`
		ReplaysFilterSQL *string `json:"replays_filter_sql"`
		CreatedAt        *string `json:"created_at"`
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
		URL:              dash.URL,
		Name:             dash.Name,
		Description:      dashDesc,
		ReplaysFilterSQL: dash.ReplaysFilterSQL,
		CreatedAt:        dashCreatedAt,
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

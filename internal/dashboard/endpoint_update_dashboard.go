package dashboard

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func (d *Dashboard) handlerUpdateDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardURL := vars["url"]
	if dashboardURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("dashboard url missing"))
		return
	}

	bs, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("error reading request POST payload"))
		return
	}

	dash, err := d.getDashboardByURL(d.ctx, dashboardURL)
	if err == sql.ErrNoRows {
		log.Printf("dashboard update: unknown url=%s", dashboardURL)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("unknown dashboard"))
		return
	}
	if err != nil {
		log.Printf("dashboard update: failed to load url=%s err=%v", dashboardURL, err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error getting dashboard: " + err.Error()))
		return
	}

	type UpdateDashboardRequest struct {
		Name             string  `json:"name"`
		Description      *string `json:"description"`
		ReplaysFilterSQL *string `json:"replays_filter_sql"`
	}
	var req UpdateDashboardRequest
	if err := json.Unmarshal(bs, &req); err != nil {
		log.Printf("dashboard update: invalid json url=%s err=%v", dashboardURL, err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json"))
		return
	}

	name := req.Name
	if name == "" {
		name = dash.Name
	}
	description := req.Description
	if description == nil && dash.Description != nil {
		description = dash.Description
	}

	var replaysFilterSQL *string
	if req.ReplaysFilterSQL != nil {
		normalized, err := d.validateReplayFilterSQL(req.ReplaysFilterSQL)
		if err != nil {
			log.Printf("dashboard update: invalid replays_filter_sql url=%s err=%v", dashboardURL, err)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid replays_filter_sql: " + err.Error()))
			return
		}
		replaysFilterSQL = &normalized
	}

	err = d.updateDashboard(d.ctx, dashboardURL, name, description, replaysFilterSQL)
	if err != nil {
		log.Printf("dashboard update: failed url=%s err=%v", dashboardURL, err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error updating dashboard: " + err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}

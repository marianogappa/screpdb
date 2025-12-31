package dashboard

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"database/sql"
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
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("unknown dashboard"))
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error getting dashboard: " + err.Error()))
		return
	}

	type UpdateDashboardRequest struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	var req UpdateDashboardRequest
	if err := json.Unmarshal(bs, &req); err != nil {
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

	err = d.updateDashboard(d.ctx, dashboardURL, name, description)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error updating dashboard: " + err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}


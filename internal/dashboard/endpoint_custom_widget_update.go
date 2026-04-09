package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"database/sql"
)

func (d *Dashboard) handlerUpdateDashboardWidget(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardWidgetID, err := strconv.Atoi(vars["wid"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("dashboard widget id missing or should be numeric"))
		return
	}

	bs, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("error reading request POST payload"))
		return
	}

	widget, err := d.getDashboardWidgetByID(d.ctx, int64(dashboardWidgetID))
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("unknown dashboard widget"))
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error getting dashboard widget: " + err.Error()))
		return
	}

	type UpdateWidgetRequest struct {
		Name        string       `json:"name"`
		Description *string      `json:"description"`
		Config      WidgetConfig `json:"config"`
		Query       string       `json:"query"`
	}

	var req UpdateWidgetRequest
	if err := json.Unmarshal(bs, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json"))
		return
	}

	currentConfig, err := bytesToWidgetConfig([]byte(widget.Config))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error parsing current config: " + err.Error()))
		return
	}

	name := req.Name
	if name == "" {
		name = widget.Name
	}
	description := req.Description
	if description == nil && widget.Description != nil {
		description = widget.Description
	}
	query := req.Query
	if query == "" {
		query = widget.Query
	}
	widgetOrder := widget.WidgetOrder
	if widgetOrder == nil {
		zero := int64(0)
		widgetOrder = &zero
	}

	configToUse := req.Config
	if configToUse.Type == "" {
		configToUse = currentConfig
	}

	configBytes, err := widgetConfigToBytes(configToUse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error marshaling config: " + err.Error()))
		return
	}

	err = d.updateDashboardWidget(d.ctx, int64(dashboardWidgetID), name, description, configBytes, query, widgetOrder)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error updating widget: " + err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}


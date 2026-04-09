package dashboard

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

func (d *Dashboard) handlerCreateDashboardWidget(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardURL := vars["url"]
	if dashboardURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("dashboard url missing"))
		return
	}

	type reqParams struct {
		Prompt string `json:"Prompt"`
	}

	bs, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("error reading request POST payload"))
		return
	}
	params := reqParams{}
	if err := json.Unmarshal(bs, &params); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json: " + err.Error()))
		return
	}

	// If prompt is provided, use AI to create widget
	if params.Prompt != "" {
		if !d.ai.IsAvailable() {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("OpenAI API key not configured. Cannot create widget with prompt."))
			return
		}
	} else {
		// Create widget without prompt - user will edit it manually
		order, err := d.getNextWidgetOrder(d.ctx, dashboardURL)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("couldn't get next widget order: " + err.Error()))
			return
		}

		emptyConfig := WidgetConfig{Type: WidgetTypeTable}
		configBytes, err := widgetConfigToBytes(emptyConfig)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("failed to marshal default config: " + err.Error()))
			return
		}

		widget, err := d.createDashboardWidget(d.ctx, dashboardURL, order, "New Widget", nil, configBytes, "")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("failed to create widget: " + err.Error()))
			return
		}

		againWidget, err := d.getDashboardWidgetByID(d.ctx, widget.ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("failed to get created widget: " + err.Error()))
			return
		}

		config, err := bytesToWidgetConfig([]byte(againWidget.Config))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error parsing config: " + err.Error()))
			return
		}

		type WidgetResponse struct {
			ID          int64        `json:"id"`
			DashboardID *string      `json:"dashboard_id"`
			WidgetOrder *int64       `json:"widget_order"`
			Name        string       `json:"name"`
			Description *string      `json:"description"`
			Config      WidgetConfig `json:"config"`
			Query       string       `json:"query"`
			CreatedAt   *string      `json:"created_at"`
			UpdatedAt   *string      `json:"updated_at"`
		}
		var createdAt *string
		if againWidget.CreatedAt != nil {
			ts := againWidget.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
			createdAt = &ts
		}
		var updatedAt *string
		if againWidget.UpdatedAt != nil {
			ts := againWidget.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
			updatedAt = &ts
		}
		response := WidgetResponse{
			ID:          againWidget.ID,
			DashboardID: againWidget.DashboardID,
			WidgetOrder: againWidget.WidgetOrder,
			Name:        againWidget.Name,
			Description: againWidget.Description,
			Config:      config,
			Query:       againWidget.Query,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Prompt-based widget creation (existing flow)
	order, err := d.getNextWidgetOrder(d.ctx, dashboardURL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("couldn't get next widget order: " + err.Error()))
		return
	}

	emptyConfig := WidgetConfig{Type: WidgetTypeTable}
	configBytes, err := widgetConfigToBytes(emptyConfig)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed to marshal default config: " + err.Error()))
		return
	}

	widget, err := d.createDashboardWidget(d.ctx, dashboardURL, order, "New Widget", nil, configBytes, "")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed to create widget: " + err.Error()))
		return
	}

	conv, err := d.ai.NewConversation(widget.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed to create conversation with AI: " + err.Error()))
		return
	}
	d.conversations[int(widget.ID)] = conv

	resp, err := conv.Prompt(params.Prompt)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed to prompt AI to create widget: " + err.Error()))
		return
	}

	configBytes, err = widgetConfigToBytes(resp.Config)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed to marshal config: " + err.Error()))
		return
	}

	var descPtr *string
	if resp.Description != "" {
		descPtr = &resp.Description
	}

	err = d.updateDashboardWidget(d.ctx, widget.ID, resp.Title, descPtr, configBytes, resp.SQLQuery, &order)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed to update widget: " + err.Error()))
		return
	}

	againWidget, err := d.getDashboardWidgetByID(d.ctx, widget.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed to get created widget: " + err.Error()))
		return
	}

	config, err := bytesToWidgetConfig([]byte(againWidget.Config))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error parsing config: " + err.Error()))
		return
	}

	type WidgetResponse struct {
		ID          int64        `json:"id"`
		DashboardID *string      `json:"dashboard_id"`
		WidgetOrder *int64       `json:"widget_order"`
		Name        string       `json:"name"`
		Description *string      `json:"description"`
		Config      WidgetConfig `json:"config"`
		Query       string       `json:"query"`
		CreatedAt   *string      `json:"created_at"`
		UpdatedAt   *string      `json:"updated_at"`
	}
	var createdAt *string
	if againWidget.CreatedAt != nil {
		ts := againWidget.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
		createdAt = &ts
	}
	var updatedAt *string
	if againWidget.UpdatedAt != nil {
		ts := againWidget.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		updatedAt = &ts
	}
	response := WidgetResponse{
		ID:          againWidget.ID,
		DashboardID: againWidget.DashboardID,
		WidgetOrder: againWidget.WidgetOrder,
		Name:        againWidget.Name,
		Description: againWidget.Description,
		Config:      config,
		Query:       againWidget.Query,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

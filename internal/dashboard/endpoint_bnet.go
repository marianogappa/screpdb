package dashboard

import (
	"encoding/json"
	"net/http"
)

func (d *Dashboard) handlerBnetStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d.getBnetStatus())
}

func (d *Dashboard) handlerBnetToggle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Disabled bool `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	d.setBnetDisabled(req.Disabled)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d.getBnetStatus())
}

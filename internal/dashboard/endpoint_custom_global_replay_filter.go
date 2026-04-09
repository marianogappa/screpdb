package dashboard

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

func (d *Dashboard) handlerGetGlobalReplayFilterConfig(w http.ResponseWriter, _ *http.Request) {
	config, err := d.getGlobalReplayFilterConfig(d.ctx)
	if err != nil {
		log.Printf("global replay filter get: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed to load global replay filter config"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(config)
}

func (d *Dashboard) handlerUpdateGlobalReplayFilterConfig(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("error reading request body"))
		return
	}

	var req globalReplayFilterConfig
	if err := json.Unmarshal(bs, &req); err != nil {
		log.Printf("global replay filter update: invalid json err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json"))
		return
	}

	config, err := d.updateGlobalReplayFilterConfig(d.ctx, req)
	if err != nil {
		log.Printf("global replay filter update: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid global replay filter config: " + err.Error()))
		return
	}
	if err := d.refreshReplayScopedDB(); err != nil {
		log.Printf("global replay filter refresh: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed to refresh replay scoped db"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(config)
}

func (d *Dashboard) handlerGetGlobalReplayFilterOptions(w http.ResponseWriter, _ *http.Request) {
	options, err := d.listGlobalReplayFilterOptions(d.ctx)
	if err != nil {
		log.Printf("global replay filter options: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed to load global replay filter options"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(options)
}

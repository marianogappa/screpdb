package dashboard

import (
	"encoding/json"
	"log"
	"net/http"
)

func (d *Dashboard) handlerHealthcheck(w http.ResponseWriter, _ *http.Request) {
	totalReplays, err := d.dbStore.CountReplays(d.ctx)
	if err != nil {
		log.Printf("healthcheck replay count failed: %v", err)
		totalReplays = 0
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":            true,
		"openai_enabled": d.ai.IsAvailable(),
		"total_replays": totalReplays,
	})
}

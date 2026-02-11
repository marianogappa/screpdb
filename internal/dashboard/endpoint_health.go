package dashboard

import (
	"encoding/json"
	"log"
	"net/http"
)

func (d *Dashboard) handlerHealthcheck(w http.ResponseWriter, _ *http.Request) {
	var totalReplays int64
	if err := d.db.QueryRow("SELECT COUNT(*) FROM replays").Scan(&totalReplays); err != nil {
		log.Printf("healthcheck replay count failed: %v", err)
		totalReplays = 0
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":            true,
		"openai_enabled": d.ai.IsAvailable(),
		"total_replays": totalReplays,
	})
}

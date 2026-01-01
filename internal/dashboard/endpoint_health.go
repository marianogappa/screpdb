package dashboard

import (
	"encoding/json"
	"net/http"
)

func (d *Dashboard) handlerHealthcheck(w http.ResponseWriter, _ *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

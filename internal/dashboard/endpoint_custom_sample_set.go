package dashboard

import (
	"encoding/json"
	"net/http"
)

// handlerLoadSampleSet extracts the embedded sample replays, points ingest at
// them and starts an async ingest. Registered as a manual route (not in the
// OpenAPI spec) so the request-validation middleware falls through to it.
func (d *Dashboard) handlerLoadSampleSet(w http.ResponseWriter, r *http.Request) {
	if err := d.loadSampleSet(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"ok":        true,
		"input_dir": d.sampleSetDir(),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

// markerDefinition is the JSON shape the frontend consumes. Mirrors markers.Marker
// minus the Rule/Custom/Expert fields which are backend-only behaviour.
type markerDefinition struct {
	FeatureKey    string           `json:"feature_key"`
	Name          string           `json:"name"`
	Kind          string           `json:"kind"`
	Race          string           `json:"race,omitempty"`
	SummaryPlayer *markers.Pill    `json:"summary_player,omitempty"`
	SummaryReplay *markers.Pill    `json:"summary_replay,omitempty"`
	GamesList     *markers.Pill    `json:"games_list,omitempty"`
	EventsList    *markers.Pill    `json:"events_list,omitempty"`
}

type markersDefinitionsResponse struct {
	AlgorithmVersion int                          `json:"algorithm_version"`
	Markers          map[string]markerDefinition `json:"markers"`
}

// handlerMarkersDefinitions serves the per-marker Pill metadata. Cached in-memory
// by the frontend; re-fetched when the server's algorithm_version differs from
// the one the frontend last saw.
func (d *Dashboard) handlerMarkersDefinitions(w http.ResponseWriter, _ *http.Request) {
	all := markers.Markers()
	out := make(map[string]markerDefinition, len(all))
	for i := range all {
		m := &all[i]
		out[m.FeatureKey] = markerDefinition{
			FeatureKey:    m.FeatureKey,
			Name:          m.Name,
			Kind:          string(m.Kind),
			Race:          string(m.Race),
			SummaryPlayer: m.SummaryPlayer,
			SummaryReplay: m.SummaryReplay,
			GamesList:     m.GamesList,
			EventsList:    m.EventsList,
		}
	}

	resp := markersDefinitionsResponse{
		AlgorithmVersion: core.AlgorithmVersion,
		Markers:          out,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

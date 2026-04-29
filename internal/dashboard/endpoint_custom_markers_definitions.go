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
	FeatureKey    string        `json:"feature_key"`
	Name          string        `json:"name"`
	Kind          string        `json:"kind"`
	Race          string        `json:"race,omitempty"`
	Matchup       []string      `json:"matchup,omitempty"`
	MapKind       []string      `json:"map_kind,omitempty"`
	SummaryPlayer *markers.Pill `json:"summary_player,omitempty"`
	SummaryReplay *markers.Pill `json:"summary_replay,omitempty"`
	GamesList     *markers.Pill `json:"games_list,omitempty"`
	EventsList    *markers.Pill `json:"events_list,omitempty"`
}

// gameEventFeature covers the game-event-only featuring chips (cannon_rush,
// bunker_rush, zergling_rush, mind_control) that aren't markers but still need
// a frontend-renderable entry in the Featuring strip.
type gameEventFeature struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	IconKey string `json:"icon_key"`
}

type markersDefinitionsResponse struct {
	AlgorithmVersion  int                         `json:"algorithm_version"`
	Markers           map[string]markerDefinition `json:"markers"`
	FeaturingOrder    []string                    `json:"featuring_order"`
	GameEventFeatures []gameEventFeature          `json:"game_event_features"`
}

// staticGameEventFeatures enumerates the featuring-chip entries the FE needs
// for narrative game_events that aren't markers. These still live in
// replay_events (event_kind='game_event') and share the featuring-strip UI
// surface with markers.
var staticGameEventFeatures = []gameEventFeature{
	{Key: "cannon_rush", Label: "Cannon rush", IconKey: "photoncannon"},
	{Key: "bunker_rush", Label: "Bunker rush", IconKey: "bunker"},
	{Key: "zergling_rush", Label: "Zergling rush", IconKey: "zergling"},
	{Key: "proxy_gate", Label: "Proxy gateway", IconKey: "gateway"},
	{Key: "proxy_rax", Label: "Proxy barracks", IconKey: "barracks"},
	{Key: "proxy_factory", Label: "Proxy factory", IconKey: "factory"},
	{Key: "mind_control", Label: "Mind control", IconKey: "darkarchon"},
}

// staticFeaturingOrder fixes the display order of featuring chips. Mixes game-
// event-only keys with marker FeatureKeys so the FE can render a single
// ordered strip without a parallel lookup table.
var staticFeaturingOrder = []string{
	// signature markers (KindMarker, persistent presence)
	"carriers",
	"battlecruisers",
	"ten_plus_scouts",
	"mech",
	// game-event-only chips (sourced from worldstate, not markers)
	"cannon_rush",
	"bunker_rush",
	"zergling_rush",
	"proxy_gate",
	"proxy_rax",
	"proxy_factory",
	"mind_control",
	// late-game custom-evaluator markers
	"threw_nukes",
	"made_recalls",
	"mech_transition",
	// transition markers (KindMarker)
	"one_one_one",
	"sk_terran",
	// initial build orders, ordered by race + ascending supply / aggression
	"bo_4_pool",
	"bo_9_pool",
	"bo_9_overpool",
	"bo_12_pool",
	"bo_9_pool_hatch",
	"bo_9_hatch",
	"bo_10_hatch",
	"bo_11_hatch",
	"bo_12_hatch",
	"bo_2_gate",
	"bo_1_gate_core",
	"bo_nexus_first",
	"bo_gate_expand",
	"bo_forge_expa",
	"bo_bbs",
	"bo_1_rax_1_fac",
	"bo_rax_cc",
	"bo_cc_first",
}

// handlerMarkersDefinitions serves the per-marker Pill metadata plus ordering
// and game-event feature metadata. Cached in-memory by the frontend; re-fetched
// when the server's algorithm_version differs from the one the frontend last saw.
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
			Matchup:       m.Matchup,
			MapKind:       m.MapKind,
			SummaryPlayer: m.SummaryPlayer,
			SummaryReplay: m.SummaryReplay,
			GamesList:     m.GamesList,
			EventsList:    m.EventsList,
		}
	}

	resp := markersDefinitionsResponse{
		AlgorithmVersion:  core.AlgorithmVersion,
		Markers:           out,
		FeaturingOrder:    staticFeaturingOrder,
		GameEventFeatures: staticGameEventFeatures,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

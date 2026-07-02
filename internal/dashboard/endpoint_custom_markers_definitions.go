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
	// Curated is true when the detection should NOT be flagged "beta" — either a
	// human verified it against a real replay (a tier-1 premise in
	// GOLDEN_TIERS.md) or it is beta-exempt (a deterministic measurement like
	// hotkey usage that can't be human-checked). When false the frontend flags
	// the detection as "beta".
	Curated bool `json:"curated"`
}

// gameEventFeature covers the game-event-only featuring chips (cannon_rush,
// bunker_rush, zergling_rush, mind_control) that aren't markers but still need
// a frontend-renderable entry in the Featuring strip.
//
// IconKeys (multi-icon) wins over IconKey when both are populated — mirrors
// the marker-filter chip layout so subtype pills (cliff drop) can
// surface a shuttle + payload-unit pair like the games-list filter row.
type gameEventFeature struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	IconKey  string   `json:"icon_key,omitempty"`
	IconKeys []string `json:"icon_keys,omitempty"`
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
	{Key: "proxy_starport", Label: "Proxy starport", IconKey: "starport"},
	{Key: "drop", Label: "Drop", IconKey: "shuttle"},
	{Key: "mind_control", Label: "Mind control", IconKey: "darkarchon"},
}

// staticFeaturingOrder fixes the display order of featuring chips. Mixes game-
// event-only keys with marker FeatureKeys so the FE can render a single
// ordered strip without a parallel lookup table.
var staticFeaturingOrder = []string{
	// game-event-only chips (sourced from worldstate, not markers)
	"cannon_rush",
	"bunker_rush",
	"zergling_rush",
	"proxy_gate",
	"proxy_rax",
	"proxy_factory",
	"proxy_starport",
	"manner_pylon",
	"drop",
	"mind_control",
	// late-game custom-evaluator markers
	"threw_nukes",
	"made_recalls",
	"made_maelstrom",
	"offensive_nydus",
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
	"bo_13_hatch",
	"bo_z_fuzzy",
	// N Hatch {Hydra|Muta|Lurker} composition markers (issue #245) — the base
	// count at the economy→army transition, layered on top of the supply opener.
	"nhatch_hydra",
	"nhatch_muta",
	"nhatch_lurker",
	"bo_2_gate",
	"bo_1_gate_core",
	"bo_nexus_first",
	"bo_gate_expand",
	"bo_forge_expa",
	// Preferred Protoss tech-pathway openers (issue #182).
	"bo_p_1gate_reaver",
	"bo_p_gate_forge_cannon",
	"bo_p_forge_cannon_gate",
	"bo_p_forge_gate_cannon",
	"bo_bbs",
	"bo_cc_first",
	// Terran composition BOs (issues #155/#226/#227): mech named by the number
	// of Factories before the first expansion ("N Fact Expa Mech"), tank and
	// tankless variants, plus 1-1-1 and the 2 Starport air openers.
	"bo_t_bio_1base",
	"bo_t_bio_2base",
	"bo_t_111_mech",
	"bo_t_111_tankless",
	"bo_t_111",
	"bo_t_mech_expa_1fac",
	"bo_t_mech_expa_2fac",
	"bo_t_mech_expa_3fac",
	"bo_t_mech_expa_4fac",
	"bo_t_mech_expa_5fac",
	"bo_t_mech_expa_6fac",
	"bo_t_goliath_expa_1fac",
	"bo_t_goliath_expa_2fac",
	"bo_t_goliath_expa_3fac",
	"bo_t_goliath_expa_4fac",
	"bo_t_goliath_expa_5fac",
	"bo_t_goliath_expa_6fac",
	"bo_t_tankless_expa_1fac",
	"bo_t_tankless_expa_2fac",
	"bo_t_tankless_expa_3fac",
	"bo_t_tankless_expa_4fac",
	"bo_t_tankless_expa_5fac",
	"bo_t_tankless_expa_6fac",
	"bo_t_mech_expand",
	"bo_t_goliath_expand",
	"bo_t_tankless_expand",
	"bo_t_mech_noexpa",
	"bo_t_goliath_noexpa",
	"bo_t_tankless_noexpa",
	// Preferred Terran air openers (N Starport cluster).
	"bo_t_2starport_wraith",
	"bo_t_2starport_valk",
	"bo_t_3starport_wraith",
	"bo_t_3starport_valk",
	// late-game / signature markers.
	"double_stargate",
	"crazy_zerg",
	"guardians",
	// money-map markers — rendered last so regular markers take priority on
	// mixed/regular game listings; on Money games they trail Mind Control etc.
	"carriers",
	"battlecruisers",
	"ten_plus_scouts",
	"cliff_drop",
}

// GameEventFeatureSpec is the exported, package-stable view of one non-marker
// "game event" featuring chip (cannon_rush, drop, …). Used by the
// SPECIFICATION.md generator. IconKeys collapses the internal single/multi icon
// fields into one ordered list.
type GameEventFeatureSpec struct {
	Key      string
	Label    string
	IconKeys []string
}

// AllGameEventFeatures returns the non-marker game-event featuring chips in
// their declared order. Used by the SPECIFICATION.md generator.
func AllGameEventFeatures() []GameEventFeatureSpec {
	out := make([]GameEventFeatureSpec, 0, len(staticGameEventFeatures))
	for _, f := range staticGameEventFeatures {
		icons := f.IconKeys
		if len(icons) == 0 && f.IconKey != "" {
			icons = []string{f.IconKey}
		}
		out = append(out, GameEventFeatureSpec{Key: f.Key, Label: f.Label, IconKeys: icons})
	}
	return out
}

// FeaturingOrder returns the fixed display order of every featuring-strip chip
// (a mix of marker FeatureKeys and game-event keys). Used by the
// SPECIFICATION.md generator and cross-consistency tests.
func FeaturingOrder() []string {
	out := make([]string, len(staticFeaturingOrder))
	copy(out, staticFeaturingOrder)
	return out
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
			Curated:       markers.IsCurated(m.FeatureKey) || markers.IsBetaExempt(m.FeatureKey),
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

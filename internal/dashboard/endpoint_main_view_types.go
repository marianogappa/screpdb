package dashboard

import "encoding/json"

const workflowSummaryVersion = "v2"

var topPlayerPalette = []string{
	"#3B82F6",
	"#F59E0B",
	"#10B981",
	"#EF4444",
	"#8B5CF6",
	"#06B6D4",
	"#84CC16",
	"#F97316",
	"#EC4899",
	"#6366F1",
	"#14B8A6",
	"#EAB308",
	"#22C55E",
	"#F43F5E",
	"#A855F7",
}

const firstUnitEfficiencyMaxGapSeconds int64 = 60
const workflowPlayerDelayMinSamples int64 = 5
const workflowPlayerDelayCutoffSeconds int64 = 7 * 60
const workflowPlayerDelayMaxGapSeconds int64 = 20
const workflowUnitCadenceStartSeconds int64 = 7 * 60
const workflowUnitCadenceEndFraction float64 = 0.8
const workflowUnitCadenceIdleGapSeconds int64 = 20
const workflowUnitCadenceMinUnitsPerReplay int64 = 12
const workflowUnitCadenceMinGapsPerReplay int64 = 8
const workflowUnitCadenceMinGames int64 = 4
const workflowUnitCadenceDefaultLimit int64 = 50
const workflowUnitCadenceMaxLimit int64 = 200

type workflowUnitCadenceFilterMode string

const (
	workflowUnitCadenceFilterStrict workflowUnitCadenceFilterMode = "strict"
	workflowUnitCadenceFilterBroad  workflowUnitCadenceFilterMode = "broad"
)

type firstUnitEfficiencyUnitOption struct {
	DisplayName string
	MatchKeys   []string
}

type firstUnitEfficiencyConfig struct {
	Race                 string
	BuildingName         string
	BuildDurationSeconds int64
	Units                []firstUnitEfficiencyUnitOption
}

type workflowFirstUnitEfficiencyState struct {
	buildTimesByUnit map[string][]int64
	unitTimesByUnit  map[string][]int64
}

type workflowPlayerDelaySample struct {
	PlayerKey            string
	PlayerName           string
	BuildingName         string
	UnitName             string
	GapAfterReadySeconds int64
}

var firstUnitEfficiencyConfigs = []firstUnitEfficiencyConfig{
	{
		Race:                 "protoss",
		BuildingName:         "Forge",
		BuildDurationSeconds: 25,
		Units:                []firstUnitEfficiencyUnitOption{{DisplayName: "Photon Cannon", MatchKeys: []string{"Photon Cannon"}}},
	},
	{
		Race:                 "protoss",
		BuildingName:         "Gateway",
		BuildDurationSeconds: 38,
		Units:                []firstUnitEfficiencyUnitOption{{DisplayName: "Zealot", MatchKeys: []string{"Zealot"}}},
	},
	{
		Race:                 "protoss",
		BuildingName:         "Stargate",
		BuildDurationSeconds: 44,
		Units: []firstUnitEfficiencyUnitOption{
			{DisplayName: "Corsair", MatchKeys: []string{"Corsair"}},
			{DisplayName: "Scout", MatchKeys: []string{"Scout"}},
		},
	},
	{
		Race:                 "protoss",
		BuildingName:         "Fleet Beacon",
		BuildDurationSeconds: 38,
		Units:                []firstUnitEfficiencyUnitOption{{DisplayName: "Carrier", MatchKeys: []string{"Carrier"}}},
	},
	{
		Race:                 "protoss",
		BuildingName:         "Arbiter Tribunal",
		BuildDurationSeconds: 38,
		Units:                []firstUnitEfficiencyUnitOption{{DisplayName: "Arbiter", MatchKeys: []string{"Arbiter"}}},
	},
	{
		Race:                 "terran",
		BuildingName:         "Barracks",
		BuildDurationSeconds: 50,
		Units:                []firstUnitEfficiencyUnitOption{{DisplayName: "Marine", MatchKeys: []string{"Marine"}}},
	},
	{
		Race:                 "terran",
		BuildingName:         "Factory",
		BuildDurationSeconds: 50,
		Units: []firstUnitEfficiencyUnitOption{
			{DisplayName: "Vulture", MatchKeys: []string{"Vulture"}},
			{DisplayName: "Siege Tank", MatchKeys: []string{"Siege Tank", "Siege Tank (Tank Mode)", "Terran Siege Tank (Siege Mode)"}},
		},
	},
	{
		Race:                 "terran",
		BuildingName:         "Physics Lab",
		BuildDurationSeconds: 25,
		Units:                []firstUnitEfficiencyUnitOption{{DisplayName: "Battlecruiser", MatchKeys: []string{"Battlecruiser"}}},
	},
	{
		Race:                 "zerg",
		BuildingName:         "Spawning Pool",
		BuildDurationSeconds: 50,
		Units:                []firstUnitEfficiencyUnitOption{{DisplayName: "Zergling", MatchKeys: []string{"Zergling"}}},
	},
	{
		Race:                 "zerg",
		BuildingName:         "Hydralisk Den",
		BuildDurationSeconds: 25,
		Units:                []firstUnitEfficiencyUnitOption{{DisplayName: "Hydralisk", MatchKeys: []string{"Hydralisk"}}},
	},
	{
		Race:                 "zerg",
		BuildingName:         "Spire",
		BuildDurationSeconds: 75,
		Units: []firstUnitEfficiencyUnitOption{
			{DisplayName: "Mutalisk", MatchKeys: []string{"Mutalisk"}},
			{DisplayName: "Scourge", MatchKeys: []string{"Scourge"}},
		},
	},
	{
		Race:                 "zerg",
		BuildingName:         "Ultralisk Cavern",
		BuildDurationSeconds: 50,
		Units:                []firstUnitEfficiencyUnitOption{{DisplayName: "Ultralisk", MatchKeys: []string{"Ultralisk"}}},
	},
	{
		Race:                 "zerg",
		BuildingName:         "Defiler Mound",
		BuildDurationSeconds: 38,
		Units:                []firstUnitEfficiencyUnitOption{{DisplayName: "Defiler", MatchKeys: []string{"Defiler"}}},
	},
}

type workflowGameListItem struct {
	ReplayID           int64                     `json:"replay_id"`
	ReplayDate         string                    `json:"replay_date"`
	FileName           string                    `json:"file_name"`
	MapName            string                    `json:"map_name"`
	MapKind            string                    `json:"map_kind,omitempty"`
	DurationSeconds    int64                     `json:"duration_seconds"`
	GameType           string                    `json:"game_type"`
	PlayersLabel       string                    `json:"players_label"`
	WinnersLabel       string                    `json:"winners_label"`
	Matchup            string                    `json:"matchup"`
	TeamStacking       bool                      `json:"team_stacking"`
	TeamInfoIncomplete bool                      `json:"team_info_incomplete"`
	Players            []workflowGameListPlayer  `json:"players"`
	Featuring          []string                  `json:"featuring"`
	CurrentPlayer      *workflowRecentGamePlayer `json:"current_player,omitempty"`
}

type workflowGameListPlayer struct {
	PlayerID  int64  `json:"player_id"`
	PlayerKey string `json:"player_key"`
	Name      string `json:"name"`
	Race      string `json:"race"`
	Team      int64  `json:"team"`
	IsWinner  bool   `json:"is_winner"`
}

type workflowRecentGamePlayer struct {
	PlayerID         int64                  `json:"player_id"`
	PlayerKey        string                 `json:"player_key"`
	Name             string                 `json:"name"`
	Race             string                 `json:"race"`
	IsWinner         bool                   `json:"is_winner"`
	DetectedPatterns []workflowPatternValue `json:"detected_patterns"`
}

type workflowGamesListFilters struct {
	PlayerKeys      []string
	MapNames        []string
	DurationBuckets []string
	FeaturingKeys   []string
	MatchupKeys     []string
	MapKindKeys     []string
}

type workflowGamesListFilterOption struct {
	Key       string   `json:"key"`
	Label     string   `json:"label"`
	Games     int64    `json:"games"`
	IconKey   string   `json:"icon_key,omitempty"`
	IconKeys  []string `json:"icon_keys,omitempty"`
	IconLabel string   `json:"icon_label,omitempty"`
	Emoji     string   `json:"emoji,omitempty"`
	Group     string   `json:"group,omitempty"`
}

type workflowGamesListFilterOptions struct {
	Players   []workflowGamesListFilterOption `json:"players"`
	Maps      []workflowGamesListFilterOption `json:"maps"`
	Durations []workflowGamesListFilterOption `json:"durations"`
	Featuring []workflowGamesListFilterOption `json:"featuring"`
	Matchups  []workflowGamesListFilterOption `json:"matchups"`
	MapKinds  []workflowGamesListFilterOption `json:"map_kinds"`
}

// workflowMatchupFilters lists the canonical matchup keys. TvZ==ZvT and
// PvZ==ZvP, so the key is always the alphabetically-sorted race-pair form.
var workflowMatchupFilters = []struct {
	Key   string
	Label string
}{
	{Key: "pvp", Label: "PvP"},
	{Key: "pvt", Label: "PvT"},
	{Key: "pvz", Label: "PvZ"},
	{Key: "tvt", Label: "TvT"},
	{Key: "tvz", Label: "TvZ"},
	{Key: "zvz", Label: "ZvZ"},
}

// workflowFeaturingFilters lists the chips on the games-list filter bar.
//
// Group splits the row visually on the frontend:
//   - "marker" → narrative/late-game/rush markers (always visible)
//   - "bo"     → opener build orders (collapsed under a disclosure by default)
//
// IconKey / IconKeys, when set, render the chip with one or more unit/building
// icons (Label remains the tooltip). IconKeys (multi-icon) wins over IconKey
// when both are present. IconLabel, when set, renders short text (e.g. "Rush",
// "Proxy") next to the icon(s) for disambiguation.
var workflowFeaturingFilters = []struct {
	Key       string
	Label     string
	Group     string
	IconKey   string
	IconKeys  []string
	IconLabel string
	Emoji     string
}{
	{Key: "carriers", Label: "Carrier", Group: "marker", IconKey: "carrier"},
	{Key: "battlecruisers", Label: "Battlecruiser", Group: "marker", IconKey: "battlecruiser"},
	{Key: "ten_plus_scouts", Label: "10+ Scouts", Group: "marker", IconKey: "scout", IconLabel: "10+"},
	{Key: "mech", Label: "Mech", Group: "marker", IconKeys: []string{"siegetank", "goliath"}, IconLabel: "Mech"},
	{Key: "sk_terran", Label: "SK Terran", Group: "marker", IconKeys: []string{"marine", "medic"}, IconLabel: "SK Terran"},
	{Key: "one_one_one", Label: "1-1-1", Group: "marker"},
	{Key: "mech_transition", Label: "Mech Transition", Group: "marker", IconKeys: []string{"siegetank", "goliath"}, IconLabel: "Mech transition"},
	{Key: "mutalisk_timing", Label: "Mutalisk timing", Group: "marker", IconKey: "mutalisk", IconLabel: "timing"},
	{Key: "turret_timing", Label: "Turret timing", Group: "marker", IconKey: "missileturret", IconLabel: "Timing"},
	{Key: "cliff_drop", Label: "Cliff drop", Group: "marker", IconKey: "dropship", IconLabel: "Cliff drop"},
	{Key: "cannon_rush", Label: "Cannon Rush", Group: "marker", IconKey: "photoncannon", IconLabel: "Rush"},
	{Key: "bunker_rush", Label: "Bunker Rush", Group: "marker", IconKey: "bunker", IconLabel: "Rush"},
	{Key: "zergling_rush", Label: "Zergling Rush", Group: "marker", IconKey: "zergling", IconLabel: "Rush"},
	{Key: "proxy_gate", Label: "Proxy Gateway", Group: "marker", IconKey: "gateway", IconLabel: "Proxy"},
	{Key: "proxy_rax", Label: "Proxy Barracks", Group: "marker", IconKey: "barracks", IconLabel: "Proxy"},
	{Key: "proxy_factory", Label: "Proxy Factory", Group: "marker", IconKey: "factory", IconLabel: "Proxy"},
	{Key: "mind_control", Label: "Mind Control", Group: "marker", IconKey: "darkarchon", IconLabel: "Mind Control"},
	{Key: "nukes", Label: "Nukes", Group: "marker", IconKey: "ghost", IconLabel: "Nuke"},
	{Key: "recalls", Label: "Recalls", Group: "marker", IconKey: "arbiter", IconLabel: "Recall"},
	{Key: "team_stacking", Label: "Team stacking", Group: "marker", Emoji: "😈"},
	// Build order pills — keys & labels kept in sync with internal/markers.
	// Suppressed in render for Money maps (game-list + replay-summary
	// featuring strips); BO tab and per-player summary pills still show.
	{Key: "bo_4_pool", Label: "4 Pool", Group: "bo"},
	{Key: "bo_9_pool", Label: "9 Pool", Group: "bo"},
	{Key: "bo_9_overpool", Label: "9 Overpool", Group: "bo"},
	{Key: "bo_12_pool", Label: "12 Pool", Group: "bo"},
	{Key: "bo_9_pool_hatch", Label: "9 Pool into Hatchery", Group: "bo"},
	{Key: "bo_9_hatch", Label: "9 Hatch", Group: "bo"},
	{Key: "bo_10_hatch", Label: "10 Hatch", Group: "bo"},
	{Key: "bo_11_hatch", Label: "11 Hatch", Group: "bo"},
	{Key: "bo_12_hatch", Label: "12 Hatch", Group: "bo"},
	{Key: "bo_1_gate_core", Label: "1 Gate Core", Group: "bo"},
	{Key: "bo_2_gate", Label: "2 Gate", Group: "bo"},
	{Key: "bo_nexus_first", Label: "Nexus First", Group: "bo"},
	{Key: "bo_gate_expand", Label: "Gate Expand", Group: "bo"},
	{Key: "bo_forge_expa", Label: "Forge Expand", Group: "bo"},
	{Key: "bo_1_rax_1_fac", Label: "1 Rax 1 Fac", Group: "bo"},
	{Key: "bo_rax_cc", Label: "1 Rax FE", Group: "bo"},
	{Key: "bo_cc_first", Label: "CC First", Group: "bo"},
	{Key: "bo_bbs", Label: "BBS", Group: "bo"},
}

var workflowDurationFilterBuckets = []struct {
	Key   string
	Label string
}{
	{Key: "under_10m", Label: "<10m"},
	{Key: "10m_plus", Label: "10m+"},
}

var workflowMapKindFilters = []struct {
	Key   string
	Label string
}{
	{Key: "money", Label: "Money maps"},
	{Key: "regular", Label: "Regular maps"},
}

type workflowGamePlayer struct {
	PlayerID           int64                  `json:"player_id"`
	PlayerKey        string                 `json:"player_key"`
	Name             string                 `json:"name"`
	Color            string                 `json:"color,omitempty"`
	Race             string                 `json:"race"`
	Team             int64                  `json:"team"`
	IsWinner         bool                   `json:"is_winner"`
	APM              int64                  `json:"apm"`
	EAPM             int64                  `json:"eapm"`
	DetectedPatterns []workflowPatternValue `json:"detected_patterns"`
}

// workflowPatternValue is the per-pattern entry shipped to the frontend inside
// detected_patterns[] on the game-summary response. EventType is the marker
// FeatureKey (e.g. "carriers", "bo_9_pool") and is the stable identifier.
// DetectedSecond is the replay second the marker committed at; the FE uses it
// to render "{minute}" interpolations on pill labels. Payload carries optional
// JSON extras for markers that store structured data (hotkey groups, viewport
// switches-per-minute).
type workflowPatternValue struct {
	EventType      string          `json:"event_type"`
	DetectedSecond int             `json:"detected_second"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

type workflowGameDetail struct {
	SummaryVersion       string                                   `json:"summary_version"`
	ReplayID             int64                                    `json:"replay_id"`
	ReplayDate           string                                   `json:"replay_date"`
	FileName             string                                   `json:"file_name"`
	FilePath             string                                   `json:"file_path"`
	MapName              string                                   `json:"map_name"`
	MapKind              string                                   `json:"map_kind,omitempty"`
	MapVisual            workflowMapVisual                        `json:"map_visual"`
	MapWidthPixels       int64                                    `json:"map_width_pixels,omitempty"`
	MapHeightPixels      int64                                    `json:"map_height_pixels,omitempty"`
	DurationSeconds      int64                                    `json:"duration_seconds"`
	GameType             string                                   `json:"game_type"`
	TeamStacking         bool                                     `json:"team_stacking"`
	TeamInfoIncomplete   bool                                     `json:"team_info_incomplete"`
	Players              []workflowGamePlayer                     `json:"players"`
	ReplayPatterns       []workflowPatternValue                   `json:"replay_patterns"`
	GameEvents           []workflowGameEvent                      `json:"game_events"`
	UnitsBySlice         []workflowUnitSlice                      `json:"units_by_slice"`
	UnitsEarlyEvents     []workflowUnitEarlyEventPlayer           `json:"units_early_events"`
	Timings              workflowReplayTimings                    `json:"timings"`
	FirstUnitEfficiency  []workflowFirstUnitEfficiencyPlayer      `json:"first_unit_efficiency"`
	UnitCadence          []workflowGameUnitCadencePlayer          `json:"unit_production_cadence"`
	ViewportMultitasking []workflowGameViewportMultitaskingPlayer `json:"viewport_multitasking"`
	Markers               []workflowMarkerPlayer         `json:"build_orders"`
	MutaliskTiming        []workflowMarkerPlayer         `json:"mutalisk_timing_chart,omitempty"`
	MutaliskTimingSummary *workflowMutaliskTimingSummary `json:"mutalisk_timing_summary,omitempty"`
	AllianceTimeline     []workflowAllianceSnapshot               `json:"alliance_timeline,omitempty"`
	AllianceStackingThresholdSeconds int64                        `json:"alliance_stacking_threshold_seconds,omitempty"`

	// EarlyGameEndsAtSecond / MidGameEndsAtSecond split the game-events list
	// into Early/Mid/Late sections. Computed from unit-completion + research
	// timings (see populatePhaseMarkersForGameDetail). Zero = no boundary
	// detected — the frontend collapses the empty section header.
	EarlyGameEndsAtSecond int64 `json:"early_game_ends_at_second,omitempty"`
	MidGameEndsAtSecond   int64 `json:"mid_game_ends_at_second,omitempty"`

	// UnitCompositionMarkers is a flat list of (player, phase) attacker-
	// composition rows for this replay. The frontend aggregates them into
	// three replay-level pills at display time (per-game summary surface)
	// and renders per-player rows on individual player strips. Source rows
	// are replay_events with event_type LIKE 'unit_composition_%'.
	UnitCompositionMarkers []workflowGameUnitComposition `json:"unit_composition_markers,omitempty"`
}

// workflowAllianceSnapshot is one observed team topology in the Alliances tab.
// Valid from Sec until the next snapshot's Sec (or duration_seconds for the
// last snapshot). Teams is a list of teams; each team is a sorted list of
// player_ids (the DB row id, not the screp player_id).
type workflowAllianceSnapshot struct {
	Sec      int64     `json:"sec"`
	Teams    [][]int64 `json:"teams"`
	Stacking bool      `json:"stacking"`
}

// workflowMarkerPlayer carries per-player Build Orders tab data:
// the detected BO name plus expert-vs-actual timing for each milestone.
// Populated by populateMarkersForGameDetail in endpoint_main_game_detail.go.
type workflowMarkerPlayer struct {
	PlayerID     int64                     `json:"player_id"`
	PlayerKey    string                    `json:"player_key"`
	Name         string                    `json:"name"`
	Race         string                    `json:"race"`
	Marker   string                    `json:"build_order"`        // e.g. "9 pool"
	FeatureKey   string                    `json:"feature_key"`        // e.g. "bo_9_pool"
	Events       []workflowMarkerEvent `json:"events"`
}

// workflowMarkerEvent is one row in the Build Orders timeline chart.
// NoExpert=true rows are sourced from the player's command stream (drone
// morph counts + pool/overlord/hatch first occurrences) rather than the
// marker definition's Expert template — render them without the golden
// tolerance band.
type workflowMarkerEvent struct {
	Key                   string `json:"key"`                     // e.g. "Spawning Pool"
	Subject               string `json:"subject"`                 // canonical unit/building name for icon lookup (e.g. "Zergling")
	TargetSecond          int64  `json:"target_second"`
	ToleranceEarlySeconds int64  `json:"tolerance_early_seconds"`
	ToleranceLateSeconds  int64  `json:"tolerance_late_seconds"`
	ActualSecond          int64  `json:"actual_second"` // valid only when Found
	Found                 bool   `json:"found"`
	DeltaSeconds          int64  `json:"delta_seconds"` // actual - target; + late, - early
	WithinTolerance       bool   `json:"within_tolerance"`
	NoExpert              bool   `json:"no_expert,omitempty"` // true = no golden range; render actual only
	// BuildTimeSeconds is the in-game build duration of the unit/building this
	// row represents. When >0 the chart renders a horizontal span from the
	// start tick to start+BuildTime, ending at the completion second — making
	// it visually obvious that the chart's tick is "build started" while the
	// unit/building is only available from start+BuildTime onward. 0 = render
	// without a span bar (caller didn't supply a build time).
	BuildTimeSeconds int64 `json:"build_time_seconds,omitempty"`
	// ActualBuiltSecond / TargetBuiltSecond, when > 0, override the naive
	// "Actual + BuildTime" / "Target + BuildTime" finish-time calculation. Used
	// by markers whose subject has prerequisites (e.g. a Mutalisk morph cmd
	// queued before its Spire finishes — the morph effectively starts when the
	// Spire pops, not when the click was registered). The frontend renders the
	// "built" marker at *BuiltSecond when supplied; otherwise it falls back to
	// start + BuildTimeSeconds.
	ActualBuiltSecond int64 `json:"actual_built_second,omitempty"`
	TargetBuiltSecond int64 `json:"target_built_second,omitempty"`
}

// workflowMutaliskTimingSummary is the per-game Mutalisk-Turret gap
// comparison rendered alongside the timeline. The "gap" is
// (turret_finished - mutalisk_finished) — i.e. how much later the first
// Missile Turret completes relative to the first Mutalisk hatching.
//
// Sweet spot: progamers aim to finish turrets just-in-time for muta arrival,
// so the median gap is small (a few seconds) — turrets land at roughly the
// same time mutas hatch, with mutas eating their travel time across the map.
// ActualGapSeconds < ExpertGapMinSeconds → turrets done too early (wasted
// economy). ActualGapSeconds > ExpertGapMaxSeconds → turrets late, Z mutas
// threaten the Terran main.
type workflowMutaliskTimingSummary struct {
	ExpertGapSeconds    int64 `json:"expert_gap_seconds"`     // median (sweet spot center)
	ExpertGapMinSeconds int64 `json:"expert_gap_min_seconds"` // p25 of corpus distribution
	ExpertGapMaxSeconds int64 `json:"expert_gap_max_seconds"` // p75 of corpus distribution
	ActualGapSeconds    int64 `json:"actual_gap_seconds"`     // turret_done - muta_done in this game
	HasActual           bool  `json:"has_actual"`             // false when one side missing
}

type workflowMapVisual struct {
	Available      bool    `json:"available"`
	URL            string  `json:"url,omitempty"`
	ThumbnailURL   string  `json:"thumbnail_url,omitempty"`
	MatchedImage   string  `json:"matched_image,omitempty"`
	MatchedScore   float64 `json:"matched_score,omitempty"`
	RequestedMap   string  `json:"requested_map,omitempty"`
	ResolutionNote string  `json:"resolution_note,omitempty"`
}

type workflowGameEvent struct {
	Type             string                   `json:"type"`
	Second           int64                    `json:"second"`
	Actor            *workflowGameEventPlayer `json:"actor,omitempty"`
	Target           *workflowGameEventPlayer `json:"target,omitempty"`
	Base             *workflowGameEventBase   `json:"base,omitempty"`
	ActorOrigin      *workflowGameEventPoint  `json:"actor_origin,omitempty"`
	ActorStartClock  *int64                   `json:"actor_start_clock,omitempty"`
	Ownership        []workflowGameOwnership  `json:"ownership,omitempty"`
	AttackUnitTypes  []string                 `json:"attack_unit_types,omitempty"`
	AttackCastCounts map[string]int64         `json:"attack_cast_counts,omitempty"`
	// Recall-specific overlay/description fields. Populated only when
	// event.Type == "recall"; the source-of-truth for these is the recall
	// event's payload JSON written by worldstate.emitRecallEvents.
	SourcePoint      *workflowGameEventPoint  `json:"source_point,omitempty"`
	TargetPoint      *workflowGameEventPoint  `json:"target_point,omitempty"`
	TargetBase       *workflowGameEventBase   `json:"target_base,omitempty"`
	TargetOwner      *workflowGameEventPlayer `json:"target_owner,omitempty"`
	RecallTargetVia  string                   `json:"recall_target_via,omitempty"`  // "a" | "p" | "t"
	RecallCount      int64                    `json:"recall_count,omitempty"`        // omitted when 1
	RecallLastSecond int64                    `json:"recall_last_second,omitempty"`  // omitted when equal to Second
}

type workflowGameEventPlayer struct {
	PlayerID int64  `json:"player_id"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
}

type workflowGameEventPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type workflowGameEventBase struct {
	Name           string                   `json:"name"`
	Kind           string                   `json:"kind,omitempty"`
	Clock          int64                    `json:"clock,omitempty"`
	NaturalOfClock *int64                   `json:"natural_of_clock,omitempty"`
	MineralOnly    *bool                    `json:"mineral_only,omitempty"`
	Center         workflowGameEventPoint   `json:"center"`
	Polygon        []workflowGameEventPoint `json:"polygon,omitempty"`
}

type workflowGameOwnership struct {
	Base  workflowGameEventBase    `json:"base"`
	Owner *workflowGameEventPlayer `json:"owner,omitempty"`
}

type workflowUnitSlice struct {
	SliceStartSecond int64                     `json:"slice_start_second"`
	SliceLabel       string                    `json:"slice_label"`
	Players          []workflowUnitSlicePlayer `json:"players"`
}

type workflowUnitSlicePlayer struct {
	PlayerID  int64               `json:"player_id"`
	PlayerKey string              `json:"player_key"`
	Name      string              `json:"name"`
	Units     []workflowUnitCount `json:"units"`
}

type workflowUnitCount struct {
	UnitType string `json:"unit_type"`
	Count    int64  `json:"count"`
}

// workflowUnitEarlyEventPlayer carries one player's individual unit/building
// production events for the first 4 minutes of the game. The frontend renders
// these as a vertical time-scaled chart (one icon per event with exact-second
// labels) so users can compare production efficiency between same-race builds.
type workflowUnitEarlyEventPlayer struct {
	PlayerID  int64                    `json:"player_id"`
	PlayerKey string                   `json:"player_key"`
	Name      string                   `json:"name"`
	Events    []workflowUnitEarlyEvent `json:"events"`
}

// workflowUnitEarlyEvent is a single Train/Morph command surfaced as an
// individual event (not aggregated into a slice count). Label is pre-formatted
// as "5th SCV"/"3rd Drone" for workers; empty for non-workers. Count is 2 for
// a Zergling Morph (one larva → two zerglings) and 1 for everything else, so
// the frontend can render an "x2" badge without re-implementing the rule.
type workflowUnitEarlyEvent struct {
	Second     int64  `json:"second"`
	UnitType   string `json:"unit_type"`
	IsBuilding bool   `json:"is_building"`
	Label      string `json:"label,omitempty"`
	Count      int64  `json:"count"`
}

type workflowReplayTimings struct {
	Gas       []workflowPlayerTimingSeries `json:"gas"`
	Expansion []workflowPlayerTimingSeries `json:"expansion"`
	Upgrades  []workflowPlayerTimingSeries `json:"upgrades"`
	Tech      []workflowPlayerTimingSeries `json:"tech"`
}

type workflowFirstUnitEfficiencyPlayer struct {
	PlayerID  int64                              `json:"player_id"`
	PlayerKey string                             `json:"player_key"`
	Name      string                             `json:"name"`
	Race      string                             `json:"race"`
	Entries   []workflowFirstUnitEfficiencyEntry `json:"entries"`
}

type workflowFirstUnitEfficiencyEntry struct {
	BuildingName         string `json:"building_name"`
	UnitName             string `json:"unit_name"`
	BuildingStartSecond  int64  `json:"building_start_second"`
	BuildingReadySecond  int64  `json:"building_ready_second"`
	UnitSecond           int64  `json:"unit_second"`
	BuildDurationSeconds int64  `json:"build_duration_seconds"`
	GapAfterReadySeconds int64  `json:"gap_after_ready_seconds"`
}

type workflowGameUnitCadencePlayer struct {
	PlayerID         int64   `json:"player_id"`
	PlayerKey        string  `json:"player_key"`
	PlayerName       string  `json:"player_name"`
	Team             int64   `json:"team"`
	IsWinner         bool    `json:"is_winner"`
	Eligible         bool    `json:"eligible"`
	WindowSeconds    int64   `json:"window_seconds"`
	UnitsProduced    int64   `json:"units_produced"`
	GapCount         int64   `json:"gap_count"`
	RatePerMinute    float64 `json:"rate_per_minute"`
	CVGap            float64 `json:"cv_gap"`
	Burstiness       float64 `json:"burstiness"`
	Idle20Ratio      float64 `json:"idle20_ratio"`
	CadenceScore     float64 `json:"cadence_score"`
	IneligibleReason string  `json:"ineligible_reason,omitempty"`
}

type workflowPlayerTimingSeries struct {
	PlayerID  int64                 `json:"player_id"`
	PlayerKey string                `json:"player_key"`
	Name      string                `json:"name"`
	Points    []workflowTimingPoint `json:"points"`
}

type workflowTimingPoint struct {
	Second int64  `json:"second"`
	Order  int64  `json:"order"`
	Label  string `json:"label,omitempty"`
}

type workflowPlayerRaceBreakdown struct {
	Race      string `json:"race"`
	GameCount int64  `json:"game_count"`
	Wins      int64  `json:"wins"`
}

// workflowPlayerMatchupCell is one (own_race, opp_race) cell of the matchup
// table. Confidence buckets the sample size: low (<5), medium (5–14),
// high (15+) — used by the UI to dim cells that don't have enough games
// to be informative.
type workflowPlayerMatchupCell struct {
	OwnRace    string  `json:"own_race"`
	OppRace    string  `json:"opp_race"`
	Games      int64   `json:"games"`
	Wins       int64   `json:"wins"`
	WinRate    float64 `json:"win_rate"`
	Confidence string  `json:"confidence"`
}

// workflowPlayerEarlyTiming is a per-(race, map_kind) summary of an early-game
// milestone. We surface median + sample size to compare a player's pacing on
// Regular vs Money maps without committing to a full histogram (the
// distributions are usually too sparse per-player to warrant one).
type workflowPlayerEarlyTiming struct {
	Race          string  `json:"race"`
	MapKind       string  `json:"map_kind"`
	Milestone     string  `json:"milestone"`
	Games         int64   `json:"games"`
	MedianSeconds float64 `json:"median_seconds"`
}

type workflowPlayerOverview struct {
	SummaryVersion      string                        `json:"summary_version"`
	PlayerKey           string                        `json:"player_key"`
	PlayerName          string                        `json:"player_name"`
	GamesPlayed         int64                         `json:"games_played"`
	Wins                int64                         `json:"wins"`
	WinRate             float64                       `json:"win_rate"`
	AverageAPM          float64                       `json:"average_apm"`
	AverageEAPM         float64                       `json:"average_eapm"`
	HotkeyUsageRate     float64                       `json:"hotkey_usage_rate"`
	CarrierCommandCount int64                         `json:"carrier_command_count"`
	RaceBreakdown       []workflowPlayerRaceBreakdown `json:"race_breakdown"`
	FingerprintMetrics  []workflowComparativeMetric   `json:"fingerprint_metrics"`
	RecentGames         []workflowGameListItem        `json:"recent_games"`
	ChatSummary         workflowPlayerChatSummary     `json:"chat_summary"`
	NarrativeHints []string                      `json:"narrative_hints"`
	Matchups       []workflowPlayerMatchupCell   `json:"matchups"`
	RaceOrders     []workflowRaceOrderSummary    `json:"race_orders"`
	MatchupOrders  []workflowMatchupOrderSummary `json:"matchup_orders"`
	EarlyTimings   []workflowPlayerEarlyTiming   `json:"early_timings"`
}

// workflowUnitCompositionUnit is one entry in the (player, phase)
// composition histogram. Counts are raw — the frontend renders by
// proportional fill across a fixed-size slot strip rather than as
// percentages.
type workflowUnitCompositionUnit struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// workflowGameUnitComposition is a per-(player, phase) row attached to
// the per-game endpoint response. Computed at request time from the
// persisted phase boundaries (mid_game_starts / late_game_starts
// markers) plus the Train / Unit Morph / Cast command stream — see
// internal/dashboard/unit_composition.go. Frontend renders per-player
// rows on individual player strips and aggregates client-side into
// three replay-level pills for the per-game summary surface.
//
// Casters covers two distinct populations merged into one strip:
//   - spell-caster units that actually cast a spell in this phase
//   - signature non-spellcaster units that the player BUILT
//     (Carrier/Reaver/BC/DT/Dropship/Nuke/Guardian/Devourer)
// Both are deduped: each unit appears at most once per phase.
type workflowGameUnitComposition struct {
	PlayerID int64                         `json:"player_id"`
	Phase    string                        `json:"phase"`
	Units    []workflowUnitCompositionUnit `json:"units"`
	Casters  []string                      `json:"casters,omitempty"`
}

type workflowPlayerChatSummary struct {
	TotalMessages   int64                   `json:"total_messages"`
	GamesWithChat   int64                   `json:"games_with_chat"`
	DistinctTerms   int64                   `json:"distinct_terms"`
	TopTerms        []workflowChatTermCount `json:"top_terms"`
	ExampleMessages []string                `json:"example_messages"`
}

type workflowChatTermCount struct {
	Term  string `json:"term"`
	Count int64  `json:"count"`
}

type workflowPlayerOutliers struct {
	SummaryVersion string                    `json:"summary_version"`
	PlayerKey      string                    `json:"player_key"`
	PlayerName     string                    `json:"player_name"`
	Thresholds     workflowOutlierThresholds `json:"thresholds"`
	Items          []workflowPlayerOutlier   `json:"items"`
}

type workflowOutlierThresholds struct {
	TFIDFMin float64 `json:"tfidf_min"`
	RatioMin float64 `json:"ratio_min"`
}

type workflowPlayerOutlier struct {
	Category        string   `json:"category"`
	Race            string   `json:"race"`
	Name            string   `json:"name"`
	PrettyName      string   `json:"pretty_name"`
	PlayerGames     int64    `json:"player_games"`
	PlayerRate      float64  `json:"player_rate"`
	BaselineRate    float64  `json:"baseline_rate"`
	RatioToBaseline float64  `json:"ratio_to_baseline"`
	TFIDF           float64  `json:"tfidf"`
	QualifiedBy     []string `json:"qualified_by"`
}

// workflowPlayerSummaryPerMatchup is the payload of
// GET /api/players/{playerKey}/summary/per-matchup. Rows are 1v1 matchups
// the player has data for; the UI lays them out as a card grid sorted by
// game count.
// workflowPlayerSummaryPerMatchup is the payload of GET /summary/per-matchup.
// Cards is a unified list of "1v1 matchup" cards and "team-format ×
// own-race" cards, sorted by games descending so the player's most-played
// context surfaces first (a Money-multi-team Random player sees their
// per-race team-format cards before any sparse 1v1 cards).
type workflowPlayerSummaryPerMatchup struct {
	SummaryVersion string                          `json:"summary_version"`
	PlayerKey      string                          `json:"player_key"`
	PlayerName     string                          `json:"player_name"`
	Cards          []workflowPlayerSummaryCard     `json:"cards"`
}

// workflowPlayerSummaryCard is one entry in the unified Summary grid. Kind
// disambiguates the two card families:
//   - "matchup": 1v1 with OwnRace + OppRace populated.
//   - "format":  team-format × map-kind × own-race with FormatClass +
//                MapKind + OwnRace populated; OppRace is "".
//
// Games/Wins/WinRate/Confidence/AvgAPM/AvgEAPM are uniform across both.
// TopBuildOrders/TopMarkers are race-scoped — a Random player gets
// distinct cards (and so distinct BO/marker top-Ns) for each race they
// play in a given format, fixing the "Zerg patterns dominate every card"
// bug for Random players.
type workflowPlayerSummaryCard struct {
	Kind           string                              `json:"kind"`
	Key            string                              `json:"key"`
	OwnRace        string                              `json:"own_race"`
	OppRace        string                              `json:"opp_race,omitempty"`
	FormatClass    string                              `json:"format_class,omitempty"`
	MapKind        string                              `json:"map_kind,omitempty"`
	Games          int64                               `json:"games"`
	Wins           int64                               `json:"wins"`
	WinRate        float64                             `json:"win_rate"`
	Confidence     string                              `json:"confidence"`
	AvgAPM         float64                             `json:"avg_apm"`
	AvgEAPM        float64                             `json:"avg_eapm"`
	TopBuildOrders []workflowPlayerMatchupPatternCount `json:"top_build_orders"`
	TopMarkers     []workflowPlayerMatchupPatternCount `json:"top_markers"`
}

type workflowPlayerMatchupPatternCount struct {
	PatternName string `json:"pattern_name"`
	Count       int64  `json:"count"`
}

// workflowPlayerSummaryOutliers is the payload of
// GET /api/players/{playerKey}/summary/outliers?category=<cat>. One
// category per request lets the FE fan out the slow per-spec corpus
// queries and render pills incrementally as each finishes.
type workflowPlayerSummaryOutliers struct {
	SummaryVersion string                              `json:"summary_version"`
	PlayerKey      string                              `json:"player_key"`
	Category       string                              `json:"category"`
	Pills          []workflowPlayerSummaryOutlierPill  `json:"pills"`
}

// workflowPlayerSummarySpecial is the payload of
// GET /api/players/{playerKey}/summary/special. It mirrors the
// "what's special about this player" pills row.
type workflowPlayerSummarySpecial struct {
	SummaryVersion       string                            `json:"summary_version"`
	PlayerKey            string                            `json:"player_key"`
	PlayerName           string                            `json:"player_name"`
	NeverAlliedMultiTeam workflowPlayerSpecialEligibleStat `json:"never_allied_multi_team"`
	NeverHotkeys         workflowPlayerSpecialEligibleStat `json:"never_hotkeys"`
	OutlierPills         []workflowPlayerSummaryOutlierPill `json:"outlier_pills"`
}

type workflowPlayerSpecialEligibleStat struct {
	Eligible bool  `json:"eligible"`
	Games    int64 `json:"games"`
}

// workflowPlayerSummaryOutlierPill is a single distinctive-outlier pill on
// the Summary tab. MapKind is "" when the pill was computed against the
// all-maps corpus; "Regular" or "Money" indicates a segment-specific pill.
// IconKey resolves to a unit/building icon via the standard
// /api/custom/game-assets/* path on the frontend (see getUnitIcon in
// gameAssets.js); empty when no icon is known for the outlier item.
// PrettyLabel is the user-facing label minus the segment suffix — the
// frontend appends a money-bag emoji for Money-segment pills instead of
// the verbose "· Money" qualifier.
type workflowPlayerSummaryOutlierPill struct {
	Category        string   `json:"category"`
	Name            string   `json:"name"`
	PrettyName      string   `json:"pretty_name"`
	PrettyLabel     string   `json:"pretty_label"`
	IconKey         string   `json:"icon_key"`
	Race            string   `json:"race"`
	MapKind         string   `json:"map_kind"`
	PlayerGames     int64    `json:"player_games"`
	PlayerRate      float64  `json:"player_rate"`
	BaselineRate    float64  `json:"baseline_rate"`
	RatioToBaseline float64  `json:"ratio_to_baseline"`
	TFIDF           float64  `json:"tfidf"`
	QualifiedBy     []string `json:"qualified_by"`
}

type workflowRareUsage struct {
	Name                string  `json:"name"`
	PrettyName          string  `json:"pretty_name"`
	PlayerCount         int64   `json:"player_count"`
	PlayerRatePerGame   float64 `json:"player_rate_per_game"`
	PopulationUsageRate float64 `json:"population_usage_rate"`
	RarityScore         float64 `json:"rarity_score"`
}

type workflowComparativeMetric struct {
	Metric      string  `json:"metric"`
	PlayerValue float64 `json:"player_value"`
}

type workflowPlayerInsightType string

const (
	workflowPlayerInsightTypeAPM                workflowPlayerInsightType = "apm"
	workflowPlayerInsightTypeFirstDelay         workflowPlayerInsightType = "first-unit-delay"
	workflowPlayerInsightTypeUnitCadence        workflowPlayerInsightType = "unit-production-cadence"
	workflowPlayerInsightTypeViewportSwitchRate workflowPlayerInsightType = "viewport-switch-rate"
)

type workflowPlayerInsightDetail struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type workflowPlayerAsyncInsight struct {
	SummaryVersion        string                        `json:"summary_version"`
	PlayerKey             string                        `json:"player_key"`
	PlayerName            string                        `json:"player_name"`
	InsightType           workflowPlayerInsightType     `json:"insight_type"`
	Title                 string                        `json:"title"`
	Eligible              bool                          `json:"eligible"`
	BetterDirection       string                        `json:"better_direction"`
	PopulationSize        int64                         `json:"population_size"`
	PerformancePercentile *float64                      `json:"performance_percentile,omitempty"`
	PlayerValue           *float64                      `json:"player_value,omitempty"`
	PlayerValueLabel      string                        `json:"player_value_label,omitempty"`
	Description           string                        `json:"description"`
	IneligibleReason      string                        `json:"ineligible_reason,omitempty"`
	Details               []workflowPlayerInsightDetail `json:"details"`
}

type workflowRaceOrderSummary struct {
	Race         string   `json:"race"`
	TechOrder    []string `json:"tech_order"`
	UpgradeOrder []string `json:"upgrade_order"`
}

// workflowMatchupOrderSummary is the most-common tech and upgrade sequence for
// a single (own_race, opp_race) matchup. Games is the sample size used to pick
// the top sequence; the UI dims rows with Games < 5 to flag low confidence.
type workflowMatchupOrderSummary struct {
	OwnRace      string   `json:"own_race"`
	OppRace      string   `json:"opp_race"`
	Games        int64    `json:"games"`
	TechOrder    []string `json:"tech_order"`
	UpgradeOrder []string `json:"upgrade_order"`
}

type workflowPlayersListFilters struct {
	NameContains      string
	OnlyFivePlus      bool
	LastPlayedBuckets []string
}

type workflowPlayersListSort struct {
	Column string
	Desc   bool
}

type workflowPlayersListItem struct {
	PlayerKey         string  `json:"player_key"`
	PlayerName        string  `json:"player_name"`
	Race              string  `json:"race"`
	GamesPlayed       int64   `json:"games_played"`
	AverageAPM        float64 `json:"average_apm"`
	LastPlayed        string  `json:"last_played"`
	LastPlayedDaysAgo int64   `json:"last_played_days_ago"`
}

type workflowPlayersListFilterOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int64  `json:"count"`
}

type workflowPlayersListFilterOptions struct {
	Races      []workflowPlayersListFilterOption `json:"races"`
	LastPlayed []workflowPlayersListFilterOption `json:"last_played"`
}

type workflowPlayerApmHistogramBin struct {
	X0    float64 `json:"x0"`
	X1    float64 `json:"x1"`
	Count int64   `json:"count"`
}

type workflowPlayerApmHistogramPoint struct {
	PlayerKey   string  `json:"player_key"`
	PlayerName  string  `json:"player_name"`
	AverageAPM  float64 `json:"average_apm"`
	GamesPlayed int64   `json:"games_played"`
}

type workflowPlayerApmHistogram struct {
	SummaryVersion   string                            `json:"summary_version"`
	PlayerKey        string                            `json:"player_key"`
	MinGames         int64                             `json:"min_games"`
	PlayersIncluded  int64                             `json:"players_included"`
	MeanAPM          float64                           `json:"mean_apm"`
	StddevAPM        float64                           `json:"stddev_apm"`
	PlayerAverageAPM *float64                          `json:"player_average_apm,omitempty"`
	PlayerEligible   bool                              `json:"player_eligible"`
	PlayerPercentile *float64                          `json:"player_percentile,omitempty"`
	Bins             []workflowPlayerApmHistogramBin   `json:"bins"`
	Players          []workflowPlayerApmHistogramPoint `json:"players"`
}

type workflowPlayerDelayHistogramBin struct {
	X0    float64 `json:"x0"`
	X1    float64 `json:"x1"`
	Count int64   `json:"count"`
}

type workflowPlayerDelayHistogramPoint struct {
	PlayerKey           string                           `json:"player_key"`
	PlayerName          string                           `json:"player_name"`
	AverageDelaySeconds float64                          `json:"average_delay_seconds"`
	SampleCount         int64                            `json:"sample_count"`
	CaseAverages        []workflowPlayerDelayCaseAverage `json:"case_averages"`
}

type workflowPlayerDelayCaseAverage struct {
	CaseKey             string  `json:"case_key"`
	BuildingName        string  `json:"building_name"`
	UnitName            string  `json:"unit_name"`
	AverageDelaySeconds float64 `json:"average_delay_seconds"`
	SampleCount         int64   `json:"sample_count"`
}

type workflowPlayerDelayCaseOption struct {
	CaseKey      string `json:"case_key"`
	BuildingName string `json:"building_name"`
	UnitName     string `json:"unit_name"`
	SampleCount  int64  `json:"sample_count"`
	PlayerCount  int64  `json:"player_count"`
}

type workflowPlayerDelayHistogram struct {
	SummaryVersion     string                              `json:"summary_version"`
	MinSamples         int64                               `json:"min_samples"`
	PlayersIncluded    int64                               `json:"players_included"`
	MeanDelaySeconds   float64                             `json:"mean_delay_seconds"`
	StddevDelaySeconds float64                             `json:"stddev_delay_seconds"`
	Bins               []workflowPlayerDelayHistogramBin   `json:"bins"`
	Players            []workflowPlayerDelayHistogramPoint `json:"players"`
	CaseOptions        []workflowPlayerDelayCaseOption     `json:"case_options"`
}

type workflowPlayerDelayPair struct {
	BuildingName        string  `json:"building_name"`
	UnitName            string  `json:"unit_name"`
	SampleCount         int64   `json:"sample_count"`
	AverageDelaySeconds float64 `json:"average_delay_seconds"`
	MinDelaySeconds     int64   `json:"min_delay_seconds"`
	MaxDelaySeconds     int64   `json:"max_delay_seconds"`
}

type workflowPlayerDelayInsight struct {
	SummaryVersion      string                    `json:"summary_version"`
	PlayerKey           string                    `json:"player_key"`
	PlayerName          string                    `json:"player_name"`
	SampleCount         int64                     `json:"sample_count"`
	AverageDelaySeconds float64                   `json:"average_delay_seconds"`
	MinDelaySeconds     int64                     `json:"min_delay_seconds"`
	MaxDelaySeconds     int64                     `json:"max_delay_seconds"`
	Pairs               []workflowPlayerDelayPair `json:"pairs"`
}

type workflowPlayerUnitCadencePoint struct {
	PlayerKey         string  `json:"player_key"`
	PlayerName        string  `json:"player_name"`
	GamesUsed         int64   `json:"games_used"`
	AverageRatePerMin float64 `json:"average_rate_per_min"`
	AverageCVGap      float64 `json:"average_cv_gap"`
	AverageBurstiness float64 `json:"average_burstiness"`
	AverageIdle20     float64 `json:"average_idle20_ratio"`
	AverageCadence    float64 `json:"average_cadence_score"`
}

type workflowPlayerUnitCadenceHistogramBin struct {
	X0    float64 `json:"x0"`
	X1    float64 `json:"x1"`
	Count int64   `json:"count"`
}

type workflowPlayerUnitCadenceLeaderboard struct {
	SummaryVersion    string                                  `json:"summary_version"`
	FilterMode        workflowUnitCadenceFilterMode           `json:"filter_mode"`
	StartSecond       int64                                   `json:"start_second"`
	EndFraction       float64                                 `json:"end_fraction"`
	IdleGapSeconds    int64                                   `json:"idle_gap_seconds"`
	MinUnitsPerReplay int64                                   `json:"min_units_per_replay"`
	MinGapsPerReplay  int64                                   `json:"min_gaps_per_replay"`
	MinGames          int64                                   `json:"min_games"`
	PlayersIncluded   int64                                   `json:"players_included"`
	MeanCadence       float64                                 `json:"mean_cadence_score"`
	StddevCadence     float64                                 `json:"stddev_cadence_score"`
	Bins              []workflowPlayerUnitCadenceHistogramBin `json:"bins"`
	Players           []workflowPlayerUnitCadencePoint        `json:"players"`
}

type workflowPlayerUnitCadenceReplay struct {
	ReplayID        int64   `json:"replay_id"`
	FileName        string  `json:"file_name"`
	DurationSeconds int64   `json:"duration_seconds"`
	WindowSeconds   int64   `json:"window_seconds"`
	UnitsProduced   int64   `json:"units_produced"`
	GapCount        int64   `json:"gap_count"`
	RatePerMinute   float64 `json:"rate_per_minute"`
	CVGap           float64 `json:"cv_gap"`
	Burstiness      float64 `json:"burstiness"`
	Idle20Ratio     float64 `json:"idle20_ratio"`
	CadenceScore    float64 `json:"cadence_score"`
}

type workflowPlayerUnitCadenceInsight struct {
	SummaryVersion      string                            `json:"summary_version"`
	PlayerKey           string                            `json:"player_key"`
	PlayerName          string                            `json:"player_name"`
	FilterMode          workflowUnitCadenceFilterMode     `json:"filter_mode"`
	StartSecond         int64                             `json:"start_second"`
	EndFraction         float64                           `json:"end_fraction"`
	IdleGapSeconds      int64                             `json:"idle_gap_seconds"`
	MinUnitsPerReplay   int64                             `json:"min_units_per_replay"`
	MinGapsPerReplay    int64                             `json:"min_gaps_per_replay"`
	GamesUsed           int64                             `json:"games_used"`
	AverageRatePerMin   float64                           `json:"average_rate_per_min"`
	AverageCVGap        float64                           `json:"average_cv_gap"`
	AverageBurstiness   float64                           `json:"average_burstiness"`
	AverageIdle20       float64                           `json:"average_idle20_ratio"`
	AverageCadenceScore float64                           `json:"average_cadence_score"`
	Replays             []workflowPlayerUnitCadenceReplay `json:"replays"`
}

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

// Deliberately limited to bread-and-butter army buildings whose first unit a
// player almost always wants out the moment it completes. Tech/utility
// buildings (Forge, Fleet Beacon, Hydralisk Den, …) are excluded: they are
// routinely built for upgrades, so the gap-after-ready signal there is mostly
// false positives (issue #166).
var firstUnitEfficiencyConfigs = []firstUnitEfficiencyConfig{
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
		Race:                 "zerg",
		BuildingName:         "Spawning Pool",
		BuildDurationSeconds: 50,
		Units:                []firstUnitEfficiencyUnitOption{{DisplayName: "Zergling", MatchKeys: []string{"Zergling"}}},
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
}

type workflowGameListItem struct {
	ReplayID           int64                     `json:"replay_id"`
	ReplayDate         string                    `json:"replay_date"`
	FileName           string                    `json:"file_name"`
	MapName            string                    `json:"map_name"`
	MapKind            string                    `json:"map_kind,omitempty"`
	GameSource         string                    `json:"game_source"`
	LobbyKind          string                    `json:"lobby_kind"`
	DurationSeconds    int64                     `json:"duration_seconds"`
	GameType           string                    `json:"game_type"`
	TeamFormat         string                    `json:"team_format,omitempty"`
	PlayersLabel       string                    `json:"players_label"`
	WinnersLabel       string                    `json:"winners_label"`
	Matchup            string                    `json:"matchup"`
	TeamStacking       bool                      `json:"team_stacking"`
	TeamInfoIncomplete bool                      `json:"team_info_incomplete"`
	Players            []workflowGameListPlayer  `json:"players"`
	Featuring          []string                  `json:"featuring"`
	FeaturingKeys      []string                  `json:"featuring_keys,omitempty"`
	CurrentPlayer      *workflowRecentGamePlayer `json:"current_player,omitempty"`
}

type workflowGameListPlayer struct {
	PlayerID     int64          `json:"player_id"`
	PlayerKey    string         `json:"player_key"`
	Name         string         `json:"name"`
	Race         string         `json:"race"`
	Team         int64          `json:"team"`
	IsWinner     bool           `json:"is_winner"`
	CountryCode  string         `json:"country_code,omitempty"`
	PrimaryBadge *IdentityBadge `json:"primary_badge,omitempty"`
}

type workflowRecentGamePlayer struct {
	PlayerID         int64                         `json:"player_id"`
	PlayerKey        string                        `json:"player_key"`
	Name             string                        `json:"name"`
	Race             string                        `json:"race"`
	IsWinner         bool                          `json:"is_winner"`
	Disconnected     bool                          `json:"disconnected,omitempty"`
	APM              int64                         `json:"apm"`
	EAPM             int64                         `json:"eapm"`
	DetectedPatterns []workflowPatternValue        `json:"detected_patterns"`
	Composition      []workflowGameUnitComposition `json:"composition,omitempty"`
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
	Race      string   `json:"race,omitempty"`
}

type workflowGamesListFilterOptions struct {
	Players   []workflowGamesListFilterOption `json:"players"`
	Maps      []workflowGamesListFilterOption `json:"maps"`
	Durations []workflowGamesListFilterOption `json:"durations"`
	Featuring []workflowGamesListFilterOption `json:"featuring"`
	Matchups  []workflowGamesListFilterOption `json:"matchups"`
	MapKinds  []workflowGamesListFilterOption `json:"map_kinds"`
}

// TvZ==ZvT and PvZ==ZvP, so the key is always the alphabetically-sorted form.
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

// The chips on the games-list filter bar. Group splits the row visually:
// "marker" is always visible, "bo" collapses under a disclosure by default.
// IconKeys (multi-icon) wins over IconKey when both are present; IconLabel adds
// short text next to the icon(s) for disambiguation.
var workflowFeaturingFilters = []struct {
	Key       string
	Label     string
	Group     string
	IconKey   string
	IconKeys  []string
	IconLabel string
	Emoji     string
	// Race groups the "bo" chips under per-race disclosures. Empty for "marker".
	Race string
}{
	// The former Terran style markers (mech / sk_terran / one_one_one /
	// mech_transition) are now first-class composition BOs in the "bo" group
	// below (issue #155).
	{Key: "cannon_rush", Label: "Cannon Rush", Group: "marker", IconKey: "photoncannon", IconLabel: "Rush"},
	{Key: "bunker_rush", Label: "Bunker Rush", Group: "marker", IconKey: "bunker", IconLabel: "Rush"},
	{Key: "zergling_rush", Label: "Zergling Rush", Group: "marker", IconKey: "zergling", IconLabel: "Rush"},
	{Key: "proxy_gate", Label: "Proxy Gateway", Group: "marker", IconKey: "gateway", IconLabel: "Proxy"},
	{Key: "proxy_rax", Label: "Proxy Barracks", Group: "marker", IconKey: "barracks", IconLabel: "Proxy"},
	{Key: "proxy_factory", Label: "Proxy Factory", Group: "marker", IconKey: "factory", IconLabel: "Proxy"},
	{Key: "proxy_starport", Label: "Proxy Starport", Group: "marker", IconKey: "starport", IconLabel: "Proxy"},
	{Key: "manner_pylon", Label: "Manner Pylon", Group: "marker", IconKey: "pylon", IconLabel: "Manner"},
	// "drop" matches ANY drop variant; the subtype chips match only their own.
	{Key: "drop", Label: "Drop", Group: "marker", IconKey: "shuttle"},
	{Key: "mind_control", Label: "Mind Control", Group: "marker", IconKey: "darkarchon", IconLabel: "Mind Control"},
	{Key: "made_maelstrom", Label: "Maelstrom", Group: "marker", IconKey: "darkarchon", IconLabel: "Maelstrom"},
	{Key: "crazy_zerg", Label: "Crazy Zerg", Group: "marker", IconKey: "ultralisk", IconLabel: "Crazy Zerg"},
	{Key: "nukes", Label: "Nukes", Group: "marker", IconKey: "ghost", IconLabel: "Nuke"},
	{Key: "recalls", Label: "Recalls", Group: "marker", IconKey: "arbiter", IconLabel: "Recall"},
	{Key: "offensive_nydus", Label: "Offensive Nydus", Group: "marker", IconKey: "nyduscanal", IconLabel: "Nydus"},
	{Key: "team_stacking", Label: "Team stacking", Group: "marker", Emoji: "😈"},
	// Money-map markers, last so regular markers take priority.
	{Key: "carriers", Label: "Carrier", Group: "marker", IconKey: "carrier"},
	{Key: "battlecruisers", Label: "Battlecruiser", Group: "marker", IconKey: "battlecruiser"},
	{Key: "double_stargate", Label: "Double Stargate", Group: "marker", IconKey: "corsair", IconLabel: "2 Stargate"},
	{Key: "wraiths", Label: "Wraith", Group: "marker", IconKey: "wraith"},
	{Key: "ten_plus_scouts", Label: "10+ Scouts", Group: "marker", IconKey: "scout", IconLabel: "10+"},
	{Key: "cliff_drop", Label: "Cliff drop", Group: "marker", IconKey: "dropship", IconLabel: "Cliff drop"},
	{Key: "muta_hitnrun", Label: "Muta hit-n-run", Group: "marker", IconKey: "mutalisk", IconLabel: "Muta hit-n-run"},
	// Keys and labels are kept in sync with internal/markers. Suppressed in render
	// for Money maps on the featuring strips; the BO tab and per-player summary
	// pills still show them.
	{Key: "bo_4_pool", Label: "4 Pool", Group: "bo", Race: "zerg"},
	{Key: "bo_9_pool", Label: "9 Pool", Group: "bo", Race: "zerg"},
	{Key: "bo_9_overpool", Label: "9 Overpool", Group: "bo", Race: "zerg"},
	{Key: "bo_12_pool", Label: "12 Pool", Group: "bo", Race: "zerg"},
	{Key: "bo_9_pool_hatch", Label: "9 Pool into Hatchery", Group: "bo", Race: "zerg"},
	{Key: "bo_9_hatch", Label: "9 Hatch", Group: "bo", Race: "zerg"},
	{Key: "bo_10_hatch", Label: "10 Hatch", Group: "bo", Race: "zerg"},
	{Key: "bo_11_hatch", Label: "11 Hatch", Group: "bo", Race: "zerg"},
	{Key: "bo_12_hatch", Label: "12 Hatch", Group: "bo", Race: "zerg"},
	{Key: "bo_13_hatch", Label: "13 Hatch", Group: "bo", Race: "zerg"},
	{Key: "bo_z_fuzzy", Label: "Approx. opener", Group: "bo", Race: "zerg"},
	{Key: "nhatch_hydra", Label: "N Hatch Hydra", Group: "marker", IconKey: "hydralisk", IconLabel: "N Hatch Hydra"},
	{Key: "nhatch_muta", Label: "N Hatch Muta", Group: "marker", IconKey: "mutalisk", IconLabel: "N Hatch Muta"},
	{Key: "nhatch_lurker", Label: "N Hatch Lurker", Group: "marker", IconKey: "lurker", IconLabel: "N Hatch Lurker"},
	{Key: "bo_1_gate_core", Label: "1 Gate Core", Group: "bo", Race: "protoss"},
	{Key: "bo_2_gate", Label: "2 Gate", Group: "bo", Race: "protoss"},
	{Key: "bo_nexus_first", Label: "Nexus First", Group: "bo", Race: "protoss"},
	{Key: "bo_gate_expand", Label: "Gate Expand", Group: "bo", Race: "protoss"},
	{Key: "bo_forge_expa", Label: "Forge Expand", Group: "bo", Race: "protoss"},
	{Key: "bo_p_1gate_reaver", Label: "1 Gate Reaver", Group: "bo", Race: "protoss"},
	{Key: "bo_p_gate_forge_cannon", Label: "Gate Forge Cannon (before expa)", Group: "bo", Race: "protoss"},
	{Key: "bo_p_forge_cannon_gate", Label: "Forge Cannon Gate (before expa)", Group: "bo", Race: "protoss"},
	{Key: "bo_p_forge_gate_cannon", Label: "Forge Gate Cannon (before expa)", Group: "bo", Race: "protoss"},
	{Key: "bo_t_bio_1base", Label: "1-Base Bio", Group: "bo", Race: "terran"},
	{Key: "bo_t_bio_2base", Label: "2-Base Bio", Group: "bo", Race: "terran"},
	{Key: "bo_t_111_mech", Label: "1-1-1 Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_111_tankless", Label: "1-1-1 Tankless Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_111", Label: "1-1-1", Group: "bo", Race: "terran"},
	{Key: "bo_t_mech_expa_1fac", Label: "1 Fact Expa Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_mech_expa_2fac", Label: "2 Fact Expa Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_mech_expa_3fac", Label: "3 Fact Expa Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_mech_expa_4fac", Label: "4 Fact Expa Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_mech_expa_5fac", Label: "5 Fact Expa Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_mech_expa_6fac", Label: "6+ Fact Expa Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_tankless_expa_1fac", Label: "1 Fact Expa Tankless Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_tankless_expa_2fac", Label: "2 Fact Expa Tankless Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_tankless_expa_3fac", Label: "3 Fact Expa Tankless Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_tankless_expa_4fac", Label: "4 Fact Expa Tankless Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_tankless_expa_5fac", Label: "5 Fact Expa Tankless Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_tankless_expa_6fac", Label: "6+ Fact Expa Tankless Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_goliath_expa_1fac", Label: "1 Fact Expa Goliath", Group: "bo", Race: "terran"},
	{Key: "bo_t_goliath_expa_2fac", Label: "2 Fact Expa Goliath", Group: "bo", Race: "terran"},
	{Key: "bo_t_goliath_expa_3fac", Label: "3 Fact Expa Goliath", Group: "bo", Race: "terran"},
	{Key: "bo_t_goliath_expa_4fac", Label: "4 Fact Expa Goliath", Group: "bo", Race: "terran"},
	{Key: "bo_t_goliath_expa_5fac", Label: "5 Fact Expa Goliath", Group: "bo", Race: "terran"},
	{Key: "bo_t_goliath_expa_6fac", Label: "6+ Fact Expa Goliath", Group: "bo", Race: "terran"},
	{Key: "bo_t_mech_expand", Label: "Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_goliath_expand", Label: "Goliath", Group: "bo", Race: "terran"},
	{Key: "bo_t_tankless_expand", Label: "Tankless Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_mech_noexpa", Label: "1-Base Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_goliath_noexpa", Label: "1-Base Goliath", Group: "bo", Race: "terran"},
	{Key: "bo_t_tankless_noexpa", Label: "1-Base Tankless Mech", Group: "bo", Race: "terran"},
	{Key: "bo_t_2starport_wraith", Label: "2 Starport Wraith", Group: "bo", Race: "terran"},
	{Key: "bo_t_2starport_valk", Label: "2 Starport Valkyrie", Group: "bo", Race: "terran"},
	{Key: "bo_t_3starport_wraith", Label: "3 Starport Wraith", Group: "bo", Race: "terran"},
	{Key: "bo_t_3starport_valk", Label: "3 Starport Valkyrie", Group: "bo", Race: "terran"},
	{Key: "bo_cc_first", Label: "CC First", Group: "bo", Race: "terran"},
	{Key: "bo_bbs", Label: "BBS", Group: "bo", Race: "terran"},
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
	PlayerID         int64                  `json:"player_id"`
	PlayerKey        string                 `json:"player_key"`
	Name             string                 `json:"name"`
	Color            string                 `json:"color,omitempty"`
	Race             string                 `json:"race"`
	Team             int64                  `json:"team"`
	IsWinner         bool                   `json:"is_winner"`
	APM              int64                  `json:"apm"`
	EAPM             int64                  `json:"eapm"`
	DetectedPatterns []workflowPatternValue `json:"detected_patterns"`
	// LeftSecond is the earliest second the player became inactive: an explicit
	// Leave Game command, or the inactivity-derived player_stopped_playing event.
	// Nil when the player played to the end.
	LeftSecond *int64 `json:"left_second,omitempty"`
	// LeaveReason mirrors the LeaveGameCmd reason when LeftSecond comes from a
	// leave_game event, or "Stopped" when it comes from player_stopped_playing.
	LeaveReason      string                    `json:"leave_reason,omitempty"`
	FingerprintMatch *workflowFingerprintMatch `json:"fingerprint_match,omitempty"`
	CountryCode      string                    `json:"country_code,omitempty"`
	PrimaryBadge     *IdentityBadge            `json:"primary_badge,omitempty"`
	SecondaryBadge   *IdentityBadge            `json:"secondary_badge,omitempty"`
}

// workflowPatternValue is one detected_patterns[] entry on the game-summary
// response. EventType is the marker FeatureKey and is the stable identifier;
// the FE uses DetectedSecond to render "{minute}" interpolations on pill labels.
type workflowPatternValue struct {
	EventType      string          `json:"event_type"`
	DetectedSecond int             `json:"detected_second"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

type workflowGameDetail struct {
	SummaryVersion                   string                                   `json:"summary_version"`
	ReplayID                         int64                                    `json:"replay_id"`
	ReplayDate                       string                                   `json:"replay_date"`
	FileName                         string                                   `json:"file_name"`
	FilePath                         string                                   `json:"file_path"`
	OwnCopyFileName                  string                                   `json:"own_copy_file_name,omitempty"`
	MapName                          string                                   `json:"map_name"`
	MapKind                          string                                   `json:"map_kind,omitempty"`
	GameSource                       string                                   `json:"game_source"`
	LobbyKind                        string                                   `json:"lobby_kind"`
	MapVisual                        workflowMapVisual                        `json:"map_visual"`
	MapWidthPixels                   int64                                    `json:"map_width_pixels,omitempty"`
	MapHeightPixels                  int64                                    `json:"map_height_pixels,omitempty"`
	DurationSeconds                  int64                                    `json:"duration_seconds"`
	GameType                         string                                   `json:"game_type"`
	TeamStacking                     bool                                     `json:"team_stacking"`
	TeamInfoIncomplete               bool                                     `json:"team_info_incomplete"`
	Players                          []workflowGamePlayer                     `json:"players"`
	ReplayPatterns                   []workflowPatternValue                   `json:"replay_patterns"`
	GameEvents                       []workflowGameEvent                      `json:"game_events"`
	UnitsBySlice                     []workflowUnitSlice                      `json:"units_by_slice"`
	UnitsEarlyEvents                 []workflowUnitEarlyEventPlayer           `json:"units_early_events"`
	ProductionTimeline               []workflowProductionTimelinePlayer       `json:"production_timeline"`
	Timings                          workflowReplayTimings                    `json:"timings"`
	FirstUnitEfficiency              []workflowFirstUnitEfficiencyPlayer      `json:"first_unit_efficiency"`
	UnitCadence                      []workflowGameUnitCadencePlayer          `json:"unit_production_cadence"`
	ViewportMultitasking             []workflowGameViewportMultitaskingPlayer `json:"viewport_multitasking"`
	Markers                          []workflowMarkerPlayer                   `json:"build_orders"`
	MutaliskTiming                   []workflowMarkerPlayer                   `json:"mutalisk_timing_chart,omitempty"`
	MutaliskTimingSummary            *workflowMutaliskTimingSummary           `json:"mutalisk_timing_summary,omitempty"`
	AllianceTimeline                 []workflowAllianceSnapshot               `json:"alliance_timeline,omitempty"`
	AllianceStackingThresholdSeconds int64                                    `json:"alliance_stacking_threshold_seconds,omitempty"`
	// AllianceTabChat is the full per-replay chat stream, surfaced only for the
	// Alliances tab's context panel. Empty for non-melee or ≤2-player games.
	AllianceTabChat []workflowAllianceChat `json:"alliance_tab_chat,omitempty"`

	// These split the game-events list into Early/Mid/Late sections (see
	// populatePhaseMarkersForGameDetail). Zero means no boundary was detected, and
	// the frontend collapses the empty section header.
	EarlyGameEndsAtSecond int64 `json:"early_game_ends_at_second,omitempty"`
	MidGameEndsAtSecond   int64 `json:"mid_game_ends_at_second,omitempty"`

	// A flat list of (player, phase) rows, sourced from replay_events with
	// event_type LIKE 'unit_composition_%'. The frontend aggregates them into three
	// replay-level pills at display time and renders per-player rows on the strips.
	UnitCompositionMarkers []workflowGameUnitComposition `json:"unit_composition_markers,omitempty"`

	// A flat per-player stream of "this unit became alive at this second" samples,
	// used by the event-map overlay. Workers and Overlord are filtered out at build
	// time; Second is the command second shifted forward by the unit's build
	// duration (Fastest speed). The frontend pre-indexes and binary-searches it.
	TrainedUnitsTimeline []workflowTrainedUnitSample `json:"trained_units_timeline,omitempty"`
}

type workflowTrainedUnitSample struct {
	PlayerID int64  `json:"player_id"`
	Second   int64  `json:"second"`
	UnitType string `json:"unit_type"`
}

// workflowAllianceSnapshot is valid from Sec until the next snapshot's Sec (or
// duration_seconds for the last). Teams hold player_ids — the DB row id, not
// the screp player_id.
type workflowAllianceSnapshot struct {
	Sec      int64     `json:"sec"`
	Teams    [][]int64 `json:"teams"`
	Stacking bool      `json:"stacking"`
}

// Player_id matches players[].player_id (the DB row id). Message is raw text;
// the frontend handles escaping.
type workflowAllianceChat struct {
	Second   int64  `json:"second"`
	PlayerID int64  `json:"player_id"`
	Message  string `json:"message"`
}

// Populated by populateMarkersForGameDetail in endpoint_main_game_detail.go.
type workflowMarkerPlayer struct {
	PlayerID   int64                 `json:"player_id"`
	PlayerKey  string                `json:"player_key"`
	Name       string                `json:"name"`
	Race       string                `json:"race"`
	Marker     string                `json:"build_order"` // e.g. "9 pool"
	FeatureKey string                `json:"feature_key"` // e.g. "bo_9_pool"
	Events     []workflowMarkerEvent `json:"events"`
	Modifiers  []string              `json:"modifiers,omitempty"` // e.g. ["all-in","proxy"]
}

// NoExpert=true rows come from the player's command stream rather than the
// marker definition's Expert template, so they render without the golden band.
type workflowMarkerEvent struct {
	Key                   string `json:"key"`     // e.g. "Spawning Pool"
	Subject               string `json:"subject"` // canonical unit/building name for icon lookup (e.g. "Zergling")
	TargetSecond          int64  `json:"target_second"`
	ToleranceEarlySeconds int64  `json:"tolerance_early_seconds"`
	ToleranceLateSeconds  int64  `json:"tolerance_late_seconds"`
	ActualSecond          int64  `json:"actual_second"` // valid only when Found
	Found                 bool   `json:"found"`
	DeltaSeconds          int64  `json:"delta_seconds"` // actual - target; + late, - early
	WithinTolerance       bool   `json:"within_tolerance"`
	NoExpert              bool   `json:"no_expert,omitempty"` // true = no golden range; render actual only
	// When >0 the chart renders a span from the start tick to start+BuildTime,
	// making it visually obvious that the tick is "build started" while the
	// unit is only available at the end. 0 renders without a span bar.
	BuildTimeSeconds int64 `json:"build_time_seconds,omitempty"`
	// These override the naive "Actual/Target + BuildTime" finish calculation, for
	// markers whose subject has prerequisites: a Mutalisk morph queued before its
	// Spire finishes effectively starts when the Spire pops, not when the click
	// registered. The frontend falls back to start + BuildTimeSeconds when unset.
	ActualBuiltSecond int64 `json:"actual_built_second,omitempty"`
	TargetBuiltSecond int64 `json:"target_built_second,omitempty"`
}

// The gap is (turret_finished - mutalisk_finished): how much later the first
// Missile Turret completes than the first Mutalisk hatches. Progamers aim to
// finish turrets just-in-time for muta arrival, so the median gap is a few
// seconds. Below ExpertGapMin the turrets were built too early (wasted
// economy); above ExpertGapMax they are late and the mutas threaten the main.
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
	TargetStartClock *int64                   `json:"target_start_clock,omitempty"`
	Ownership        []workflowGameOwnership  `json:"ownership,omitempty"`
	AttackUnitTypes  []string                 `json:"attack_unit_types,omitempty"`
	AttackCastCounts map[string]int64         `json:"attack_cast_counts,omitempty"`
	// Recall-only fields; the source of truth is the recall event's payload JSON
	// written by worldstate.emitRecallEvents.
	SourcePoint      *workflowGameEventPoint  `json:"source_point,omitempty"`
	TargetPoint      *workflowGameEventPoint  `json:"target_point,omitempty"`
	TargetBase       *workflowGameEventBase   `json:"target_base,omitempty"`
	TargetOwner      *workflowGameEventPlayer `json:"target_owner,omitempty"`
	RecallTargetVia  string                   `json:"recall_target_via,omitempty"`  // "a" | "p" | "t"
	RecallCount      int64                    `json:"recall_count,omitempty"`       // omitted when 1
	RecallLastSecond int64                    `json:"recall_last_second,omitempty"` // omitted when equal to Second
	// SourceBase is drops-only: drops store the destination polygon in event.base,
	// so the source comes from the payload's `sb` field. Recalls keep their source
	// on event.base.
	SourceBase *workflowGameEventBase `json:"source_base,omitempty"`
	// Drop-only fields; the source of truth is the drop event's payload JSON
	// written by worldstate.emitDropEvents.
	DropTargetVia  string `json:"drop_target_via,omitempty"`  // "a" | "p"
	DropCount      int64  `json:"drop_count,omitempty"`       // omitted when 1
	DropLastSecond int64  `json:"drop_last_second,omitempty"` // omitted when equal to Second
	// late_alliance events only. Each entry is one team grouping at the moment the
	// topology changed; solos are filtered for clarity.
	AllianceTeams [][]workflowGameEventPlayer `json:"alliance_teams,omitempty"`
	// The consolidated "bo_openers" event at second 0 only — one entry per
	// (player × detected opener BO). The FE groups by player to render one line
	// each and label their starting location. BO timing is deliberately dropped.
	BuildOrders []workflowGameEventBuildOrder `json:"build_orders,omitempty"`
}

type workflowGameEventBuildOrder struct {
	PlayerID      int64    `json:"player_id"`
	Name          string   `json:"name"`
	Color         string   `json:"color,omitempty"`
	Race          string   `json:"race,omitempty"`
	IsWinner      bool     `json:"is_winner,omitempty"`
	Team          int64    `json:"team"`
	StartLocation string   `json:"start_location,omitempty"`
	BuildOrder    string   `json:"build_order"`
	FeatureKey    string   `json:"feature_key"`
	Modifiers     []string `json:"modifiers,omitempty"`
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

// One player's individual production events for the first 4 minutes, rendered
// as a vertical time-scaled chart so users can compare production efficiency
// between same-race builds.
type workflowUnitEarlyEventPlayer struct {
	PlayerID  int64                    `json:"player_id"`
	PlayerKey string                   `json:"player_key"`
	Name      string                   `json:"name"`
	Events    []workflowUnitEarlyEvent `json:"events"`
}

// One Train/Morph command as an individual event, not aggregated into a slice
// count. Label is pre-formatted ("5th SCV") for workers, empty otherwise. Count
// is 2 for a Zergling Morph (one larva → two zerglings) so the frontend can
// render an "x2" badge without re-implementing the rule.
type workflowUnitEarlyEvent struct {
	Second     int64  `json:"second"`
	UnitType   string `json:"unit_type"`
	IsBuilding bool   `json:"is_building"`
	Label      string `json:"label,omitempty"`
	Count      int64  `json:"count"`
}

// One player's full-game production stream. Unlike units_by_slice, which
// buckets and discards per-event timing after 4 minutes, this keeps every
// event's exact second so the frontend can scrub army construction over time.
// Same row set, no extra query.
type workflowProductionTimelinePlayer struct {
	PlayerID  int64                     `json:"player_id"`
	PlayerKey string                    `json:"player_key"`
	Name      string                    `json:"name"`
	Events    []workflowProductionEvent `json:"events"`
}

type workflowProductionEvent struct {
	Second     int64  `json:"second"`
	UnitType   string `json:"unit_type"`
	IsBuilding bool   `json:"is_building"`
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
	PlayerID             int64    `json:"player_id"`
	PlayerKey            string   `json:"player_key"`
	PlayerName           string   `json:"player_name"`
	Team                 int64    `json:"team"`
	IsWinner             bool     `json:"is_winner"`
	Eligible             bool     `json:"eligible"`
	WindowSeconds        int64    `json:"window_seconds"`
	UnitsProduced        int64    `json:"units_produced"`
	GapCount             int64    `json:"gap_count"`
	RatePerMinute        float64  `json:"rate_per_minute"`
	CVGap                float64  `json:"cv_gap"`
	Burstiness           float64  `json:"burstiness"`
	Idle20Ratio          float64  `json:"idle20_ratio"`
	CadenceScore         float64  `json:"cadence_score"`
	IneligibleReason     string   `json:"ineligible_reason,omitempty"`
	IneligibleReasonKey  string   `json:"ineligible_reason_key,omitempty"`
	IneligibleReasonArgs []string `json:"ineligible_reason_args,omitempty"`
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

// Confidence buckets the sample size — low (<5), medium (5–14), high (15+) —
// so the UI can dim cells with too few games to be informative.
type workflowPlayerMatchupCell struct {
	OwnRace    string  `json:"own_race"`
	OppRace    string  `json:"opp_race"`
	Games      int64   `json:"games"`
	Wins       int64   `json:"wins"`
	WinRate    float64 `json:"win_rate"`
	Confidence string  `json:"confidence"`
}

// Median + sample size, rather than a full histogram: per-player distributions
// are usually too sparse to warrant one.
type workflowPlayerEarlyTiming struct {
	Race          string  `json:"race"`
	MapKind       string  `json:"map_kind"`
	Milestone     string  `json:"milestone"`
	Games         int64   `json:"games"`
	MedianSeconds float64 `json:"median_seconds"`
}

// Reports how many of a player's games contributed scfingerprint vectors under
// the current feature version, which is what explains why identification is or
// isn't available: short games and low-command players yield no vector.
type workflowFingerprintCoverage struct {
	GamesWithVectors int64 `json:"games_with_vectors"`
	FeatureVersion   int   `json:"feature_version"`
}

type workflowFingerprintMatch struct {
	Label             string                   `json:"label"`
	Liquipedia        string                   `json:"liquipedia,omitempty"`
	Z                 float64                  `json:"z"`
	EvidenceN         int                      `json:"evidence_n"`
	SearchFPR         float64                  `json:"search_fpr"`
	Tier              string                   `json:"tier"`
	IdentityBar       float64                  `json:"identity_bar,omitempty"`
	ClearsIdentityBar bool                     `json:"clears_identity_bar"`
	Registry          *workflowRegistryOpinion `json:"registry,omitempty"`
	Confidence        string                   `json:"confidence"`
	ModelSynthetic    bool                     `json:"model_is_synthetic"`
}

type workflowRegistryOpinion struct {
	Name     string `json:"name"`
	Toon     string `json:"toon"`
	AuroraID int64  `json:"aurora_id"`
	Agrees   bool   `json:"agrees"`
}

type IdentityBadge struct {
	Kind         string   `json:"kind"`
	Tier         string   `json:"tier"`
	Label        string   `json:"label"`
	Liquipedia   string   `json:"liquipedia,omitempty"`
	EvidenceKey  string   `json:"evidence_key"`
	EvidenceArgs []string `json:"evidence_args,omitempty"`
}

type workflowPlayerOverview struct {
	SummaryVersion      string                        `json:"summary_version"`
	PlayerKey           string                        `json:"player_key"`
	PlayerName          string                        `json:"player_name"`
	GamesPlayed         int64                         `json:"games_played"`
	Wins                int64                         `json:"wins"`
	Undecided           int64                         `json:"undecided"`
	WinRate             float64                       `json:"win_rate"`
	AverageAPM          float64                       `json:"average_apm"`
	AverageEAPM         float64                       `json:"average_eapm"`
	HotkeyUsageRate     float64                       `json:"hotkey_usage_rate"`
	CarrierCommandCount int64                         `json:"carrier_command_count"`
	RaceBreakdown       []workflowPlayerRaceBreakdown `json:"race_breakdown"`
	FingerprintMetrics  []workflowComparativeMetric   `json:"fingerprint_metrics"`
	FingerprintCoverage workflowFingerprintCoverage   `json:"fingerprint_coverage"`
	FingerprintMatch    *workflowFingerprintMatch     `json:"fingerprint_match,omitempty"`
	PrimaryBadge        *IdentityBadge                `json:"primary_badge,omitempty"`
	SecondaryBadge      *IdentityBadge                `json:"secondary_badge,omitempty"`
	CountryCode         string                        `json:"country_code,omitempty"`
	BnetProfile         *bnetProfileDetail            `json:"bnet_profile,omitempty"`
	// The summary tab shows its Battle.net section only when this is non-zero.
	BnetGames int64 `json:"bnet_games"`
	// Set only for built-in progamer profiles (internal/propack); carries what the
	// page needs to explain where the data comes from.
	Featured       *featuredProfile              `json:"featured,omitempty"`
	RecentGames    []workflowGameListItem        `json:"recent_games"`
	ChatSummary    workflowPlayerChatSummary     `json:"chat_summary"`
	NarrativeHints []string                      `json:"narrative_hints"`
	Matchups       []workflowPlayerMatchupCell   `json:"matchups"`
	RaceOrders     []workflowRaceOrderSummary    `json:"race_orders"`
	MatchupOrders  []workflowMatchupOrderSummary `json:"matchup_orders"`
	EarlyTimings   []workflowPlayerEarlyTiming   `json:"early_timings"`
}

// Counts are raw: the frontend renders proportional fill across a fixed-size
// slot strip rather than percentages.
type workflowUnitCompositionUnit struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// The same unit can appear several times across a phase's Spells when it cast
// distinct spells (Science Vessel → Irradiate + EMP Shockwave).
type workflowUnitCompositionSpell struct {
	Unit  string `json:"unit"`
	Spell string `json:"spell"`
}

// One per-(player, phase) row, computed at request time from the persisted
// phase boundaries plus the Train / Unit Morph / Cast stream (see
// unit_composition.go).
//
// Spells is deduped by (unit, spell) and surfaced in the "Spellcasts" pill, not
// the composition bars. Spellcaster and signature non-army units are kept out of
// Units; Battlecruiser surfaces only via its Yamato Gun spell.
type workflowGameUnitComposition struct {
	PlayerID int64                          `json:"player_id"`
	Phase    string                         `json:"phase"`
	Units    []workflowUnitCompositionUnit  `json:"units"`
	Spells   []workflowUnitCompositionSpell `json:"spells,omitempty"`
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

type workflowComparativeMetric struct {
	Metric      string  `json:"metric"`
	PlayerValue float64 `json:"player_value"`
}

type workflowPlayerInsightType string

const (
	workflowPlayerInsightTypeAPM                workflowPlayerInsightType = "apm"
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
	IneligibleReasonKey   string                        `json:"ineligible_reason_key,omitempty"`
	IneligibleReasonArgs  []string                      `json:"ineligible_reason_args,omitempty"`
	Details               []workflowPlayerInsightDetail `json:"details"`
}

type workflowRaceOrderSummary struct {
	Race         string   `json:"race"`
	TechOrder    []string `json:"tech_order"`
	UpgradeOrder []string `json:"upgrade_order"`
}

// Games is the sample size used to pick the top sequence; the UI dims rows with
// Games < 5 to flag low confidence.
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
	PlayerKey         string         `json:"player_key"`
	PlayerName        string         `json:"player_name"`
	Race              string         `json:"race"`
	GamesPlayed       int64          `json:"games_played"`
	AverageAPM        float64        `json:"average_apm"`
	LastPlayed        string         `json:"last_played"`
	LastPlayedDaysAgo int64          `json:"last_played_days_ago"`
	CountryCode       string         `json:"country_code,omitempty"`
	PrimaryBadge      *IdentityBadge `json:"primary_badge,omitempty"`
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
	FeaturedPlayers  []workflowFeaturedPoint           `json:"featured_players,omitempty"`
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
	FeaturedPlayers   []workflowFeaturedPoint                 `json:"featured_players,omitempty"`
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

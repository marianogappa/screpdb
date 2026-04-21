package dashboard

const workflowSummaryVersion = "v1"

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
	ReplayID        int64                     `json:"replay_id"`
	ReplayDate      string                    `json:"replay_date"`
	FileName        string                    `json:"file_name"`
	MapName         string                    `json:"map_name"`
	DurationSeconds int64                     `json:"duration_seconds"`
	GameType        string                    `json:"game_type"`
	PlayersLabel    string                    `json:"players_label"`
	WinnersLabel    string                    `json:"winners_label"`
	Players         []workflowGameListPlayer  `json:"players"`
	Featuring       []string                  `json:"featuring"`
	CurrentPlayer   *workflowRecentGamePlayer `json:"current_player,omitempty"`
}

type workflowGameListPlayer struct {
	PlayerID  int64  `json:"player_id"`
	PlayerKey string `json:"player_key"`
	Name      string `json:"name"`
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
}

type workflowGamesListFilterOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Games int64  `json:"games"`
}

type workflowGamesListFilterOptions struct {
	Players   []workflowGamesListFilterOption `json:"players"`
	Maps      []workflowGamesListFilterOption `json:"maps"`
	Durations []workflowGamesListFilterOption `json:"durations"`
	Featuring []workflowGamesListFilterOption `json:"featuring"`
}

var workflowFeaturingFilters = []struct {
	Key   string
	Label string
}{
	{Key: "carriers", Label: "Carrier"},
	{Key: "battlecruisers", Label: "Battlecruiser"},
	{Key: "cannon_rush", Label: "Cannon Rush"},
	{Key: "bunker_rush", Label: "Bunker Rush"},
	{Key: "zergling_rush", Label: "Zergling Rush"},
	{Key: "mind_control", Label: "Mind Control"},
	{Key: "nukes", Label: "Nukes"},
	{Key: "recalls", Label: "Recalls"},
}

var workflowDurationFilterBuckets = []struct {
	Key   string
	Label string
}{
	{Key: "under_10m", Label: "Under 10m"},
	{Key: "10_20m", Label: "10m - 20m"},
	{Key: "20_30m", Label: "20m - 30m"},
	{Key: "30_45m", Label: "30m - 45m"},
	{Key: "45m_plus", Label: "45m+"},
}

type workflowGamePlayer struct {
	PlayerID           int64                  `json:"player_id"`
	PlayerKey          string                 `json:"player_key"`
	Name               string                 `json:"name"`
	Color              string                 `json:"color,omitempty"`
	Race               string                 `json:"race"`
	Team               int64                  `json:"team"`
	IsWinner           bool                   `json:"is_winner"`
	APM                int64                  `json:"apm"`
	EAPM               int64                  `json:"eapm"`
	CommandCount       int64                  `json:"command_count"`
	HotkeyCommandCount int64                  `json:"hotkey_command_count"`
	HotkeyUsageRate    float64                `json:"hotkey_usage_rate"`
	DetectedPatterns   []workflowPatternValue `json:"detected_patterns"`
}

type workflowPatternValue struct {
	PatternName string `json:"pattern_name"`
	Value       string `json:"value"`
}

type workflowGameDetail struct {
	SummaryVersion       string                                   `json:"summary_version"`
	ReplayID             int64                                    `json:"replay_id"`
	ReplayDate           string                                   `json:"replay_date"`
	FileName             string                                   `json:"file_name"`
	MapName              string                                   `json:"map_name"`
	MapVisual            workflowMapVisual                        `json:"map_visual"`
	DurationSeconds      int64                                    `json:"duration_seconds"`
	GameType             string                                   `json:"game_type"`
	Players              []workflowGamePlayer                     `json:"players"`
	ReplayPatterns       []workflowPatternValue                   `json:"replay_patterns"`
	GameEvents           []workflowGameEvent                      `json:"game_events"`
	UnitsBySlice         []workflowUnitSlice                      `json:"units_by_slice"`
	Timings              workflowReplayTimings                    `json:"timings"`
	FirstUnitEfficiency  []workflowFirstUnitEfficiencyPlayer      `json:"first_unit_efficiency"`
	UnitCadence          []workflowGameUnitCadencePlayer          `json:"unit_production_cadence"`
	ViewportMultitasking []workflowGameViewportMultitaskingPlayer `json:"viewport_multitasking"`
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
	Type            string                   `json:"type"`
	Second          int64                    `json:"second"`
	Actor           *workflowGameEventPlayer `json:"actor,omitempty"`
	Target          *workflowGameEventPlayer `json:"target,omitempty"`
	Base            *workflowGameEventBase   `json:"base,omitempty"`
	ActorOrigin     *workflowGameEventPoint  `json:"actor_origin,omitempty"`
	ActorStartClock *int64                   `json:"actor_start_clock,omitempty"`
	Ownership       []workflowGameOwnership  `json:"ownership,omitempty"`
	AttackUnitTypes []string                 `json:"attack_unit_types,omitempty"`
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
	CommonBehaviours    []workflowCommonBehaviour     `json:"common_behaviours"`
	FingerprintMetrics  []workflowComparativeMetric   `json:"fingerprint_metrics"`
	QueuedGames         int64                         `json:"queued_games"`
	QueuedGameRate      float64                       `json:"queued_game_rate"`
	RecentGames         []workflowGameListItem        `json:"recent_games"`
	ChatSummary         workflowPlayerChatSummary     `json:"chat_summary"`
	NarrativeHints      []string                      `json:"narrative_hints"`
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

type workflowCommonBehaviour struct {
	Name        string  `json:"name"`
	PrettyName  string  `json:"pretty_name"`
	ReplayCount int64   `json:"replay_count"`
	GameRate    float64 `json:"game_rate"`
}

type workflowPlayerOutliers struct {
	SummaryVersion string                    `json:"summary_version"`
	PlayerKey      string                    `json:"player_key"`
	PlayerName     string                    `json:"player_name"`
	Thresholds     workflowOutlierThresholds `json:"thresholds"`
	Items          []workflowPlayerOutlier   `json:"items"`
}

type workflowPlayerMetrics struct {
	SummaryVersion        string                         `json:"summary_version"`
	PlayerKey             string                         `json:"player_key"`
	RaceBehaviourSections []workflowRaceBehaviourSection `json:"race_behaviour_sections"`
	FingerprintMetrics    []workflowComparativeMetric    `json:"fingerprint_metrics"`
}

type workflowRaceBehaviourSection struct {
	Race             string                    `json:"race"`
	GameCount        int64                     `json:"game_count"`
	GameRate         float64                   `json:"game_rate"`
	Wins             int64                     `json:"wins"`
	WinRate          float64                   `json:"win_rate"`
	CommonBehaviours []workflowCommonBehaviour `json:"common_behaviours"`
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

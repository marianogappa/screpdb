package dashboard

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/gorilla/mux"
	"github.com/marianogappa/screpdb/internal/dashboard/variables"
)

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
	ReplayID        int64                    `json:"replay_id"`
	ReplayDate      string                   `json:"replay_date"`
	FileName        string                   `json:"file_name"`
	MapName         string                   `json:"map_name"`
	DurationSeconds int64                    `json:"duration_seconds"`
	GameType        string                   `json:"game_type"`
	PlayersLabel    string                   `json:"players_label"`
	WinnersLabel    string                   `json:"winners_label"`
	Players         []workflowGameListPlayer `json:"players"`
	Featuring       []string                 `json:"featuring"`
}

type workflowGameListPlayer struct {
	PlayerID  int64  `json:"player_id"`
	PlayerKey string `json:"player_key"`
	Name      string `json:"name"`
	Team      int64  `json:"team"`
	IsWinner  bool   `json:"is_winner"`
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
	{Key: "carriers", Label: "10+ Carriers"},
	{Key: "battlecruisers", Label: "10+ Battlecruisers"},
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
	SQL   string
}{
	{Key: "under_10m", Label: "Under 10m", SQL: "r.duration_seconds < 600"},
	{Key: "10_20m", Label: "10m - 20m", SQL: "r.duration_seconds >= 600 AND r.duration_seconds < 1200"},
	{Key: "20_30m", Label: "20m - 30m", SQL: "r.duration_seconds >= 1200 AND r.duration_seconds < 1800"},
	{Key: "30_45m", Label: "30m - 45m", SQL: "r.duration_seconds >= 1800 AND r.duration_seconds < 2700"},
	{Key: "45m_plus", Label: "45m+", SQL: "r.duration_seconds >= 2700"},
}

type workflowGamePlayer struct {
	PlayerID           int64                  `json:"player_id"`
	PlayerKey          string                 `json:"player_key"`
	Name               string                 `json:"name"`
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

type workflowTeamPattern struct {
	Team        int64  `json:"team"`
	PatternName string `json:"pattern_name"`
	Value       string `json:"value"`
}

type workflowGameDetail struct {
	SummaryVersion      string                              `json:"summary_version"`
	ReplayID            int64                               `json:"replay_id"`
	ReplayDate          string                              `json:"replay_date"`
	FileName            string                              `json:"file_name"`
	MapName             string                              `json:"map_name"`
	DurationSeconds     int64                               `json:"duration_seconds"`
	GameType            string                              `json:"game_type"`
	Players             []workflowGamePlayer                `json:"players"`
	ReplayPatterns      []workflowPatternValue              `json:"replay_patterns"`
	TeamPatterns        []workflowTeamPattern               `json:"team_patterns"`
	GameEvents          []workflowGameEvent                 `json:"game_events"`
	UnitsBySlice        []workflowUnitSlice                 `json:"units_by_slice"`
	Timings             workflowReplayTimings               `json:"timings"`
	FirstUnitEfficiency []workflowFirstUnitEfficiencyPlayer `json:"first_unit_efficiency"`
}

type workflowGameEvent struct {
	Type        string `json:"type"`
	Second      int64  `json:"second"`
	Description string `json:"description"`
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
	TopPhrases      []workflowChatTermCount `json:"top_phrases"`
	ToneHints       []string                `json:"tone_hints"`
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

type workflowRaceOrderSummary struct {
	Race         string   `json:"race"`
	TechOrder    []string `json:"tech_order"`
	UpgradeOrder []string `json:"upgrade_order"`
}

func (d *Dashboard) handlerWorkflowGamesList(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r, 20, 200)
	filters := parseWorkflowGamesListFilters(r)
	whereSQL, whereArgs := buildWorkflowGamesListWhere(filters)

	countQuery := "SELECT COUNT(*) FROM replays r " + whereSQL
	var total int64
	if err := d.db.QueryRowContext(d.ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		http.Error(w, "failed to count games: "+err.Error(), http.StatusInternalServerError)
		return
	}

	listArgs := append([]any{}, whereArgs...)
	listArgs = append(listArgs, limit, offset)
	rows, err := d.db.QueryContext(d.ctx, `
		SELECT
			r.id,
			r.replay_date,
			r.file_name,
			r.map_name,
			r.duration_seconds,
			r.game_type
		FROM replays r
	`+whereSQL+`
		ORDER BY r.replay_date DESC, r.id DESC
		LIMIT ? OFFSET ?
	`, listArgs...)
	if err != nil {
		http.Error(w, "failed to list games: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []workflowGameListItem{}
	for rows.Next() {
		var item workflowGameListItem
		if err := rows.Scan(
			&item.ReplayID,
			&item.ReplayDate,
			&item.FileName,
			&item.MapName,
			&item.DurationSeconds,
			&item.GameType,
		); err != nil {
			http.Error(w, "failed to parse games list: "+err.Error(), http.StatusInternalServerError)
			return
		}
		item.Players = []workflowGameListPlayer{}
		item.Featuring = []string{}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "failed to iterate games list: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := d.populateWorkflowGameListPlayers(items); err != nil {
		http.Error(w, "failed to enrich games list players: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := d.populateWorkflowGameListFeaturing(items); err != nil {
		http.Error(w, "failed to enrich games list featuring: "+err.Error(), http.StatusInternalServerError)
		return
	}
	filterOptions, err := d.workflowGamesListFilterOptions()
	if err != nil {
		http.Error(w, "failed to build games list filters: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"summary_version": workflowSummaryVersion,
		"items":           items,
		"limit":           limit,
		"offset":          offset,
		"total":           total,
		"filter_options":  filterOptions,
	})
}

func parseWorkflowGamesListFilters(r *http.Request) workflowGamesListFilters {
	return workflowGamesListFilters{
		PlayerKeys:      parseCSVQueryValues(r.URL.Query()["player"], true),
		MapNames:        parseCSVQueryValues(r.URL.Query()["map"], false),
		DurationBuckets: parseCSVQueryValues(r.URL.Query()["duration"], true),
		FeaturingKeys:   parseCSVQueryValues(r.URL.Query()["featuring"], true),
	}
}

func parseCSVQueryValues(values []string, forceLower bool) []string {
	dedup := map[string]struct{}{}
	out := []string{}
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}
			if forceLower {
				value = strings.ToLower(value)
			}
			if _, ok := dedup[value]; ok {
				continue
			}
			dedup[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}

func buildWorkflowGamesListWhere(filters workflowGamesListFilters) (string, []any) {
	clauses := []string{}
	args := []any{}

	if len(filters.PlayerKeys) > 0 {
		playerPlaceholders := buildInClausePlaceholders(len(filters.PlayerKeys))
		clauses = append(clauses, "EXISTS (SELECT 1 FROM players p WHERE p.replay_id = r.id AND p.is_observer = 0 AND lower(trim(p.name)) IN ("+playerPlaceholders+"))")
		for _, key := range filters.PlayerKeys {
			args = append(args, key)
		}
	}

	if len(filters.MapNames) > 0 {
		mapPlaceholders := buildInClausePlaceholders(len(filters.MapNames))
		clauses = append(clauses, "lower(trim(r.map_name)) IN ("+mapPlaceholders+")")
		for _, mapName := range filters.MapNames {
			args = append(args, strings.ToLower(strings.TrimSpace(mapName)))
		}
	}

	if len(filters.DurationBuckets) > 0 {
		durationClauses := []string{}
		for _, key := range filters.DurationBuckets {
			for _, bucket := range workflowDurationFilterBuckets {
				if key == bucket.Key {
					durationClauses = append(durationClauses, "("+bucket.SQL+")")
					break
				}
			}
		}
		if len(durationClauses) > 0 {
			clauses = append(clauses, "("+strings.Join(durationClauses, " OR ")+")")
		}
	}

	if len(filters.FeaturingKeys) > 0 {
		featureClauses := []string{}
		for _, featureKey := range filters.FeaturingKeys {
			existsSQL, ok := workflowFeaturingExistsSQL(featureKey)
			if !ok {
				continue
			}
			featureClauses = append(featureClauses, existsSQL)
		}
		if len(featureClauses) > 0 {
			clauses = append(clauses, "("+strings.Join(featureClauses, " OR ")+")")
		}
	}

	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func workflowFeaturingExistsSQL(featureKey string) (string, bool) {
	switch strings.TrimSpace(strings.ToLower(featureKey)) {
	case "carriers":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay_player dprp
			WHERE dprp.replay_id = r.id
				AND lower(trim(dprp.pattern_name)) = 'carriers'
				AND dprp.value_bool = 1
		)`, true
	case "battlecruisers":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay_player dprp
			WHERE dprp.replay_id = r.id
				AND lower(trim(dprp.pattern_name)) = 'battlecruisers'
				AND dprp.value_bool = 1
		)`, true
	case "mind_control":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay_player dprp
			WHERE dprp.replay_id = r.id
				AND lower(trim(dprp.pattern_name)) IN ('became terran', 'became zerg')
				AND (dprp.value_timestamp IS NOT NULL OR dprp.value_int IS NOT NULL OR dprp.value_string IS NOT NULL)
		)`, true
	case "nukes":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay_player dprp
			WHERE dprp.replay_id = r.id
				AND lower(trim(dprp.pattern_name)) = 'threw nukes'
				AND (dprp.value_timestamp IS NOT NULL OR dprp.value_int IS NOT NULL OR dprp.value_string IS NOT NULL OR dprp.value_bool = 1)
		)`, true
	case "recalls":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay_player dprp
			WHERE dprp.replay_id = r.id
				AND lower(trim(dprp.pattern_name)) = 'made recalls'
				AND (dprp.value_timestamp IS NOT NULL OR dprp.value_int IS NOT NULL OR dprp.value_string IS NOT NULL OR dprp.value_bool = 1)
		)`, true
	case "cannon_rush", "bunker_rush":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay dpr
			WHERE dpr.replay_id = r.id
				AND lower(trim(dpr.pattern_name)) = 'game events'
				AND lower(coalesce(dpr.value_string, '')) LIKE '%cannon/bunker rushes%'
		)`, true
	case "zergling_rush":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay dpr
			WHERE dpr.replay_id = r.id
				AND lower(trim(dpr.pattern_name)) = 'game events'
				AND lower(coalesce(dpr.value_string, '')) LIKE '%zergling rushes%'
		)`, true
	default:
		return "", false
	}
}

func buildInClausePlaceholders(size int) string {
	if size <= 0 {
		return ""
	}
	parts := make([]string, 0, size)
	for i := 0; i < size; i++ {
		parts = append(parts, "?")
	}
	return strings.Join(parts, ", ")
}

func (d *Dashboard) populateWorkflowGameListPlayers(items []workflowGameListItem) error {
	replayIDs := make([]int64, 0, len(items))
	itemIndexByReplayID := map[int64]int{}
	for i, item := range items {
		replayIDs = append(replayIDs, item.ReplayID)
		itemIndexByReplayID[item.ReplayID] = i
	}
	if len(replayIDs) == 0 {
		return nil
	}
	placeholders := buildInClausePlaceholders(len(replayIDs))
	args := make([]any, 0, len(replayIDs))
	for _, replayID := range replayIDs {
		args = append(args, replayID)
	}
	rows, err := d.db.QueryContext(d.ctx, `
		SELECT replay_id, id, name, team, is_winner
		FROM players
		WHERE is_observer = 0
			AND replay_id IN (`+placeholders+`)
		ORDER BY replay_id ASC, team ASC, id ASC
	`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var replayID int64
		var player workflowGameListPlayer
		if err := rows.Scan(&replayID, &player.PlayerID, &player.Name, &player.Team, &player.IsWinner); err != nil {
			return err
		}
		player.PlayerKey = normalizePlayerKey(player.Name)
		idx, ok := itemIndexByReplayID[replayID]
		if !ok {
			continue
		}
		items[idx].Players = append(items[idx].Players, player)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range items {
		items[i].PlayersLabel = formatWorkflowPlayersLabelFromList(items[i].Players)
	}
	return nil
}

func (d *Dashboard) populateWorkflowGameListFeaturing(items []workflowGameListItem) error {
	replayIDs := make([]int64, 0, len(items))
	itemIndexByReplayID := map[int64]int{}
	featureSets := map[int64]map[string]struct{}{}
	for i, item := range items {
		replayIDs = append(replayIDs, item.ReplayID)
		itemIndexByReplayID[item.ReplayID] = i
		featureSets[item.ReplayID] = map[string]struct{}{}
	}
	if len(replayIDs) == 0 {
		return nil
	}
	placeholders := buildInClausePlaceholders(len(replayIDs))
	args := make([]any, 0, len(replayIDs))
	for _, replayID := range replayIDs {
		args = append(args, replayID)
	}

	rowsPlayerPatterns, err := d.db.QueryContext(d.ctx, `
		SELECT replay_id, pattern_name, value_bool, value_int, value_string, value_timestamp
		FROM detected_patterns_replay_player
		WHERE replay_id IN (`+placeholders+`)
			AND lower(trim(pattern_name)) IN ('carriers', 'battlecruisers', 'made recalls', 'threw nukes', 'became terran', 'became zerg')
	`, args...)
	if err != nil {
		return err
	}
	defer rowsPlayerPatterns.Close()
	for rowsPlayerPatterns.Next() {
		var replayID int64
		var patternName string
		var valueBool sql.NullBool
		var valueInt sql.NullInt64
		var valueString sql.NullString
		var valueTimestamp sql.NullInt64
		if err := rowsPlayerPatterns.Scan(&replayID, &patternName, &valueBool, &valueInt, &valueString, &valueTimestamp); err != nil {
			return err
		}
		if !workflowTruthyPatternValue(valueBool, valueInt, valueString, valueTimestamp) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(patternName)) {
		case "carriers":
			featureSets[replayID]["carriers"] = struct{}{}
		case "battlecruisers":
			featureSets[replayID]["battlecruisers"] = struct{}{}
		case "made recalls":
			featureSets[replayID]["recalls"] = struct{}{}
		case "threw nukes":
			featureSets[replayID]["nukes"] = struct{}{}
		case "became terran", "became zerg":
			featureSets[replayID]["mind_control"] = struct{}{}
		}
	}
	if err := rowsPlayerPatterns.Err(); err != nil {
		return err
	}

	rowsReplayPatterns, err := d.db.QueryContext(d.ctx, `
		SELECT replay_id, value_string
		FROM detected_patterns_replay
		WHERE replay_id IN (`+placeholders+`)
			AND lower(trim(pattern_name)) = 'game events'
	`, args...)
	if err != nil {
		return err
	}
	defer rowsReplayPatterns.Close()
	for rowsReplayPatterns.Next() {
		var replayID int64
		var gameEventsRaw sql.NullString
		if err := rowsReplayPatterns.Scan(&replayID, &gameEventsRaw); err != nil {
			return err
		}
		if !gameEventsRaw.Valid {
			continue
		}
		events := parseGameEvents(gameEventsRaw.String)
		for _, event := range events {
			description := strings.ToLower(strings.TrimSpace(event.Description))
			if strings.Contains(description, "zergling rushes") {
				featureSets[replayID]["zergling_rush"] = struct{}{}
			}
			if strings.Contains(description, "cannon/bunker rushes") {
				featureSets[replayID]["cannon_rush"] = struct{}{}
				featureSets[replayID]["bunker_rush"] = struct{}{}
			}
		}
	}
	if err := rowsReplayPatterns.Err(); err != nil {
		return err
	}

	for replayID, set := range featureSets {
		idx, ok := itemIndexByReplayID[replayID]
		if !ok {
			continue
		}
		labels := make([]string, 0, len(set))
		for _, cfg := range workflowFeaturingFilters {
			if _, has := set[cfg.Key]; has {
				labels = append(labels, cfg.Label)
			}
		}
		items[idx].Featuring = labels
	}
	return nil
}

func workflowTruthyPatternValue(valueBool sql.NullBool, valueInt sql.NullInt64, valueString sql.NullString, valueTimestamp sql.NullInt64) bool {
	if valueBool.Valid {
		return valueBool.Bool
	}
	if valueInt.Valid {
		return valueInt.Int64 > 0
	}
	if valueTimestamp.Valid {
		return valueTimestamp.Int64 > 0
	}
	if valueString.Valid {
		v := strings.TrimSpace(strings.ToLower(valueString.String))
		return v != "" && v != "false" && v != "no" && v != "-"
	}
	return false
}

func formatWorkflowPlayersLabelFromList(players []workflowGameListPlayer) string {
	if len(players) == 0 {
		return ""
	}
	playersByTeam := map[int64][]string{}
	teamOrder := []int64{}
	for _, player := range players {
		if _, ok := playersByTeam[player.Team]; !ok {
			teamOrder = append(teamOrder, player.Team)
		}
		playersByTeam[player.Team] = append(playersByTeam[player.Team], player.Name)
	}
	usesTeams := false
	for _, team := range teamOrder {
		if len(playersByTeam[team]) > 1 {
			usesTeams = true
			break
		}
	}
	sides := make([]string, 0, len(teamOrder))
	for _, team := range teamOrder {
		teamPlayers := playersByTeam[team]
		if usesTeams && len(teamPlayers) > 1 {
			sides = append(sides, "("+strings.Join(teamPlayers, " & ")+")")
			continue
		}
		sides = append(sides, strings.Join(teamPlayers, ", "))
	}
	return strings.Join(sides, " vs ")
}

func (d *Dashboard) workflowGamesListFilterOptions() (workflowGamesListFilterOptions, error) {
	result := workflowGamesListFilterOptions{
		Players:   []workflowGamesListFilterOption{},
		Maps:      []workflowGamesListFilterOption{},
		Durations: []workflowGamesListFilterOption{},
		Featuring: []workflowGamesListFilterOption{},
	}

	rowsPlayers, err := d.db.QueryContext(d.ctx, `
		SELECT lower(trim(name)) AS player_key, MIN(name) AS player_name, COUNT(*) AS games
		FROM players
		WHERE is_observer = 0
		GROUP BY lower(trim(name))
		HAVING COUNT(*) >= 5
		ORDER BY games DESC, player_name ASC
		LIMIT 200
	`)
	if err != nil {
		return result, err
	}
	defer rowsPlayers.Close()
	for rowsPlayers.Next() {
		var option workflowGamesListFilterOption
		if err := rowsPlayers.Scan(&option.Key, &option.Label, &option.Games); err != nil {
			return result, err
		}
		result.Players = append(result.Players, option)
	}
	if err := rowsPlayers.Err(); err != nil {
		return result, err
	}

	rowsMaps, err := d.db.QueryContext(d.ctx, `
		SELECT MIN(map_name) AS map_name, COUNT(*) AS games
		FROM replays
		GROUP BY lower(trim(map_name))
		ORDER BY games DESC, map_name ASC
		LIMIT 15
	`)
	if err != nil {
		return result, err
	}
	defer rowsMaps.Close()
	for rowsMaps.Next() {
		var option workflowGamesListFilterOption
		if err := rowsMaps.Scan(&option.Label, &option.Games); err != nil {
			return result, err
		}
		option.Key = strings.ToLower(strings.TrimSpace(option.Label))
		result.Maps = append(result.Maps, option)
	}
	if err := rowsMaps.Err(); err != nil {
		return result, err
	}

	durationCountQuery := `
		SELECT
			COALESCE(SUM(CASE WHEN duration_seconds < 600 THEN 1 ELSE 0 END), 0) AS under_10m,
			COALESCE(SUM(CASE WHEN duration_seconds >= 600 AND duration_seconds < 1200 THEN 1 ELSE 0 END), 0) AS m10_20,
			COALESCE(SUM(CASE WHEN duration_seconds >= 1200 AND duration_seconds < 1800 THEN 1 ELSE 0 END), 0) AS m20_30,
			COALESCE(SUM(CASE WHEN duration_seconds >= 1800 AND duration_seconds < 2700 THEN 1 ELSE 0 END), 0) AS m30_45,
			COALESCE(SUM(CASE WHEN duration_seconds >= 2700 THEN 1 ELSE 0 END), 0) AS m45_plus
		FROM replays
	`
	var under10m, m10to20, m20to30, m30to45, m45Plus int64
	if err := d.db.QueryRowContext(d.ctx, durationCountQuery).Scan(&under10m, &m10to20, &m20to30, &m30to45, &m45Plus); err != nil {
		return result, err
	}
	durationCounts := map[string]int64{
		"under_10m": under10m,
		"10_20m":    m10to20,
		"20_30m":    m20to30,
		"30_45m":    m30to45,
		"45m_plus":  m45Plus,
	}
	for _, bucket := range workflowDurationFilterBuckets {
		result.Durations = append(result.Durations, workflowGamesListFilterOption{
			Key:   bucket.Key,
			Label: bucket.Label,
			Games: durationCounts[bucket.Key],
		})
	}

	for _, feature := range workflowFeaturingFilters {
		result.Featuring = append(result.Featuring, workflowGamesListFilterOption{
			Key:   feature.Key,
			Label: feature.Label,
		})
	}
	return result, nil
}

func (d *Dashboard) handlerWorkflowGameDetail(w http.ResponseWriter, r *http.Request) {
	replayID, err := parseReplayID(mux.Vars(r)["replayID"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	detail, err := d.buildWorkflowGameDetail(replayID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	_ = json.NewEncoder(w).Encode(detail)
}

func (d *Dashboard) handlerWorkflowPlayerDetail(w http.ResponseWriter, r *http.Request) {
	playerKey := normalizePlayerKey(mux.Vars(r)["playerKey"])
	if playerKey == "" {
		http.Error(w, "player key missing", http.StatusBadRequest)
		return
	}
	player, err := d.buildWorkflowPlayerOverview(playerKey)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	_ = json.NewEncoder(w).Encode(player)
}

func (d *Dashboard) handlerWorkflowPlayerOutliers(w http.ResponseWriter, r *http.Request) {
	playerKey := normalizePlayerKey(mux.Vars(r)["playerKey"])
	if playerKey == "" {
		http.Error(w, "player key missing", http.StatusBadRequest)
		return
	}
	outliers, err := d.buildWorkflowPlayerOutliers(playerKey)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	_ = json.NewEncoder(w).Encode(outliers)
}

func (d *Dashboard) handlerWorkflowPlayerMetrics(w http.ResponseWriter, r *http.Request) {
	playerKey := normalizePlayerKey(mux.Vars(r)["playerKey"])
	if playerKey == "" {
		http.Error(w, "player key missing", http.StatusBadRequest)
		return
	}
	metrics, err := d.buildWorkflowPlayerMetrics(playerKey)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	_ = json.NewEncoder(w).Encode(metrics)
}

func (d *Dashboard) handlerWorkflowPlayerColors(w http.ResponseWriter, _ *http.Request) {
	rows, err := d.db.QueryContext(d.ctx, `
		SELECT lower(trim(name)) AS player_key, COUNT(*) AS games
		FROM players
		WHERE is_observer = 0
		GROUP BY lower(trim(name))
		ORDER BY games DESC, player_key ASC
		LIMIT 15
	`)
	if err != nil {
		http.Error(w, "failed to compute player colors: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	playerColors := map[string]string{}
	i := 0
	for rows.Next() {
		if i >= len(topPlayerPalette) {
			break
		}
		var key string
		var games int64
		if err := rows.Scan(&key, &games); err != nil {
			http.Error(w, "failed to parse player colors: "+err.Error(), http.StatusInternalServerError)
			return
		}
		playerColors[key] = topPlayerPalette[i]
		i++
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "failed to iterate player colors: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"player_colors": playerColors,
		"palette":       topPlayerPalette,
	})
}

func (d *Dashboard) handlerWorkflowAskGame(w http.ResponseWriter, r *http.Request) {
	replayID, err := parseReplayID(mux.Vars(r)["replayID"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	question, err := decodeAskQuestion(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !d.ai.IsAvailable() {
		http.Error(w, "AI is not configured", http.StatusBadRequest)
		return
	}
	detail, err := d.buildWorkflowGameDetail(replayID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	scope := fmt.Sprintf("The answer MUST be scoped to replay_id=%d. Prefer SQL WHERE replay_id = %d when querying replay/player/command tables.", replayID, replayID)
	answer, err := d.ai.AnswerWorkflowQuestion(question, detail, scope)
	if err != nil {
		http.Error(w, "failed to answer question: "+err.Error(), http.StatusInternalServerError)
		return
	}
	results := []map[string]any{}
	columns := []string{}
	if answer.Config.Type != WidgetTypeText && strings.TrimSpace(answer.SQLQuery) != "" {
		filter := fmt.Sprintf("SELECT * FROM replays WHERE id = %d", replayID)
		qResults, qColumns, err := d.executeQuery(answer.SQLQuery, map[string]variables.Variable{}, &filter)
		if err != nil {
			answer.Config.Type = WidgetTypeText
			answer.TextAnswer = "I generated SQL but it did not execute successfully in this context. Please try rephrasing your question."
			answer.SQLQuery = ""
		} else {
			results = qResults
			columns = qColumns
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"title":       answer.Title,
		"description": answer.Description,
		"config":      answer.Config,
		"sql_query":   answer.SQLQuery,
		"text_answer": answer.TextAnswer,
		"results":     results,
		"columns":     columns,
	})
}

func (d *Dashboard) handlerWorkflowAskPlayer(w http.ResponseWriter, r *http.Request) {
	playerKey := normalizePlayerKey(mux.Vars(r)["playerKey"])
	if playerKey == "" {
		http.Error(w, "player key missing", http.StatusBadRequest)
		return
	}
	question, err := decodeAskQuestion(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !d.ai.IsAvailable() {
		http.Error(w, "AI is not configured", http.StatusBadRequest)
		return
	}
	player, err := d.buildWorkflowPlayerOverview(playerKey)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	scope := fmt.Sprintf("The answer MUST be scoped to player_key=%q (normalized player name). Prefer SQL WHERE lower(trim(name)) = %q for player-specific analysis.", playerKey, playerKey)
	answer, err := d.ai.AnswerWorkflowQuestion(question, player, scope)
	if err != nil {
		http.Error(w, "failed to answer question: "+err.Error(), http.StatusInternalServerError)
		return
	}
	results := []map[string]any{}
	columns := []string{}
	if answer.Config.Type != WidgetTypeText && strings.TrimSpace(answer.SQLQuery) != "" {
		qResults, qColumns, err := d.executeQuery(answer.SQLQuery, map[string]variables.Variable{}, nil)
		if err != nil {
			answer.Config.Type = WidgetTypeText
			answer.TextAnswer = "I generated SQL but it did not execute successfully in this context. Please try rephrasing your question."
			answer.SQLQuery = ""
		} else {
			results = qResults
			columns = qColumns
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"title":       answer.Title,
		"description": answer.Description,
		"config":      answer.Config,
		"sql_query":   answer.SQLQuery,
		"text_answer": answer.TextAnswer,
		"results":     results,
		"columns":     columns,
	})
}

func (d *Dashboard) buildWorkflowGameDetail(replayID int64) (workflowGameDetail, error) {
	detail := workflowGameDetail{SummaryVersion: workflowSummaryVersion}
	err := d.db.QueryRowContext(d.ctx, `
		SELECT id, replay_date, file_name, map_name, duration_seconds, game_type
		FROM replays WHERE id = ?
	`, replayID).Scan(
		&detail.ReplayID,
		&detail.ReplayDate,
		&detail.FileName,
		&detail.MapName,
		&detail.DurationSeconds,
		&detail.GameType,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return detail, sql.ErrNoRows
		}
		return detail, fmt.Errorf("failed to load replay: %w", err)
	}

	rows, err := d.db.QueryContext(d.ctx, `
		SELECT
			p.id,
			p.name,
			p.race,
			p.team,
			p.is_winner,
			p.apm,
			p.eapm,
			COUNT(c.id) AS command_count,
			(
				SELECT COUNT(*)
				FROM commands_low_value clv
				WHERE clv.player_id = p.id
					AND clv.action_type = 'Hotkey'
					AND clv.hotkey_type IS NOT NULL
			) AS hotkey_count,
			(
				SELECT COUNT(*)
				FROM commands_low_value clv
				WHERE clv.player_id = p.id
			) AS low_value_command_count
		FROM players p
		LEFT JOIN commands c ON c.player_id = p.id
		WHERE p.replay_id = ? AND p.is_observer = 0
		GROUP BY p.id, p.name, p.race, p.team, p.is_winner, p.apm, p.eapm
		ORDER BY p.team ASC, p.id ASC
	`, replayID)
	if err != nil {
		return detail, fmt.Errorf("failed to load players: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p workflowGamePlayer
		var lowValueCommandCount int64
		if err := rows.Scan(
			&p.PlayerID,
			&p.Name,
			&p.Race,
			&p.Team,
			&p.IsWinner,
			&p.APM,
			&p.EAPM,
			&p.CommandCount,
			&p.HotkeyCommandCount,
			&lowValueCommandCount,
		); err != nil {
			return detail, fmt.Errorf("failed to parse players: %w", err)
		}
		p.PlayerKey = normalizePlayerKey(p.Name)
		totalCommandCount := p.CommandCount + lowValueCommandCount
		if totalCommandCount > 0 {
			p.HotkeyUsageRate = float64(p.HotkeyCommandCount) / float64(totalCommandCount)
		}
		p.DetectedPatterns = []workflowPatternValue{}
		detail.Players = append(detail.Players, p)
	}
	if err := rows.Err(); err != nil {
		return detail, fmt.Errorf("failed to iterate players: %w", err)
	}

	if err := d.populateDetectedPatternsForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populateUnitsBySliceForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populateTimingsForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populateFirstUnitEfficiencyForGameDetail(&detail); err != nil {
		return detail, err
	}

	return detail, nil
}

func (d *Dashboard) populateDetectedPatternsForGameDetail(detail *workflowGameDetail) error {
	detail.ReplayPatterns = []workflowPatternValue{}
	detail.TeamPatterns = []workflowTeamPattern{}
	detail.GameEvents = []workflowGameEvent{}

	rowsReplay, err := d.db.QueryContext(d.ctx, `
		SELECT
			pattern_name,
			CASE
				WHEN value_bool IS NOT NULL THEN CASE WHEN value_bool = 1 THEN 'true' ELSE 'false' END
				WHEN value_int IS NOT NULL THEN CAST(value_int AS TEXT)
				WHEN value_string IS NOT NULL THEN value_string
				WHEN value_timestamp IS NOT NULL THEN CAST(value_timestamp AS TEXT)
				ELSE ''
			END AS pattern_value
		FROM detected_patterns_replay
		WHERE replay_id = ?
		ORDER BY pattern_name ASC
	`, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to query replay patterns: %w", err)
	}
	defer rowsReplay.Close()
	for rowsReplay.Next() {
		var pattern workflowPatternValue
		if err := rowsReplay.Scan(&pattern.PatternName, &pattern.Value); err != nil {
			return fmt.Errorf("failed to parse replay patterns: %w", err)
		}
		if strings.EqualFold(pattern.PatternName, "Game Events") {
			detail.GameEvents = parseGameEvents(pattern.Value)
			continue
		}
		pattern.Value = formatPatternValueForUI(pattern.PatternName, pattern.Value)
		detail.ReplayPatterns = append(detail.ReplayPatterns, pattern)
	}
	if err := rowsReplay.Err(); err != nil {
		return fmt.Errorf("failed iterating replay patterns: %w", err)
	}

	rowsTeam, err := d.db.QueryContext(d.ctx, `
		SELECT
			team,
			pattern_name,
			CASE
				WHEN value_bool IS NOT NULL THEN CASE WHEN value_bool = 1 THEN 'true' ELSE 'false' END
				WHEN value_int IS NOT NULL THEN CAST(value_int AS TEXT)
				WHEN value_string IS NOT NULL THEN value_string
				WHEN value_timestamp IS NOT NULL THEN CAST(value_timestamp AS TEXT)
				ELSE ''
			END AS pattern_value
		FROM detected_patterns_replay_team
		WHERE replay_id = ?
		ORDER BY team ASC, pattern_name ASC
	`, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to query team patterns: %w", err)
	}
	defer rowsTeam.Close()
	for rowsTeam.Next() {
		var pattern workflowTeamPattern
		if err := rowsTeam.Scan(&pattern.Team, &pattern.PatternName, &pattern.Value); err != nil {
			return fmt.Errorf("failed to parse team patterns: %w", err)
		}
		pattern.Value = formatPatternValueForUI(pattern.PatternName, pattern.Value)
		detail.TeamPatterns = append(detail.TeamPatterns, pattern)
	}
	if err := rowsTeam.Err(); err != nil {
		return fmt.Errorf("failed iterating team patterns: %w", err)
	}

	playerByID := map[int64]*workflowGamePlayer{}
	for i := range detail.Players {
		player := &detail.Players[i]
		playerByID[player.PlayerID] = player
	}

	rowsPlayer, err := d.db.QueryContext(d.ctx, `
		SELECT
			player_id,
			pattern_name,
			CASE
				WHEN value_bool IS NOT NULL THEN CASE WHEN value_bool = 1 THEN 'true' ELSE 'false' END
				WHEN value_int IS NOT NULL THEN CAST(value_int AS TEXT)
				WHEN value_string IS NOT NULL THEN value_string
				WHEN value_timestamp IS NOT NULL THEN CAST(value_timestamp AS TEXT)
				ELSE ''
			END AS pattern_value
		FROM detected_patterns_replay_player
		WHERE replay_id = ?
		ORDER BY player_id ASC, pattern_name ASC
	`, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to query player patterns: %w", err)
	}
	defer rowsPlayer.Close()
	for rowsPlayer.Next() {
		var playerID int64
		var pattern workflowPatternValue
		if err := rowsPlayer.Scan(&playerID, &pattern.PatternName, &pattern.Value); err != nil {
			return fmt.Errorf("failed to parse player patterns: %w", err)
		}
		pattern.Value = formatPatternValueForUI(pattern.PatternName, pattern.Value)
		if player, ok := playerByID[playerID]; ok {
			player.DetectedPatterns = append(player.DetectedPatterns, pattern)
		}
	}
	if err := rowsPlayer.Err(); err != nil {
		return fmt.Errorf("failed iterating player patterns: %w", err)
	}
	return nil
}

func (d *Dashboard) buildWorkflowPlayerOverview(playerKey string) (workflowPlayerOverview, error) {
	result := workflowPlayerOverview{
		SummaryVersion: workflowSummaryVersion,
		PlayerKey:      playerKey,
	}

	err := d.db.QueryRowContext(d.ctx, `
		SELECT
			MIN(p.name) AS player_name,
			COUNT(*) AS games_played,
			SUM(CASE WHEN p.is_winner = 1 THEN 1 ELSE 0 END) AS wins,
			AVG(p.apm) AS avg_apm,
			AVG(p.eapm) AS avg_eapm
		FROM players p
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0
	`, playerKey).Scan(
		&result.PlayerName,
		&result.GamesPlayed,
		&result.Wins,
		&result.AverageAPM,
		&result.AverageEAPM,
	)
	if err != nil {
		return result, fmt.Errorf("failed to load player summary: %w", err)
	}
	if result.GamesPlayed == 0 {
		return result, sql.ErrNoRows
	}
	result.WinRate = float64(result.Wins) / float64(result.GamesPlayed)

	raceRows, err := d.db.QueryContext(d.ctx, `
		SELECT p.race, COUNT(*) AS game_count, SUM(CASE WHEN p.is_winner = 1 THEN 1 ELSE 0 END) AS wins
		FROM players p
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0
		GROUP BY p.race
		ORDER BY game_count DESC
	`, playerKey)
	if err != nil {
		return result, fmt.Errorf("failed to load race breakdown: %w", err)
	}
	defer raceRows.Close()
	for raceRows.Next() {
		var race workflowPlayerRaceBreakdown
		if err := raceRows.Scan(&race.Race, &race.GameCount, &race.Wins); err != nil {
			return result, fmt.Errorf("failed to parse race breakdown: %w", err)
		}
		result.RaceBreakdown = append(result.RaceBreakdown, race)
	}

	recentRows, err := d.db.QueryContext(d.ctx, `
		SELECT
			r.id,
			r.replay_date,
			r.file_name,
			r.map_name,
			r.duration_seconds,
			r.game_type,
			COALESCE((
				SELECT group_concat(name, ' vs ')
				FROM (
					SELECT p2.name AS name
					FROM players p2
					WHERE p2.replay_id = r.id AND p2.is_observer = 0
					ORDER BY p2.team ASC, p2.id ASC
				)
			), ''),
			COALESCE((
				SELECT group_concat(p3.name, ', ')
				FROM players p3
				WHERE p3.replay_id = r.id AND p3.is_winner = 1 AND p3.is_observer = 0
			), '')
		FROM replays r
		JOIN players p ON p.replay_id = r.id
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0
		ORDER BY r.replay_date DESC, r.id DESC
		LIMIT 12
	`, playerKey)
	if err != nil {
		return result, fmt.Errorf("failed to load recent games: %w", err)
	}
	defer recentRows.Close()
	for recentRows.Next() {
		var g workflowGameListItem
		if err := recentRows.Scan(
			&g.ReplayID,
			&g.ReplayDate,
			&g.FileName,
			&g.MapName,
			&g.DurationSeconds,
			&g.GameType,
			&g.PlayersLabel,
			&g.WinnersLabel,
		); err != nil {
			return result, fmt.Errorf("failed to parse recent games: %w", err)
		}
		result.RecentGames = append(result.RecentGames, g)
	}

	chatSummary, err := d.buildPlayerChatSummary(playerKey)
	if err != nil {
		return result, fmt.Errorf("failed to build player chat summary: %w", err)
	}
	result.ChatSummary = chatSummary

	result.NarrativeHints = buildPlayerNarrativeHints(result)
	return result, nil
}

func (d *Dashboard) buildWorkflowPlayerMetrics(playerKey string) (workflowPlayerMetrics, error) {
	var gamesPlayed int64
	if err := d.db.QueryRowContext(d.ctx, `
		SELECT COUNT(*)
		FROM players p
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0
	`, playerKey).Scan(&gamesPlayed); err != nil {
		return workflowPlayerMetrics{}, fmt.Errorf("failed to load player games for metrics: %w", err)
	}
	if gamesPlayed <= 0 {
		return workflowPlayerMetrics{}, sql.ErrNoRows
	}
	raceSections, err := d.raceBehaviourSectionsForPlayer(playerKey, gamesPlayed)
	if err != nil {
		return workflowPlayerMetrics{}, err
	}

	tmp := workflowPlayerOverview{
		PlayerKey:   playerKey,
		GamesPlayed: gamesPlayed,
	}
	if err := d.populateAdvancedPlayerOverview(playerKey, &tmp); err != nil {
		return workflowPlayerMetrics{}, err
	}
	return workflowPlayerMetrics{
		SummaryVersion:        workflowSummaryVersion,
		PlayerKey:             playerKey,
		RaceBehaviourSections: raceSections,
		FingerprintMetrics:    tmp.FingerprintMetrics,
	}, nil
}

func (d *Dashboard) raceBehaviourSectionsForPlayer(playerKey string, totalGames int64) ([]workflowRaceBehaviourSection, error) {
	raceRows, err := d.db.QueryContext(d.ctx, `
		SELECT p.race, COUNT(*) AS game_count, SUM(CASE WHEN p.is_winner = 1 THEN 1 ELSE 0 END) AS wins
		FROM players p
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0
		GROUP BY p.race
		ORDER BY game_count DESC, p.race ASC
	`, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load race behaviour sections: %w", err)
	}
	defer raceRows.Close()

	sections := []workflowRaceBehaviourSection{}
	byRace := map[string]*workflowRaceBehaviourSection{}
	for raceRows.Next() {
		var race string
		var gameCount int64
		var wins int64
		if err := raceRows.Scan(&race, &gameCount, &wins); err != nil {
			return nil, fmt.Errorf("failed to parse race behaviour sections: %w", err)
		}
		section := workflowRaceBehaviourSection{
			Race:             strings.TrimSpace(race),
			GameCount:        gameCount,
			GameRate:         0,
			Wins:             wins,
			WinRate:          0,
			CommonBehaviours: []workflowCommonBehaviour{},
		}
		if totalGames > 0 {
			section.GameRate = float64(gameCount) / float64(totalGames)
		}
		if gameCount > 0 {
			section.WinRate = float64(wins) / float64(gameCount)
		}
		sections = append(sections, section)
		byRace[section.Race] = &sections[len(sections)-1]
	}
	if err := raceRows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating race behaviour sections: %w", err)
	}

	patternRows, err := d.db.QueryContext(d.ctx, `
		SELECT p.race, dp.pattern_name, COUNT(DISTINCT dp.replay_id) AS replay_count
		FROM detected_patterns_replay_player dp
		JOIN players p ON p.id = dp.player_id
		WHERE lower(trim(p.name)) = ?
			AND p.is_observer = 0
			AND dp.pattern_name IS NOT NULL
			AND dp.pattern_name <> ''
			AND lower(replace(replace(dp.pattern_name, ' ', ''), '_', '')) <> 'usedhotkeygroups'
			AND (
				dp.value_bool = 1
				OR dp.value_int IS NOT NULL
				OR dp.value_timestamp IS NOT NULL
				OR (
					dp.value_string IS NOT NULL
					AND trim(dp.value_string) <> ''
					AND lower(trim(dp.value_string)) NOT IN ('0', 'false', 'no', '-')
				)
			)
		GROUP BY p.race, dp.pattern_name
	`, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load race common behaviours: %w", err)
	}
	defer patternRows.Close()
	for patternRows.Next() {
		var race string
		var patternName string
		var replayCount int64
		if err := patternRows.Scan(&race, &patternName, &replayCount); err != nil {
			return nil, fmt.Errorf("failed to parse race common behaviours: %w", err)
		}
		raceKey := strings.TrimSpace(race)
		section, ok := byRace[raceKey]
		if !ok || section.GameCount <= 0 {
			continue
		}
		gameRate := float64(replayCount) / float64(section.GameCount)
		if gameRate < 0.2 {
			continue
		}
		section.CommonBehaviours = append(section.CommonBehaviours, workflowCommonBehaviour{
			Name:        patternName,
			PrettyName:  prettySplitUppercase(patternName),
			ReplayCount: replayCount,
			GameRate:    gameRate,
		})
	}
	if err := patternRows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating race common behaviours: %w", err)
	}

	for i := range sections {
		sort.Slice(sections[i].CommonBehaviours, func(a, b int) bool {
			if sections[i].CommonBehaviours[a].ReplayCount == sections[i].CommonBehaviours[b].ReplayCount {
				return sections[i].CommonBehaviours[a].Name < sections[i].CommonBehaviours[b].Name
			}
			return sections[i].CommonBehaviours[a].ReplayCount > sections[i].CommonBehaviours[b].ReplayCount
		})
		if len(sections[i].CommonBehaviours) > 12 {
			sections[i].CommonBehaviours = sections[i].CommonBehaviours[:12]
		}
	}
	return sections, nil
}

func (d *Dashboard) topActionTypesForPlayer(playerID int64, limit int) ([]string, error) {
	rows, err := d.db.QueryContext(d.ctx, `
		SELECT c.action_type, COUNT(*) AS n
		FROM commands c
		WHERE c.player_id = ?
		GROUP BY c.action_type
		ORDER BY n DESC
		LIMIT ?
	`, playerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var actionType string
		var n int64
		if err := rows.Scan(&actionType, &n); err != nil {
			return nil, err
		}
		out = append(out, actionType)
	}
	return out, rows.Err()
}

func parseGameEvents(raw string) []workflowGameEvent {
	events := []workflowGameEvent{}
	if strings.TrimSpace(raw) == "" {
		return events
	}
	if err := json.Unmarshal([]byte(raw), &events); err != nil {
		return events
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Second == events[j].Second {
			return events[i].Description < events[j].Description
		}
		return events[i].Second < events[j].Second
	})
	return events
}

func formatPatternValueForUI(patternName, value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return "-"
	}
	if strings.EqualFold(v, "true") {
		return "Yes"
	}
	if strings.EqualFold(v, "false") {
		return "No"
	}
	lowerName := strings.ToLower(strings.TrimSpace(patternName))
	if strings.Contains(lowerName, "second") || strings.Contains(lowerName, "fast expa") || strings.Contains(lowerName, "quick factory") {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return formatClockFromSeconds(n)
		}
	}
	return v
}

func formatClockFromSeconds(second int64) string {
	if second < 0 {
		second = 0
	}
	minute := second / 60
	sec := second % 60
	return fmt.Sprintf("%d:%02d", minute, sec)
}

func workflowSliceBoundaries(durationSeconds int64) []int64 {
	base := []int64{0, 145, 300, 360, 420, 600, 900, 1200, 1500, 1800, 2400, 3000, 3600}
	boundaries := []int64{0}
	for _, point := range base {
		if point <= 0 {
			continue
		}
		if point > durationSeconds {
			break
		}
		boundaries = append(boundaries, point)
	}
	for next := int64(4200); next <= durationSeconds; next += 600 {
		boundaries = append(boundaries, next)
	}
	return boundaries
}

func sliceStartForSecond(second int64, boundaries []int64) int64 {
	if len(boundaries) == 0 {
		return 0
	}
	idx := sort.Search(len(boundaries), func(i int) bool { return boundaries[i] > second }) - 1
	if idx < 0 {
		return boundaries[0]
	}
	return boundaries[idx]
}

func formatWorkflowSliceLabel(start, endExclusive int64) string {
	if endExclusive <= start {
		return fmt.Sprintf("%s-%s", formatClockFromSeconds(start), formatClockFromSeconds(start))
	}
	return fmt.Sprintf("%s-%s", formatClockFromSeconds(start), formatClockFromSeconds(endExclusive-1))
}

func (d *Dashboard) populateUnitsBySliceForGameDetail(detail *workflowGameDetail) error {
	detail.UnitsBySlice = []workflowUnitSlice{}
	playerOrder := make([]int64, 0, len(detail.Players))
	playerByID := map[int64]workflowGamePlayer{}
	for _, player := range detail.Players {
		playerOrder = append(playerOrder, player.PlayerID)
		playerByID[player.PlayerID] = player
	}

	rows, err := d.db.QueryContext(d.ctx, `
		SELECT c.player_id, c.seconds_from_game_start, c.unit_type
		FROM commands c
		WHERE c.replay_id = ?
			AND c.action_type IN ('Train', 'Unit Morph', 'Building Morph', 'Build')
			AND c.unit_type IS NOT NULL
			AND c.unit_type <> ''
		ORDER BY c.seconds_from_game_start ASC, c.player_id ASC
	`, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load unit slices: %w", err)
	}
	defer rows.Close()

	perSlice := map[int64]map[int64]map[string]int64{}
	boundaries := workflowSliceBoundaries(detail.DurationSeconds)
	for rows.Next() {
		var playerID int64
		var second int64
		var unitType string
		if err := rows.Scan(&playerID, &second, &unitType); err != nil {
			return fmt.Errorf("failed to parse unit slices: %w", err)
		}
		if second < 0 {
			second = 0
		}
		sliceStart := sliceStartForSecond(second, boundaries)
		if _, ok := perSlice[sliceStart]; !ok {
			perSlice[sliceStart] = map[int64]map[string]int64{}
		}
		if _, ok := perSlice[sliceStart][playerID]; !ok {
			perSlice[sliceStart][playerID] = map[string]int64{}
		}
		perSlice[sliceStart][playerID][unitType]++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed iterating unit slices: %w", err)
	}

	for i, sliceStart := range boundaries {
		endExclusive := detail.DurationSeconds + 1
		if i+1 < len(boundaries) {
			endExclusive = boundaries[i+1]
		}
		slice := workflowUnitSlice{
			SliceStartSecond: sliceStart,
			SliceLabel:       formatWorkflowSliceLabel(sliceStart, endExclusive),
			Players:          []workflowUnitSlicePlayer{},
		}
		for _, playerID := range playerOrder {
			player := playerByID[playerID]
			unitCounts := []workflowUnitCount{}
			if byUnit, ok := perSlice[sliceStart][playerID]; ok {
				for unitType, count := range byUnit {
					unitCounts = append(unitCounts, workflowUnitCount{UnitType: unitType, Count: count})
				}
			}
			sort.Slice(unitCounts, func(i, j int) bool {
				if unitCounts[i].Count == unitCounts[j].Count {
					return unitCounts[i].UnitType < unitCounts[j].UnitType
				}
				return unitCounts[i].Count > unitCounts[j].Count
			})
			slice.Players = append(slice.Players, workflowUnitSlicePlayer{
				PlayerID:  player.PlayerID,
				PlayerKey: player.PlayerKey,
				Name:      player.Name,
				Units:     unitCounts,
			})
		}
		detail.UnitsBySlice = append(detail.UnitsBySlice, slice)
	}
	return nil
}

func (d *Dashboard) populateTimingsForGameDetail(detail *workflowGameDetail) error {
	timings := workflowReplayTimings{}
	gas, err := d.playerTimingsFromReplayCommands(detail.ReplayID, detail.Players, `
		SELECT c.player_id, c.seconds_from_game_start, c.unit_type
		FROM commands c
		WHERE c.replay_id = ?
			AND c.action_type = 'Build'
			AND c.unit_type IN ('Assimilator', 'Extractor', 'Refinery')
		ORDER BY c.player_id ASC, c.seconds_from_game_start ASC
	`)
	if err != nil {
		return err
	}
	for i := range gas {
		if len(gas[i].Points) > 4 {
			gas[i].Points = gas[i].Points[:4]
		}
	}
	timings.Gas = gas
	timings.Expansion = playerExpansionTimingsFromGameEvents(detail.GameEvents, detail.Players)

	upgrades, err := d.playerLabeledTimingsFromReplayCommands(detail.ReplayID, detail.Players, `
		SELECT c.player_id, c.seconds_from_game_start, c.upgrade_name
		FROM commands c
		WHERE c.replay_id = ?
			AND c.action_type = 'Upgrade'
			AND c.upgrade_name IS NOT NULL
			AND c.upgrade_name <> ''
		ORDER BY c.player_id ASC, c.seconds_from_game_start ASC
	`)
	if err != nil {
		return err
	}
	timings.Upgrades = upgrades

	tech, err := d.playerLabeledTimingsFromReplayCommands(detail.ReplayID, detail.Players, `
		SELECT c.player_id, c.seconds_from_game_start, c.tech_name
		FROM commands c
		WHERE c.replay_id = ?
			AND c.action_type = 'Tech'
			AND c.tech_name IS NOT NULL
			AND c.tech_name <> ''
		ORDER BY c.player_id ASC, c.seconds_from_game_start ASC
	`)
	if err != nil {
		return err
	}
	timings.Tech = tech
	detail.Timings = timings
	return nil
}

func (d *Dashboard) populateFirstUnitEfficiencyForGameDetail(detail *workflowGameDetail) error {
	detail.FirstUnitEfficiency = []workflowFirstUnitEfficiencyPlayer{}
	if len(detail.Players) == 0 {
		return nil
	}

	type playerEfficiencyState struct {
		buildTimesByUnit map[string][]int64
		unitTimesByUnit  map[string][]int64
	}

	stateByPlayer := map[int64]*playerEfficiencyState{}
	for _, player := range detail.Players {
		stateByPlayer[player.PlayerID] = &playerEfficiencyState{
			buildTimesByUnit: map[string][]int64{},
			unitTimesByUnit:  map[string][]int64{},
		}
	}

	rows, err := d.db.QueryContext(d.ctx, `
		SELECT c.player_id, c.seconds_from_game_start, c.action_type, c.unit_type, c.unit_types
		FROM commands c
		WHERE c.replay_id = ?
			AND c.action_type IN ('Build', 'Train', 'Unit Morph')
		ORDER BY c.player_id ASC, c.seconds_from_game_start ASC, c.id ASC
	`, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load first unit efficiency commands: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var playerID int64
		var second int64
		var actionType string
		var unitType sql.NullString
		var unitTypes sql.NullString
		if err := rows.Scan(&playerID, &second, &actionType, &unitType, &unitTypes); err != nil {
			return fmt.Errorf("failed to parse first unit efficiency command row: %w", err)
		}
		playerState, ok := stateByPlayer[playerID]
		if !ok {
			continue
		}
		commandUnits := parseCommandUnitNames(unitType, unitTypes)
		if len(commandUnits) == 0 {
			continue
		}
		for _, name := range commandUnits {
			aliases := unitNameAliases(name)
			if len(aliases) == 0 {
				continue
			}
			if actionType == "Build" {
				for _, alias := range aliases {
					playerState.buildTimesByUnit[alias] = append(playerState.buildTimesByUnit[alias], second)
				}
				continue
			}
			for _, alias := range aliases {
				playerState.unitTimesByUnit[alias] = append(playerState.unitTimesByUnit[alias], second)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed iterating first unit efficiency commands: %w", err)
	}

	for _, player := range detail.Players {
		playerState, ok := stateByPlayer[player.PlayerID]
		if !ok {
			continue
		}
		playerRace := strings.ToLower(strings.TrimSpace(player.Race))
		entries := []workflowFirstUnitEfficiencyEntry{}
		for _, cfg := range firstUnitEfficiencyConfigs {
			if cfg.Race != playerRace {
				continue
			}
			buildingKey := normalizeUnitName(cfg.BuildingName)
			buildStarts := playerState.buildTimesByUnit[buildingKey]
			if len(buildStarts) == 0 {
				continue
			}
			buildingStartSecond := buildStarts[0]
			buildingReadySecond := buildingStartSecond + cfg.BuildDurationSeconds
			bestUnitSecond := int64(-1)
			bestUnitName := ""
			for _, unitOption := range cfg.Units {
				for _, matchKeyRaw := range unitOption.MatchKeys {
					matchKey := normalizeUnitName(matchKeyRaw)
					timings := playerState.unitTimesByUnit[matchKey]
					if len(timings) == 0 {
						continue
					}
					idx := sort.Search(len(timings), func(i int) bool {
						return timings[i] >= buildingReadySecond
					})
					if idx >= len(timings) {
						continue
					}
					candidateSecond := timings[idx]
					if bestUnitSecond < 0 || candidateSecond < bestUnitSecond {
						bestUnitSecond = candidateSecond
						bestUnitName = unitOption.DisplayName
					}
				}
			}
			if bestUnitSecond < 0 {
				continue
			}
			gapAfterReadySeconds := bestUnitSecond - buildingReadySecond
			if gapAfterReadySeconds < 0 || gapAfterReadySeconds > firstUnitEfficiencyMaxGapSeconds {
				continue
			}
			entries = append(entries, workflowFirstUnitEfficiencyEntry{
				BuildingName:         cfg.BuildingName,
				UnitName:             bestUnitName,
				BuildingStartSecond:  buildingStartSecond,
				BuildingReadySecond:  buildingReadySecond,
				UnitSecond:           bestUnitSecond,
				BuildDurationSeconds: cfg.BuildDurationSeconds,
				GapAfterReadySeconds: gapAfterReadySeconds,
			})
		}
		if len(entries) == 0 {
			continue
		}
		detail.FirstUnitEfficiency = append(detail.FirstUnitEfficiency, workflowFirstUnitEfficiencyPlayer{
			PlayerID:  player.PlayerID,
			PlayerKey: player.PlayerKey,
			Name:      player.Name,
			Race:      player.Race,
			Entries:   entries,
		})
	}
	return nil
}

func parseCommandUnitNames(unitType sql.NullString, unitTypes sql.NullString) []string {
	unique := map[string]struct{}{}
	names := []string{}
	appendName := func(raw string) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return
		}
		key := normalizeUnitName(trimmed)
		if key == "" {
			return
		}
		if _, exists := unique[key]; exists {
			return
		}
		unique[key] = struct{}{}
		names = append(names, trimmed)
	}

	if unitType.Valid {
		appendName(unitType.String)
	}
	if unitTypes.Valid {
		list := []string{}
		if err := json.Unmarshal([]byte(unitTypes.String), &list); err == nil {
			for _, item := range list {
				appendName(item)
			}
		}
	}
	return names
}

func unitNameAliases(name string) []string {
	base := normalizeUnitName(name)
	if base == "" {
		return nil
	}
	aliases := map[string]struct{}{
		base: {},
	}
	for _, prefix := range []string{"terran", "zerg", "protoss"} {
		if strings.HasPrefix(base, prefix) && len(base) > len(prefix) {
			aliases[strings.TrimPrefix(base, prefix)] = struct{}{}
		}
	}
	out := make([]string, 0, len(aliases))
	for key := range aliases {
		out = append(out, key)
	}
	return out
}

func normalizeUnitName(value string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	if raw == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (d *Dashboard) playerTimingsFromReplayCommands(replayID int64, players []workflowGamePlayer, query string) ([]workflowPlayerTimingSeries, error) {
	seriesByPlayer, playerOrder := initPlayerTimingSeries(players)
	rows, err := d.db.QueryContext(d.ctx, query, replayID)
	if err != nil {
		return nil, fmt.Errorf("failed to load replay timings: %w", err)
	}
	defer rows.Close()
	orderByPlayer := map[int64]int64{}
	for rows.Next() {
		var playerID int64
		var second int64
		var ignoredLabel string
		if err := rows.Scan(&playerID, &second, &ignoredLabel); err != nil {
			return nil, fmt.Errorf("failed to parse replay timings: %w", err)
		}
		current := orderByPlayer[playerID] + 1
		orderByPlayer[playerID] = current
		if s, ok := seriesByPlayer[playerID]; ok {
			s.Points = append(s.Points, workflowTimingPoint{Second: second, Order: current})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating replay timings: %w", err)
	}
	return orderedTimingSeries(seriesByPlayer, playerOrder), nil
}

func (d *Dashboard) playerLabeledTimingsFromReplayCommands(replayID int64, players []workflowGamePlayer, query string) ([]workflowPlayerTimingSeries, error) {
	seriesByPlayer, playerOrder := initPlayerTimingSeries(players)
	rows, err := d.db.QueryContext(d.ctx, query, replayID)
	if err != nil {
		return nil, fmt.Errorf("failed to load labeled replay timings: %w", err)
	}
	defer rows.Close()
	orderByPlayerAndLabel := map[int64]map[string]int64{}
	for rows.Next() {
		var playerID int64
		var second int64
		var label string
		if err := rows.Scan(&playerID, &second, &label); err != nil {
			return nil, fmt.Errorf("failed to parse labeled replay timings: %w", err)
		}
		if _, ok := orderByPlayerAndLabel[playerID]; !ok {
			orderByPlayerAndLabel[playerID] = map[string]int64{}
		}
		current := orderByPlayerAndLabel[playerID][label] + 1
		orderByPlayerAndLabel[playerID][label] = current
		if s, ok := seriesByPlayer[playerID]; ok {
			s.Points = append(s.Points, workflowTimingPoint{Second: second, Order: current, Label: label})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating labeled replay timings: %w", err)
	}
	return orderedTimingSeries(seriesByPlayer, playerOrder), nil
}

func playerExpansionTimingsFromGameEvents(events []workflowGameEvent, players []workflowGamePlayer) []workflowPlayerTimingSeries {
	seriesByPlayer, playerOrder := initPlayerTimingSeries(players)
	playersSorted := make([]workflowGamePlayer, len(players))
	copy(playersSorted, players)
	sort.Slice(playersSorted, func(i, j int) bool {
		return len(playersSorted[i].Name) > len(playersSorted[j].Name)
	})
	orderByPlayer := map[int64]int64{}
	for _, event := range events {
		typeLower := strings.ToLower(event.Type)
		if typeLower != "expansion" {
			continue
		}
		playerID := matchPlayerIDInEvent(event.Description, playersSorted)
		if playerID == 0 {
			continue
		}
		current := orderByPlayer[playerID] + 1
		orderByPlayer[playerID] = current
		if current > 4 {
			continue
		}
		if s, ok := seriesByPlayer[playerID]; ok {
			s.Points = append(s.Points, workflowTimingPoint{Second: event.Second, Order: current})
		}
	}
	return orderedTimingSeries(seriesByPlayer, playerOrder)
}

func matchPlayerIDInEvent(description string, players []workflowGamePlayer) int64 {
	desc := strings.ToLower(strings.TrimSpace(description))
	if desc == "" {
		return 0
	}
	for _, player := range players {
		nameLower := strings.ToLower(strings.TrimSpace(player.Name))
		if nameLower == "" {
			continue
		}
		if strings.HasPrefix(desc, nameLower+" ") || strings.HasPrefix(desc, nameLower) {
			return player.PlayerID
		}
	}
	return 0
}

func initPlayerTimingSeries(players []workflowGamePlayer) (map[int64]*workflowPlayerTimingSeries, []int64) {
	seriesByPlayer := map[int64]*workflowPlayerTimingSeries{}
	playerOrder := make([]int64, 0, len(players))
	for _, player := range players {
		playerOrder = append(playerOrder, player.PlayerID)
		seriesByPlayer[player.PlayerID] = &workflowPlayerTimingSeries{
			PlayerID:  player.PlayerID,
			PlayerKey: player.PlayerKey,
			Name:      player.Name,
			Points:    []workflowTimingPoint{},
		}
	}
	return seriesByPlayer, playerOrder
}

func orderedTimingSeries(seriesByPlayer map[int64]*workflowPlayerTimingSeries, playerOrder []int64) []workflowPlayerTimingSeries {
	out := make([]workflowPlayerTimingSeries, 0, len(playerOrder))
	for _, playerID := range playerOrder {
		if s, ok := seriesByPlayer[playerID]; ok {
			sort.Slice(s.Points, func(i, j int) bool {
				if s.Points[i].Second == s.Points[j].Second {
					if s.Points[i].Label == s.Points[j].Label {
						return s.Points[i].Order < s.Points[j].Order
					}
					return s.Points[i].Label < s.Points[j].Label
				}
				return s.Points[i].Second < s.Points[j].Second
			})
			out = append(out, *s)
		}
	}
	return out
}

func (d *Dashboard) populateAdvancedPlayerOverview(playerKey string, result *workflowPlayerOverview) error {
	commonBehaviours, err := d.commonBehavioursForPlayer(playerKey, result.GamesPlayed)
	if err != nil {
		return err
	}
	result.CommonBehaviours = commonBehaviours

	hotkeyGamesRate, err := d.simpleMetricByPlayer(`
		WITH game_level AS (
			SELECT
				lower(trim(p.name)) AS player_key,
				CASE WHEN SUM(CASE WHEN c.action_type = 'Hotkey' AND c.hotkey_type IS NOT NULL THEN 1 ELSE 0 END) > 0 THEN 100.0 ELSE 0.0 END AS metric_value
			FROM players p
			LEFT JOIN commands_low_value c ON c.player_id = p.id
			WHERE p.is_observer = 0
			GROUP BY p.id
		)
		SELECT player_key, AVG(metric_value) AS metric_value
		FROM game_level
		GROUP BY player_key
	`)
	if err != nil {
		return err
	}
	assignGroupReuseRate, err := d.simpleMetricByPlayer(`
		WITH game_level AS (
			SELECT
				lower(trim(p.name)) AS player_key,
				CASE
					WHEN SUM(CASE WHEN c.action_type = 'Hotkey' AND c.hotkey_type = 'Select' THEN 1 ELSE 0 END) > 0 THEN
						100.0 * CAST(SUM(CASE WHEN c.action_type = 'Hotkey' AND c.hotkey_type = 'Assign' THEN 1 ELSE 0 END) AS FLOAT)
						/
						CAST(SUM(CASE WHEN c.action_type = 'Hotkey' AND c.hotkey_type = 'Select' THEN 1 ELSE 0 END) AS FLOAT)
					ELSE 0.0
				END AS metric_value
			FROM players p
			LEFT JOIN commands_low_value c ON c.player_id = p.id
			WHERE p.is_observer = 0
			GROUP BY p.id
		)
		SELECT player_key, AVG(metric_value) AS metric_value
		FROM game_level
		GROUP BY player_key
	`)
	if err != nil {
		return err
	}
	hotkeyCommandsPct, err := d.simpleMetricByPlayer(`
		WITH high_value_counts AS (
			SELECT
				p.id AS player_id,
				lower(trim(p.name)) AS player_key,
				COUNT(c.id) AS n
			FROM players p
			LEFT JOIN commands c ON c.player_id = p.id
			WHERE p.is_observer = 0
			GROUP BY p.id
		),
		low_value_counts AS (
			SELECT
				p.id AS player_id,
				COUNT(clv.id) AS total_low_value,
				SUM(CASE WHEN clv.action_type = 'Hotkey' AND clv.hotkey_type IS NOT NULL THEN 1 ELSE 0 END) AS hotkey_count
			FROM players p
			LEFT JOIN commands_low_value clv ON clv.player_id = p.id
			WHERE p.is_observer = 0
			GROUP BY p.id
		),
		game_level AS (
			SELECT
				hvc.player_key,
				CASE
					WHEN hvc.n + COALESCE(lvc.total_low_value, 0) > 0 THEN
						100.0 * CAST(COALESCE(lvc.hotkey_count, 0) AS FLOAT) / CAST((hvc.n + COALESCE(lvc.total_low_value, 0)) AS FLOAT)
					ELSE 0.0
				END AS metric_value
			FROM high_value_counts hvc
			LEFT JOIN low_value_counts lvc ON lvc.player_id = hvc.player_id
		)
		SELECT player_key, AVG(metric_value) AS metric_value
		FROM game_level
		GROUP BY player_key
	`)
	if err != nil {
		return err
	}

	rallyReplayRate, err := d.simpleMetricByPlayer(`
		WITH player_replays AS (
			SELECT DISTINCT
				lower(trim(p.name)) AS player_key,
				c.replay_id
			FROM players p
			JOIN commands c ON c.player_id = p.id
			WHERE p.is_observer = 0
		),
		rally_replays AS (
			SELECT DISTINCT
				lower(trim(p.name)) AS player_key,
				c.replay_id
			FROM players p
			JOIN commands c ON c.player_id = p.id
			WHERE p.is_observer = 0
				AND c.action_type = 'Targeted Order'
				AND c.order_name LIKE 'RallyPoint%'
		)
		SELECT
			pr.player_key,
			100.0 * CAST(COUNT(DISTINCT rr.replay_id) AS FLOAT) / CAST(COUNT(DISTINCT pr.replay_id) AS FLOAT) AS metric_value
		FROM player_replays pr
		LEFT JOIN rally_replays rr
			ON rr.player_key = pr.player_key
			AND rr.replay_id = pr.replay_id
		GROUP BY pr.player_key
	`)
	if err != nil {
		return err
	}

	rallyPer10MinOnRallyReplays, err := d.simpleMetricByPlayer(`
		WITH rally_counts AS (
			SELECT
				lower(trim(p.name)) AS player_key,
				c.replay_id,
				COUNT(*) AS rally_count
			FROM players p
			JOIN commands c ON c.player_id = p.id
			WHERE p.is_observer = 0
				AND c.action_type = 'Targeted Order'
				AND c.order_name LIKE 'RallyPoint%'
			GROUP BY lower(trim(p.name)), c.replay_id
		)
		SELECT
			rc.player_key,
			AVG((CAST(rc.rally_count AS FLOAT) * 600.0) / CAST(r.duration_seconds AS FLOAT)) AS metric_value
		FROM rally_counts rc
		JOIN replays r ON r.id = rc.replay_id
		WHERE r.duration_seconds > 0
		GROUP BY rc.player_key
	`)
	if err != nil {
		return err
	}

	actionDiversity, err := d.simpleMetricByPlayer(`
		WITH game_level AS (
			SELECT
				lower(trim(p.name)) AS player_key,
				COUNT(DISTINCT c.action_type) AS action_count,
				COUNT(DISTINCT CASE WHEN c.action_type = 'Targeted Order' AND c.order_name IS NOT NULL AND c.order_name <> '' THEN c.order_name END) AS targeted_order_count
			FROM players p
			LEFT JOIN commands c ON c.player_id = p.id
			WHERE p.is_observer = 0
			GROUP BY p.id
		)
		SELECT
			player_key,
			AVG(
				CAST(action_count AS FLOAT)
				+ CASE WHEN targeted_order_count > 0 THEN CAST(targeted_order_count - 1 AS FLOAT) ELSE 0.0 END
			) AS metric_value
		FROM game_level
		GROUP BY player_key
	`)
	if err != nil {
		return err
	}

	queuedGames, err := d.countQueuedGamesForPlayer(playerKey)
	if err != nil {
		return err
	}
	result.QueuedGames = queuedGames
	if result.GamesPlayed > 0 {
		result.QueuedGameRate = float64(queuedGames) / float64(result.GamesPlayed)
	}

	result.FingerprintMetrics = []workflowComparativeMetric{
		buildComparativeMetric("Games using hotkeys (%)", playerKey, hotkeyGamesRate),
		{Metric: "Games with queued orders (%)", PlayerValue: result.QueuedGameRate * 100.0},
		buildComparativeMetric("Assigned hotkey / Used Hotkey actions", playerKey, assignGroupReuseRate),
		buildComparativeMetric("Hotkey commands as % of total", playerKey, hotkeyCommandsPct),
		buildComparativeMetric("Replays with at least one Rally Point (%)", playerKey, rallyReplayRate),
		buildComparativeMetric("Rally Points per 10 minutes (rally replays)", playerKey, rallyPer10MinOnRallyReplays),
		buildComparativeMetric("Action type diversity", playerKey, actionDiversity),
	}

	return nil
}

func (d *Dashboard) commonBehavioursForPlayer(playerKey string, gamesPlayed int64) ([]workflowCommonBehaviour, error) {
	if gamesPlayed <= 0 {
		return []workflowCommonBehaviour{}, nil
	}
	rows, err := d.db.QueryContext(d.ctx, `
		SELECT dp.pattern_name, COUNT(DISTINCT dp.replay_id) AS replay_count
		FROM detected_patterns_replay_player dp
		JOIN players p ON p.id = dp.player_id
		WHERE lower(trim(p.name)) = ?
			AND p.is_observer = 0
			AND dp.pattern_name IS NOT NULL
			AND dp.pattern_name <> ''
			AND lower(replace(replace(dp.pattern_name, ' ', ''), '_', '')) <> 'usedhotkeygroups'
			AND (
				dp.value_bool = 1
				OR dp.value_int IS NOT NULL
				OR dp.value_timestamp IS NOT NULL
				OR (
					dp.value_string IS NOT NULL
					AND trim(dp.value_string) <> ''
					AND lower(trim(dp.value_string)) NOT IN ('0', 'false', 'no', '-')
				)
			)
		GROUP BY dp.pattern_name
	`, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load common behaviours: %w", err)
	}
	defer rows.Close()
	out := []workflowCommonBehaviour{}
	for rows.Next() {
		var patternName string
		var replayCount int64
		if err := rows.Scan(&patternName, &replayCount); err != nil {
			return nil, fmt.Errorf("failed to parse common behaviours: %w", err)
		}
		gameRate := float64(replayCount) / float64(gamesPlayed)
		if gameRate < 0.2 {
			continue
		}
		out = append(out, workflowCommonBehaviour{
			Name:        patternName,
			PrettyName:  prettySplitUppercase(patternName),
			ReplayCount: replayCount,
			GameRate:    gameRate,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating common behaviours: %w", err)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ReplayCount == out[j].ReplayCount {
			return out[i].Name < out[j].Name
		}
		return out[i].ReplayCount > out[j].ReplayCount
	})
	if len(out) > 24 {
		out = out[:24]
	}
	return out, nil
}

const (
	workflowOutlierTFIDFMin = 1.40
	workflowOutlierRatioMin = 3.50
)

var workflowProtossAllowedTechs = map[string]struct{}{
	"psionicstorm":   {},
	"hallucination":  {},
	"recall":         {},
	"stasisfield":    {},
	"archonwarp":     {},
	"disruptionweb":  {},
	"mindcontrol":    {},
	"darkarchonmeld": {},
	"feedback":       {},
	"maelstrom":      {},
}

var workflowProtossAllowedUpgrades = map[string]struct{}{
	"protossgroundarmor":            {},
	"protossairarmor":               {},
	"protossgroundweapons":          {},
	"protossairweapons":             {},
	"protossplasmashields":          {},
	"singularitychargedragoonrange": {},
	"legenhancementzealotspeed":     {},
	"scarabdamage":                  {},
	"reavercapacity":                {},
	"graviticdriveshuttlespeed":     {},
	"sensorarrayobserversight":      {},
	"graviticboosterobserverspeed":  {},
	"khaydarinamulettemplarenergy":  {},
	"apialsensorsscoutsight":        {},
	"graviticthrustersscoutspeed":   {},
	"carriercapacity":               {},
	"khaydarincorearbiterenergy":    {},
	"argusjewelcorsairenergy":       {},
	"argustalismandarkarchonenergy": {},
}

var workflowProtossAllowedCastOrders = map[string]struct{}{
	"castpsionicstorm":  {},
	"casthallucination": {},
	"castrecall":        {},
	"caststasisfield":   {},
	"castdisruptionweb": {},
	"castmindcontrol":   {},
	"castfeedback":      {},
	"castmaelstrom":     {},
}

type workflowOutlierCategorySpec struct {
	CategoryLabel string
	ActionType    string
	NameColumn    string
}

func (d *Dashboard) buildWorkflowPlayerOutliers(playerKey string) (workflowPlayerOutliers, error) {
	result := workflowPlayerOutliers{
		SummaryVersion: workflowSummaryVersion,
		PlayerKey:      playerKey,
		Thresholds: workflowOutlierThresholds{
			TFIDFMin: workflowOutlierTFIDFMin,
			RatioMin: workflowOutlierRatioMin,
		},
		Items: []workflowPlayerOutlier{},
	}
	var playerName sql.NullString
	var gamesPlayed int64
	if err := d.db.QueryRowContext(d.ctx, `
		SELECT MIN(p.name), COUNT(*)
		FROM players p
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND p.type = 'Human'
	`, playerKey).Scan(&playerName, &gamesPlayed); err != nil {
		return result, fmt.Errorf("failed to load player for outliers: %w", err)
	}
	if gamesPlayed <= 0 || !playerName.Valid || strings.TrimSpace(playerName.String) == "" {
		return result, sql.ErrNoRows
	}
	result.PlayerName = playerName.String

	playerGamesByRace, err := d.playerGamesByRace(playerKey)
	if err != nil {
		return result, err
	}
	if len(playerGamesByRace) == 0 {
		return result, sql.ErrNoRows
	}
	primaryRace := ""
	primaryGames := int64(0)
	for race, games := range playerGamesByRace {
		if games > primaryGames {
			primaryRace = race
			primaryGames = games
		}
	}
	popGamesByRace, err := d.populationGamesByRace()
	if err != nil {
		return result, err
	}
	popDistinctPlayersByRace, err := d.populationDistinctPlayersByRace()
	if err != nil {
		return result, err
	}

	specs := []workflowOutlierCategorySpec{
		{CategoryLabel: "Targeted orders", ActionType: "Targeted Order", NameColumn: "order_name"},
		{CategoryLabel: "Tech researched", ActionType: "Tech", NameColumn: "tech_name"},
		{CategoryLabel: "Upgrades researched", ActionType: "Upgrade", NameColumn: "upgrade_name"},
	}
	all := []workflowPlayerOutlier{}
	for _, spec := range specs {
		items, err := d.outliersForCategory(playerKey, primaryRace, spec, playerGamesByRace, popGamesByRace, popDistinctPlayersByRace, result.Thresholds)
		if err != nil {
			return result, err
		}
		all = append(all, items...)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].TFIDF == all[j].TFIDF {
			return all[i].RatioToBaseline > all[j].RatioToBaseline
		}
		return all[i].TFIDF > all[j].TFIDF
	})
	if len(all) > 30 {
		all = all[:30]
	}
	result.Items = all
	return result, nil
}

func (d *Dashboard) playerGamesByRace(playerKey string) (map[string]int64, error) {
	rows, err := d.db.QueryContext(d.ctx, `
		SELECT p.race, COUNT(*) AS games
		FROM players p
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND p.type = 'Human'
		GROUP BY p.race
	`, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load player games by race: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var race string
		var games int64
		if err := rows.Scan(&race, &games); err != nil {
			return nil, fmt.Errorf("failed to parse player games by race: %w", err)
		}
		out[strings.TrimSpace(race)] = games
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating player games by race: %w", err)
	}
	return out, nil
}

func (d *Dashboard) populationGamesByRace() (map[string]int64, error) {
	rows, err := d.db.QueryContext(d.ctx, `
		SELECT p.race, COUNT(*) AS games
		FROM players p
		WHERE p.is_observer = 0 AND p.type = 'Human'
		GROUP BY p.race
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to load population games by race: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var race string
		var games int64
		if err := rows.Scan(&race, &games); err != nil {
			return nil, fmt.Errorf("failed to parse population games by race: %w", err)
		}
		out[strings.TrimSpace(race)] = games
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating population games by race: %w", err)
	}
	return out, nil
}

func (d *Dashboard) populationDistinctPlayersByRace() (map[string]float64, error) {
	rows, err := d.db.QueryContext(d.ctx, `
		SELECT p.race, CAST(COUNT(*) AS FLOAT)
		FROM (
			SELECT lower(trim(name)) AS player_key, race
			FROM players
			WHERE is_observer = 0 AND type = 'Human'
			GROUP BY lower(trim(name)), race
		) p
		GROUP BY p.race
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to load distinct players by race: %w", err)
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var race string
		var players float64
		if err := rows.Scan(&race, &players); err != nil {
			return nil, fmt.Errorf("failed to parse distinct players by race: %w", err)
		}
		out[strings.TrimSpace(race)] = players
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating distinct players by race: %w", err)
	}
	return out, nil
}

func (d *Dashboard) outliersForCategory(
	playerKey string,
	primaryRace string,
	spec workflowOutlierCategorySpec,
	playerGamesByRace map[string]int64,
	popGamesByRace map[string]int64,
	popDistinctPlayersByRace map[string]float64,
	thresholds workflowOutlierThresholds,
) ([]workflowPlayerOutlier, error) {
	playerQuery := fmt.Sprintf(`
		SELECT p.race, c.%s AS item_name,
			CASE
				WHEN ? = 'Targeted Order' THEN COUNT(c.id)
				ELSE COUNT(DISTINCT p.id)
			END AS player_games
		FROM players p
		JOIN commands c ON c.player_id = p.id
		WHERE lower(trim(p.name)) = ?
			AND p.is_observer = 0
			AND p.type = 'Human'
			AND c.action_type = ?
			AND c.%s IS NOT NULL
			AND c.%s <> ''
		GROUP BY p.race, c.%s
	`, spec.NameColumn, spec.NameColumn, spec.NameColumn, spec.NameColumn)
	playerRows, err := d.db.QueryContext(d.ctx, playerQuery, spec.ActionType, playerKey, spec.ActionType)
	if err != nil {
		return nil, fmt.Errorf("failed to query player outliers for %s: %w", spec.CategoryLabel, err)
	}
	defer playerRows.Close()

	type pair struct {
		race string
		name string
	}
	playerCounts := map[pair]int64{}
	for playerRows.Next() {
		var race string
		var name string
		var games int64
		if err := playerRows.Scan(&race, &name, &games); err != nil {
			return nil, fmt.Errorf("failed to parse player outliers for %s: %w", spec.CategoryLabel, err)
		}
		playerCounts[pair{race: strings.TrimSpace(race), name: strings.TrimSpace(name)}] = games
	}
	if err := playerRows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating player outliers for %s: %w", spec.CategoryLabel, err)
	}

	globalQuery := fmt.Sprintf(`
		SELECT
			p.race,
			c.%s AS item_name,
			CASE
				WHEN ? = 'Targeted Order' THEN COUNT(c.id)
				ELSE COUNT(DISTINCT p.id)
			END AS global_games,
			COUNT(DISTINCT lower(trim(p.name))) AS global_players
		FROM players p
		JOIN commands c ON c.player_id = p.id
		WHERE p.is_observer = 0
			AND p.type = 'Human'
			AND c.action_type = ?
			AND c.%s IS NOT NULL
			AND c.%s <> ''
		GROUP BY p.race, c.%s
	`, spec.NameColumn, spec.NameColumn, spec.NameColumn, spec.NameColumn)
	globalRows, err := d.db.QueryContext(d.ctx, globalQuery, spec.ActionType, spec.ActionType)
	if err != nil {
		return nil, fmt.Errorf("failed to query baseline outliers for %s: %w", spec.CategoryLabel, err)
	}
	defer globalRows.Close()
	globalGames := map[pair]int64{}
	globalPlayers := map[pair]float64{}
	for globalRows.Next() {
		var race string
		var name string
		var games int64
		var players float64
		if err := globalRows.Scan(&race, &name, &games, &players); err != nil {
			return nil, fmt.Errorf("failed to parse baseline outliers for %s: %w", spec.CategoryLabel, err)
		}
		key := pair{race: strings.TrimSpace(race), name: strings.TrimSpace(name)}
		globalGames[key] = games
		globalPlayers[key] = players
	}
	if err := globalRows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating baseline outliers for %s: %w", spec.CategoryLabel, err)
	}

	// For targeted orders we compare usage share in terms of raw order instances,
	// not replay incidence. These totals are intentionally built from the filtered
	// item universe so numerator and denominator stay aligned.
	playerTargetedTotalsByRace := map[string]int64{}
	globalTargetedTotalsByRace := map[string]int64{}
	if spec.ActionType == "Targeted Order" {
		for key, count := range playerCounts {
			if strings.EqualFold(strings.TrimSpace(key.race), strings.TrimSpace(primaryRace)) &&
				workflowItemAllowedForPrimaryRace(primaryRace, spec, key.name) &&
				!workflowSkipGenericTargetedOrder(key.name) {
				playerTargetedTotalsByRace[key.race] += count
			}
		}
		for key, count := range globalGames {
			if strings.EqualFold(strings.TrimSpace(key.race), strings.TrimSpace(primaryRace)) &&
				workflowItemAllowedForPrimaryRace(primaryRace, spec, key.name) &&
				!workflowSkipGenericTargetedOrder(key.name) {
				globalTargetedTotalsByRace[key.race] += count
			}
		}
	}

	out := []workflowPlayerOutlier{}
	for key, playerGames := range playerCounts {
		// Outliers are always same-race relative to the player's primary race.
		if !strings.EqualFold(strings.TrimSpace(key.race), strings.TrimSpace(primaryRace)) {
			continue
		}
		// Protoss-specific safety rule: ignore non-Protoss tech/upgrades/targeted
		// spell orders caused by mind-control race leakage.
		if !workflowItemAllowedForPrimaryRace(primaryRace, spec, key.name) {
			continue
		}
		if playerGames < 3 {
			continue
		}
		if spec.ActionType == "Targeted Order" {
			if workflowSkipGenericTargetedOrder(key.name) {
				continue
			}
		}
		playerRaceGames := playerGamesByRace[key.race]
		popRaceGames := popGamesByRace[key.race]
		popRacePlayers := popDistinctPlayersByRace[key.race]
		itemGlobalGames := globalGames[key]
		itemGlobalPlayers := globalPlayers[key]
		if playerRaceGames <= 0 || popRaceGames <= 0 || popRacePlayers <= 0 || itemGlobalGames <= 0 {
			continue
		}
		playerDenominator := float64(playerRaceGames)
		baselineDenominator := float64(popRaceGames)
		if spec.ActionType == "Targeted Order" {
			playerTargetedTotal := playerTargetedTotalsByRace[key.race]
			globalTargetedTotal := globalTargetedTotalsByRace[key.race]
			if playerTargetedTotal <= 0 || globalTargetedTotal <= 0 {
				continue
			}
			playerDenominator = float64(playerTargetedTotal)
			baselineDenominator = float64(globalTargetedTotal)
		}
		playerRate := float64(playerGames) / playerDenominator
		baselineRate := float64(itemGlobalGames) / baselineDenominator
		if baselineRate <= 0 {
			continue
		}
		ratio := playerRate / baselineRate
		if playerRate < 0.15 {
			continue
		}
		idf := math.Log((1.0+popRacePlayers)/(1.0+itemGlobalPlayers)) + 1.0
		tfidf := playerRate * idf

		qualifiedBy := []string{}
		if tfidf >= thresholds.TFIDFMin {
			qualifiedBy = append(qualifiedBy, "Rare signature")
		}
		if ratio >= thresholds.RatioMin {
			qualifiedBy = append(qualifiedBy, "Much more frequent than peers")
		}
		if len(qualifiedBy) == 0 {
			continue
		}
		out = append(out, workflowPlayerOutlier{
			Category:        spec.CategoryLabel,
			Race:            key.race,
			Name:            key.name,
			PrettyName:      prettySplitUppercase(key.name),
			PlayerGames:     playerGames,
			PlayerRate:      playerRate,
			BaselineRate:    baselineRate,
			RatioToBaseline: ratio,
			TFIDF:           tfidf,
			QualifiedBy:     qualifiedBy,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TFIDF == out[j].TFIDF {
			return out[i].RatioToBaseline > out[j].RatioToBaseline
		}
		return out[i].TFIDF > out[j].TFIDF
	})
	return out, nil
}

func workflowSkipGenericTargetedOrder(name string) bool {
	switch workflowCanonicalOutlierName(name) {
	case "attackmove", "attack1", "move", "patrol", "stop", "holdposition":
		return true
	default:
		return false
	}
}

func workflowItemAllowedForPrimaryRace(primaryRace string, spec workflowOutlierCategorySpec, itemName string) bool {
	if !strings.EqualFold(strings.TrimSpace(primaryRace), "Protoss") {
		return true
	}
	canonical := workflowCanonicalOutlierName(itemName)
	if canonical == "" {
		return false
	}
	switch spec.ActionType {
	case "Tech":
		_, ok := workflowProtossAllowedTechs[canonical]
		return ok
	case "Upgrade":
		_, ok := workflowProtossAllowedUpgrades[canonical]
		return ok
	case "Targeted Order":
		// Keep generic non-cast orders, but require explicit Protoss ownership
		// for spell-like cast orders to avoid cross-race leakage.
		if strings.HasPrefix(canonical, "cast") {
			_, ok := workflowProtossAllowedCastOrders[canonical]
			return ok
		}
		return true
	default:
		return true
	}
}

func workflowCanonicalOutlierName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(normalized))
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (d *Dashboard) totalDistinctPlayers() (float64, error) {
	var total float64
	if err := d.db.QueryRowContext(d.ctx, `
		SELECT CAST(COUNT(*) AS FLOAT)
		FROM (
			SELECT lower(trim(name)) AS player_key
			FROM players
			WHERE is_observer = 0
			GROUP BY lower(trim(name))
		)
	`).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to count distinct players: %w", err)
	}
	return total, nil
}

func (d *Dashboard) totalDistinctPlayersByRace(race string) (float64, error) {
	var total float64
	if err := d.db.QueryRowContext(d.ctx, `
		SELECT CAST(COUNT(*) AS FLOAT)
		FROM (
			SELECT lower(trim(name)) AS player_key
			FROM players
			WHERE is_observer = 0
				AND race = ?
			GROUP BY lower(trim(name))
		)
	`, race).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to count distinct players by race: %w", err)
	}
	return total, nil
}

func (d *Dashboard) rareUsageOutliersForPlayerByRace(playerKey, race string, gamesPlayed int64, playerQuery, populationQuery string) ([]workflowRareUsage, error) {
	if gamesPlayed == 0 {
		return []workflowRareUsage{}, nil
	}
	populationPlayers := 0.0
	var err error
	if strings.TrimSpace(race) == "" {
		populationPlayers, err = d.totalDistinctPlayers()
	} else {
		populationPlayers, err = d.totalDistinctPlayersByRace(race)
	}
	if err != nil {
		return nil, err
	}
	if populationPlayers <= 0 {
		return []workflowRareUsage{}, nil
	}

	playerRows, err := d.db.QueryContext(d.ctx, playerQuery, playerKey, race)
	if err != nil {
		return nil, fmt.Errorf("failed to query player rare usage: %w", err)
	}
	defer playerRows.Close()

	playerCountByName := map[string]int64{}
	for playerRows.Next() {
		var name string
		var usageCount int64
		if err := playerRows.Scan(&name, &usageCount); err != nil {
			return nil, fmt.Errorf("failed to parse player rare usage: %w", err)
		}
		playerCountByName[name] = usageCount
	}
	if err := playerRows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating player rare usage: %w", err)
	}

	popRows, err := d.db.QueryContext(d.ctx, populationQuery, race)
	if err != nil {
		return nil, fmt.Errorf("failed to query population rare usage: %w", err)
	}
	defer popRows.Close()
	popCountByName := map[string]int64{}
	for popRows.Next() {
		var name string
		var playerCount int64
		if err := popRows.Scan(&name, &playerCount); err != nil {
			return nil, fmt.Errorf("failed to parse population rare usage: %w", err)
		}
		popCountByName[name] = playerCount
	}
	if err := popRows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating population rare usage: %w", err)
	}

	outliers := make([]workflowRareUsage, 0, len(playerCountByName))
	for name, usageCount := range playerCountByName {
		playerRate := float64(usageCount) / float64(gamesPlayed)
		popRate := float64(popCountByName[name]) / populationPlayers
		if usageCount < 2 || popRate >= 0.35 || playerRate < 0.05 {
			continue
		}
		score := playerRate * (1.0 - popRate)
		outliers = append(outliers, workflowRareUsage{
			Name:                name,
			PrettyName:          prettySplitUppercase(name),
			PlayerCount:         usageCount,
			PlayerRatePerGame:   playerRate,
			PopulationUsageRate: popRate,
			RarityScore:         score,
		})
	}
	sort.Slice(outliers, func(i, j int) bool {
		if outliers[i].RarityScore == outliers[j].RarityScore {
			return outliers[i].PlayerCount > outliers[j].PlayerCount
		}
		return outliers[i].RarityScore > outliers[j].RarityScore
	})
	if len(outliers) > 8 {
		outliers = outliers[:8]
	}
	return outliers, nil
}

func primaryRaceFromBreakdown(breakdown []workflowPlayerRaceBreakdown) string {
	if len(breakdown) == 0 {
		return ""
	}
	bestRace := strings.TrimSpace(breakdown[0].Race)
	bestGames := breakdown[0].GameCount
	for _, race := range breakdown[1:] {
		if race.GameCount > bestGames {
			bestRace = strings.TrimSpace(race.Race)
			bestGames = race.GameCount
		}
	}
	return bestRace
}

func (d *Dashboard) simpleMetricByPlayer(query string) (map[string]float64, error) {
	rows, err := d.db.QueryContext(d.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query metric by player: %w", err)
	}
	defer rows.Close()
	valuesByPlayer := map[string]float64{}
	for rows.Next() {
		var playerKey string
		var value float64
		if err := rows.Scan(&playerKey, &value); err != nil {
			return nil, fmt.Errorf("failed to parse metric by player: %w", err)
		}
		valuesByPlayer[playerKey] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating metric by player: %w", err)
	}
	return valuesByPlayer, nil
}

func (d *Dashboard) firstExpansionAverageByPlayer() (map[string]float64, error) {
	rows, err := d.db.QueryContext(d.ctx, `
		SELECT replay_id, value_string
		FROM detected_patterns_replay
		WHERE pattern_name = 'Game Events'
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to load game events for expansion averages: %w", err)
	}
	defer rows.Close()

	playersByReplay, err := d.playersByReplay()
	if err != nil {
		return nil, err
	}
	valuesByPlayer := map[string][]int64{}
	for rows.Next() {
		var replayID int64
		var valueString string
		if err := rows.Scan(&replayID, &valueString); err != nil {
			return nil, fmt.Errorf("failed to parse game events for expansion averages: %w", err)
		}
		events := parseGameEvents(valueString)
		players := playersByReplay[replayID]
		if len(players) == 0 {
			continue
		}
		sortedPlayers := make([]workflowGamePlayer, len(players))
		copy(sortedPlayers, players)
		sort.Slice(sortedPlayers, func(i, j int) bool {
			return len(sortedPlayers[i].Name) > len(sortedPlayers[j].Name)
		})
		firstByPlayerInReplay := map[string]int64{}
		for _, event := range events {
			t := strings.ToLower(strings.TrimSpace(event.Type))
			if t != "expansion" {
				continue
			}
			playerID := matchPlayerIDInEvent(event.Description, sortedPlayers)
			if playerID == 0 {
				continue
			}
			playerKey := normalizePlayerKey(playerNameByID(playerID, players))
			if playerKey == "" {
				continue
			}
			if _, exists := firstByPlayerInReplay[playerKey]; !exists {
				firstByPlayerInReplay[playerKey] = event.Second
			}
		}
		for playerKey, second := range firstByPlayerInReplay {
			valuesByPlayer[playerKey] = append(valuesByPlayer[playerKey], second)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating game events for expansion averages: %w", err)
	}

	averages := map[string]float64{}
	for playerKey, values := range valuesByPlayer {
		if len(values) == 0 {
			continue
		}
		var sum float64
		for _, v := range values {
			sum += float64(v)
		}
		averages[playerKey] = sum / float64(len(values))
	}
	return averages, nil
}

func (d *Dashboard) playersByReplay() (map[int64][]workflowGamePlayer, error) {
	rows, err := d.db.QueryContext(d.ctx, `
		SELECT replay_id, id, name
		FROM players
		WHERE is_observer = 0
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to load players by replay: %w", err)
	}
	defer rows.Close()
	out := map[int64][]workflowGamePlayer{}
	for rows.Next() {
		var replayID int64
		var playerID int64
		var name string
		if err := rows.Scan(&replayID, &playerID, &name); err != nil {
			return nil, fmt.Errorf("failed parsing players by replay: %w", err)
		}
		out[replayID] = append(out[replayID], workflowGamePlayer{
			PlayerID:  playerID,
			PlayerKey: normalizePlayerKey(name),
			Name:      name,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating players by replay: %w", err)
	}
	return out, nil
}

func playerNameByID(playerID int64, players []workflowGamePlayer) string {
	for _, player := range players {
		if player.PlayerID == playerID {
			return player.Name
		}
	}
	return ""
}

func buildComparativeMetric(metricName, playerKey string, valuesByPlayer map[string]float64) workflowComparativeMetric {
	playerValue := valuesByPlayer[playerKey]
	return workflowComparativeMetric{
		Metric:      metricName,
		PlayerValue: playerValue,
	}
}

func (d *Dashboard) loadRaceOrderSummaryForPlayer(playerKey string) ([]workflowRaceOrderSummary, error) {
	rows, err := d.db.QueryContext(d.ctx, `
		SELECT p.id, p.race, c.action_type, c.tech_name, c.upgrade_name, c.seconds_from_game_start
		FROM players p
		LEFT JOIN commands c ON c.player_id = p.id
		WHERE lower(trim(p.name)) = ?
			AND p.is_observer = 0
			AND (
				(c.action_type = 'Tech' AND c.tech_name IS NOT NULL AND c.tech_name <> '')
				OR
				(c.action_type = 'Upgrade' AND c.upgrade_name IS NOT NULL AND c.upgrade_name <> '')
			)
		ORDER BY p.id ASC, c.seconds_from_game_start ASC
	`, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load race order summary: %w", err)
	}
	defer rows.Close()

	type gameOrders struct {
		race     string
		techs    []string
		upgrades []string
	}
	byGame := map[int64]*gameOrders{}
	for rows.Next() {
		var playerID int64
		var race string
		var actionType string
		var techName sql.NullString
		var upgradeName sql.NullString
		var second int64
		if err := rows.Scan(&playerID, &race, &actionType, &techName, &upgradeName, &second); err != nil {
			return nil, fmt.Errorf("failed to parse race order summary: %w", err)
		}
		_ = second
		if _, ok := byGame[playerID]; !ok {
			byGame[playerID] = &gameOrders{race: race, techs: []string{}, upgrades: []string{}}
		}
		entry := byGame[playerID]
		if actionType == "Tech" && techName.Valid && len(entry.techs) < 6 {
			entry.techs = append(entry.techs, techName.String)
		}
		if actionType == "Upgrade" && upgradeName.Valid && len(entry.upgrades) < 6 {
			entry.upgrades = append(entry.upgrades, upgradeName.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating race order summary: %w", err)
	}

	techSeqByRace := map[string]map[string]int64{}
	upgradeSeqByRace := map[string]map[string]int64{}
	for _, entry := range byGame {
		if _, ok := techSeqByRace[entry.race]; !ok {
			techSeqByRace[entry.race] = map[string]int64{}
		}
		if _, ok := upgradeSeqByRace[entry.race]; !ok {
			upgradeSeqByRace[entry.race] = map[string]int64{}
		}
		techSeqByRace[entry.race][strings.Join(entry.techs, " -> ")]++
		upgradeSeqByRace[entry.race][strings.Join(entry.upgrades, " -> ")]++
	}

	races := make([]string, 0, len(techSeqByRace))
	for race := range techSeqByRace {
		races = append(races, race)
	}
	sort.Strings(races)
	out := make([]workflowRaceOrderSummary, 0, len(races))
	for _, race := range races {
		out = append(out, workflowRaceOrderSummary{
			Race:         race,
			TechOrder:    splitSequence(bestSequence(techSeqByRace[race])),
			UpgradeOrder: splitSequence(bestSequence(upgradeSeqByRace[race])),
		})
	}
	return out, nil
}

func bestSequence(sequences map[string]int64) string {
	best := ""
	bestCount := int64(-1)
	for sequence, count := range sequences {
		if count > bestCount {
			best = sequence
			bestCount = count
			continue
		}
		if count == bestCount && sequence < best {
			best = sequence
		}
	}
	return best
}

func splitSequence(seq string) []string {
	trimmed := strings.TrimSpace(seq)
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, " -> ")
}

func (d *Dashboard) countQueuedGamesForPlayer(playerKey string) (int64, error) {
	var count int64
	if err := d.db.QueryRowContext(d.ctx, `
		SELECT COUNT(DISTINCT p.id)
		FROM players p
		JOIN commands c ON c.player_id = p.id
		WHERE lower(trim(p.name)) = ?
			AND p.is_observer = 0
			AND c.is_queued = 1
	`, playerKey).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count queued games: %w", err)
	}
	return count, nil
}

func (d *Dashboard) countCarrierGamesForPlayer(playerKey string) (int64, error) {
	var count int64
	if err := d.db.QueryRowContext(d.ctx, `
		SELECT COUNT(DISTINCT p.replay_id)
		FROM detected_patterns_replay_player dp
		JOIN players p ON p.id = dp.player_id
		WHERE lower(trim(p.name)) = ?
			AND p.is_observer = 0
			AND dp.pattern_name = 'Carriers'
			AND dp.value_bool = 1
	`, playerKey).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count carrier games: %w", err)
	}
	return count, nil
}

var uppercaseSplitter = regexp.MustCompile(`([a-z0-9])([A-Z])`)
var workflowChatWordSplitter = regexp.MustCompile(`[a-z][a-z0-9']+`)

var workflowChatStopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "been": {}, "but": {}, "by": {},
	"for": {}, "from": {}, "had": {}, "has": {}, "have": {}, "he": {}, "her": {}, "hers": {}, "him": {}, "his": {},
	"i": {}, "if": {}, "in": {}, "is": {}, "it": {}, "its": {}, "just": {}, "me": {}, "my": {}, "not": {}, "of": {},
	"on": {}, "or": {}, "our": {}, "ours": {}, "she": {}, "so": {}, "that": {}, "the": {}, "their": {}, "theirs": {},
	"them": {}, "they": {}, "this": {}, "to": {}, "too": {}, "us": {}, "was": {}, "we": {}, "were": {}, "what": {}, "when": {},
	"where": {}, "who": {}, "why": {}, "with": {}, "you": {}, "your": {}, "yours": {},
	"gg": {}, "gl": {}, "hf": {}, "wp": {}, "pls": {}, "plz": {}, "ok": {}, "yes": {}, "no": {}, "nah": {}, "lol": {},
}

func prettySplitUppercase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	withSpaces := uppercaseSplitter.ReplaceAllString(trimmed, `$1 $2`)
	var out []rune
	prevSpace := false
	for _, r := range withSpaces {
		isSpace := unicode.IsSpace(r)
		if isSpace {
			if prevSpace {
				continue
			}
			prevSpace = true
			out = append(out, ' ')
			continue
		}
		prevSpace = false
		out = append(out, r)
	}
	return strings.TrimSpace(string(out))
}

func (d *Dashboard) buildPlayerChatSummary(playerKey string) (workflowPlayerChatSummary, error) {
	summary := workflowPlayerChatSummary{
		TopTerms:        []workflowChatTermCount{},
		TopPhrases:      []workflowChatTermCount{},
		ToneHints:       []string{},
		ExampleMessages: []string{},
	}

	rows, err := d.db.QueryContext(d.ctx, `
		SELECT c.replay_id, c.chat_message
		FROM commands c
		JOIN players p ON p.id = c.player_id
		WHERE lower(trim(p.name)) = ?
			AND p.is_observer = 0
			AND c.action_type = 'Chat'
			AND c.chat_message IS NOT NULL
			AND trim(c.chat_message) <> ''
		ORDER BY c.replay_id DESC, c.seconds_from_game_start DESC
	`, playerKey)
	if err != nil {
		return summary, err
	}
	defer rows.Close()

	termCounts := map[string]int64{}
	phraseCounts := map[string]int64{}
	gamesWithChat := map[int64]struct{}{}
	rawMessages := []string{}

	sportsmanshipCount := int64(0)
	coordinationCount := int64(0)
	tauntCount := int64(0)
	questionCount := int64(0)

	for rows.Next() {
		var replayID int64
		var raw string
		if err := rows.Scan(&replayID, &raw); err != nil {
			return summary, err
		}
		msg := strings.TrimSpace(raw)
		if msg == "" {
			continue
		}
		rawMessages = append(rawMessages, msg)
		gamesWithChat[replayID] = struct{}{}

		lowerMsg := strings.ToLower(msg)
		if strings.Contains(lowerMsg, "gg") || strings.Contains(lowerMsg, "well played") || strings.Contains(lowerMsg, "gl hf") || strings.Contains(lowerMsg, "good luck") {
			sportsmanshipCount++
		}
		if strings.Contains(lowerMsg, "rush") || strings.Contains(lowerMsg, "defend") || strings.Contains(lowerMsg, "attack") || strings.Contains(lowerMsg, "help") || strings.Contains(lowerMsg, "scout") {
			coordinationCount++
		}
		if strings.Contains(lowerMsg, "noob") || strings.Contains(lowerMsg, "ez") || strings.Contains(lowerMsg, "trash") || strings.Contains(lowerMsg, "owned") {
			tauntCount++
		}
		if strings.Contains(msg, "?") {
			questionCount++
		}

		tokens := summarizeChatTokens(msg)
		for _, token := range tokens {
			termCounts[token]++
		}
		for i := 0; i+1 < len(tokens); i++ {
			phrase := tokens[i] + " " + tokens[i+1]
			phraseCounts[phrase]++
		}
	}
	if err := rows.Err(); err != nil {
		return summary, err
	}

	summary.TotalMessages = int64(len(rawMessages))
	summary.GamesWithChat = int64(len(gamesWithChat))
	summary.DistinctTerms = int64(len(termCounts))
	summary.TopTerms = summarizeChatCounts(termCounts, 10)
	summary.TopPhrases = summarizeChatCounts(phraseCounts, 6)
	summary.ExampleMessages = summarizeChatExamples(rawMessages, 5)

	if summary.TotalMessages > 0 {
		if sportsmanshipCount*5 >= summary.TotalMessages {
			summary.ToneHints = append(summary.ToneHints, "Frequent sportsmanship language (GG/GLHF/well-played style phrases).")
		}
		if coordinationCount*5 >= summary.TotalMessages {
			summary.ToneHints = append(summary.ToneHints, "Often uses tactical coordination terms (rush/defend/attack/help/scout).")
		}
		if questionCount*4 >= summary.TotalMessages {
			summary.ToneHints = append(summary.ToneHints, "Asks questions frequently, suggesting active in-game coordination.")
		}
		if tauntCount*10 >= summary.TotalMessages {
			summary.ToneHints = append(summary.ToneHints, "Contains occasional taunting or provocative wording.")
		}
	}
	if len(summary.ToneHints) == 0 {
		summary.ToneHints = append(summary.ToneHints, "No dominant chat tone was confidently inferred from available messages.")
	}

	return summary, nil
}

func summarizeChatTokens(message string) []string {
	lowered := strings.ToLower(message)
	rawTokens := workflowChatWordSplitter.FindAllString(lowered, -1)
	result := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		token = strings.Trim(token, "'")
		if len(token) < 3 {
			continue
		}
		if _, isStopWord := workflowChatStopWords[token]; isStopWord {
			continue
		}
		result = append(result, token)
	}
	return result
}

func summarizeChatCounts(counts map[string]int64, maxItems int) []workflowChatTermCount {
	items := make([]workflowChatTermCount, 0, len(counts))
	for term, count := range counts {
		if count <= 0 {
			continue
		}
		items = append(items, workflowChatTermCount{
			Term:  term,
			Count: count,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Term < items[j].Term
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > maxItems {
		items = items[:maxItems]
	}
	return items
}

func summarizeChatExamples(messages []string, maxItems int) []string {
	if len(messages) == 0 {
		return []string{}
	}
	result := []string{}
	for _, msg := range messages {
		msg = strings.Join(strings.Fields(strings.TrimSpace(msg)), " ")
		if msg == "" {
			continue
		}
		if len(msg) > 160 {
			msg = msg[:157] + "..."
		}
		result = append(result, msg)
		if len(result) >= maxItems {
			break
		}
	}
	return result
}

func buildGameNarrativeHints(players []workflowGamePlayer) []string {
	hints := []string{}
	for _, p := range players {
		if p.CommandCount > 0 && p.HotkeyUsageRate >= 0.15 {
			hints = append(hints, fmt.Sprintf("%s uses hotkeys frequently (%.1f%% of commands).", p.Name, p.HotkeyUsageRate*100))
		}
	}
	if len(hints) == 0 {
		hints = append(hints, "No strong command-pattern outliers were detected in this match.")
	}
	return hints
}

func buildPlayerNarrativeHints(player workflowPlayerOverview) []string {
	hints := []string{
		fmt.Sprintf("%s appears in %d games with a %.1f%% win rate.", player.PlayerName, player.GamesPlayed, player.WinRate*100),
	}
	if player.HotkeyUsageRate > 0 {
		hints = append(hints, fmt.Sprintf("Hotkeys appear in %.1f%% of this player's commands.", player.HotkeyUsageRate*100))
	}
	if player.CarrierCommandCount > 0 {
		hints = append(hints, fmt.Sprintf("Carrier-related commands detected: %d.", player.CarrierCommandCount))
	}
	if player.QueuedGameRate >= 0.25 {
		hints = append(hints, fmt.Sprintf("Queued orders appear in %.1f%% of this player's games.", player.QueuedGameRate*100))
	}
	return hints
}

func parseReplayID(value string) (int64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, errors.New("replay ID missing")
	}
	replayID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, errors.New("replay ID should be numeric")
	}
	return replayID, nil
}

func parsePagination(r *http.Request, defaultLimit, maxLimit int) (int, int) {
	limit := defaultLimit
	offset := 0
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			if parsed > maxLimit {
				parsed = maxLimit
			}
			limit = parsed
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("offset")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return limit, offset
}

func normalizePlayerKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func decodeAskQuestion(r *http.Request) (string, error) {
	type askRequest struct {
		Question string `json:"question"`
	}
	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", errors.New("invalid request body")
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		return "", errors.New("question is required")
	}
	return question, nil
}

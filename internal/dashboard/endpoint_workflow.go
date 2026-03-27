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

type workflowGameListItem struct {
	ReplayID        int64  `json:"replay_id"`
	ReplayDate      string `json:"replay_date"`
	FileName        string `json:"file_name"`
	MapName         string `json:"map_name"`
	DurationSeconds int64  `json:"duration_seconds"`
	GameType        string `json:"game_type"`
	PlayersLabel    string `json:"players_label"`
	WinnersLabel    string `json:"winners_label"`
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
	SummaryVersion  string                 `json:"summary_version"`
	ReplayID        int64                  `json:"replay_id"`
	ReplayDate      string                 `json:"replay_date"`
	FileName        string                 `json:"file_name"`
	MapName         string                 `json:"map_name"`
	DurationSeconds int64                  `json:"duration_seconds"`
	GameType        string                 `json:"game_type"`
	Players         []workflowGamePlayer   `json:"players"`
	ReplayPatterns  []workflowPatternValue `json:"replay_patterns"`
	TeamPatterns    []workflowTeamPattern  `json:"team_patterns"`
	GameEvents      []workflowGameEvent    `json:"game_events"`
	UnitsBySlice    []workflowUnitSlice    `json:"units_by_slice"`
	Timings         workflowReplayTimings  `json:"timings"`
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
	SummaryVersion            string                        `json:"summary_version"`
	PlayerKey                 string                        `json:"player_key"`
	PlayerName                string                        `json:"player_name"`
	GamesPlayed               int64                         `json:"games_played"`
	Wins                      int64                         `json:"wins"`
	WinRate                   float64                       `json:"win_rate"`
	AverageAPM                float64                       `json:"average_apm"`
	AverageEAPM               float64                       `json:"average_eapm"`
	HotkeyUsageRate           float64                       `json:"hotkey_usage_rate"`
	CarrierCommandCount       int64                         `json:"carrier_command_count"`
	RaceBreakdown             []workflowPlayerRaceBreakdown `json:"race_breakdown"`
	TargetedOrderOutliers     []workflowRareUsage           `json:"targeted_order_outliers"`
	TechOutliers              []workflowRareUsage           `json:"tech_outliers"`
	TimingComparisons         []workflowComparativeMetric   `json:"timing_comparisons"`
	HotkeyComparisons         []workflowComparativeMetric   `json:"hotkey_comparisons"`
	RallyPointComparison      workflowComparativeMetric     `json:"rally_point_comparison"`
	ActionDiversityComparison workflowComparativeMetric     `json:"action_diversity_comparison"`
	RaceOrders                []workflowRaceOrderSummary    `json:"race_orders"`
	QueuedGames               int64                         `json:"queued_games"`
	QueuedGameRate            float64                       `json:"queued_game_rate"`
	CarrierGames              int64                         `json:"carrier_games"`
	CarrierGameRate           float64                       `json:"carrier_game_rate"`
	RecentGames               []workflowGameListItem        `json:"recent_games"`
	NarrativeHints            []string                      `json:"narrative_hints"`
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
	Metric            string  `json:"metric"`
	PlayerValue       float64 `json:"player_value"`
	PopulationAverage float64 `json:"population_average"`
	PopulationStdDev  float64 `json:"population_std_dev"`
	ZScore            float64 `json:"z_score"`
	Direction         string  `json:"direction"`
}

type workflowRaceOrderSummary struct {
	Race         string   `json:"race"`
	TechOrder    []string `json:"tech_order"`
	UpgradeOrder []string `json:"upgrade_order"`
}

func (d *Dashboard) handlerWorkflowGamesList(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r, 20, 200)
	rows, err := d.db.QueryContext(d.ctx, `
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
					SELECT p.name AS name
					FROM players p
					WHERE p.replay_id = r.id AND p.is_observer = 0
					ORDER BY p.team ASC, p.id ASC
				)
			), ''),
			COALESCE((
				SELECT group_concat(p.name, ', ')
				FROM players p
				WHERE p.replay_id = r.id AND p.is_winner = 1 AND p.is_observer = 0
			), '')
		FROM replays r
		ORDER BY r.replay_date DESC, r.id DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
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
			&item.PlayersLabel,
			&item.WinnersLabel,
		); err != nil {
			http.Error(w, "failed to parse games list: "+err.Error(), http.StatusInternalServerError)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "failed to iterate games list: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"summary_version": workflowSummaryVersion,
		"items":           items,
		"limit":           limit,
		"offset":          offset,
	})
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
			SUM(CASE WHEN c.hotkey_type IS NOT NULL THEN 1 ELSE 0 END) AS hotkey_count
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
		); err != nil {
			return detail, fmt.Errorf("failed to parse players: %w", err)
		}
		p.PlayerKey = normalizePlayerKey(p.Name)
		if p.CommandCount > 0 {
			p.HotkeyUsageRate = float64(p.HotkeyCommandCount) / float64(p.CommandCount)
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

	var hotkeyCount int64
	var commandCount int64
	var carrierCount int64
	if err := d.db.QueryRowContext(d.ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN c.hotkey_type IS NOT NULL THEN 1 ELSE 0 END), 0),
			COALESCE(COUNT(c.id), 0),
			COALESCE(SUM(CASE WHEN lower(COALESCE(c.unit_type, '')) LIKE '%carrier%' OR lower(COALESCE(c.unit_types, '')) LIKE '%carrier%' THEN 1 ELSE 0 END), 0)
		FROM players p
		LEFT JOIN commands c ON c.player_id = p.id
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0
	`, playerKey).Scan(&hotkeyCount, &commandCount, &carrierCount); err != nil {
		return result, fmt.Errorf("failed to load player command summary: %w", err)
	}
	result.CarrierCommandCount = carrierCount
	if commandCount > 0 {
		result.HotkeyUsageRate = float64(hotkeyCount) / float64(commandCount)
	}

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
	if err := d.populateAdvancedPlayerOverview(playerKey, &result); err != nil {
		return result, err
	}
	result.NarrativeHints = buildPlayerNarrativeHints(result)
	return result, nil
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
			AND c.action_type IN ('Train', 'Unit Morph', 'Building Morph')
			AND c.unit_type IS NOT NULL
			AND c.unit_type <> ''
		ORDER BY c.seconds_from_game_start ASC, c.player_id ASC
	`, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load unit slices: %w", err)
	}
	defer rows.Close()

	perSlice := map[int64]map[int64]map[string]int64{}
	for rows.Next() {
		var playerID int64
		var second int64
		var unitType string
		if err := rows.Scan(&playerID, &second, &unitType); err != nil {
			return fmt.Errorf("failed to parse unit slices: %w", err)
		}
		sliceStart := (second / 300) * 300
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

	maxSlice := (detail.DurationSeconds / 300) * 300
	for sliceStart := int64(0); sliceStart <= maxSlice; sliceStart += 300 {
		slice := workflowUnitSlice{
			SliceStartSecond: sliceStart,
			SliceLabel:       fmt.Sprintf("%s-%s", formatClockFromSeconds(sliceStart), formatClockFromSeconds(sliceStart+299)),
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
		if typeLower != "expansion" && typeLower != "takeover" {
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
	primaryRace := primaryRaceFromBreakdown(result.RaceBreakdown)
	comparisonGamesPlayed := result.GamesPlayed
	for _, race := range result.RaceBreakdown {
		if strings.EqualFold(strings.TrimSpace(race.Race), strings.TrimSpace(primaryRace)) {
			comparisonGamesPlayed = race.GameCount
			break
		}
	}
	targetedOrderOutliers, err := d.rareUsageOutliersForPlayerByRace(
		playerKey,
		primaryRace,
		comparisonGamesPlayed,
		`
		SELECT c.order_name, COUNT(*) AS usage_count
		FROM commands c
		JOIN players p ON p.id = c.player_id
		WHERE lower(trim(p.name)) = ?
			AND p.race = ?
			AND p.is_observer = 0
			AND c.action_type = 'Targeted Order'
			AND c.order_name IS NOT NULL
			AND c.order_name <> ''
			AND lower(c.order_name) NOT IN ('attackmove', 'attack move', 'move', 'stop', 'holdposition', 'hold position', 'patrol')
		GROUP BY c.order_name
		`,
		`
		SELECT c.order_name, COUNT(DISTINCT lower(trim(p.name))) AS player_count
		FROM commands c
		JOIN players p ON p.id = c.player_id
		WHERE p.is_observer = 0
			AND p.race = ?
			AND c.action_type = 'Targeted Order'
			AND c.order_name IS NOT NULL
			AND c.order_name <> ''
			AND lower(c.order_name) NOT IN ('attackmove', 'attack move', 'move', 'stop', 'holdposition', 'hold position', 'patrol')
		GROUP BY c.order_name
		`,
	)
	if err != nil {
		return err
	}
	result.TargetedOrderOutliers = targetedOrderOutliers

	techOutliers, err := d.rareUsageOutliersForPlayerByRace(
		playerKey,
		primaryRace,
		comparisonGamesPlayed,
		`
		SELECT c.tech_name, COUNT(*) AS usage_count
		FROM commands c
		JOIN players p ON p.id = c.player_id
		WHERE lower(trim(p.name)) = ?
			AND p.race = ?
			AND p.is_observer = 0
			AND c.action_type = 'Tech'
			AND c.tech_name IS NOT NULL
			AND c.tech_name <> ''
		GROUP BY c.tech_name
		`,
		`
		SELECT c.tech_name, COUNT(DISTINCT lower(trim(p.name))) AS player_count
		FROM commands c
		JOIN players p ON p.id = c.player_id
		WHERE p.is_observer = 0
			AND p.race = ?
			AND c.action_type = 'Tech'
			AND c.tech_name IS NOT NULL
			AND c.tech_name <> ''
		GROUP BY c.tech_name
		`,
	)
	if err != nil {
		return err
	}
	result.TechOutliers = techOutliers

	firstGas, err := d.simpleMetricByPlayer(`
		WITH first_gas AS (
			SELECT
				lower(trim(p.name)) AS player_key,
				MIN(c.seconds_from_game_start) AS first_sec
			FROM players p
			JOIN commands c ON c.player_id = p.id
			WHERE p.is_observer = 0
				AND c.action_type = 'Build'
				AND c.unit_type IN ('Assimilator', 'Extractor', 'Refinery')
			GROUP BY p.id
		)
		SELECT player_key, AVG(first_sec) AS metric_value
		FROM first_gas
		GROUP BY player_key
	`)
	if err != nil {
		return err
	}
	firstUpgrade, err := d.simpleMetricByPlayer(`
		WITH first_upgrade AS (
			SELECT
				lower(trim(p.name)) AS player_key,
				MIN(c.seconds_from_game_start) AS first_sec
			FROM players p
			JOIN commands c ON c.player_id = p.id
			WHERE p.is_observer = 0
				AND c.action_type = 'Upgrade'
			GROUP BY p.id
		)
		SELECT player_key, AVG(first_sec) AS metric_value
		FROM first_upgrade
		GROUP BY player_key
	`)
	if err != nil {
		return err
	}
	firstExpansion, err := d.firstExpansionAverageByPlayer()
	if err != nil {
		return err
	}
	result.TimingComparisons = []workflowComparativeMetric{
		buildComparativeMetric("Average first gas timing (seconds)", playerKey, firstGas),
		buildComparativeMetric("Average first expansion timing (seconds)", playerKey, firstExpansion),
		buildComparativeMetric("Average first upgrade timing (seconds)", playerKey, firstUpgrade),
	}

	hotkeyGamesRate, err := d.simpleMetricByPlayer(`
		WITH game_level AS (
			SELECT
				lower(trim(p.name)) AS player_key,
				CASE WHEN SUM(CASE WHEN c.hotkey_type IS NOT NULL THEN 1 ELSE 0 END) > 0 THEN 1.0 ELSE 0.0 END AS metric_value
			FROM players p
			LEFT JOIN commands c ON c.player_id = p.id
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
	assignToUseRatio, err := d.simpleMetricByPlayer(`
		WITH game_level AS (
			SELECT
				lower(trim(p.name)) AS player_key,
				CASE
					WHEN SUM(CASE WHEN c.hotkey_type = 'Select' THEN 1 ELSE 0 END) > 0 THEN
						CAST(SUM(CASE WHEN c.hotkey_type = 'Assign' THEN 1 ELSE 0 END) AS FLOAT)
						/
						CAST(SUM(CASE WHEN c.hotkey_type = 'Select' THEN 1 ELSE 0 END) AS FLOAT)
					ELSE 0.0
				END AS metric_value
			FROM players p
			LEFT JOIN commands c ON c.player_id = p.id
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
		WITH game_level AS (
			SELECT
				lower(trim(p.name)) AS player_key,
				CASE
					WHEN COUNT(c.id) > 0 THEN CAST(SUM(CASE WHEN c.hotkey_type IS NOT NULL THEN 1 ELSE 0 END) AS FLOAT) / CAST(COUNT(c.id) AS FLOAT)
					ELSE 0.0
				END AS metric_value
			FROM players p
			LEFT JOIN commands c ON c.player_id = p.id
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
	result.HotkeyComparisons = []workflowComparativeMetric{
		buildComparativeMetric("Games using hotkeys (%)", playerKey, hotkeyGamesRate),
		buildComparativeMetric("Assign-to-use hotkey ratio", playerKey, assignToUseRatio),
		buildComparativeMetric("Hotkey commands as % of all commands", playerKey, hotkeyCommandsPct),
	}

	rallyPoints, err := d.simpleMetricByPlayer(`
		WITH game_level AS (
			SELECT
				lower(trim(p.name)) AS player_key,
				CAST(SUM(CASE WHEN c.action_type = 'Targeted Order' AND c.order_name LIKE 'RallyPoint%' THEN 1 ELSE 0 END) AS FLOAT) AS metric_value
			FROM players p
			LEFT JOIN commands c ON c.player_id = p.id
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
	result.RallyPointComparison = buildComparativeMetric("Rally point commands per game", playerKey, rallyPoints)

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
	result.ActionDiversityComparison = buildComparativeMetric("Action type diversity", playerKey, actionDiversity)

	raceOrders, err := d.loadRaceOrderSummaryForPlayer(playerKey)
	if err != nil {
		return err
	}
	result.RaceOrders = raceOrders

	queuedGames, err := d.countQueuedGamesForPlayer(playerKey)
	if err != nil {
		return err
	}
	result.QueuedGames = queuedGames
	if result.GamesPlayed > 0 {
		result.QueuedGameRate = float64(queuedGames) / float64(result.GamesPlayed)
	}

	carrierGames, err := d.countCarrierGamesForPlayer(playerKey)
	if err != nil {
		return err
	}
	result.CarrierGames = carrierGames
	if result.GamesPlayed > 0 {
		result.CarrierGameRate = float64(carrierGames) / float64(result.GamesPlayed)
	}

	return nil
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
			if t != "expansion" && t != "takeover" {
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
	values := make([]float64, 0, len(valuesByPlayer))
	for _, value := range valuesByPlayer {
		values = append(values, value)
	}
	playerValue := valuesByPlayer[playerKey]
	mean, stdDev := meanAndStdDev(values)
	zScore := 0.0
	if stdDev > 0 {
		zScore = (playerValue - mean) / stdDev
	}
	direction := "neutral"
	if zScore >= 1.5 {
		direction = "up"
	} else if zScore <= -1.5 {
		direction = "down"
	}
	return workflowComparativeMetric{
		Metric:            metricName,
		PlayerValue:       playerValue,
		PopulationAverage: mean,
		PopulationStdDev:  stdDev,
		ZScore:            zScore,
		Direction:         direction,
	}
}

func meanAndStdDev(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	mean := sum / float64(len(values))
	variance := 0.0
	for _, value := range values {
		diff := value - mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	return mean, math.Sqrt(variance)
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
	if player.CarrierGameRate >= 0.5 {
		hints = append(hints, fmt.Sprintf("Carrier play appears in %.1f%% of this player's games.", player.CarrierGameRate*100))
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

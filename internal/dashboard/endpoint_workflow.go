package dashboard

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
	PlayerID               int64                  `json:"player_id"`
	PlayerKey              string                 `json:"player_key"`
	Name                   string                 `json:"name"`
	Race                   string                 `json:"race"`
	Team                   int64                  `json:"team"`
	IsWinner               bool                   `json:"is_winner"`
	APM                    int64                  `json:"apm"`
	EAPM                   int64                  `json:"eapm"`
	StartLocationOClock    *int64                 `json:"start_location_oclock,omitempty"`
	CommandCount           int64                  `json:"command_count"`
	HotkeyCommandCount     int64                  `json:"hotkey_command_count"`
	CarrierCommandCount    int64                  `json:"carrier_command_count"`
	HotkeyUsageRate        float64                `json:"hotkey_usage_rate"`
	AverageCommandPosition []int64                `json:"average_command_position,omitempty"`
	TopActionTypes         []string               `json:"top_action_types"`
	DetectedPatterns       []workflowPatternValue `json:"detected_patterns"`
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
	NarrativeHints  []string               `json:"narrative_hints"`
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
	RecentGames         []workflowGameListItem        `json:"recent_games"`
	NarrativeHints      []string                      `json:"narrative_hints"`
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
			p.start_location_oclock,
			COUNT(c.id) AS command_count,
			SUM(CASE WHEN c.hotkey_type IS NOT NULL THEN 1 ELSE 0 END) AS hotkey_count,
			SUM(CASE WHEN lower(COALESCE(c.unit_type, '')) LIKE '%carrier%' OR lower(COALESCE(c.unit_types, '')) LIKE '%carrier%' THEN 1 ELSE 0 END) AS carrier_count,
			CAST(AVG(c.x) AS INTEGER) AS avg_x,
			CAST(AVG(c.y) AS INTEGER) AS avg_y
		FROM players p
		LEFT JOIN commands c ON c.player_id = p.id
		WHERE p.replay_id = ? AND p.is_observer = 0
		GROUP BY p.id, p.name, p.race, p.team, p.is_winner, p.apm, p.eapm, p.start_location_oclock
		ORDER BY p.team ASC, p.id ASC
	`, replayID)
	if err != nil {
		return detail, fmt.Errorf("failed to load players: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p workflowGamePlayer
		var startOClock sql.NullInt64
		var avgX sql.NullInt64
		var avgY sql.NullInt64
		if err := rows.Scan(
			&p.PlayerID,
			&p.Name,
			&p.Race,
			&p.Team,
			&p.IsWinner,
			&p.APM,
			&p.EAPM,
			&startOClock,
			&p.CommandCount,
			&p.HotkeyCommandCount,
			&p.CarrierCommandCount,
			&avgX,
			&avgY,
		); err != nil {
			return detail, fmt.Errorf("failed to parse players: %w", err)
		}
		p.PlayerKey = normalizePlayerKey(p.Name)
		if p.CommandCount > 0 {
			p.HotkeyUsageRate = float64(p.HotkeyCommandCount) / float64(p.CommandCount)
		}
		if startOClock.Valid {
			v := startOClock.Int64
			p.StartLocationOClock = &v
		}
		if avgX.Valid && avgY.Valid {
			p.AverageCommandPosition = []int64{avgX.Int64, avgY.Int64}
		}
		p.TopActionTypes, _ = d.topActionTypesForPlayer(p.PlayerID, 3)
		p.DetectedPatterns = []workflowPatternValue{}
		detail.Players = append(detail.Players, p)
	}
	if err := rows.Err(); err != nil {
		return detail, fmt.Errorf("failed to iterate players: %w", err)
	}

	if err := d.populateDetectedPatternsForGameDetail(&detail); err != nil {
		return detail, err
	}

	detail.NarrativeHints = buildGameNarrativeHints(detail.Players)
	return detail, nil
}

func (d *Dashboard) populateDetectedPatternsForGameDetail(detail *workflowGameDetail) error {
	detail.ReplayPatterns = []workflowPatternValue{}
	detail.TeamPatterns = []workflowTeamPattern{}

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

func buildGameNarrativeHints(players []workflowGamePlayer) []string {
	hints := []string{}
	for _, p := range players {
		if p.CarrierCommandCount > 0 {
			hints = append(hints, fmt.Sprintf("%s shows carrier-related commands (%d).", p.Name, p.CarrierCommandCount))
		}
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

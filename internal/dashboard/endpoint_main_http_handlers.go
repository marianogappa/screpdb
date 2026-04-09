package dashboard

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/marianogappa/screpdb/internal/dashboard/variables"
)

func (d *Dashboard) handlerGameDetail(w http.ResponseWriter, r *http.Request) {
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

func (d *Dashboard) handlerPlayerDetail(w http.ResponseWriter, r *http.Request) {
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

func (d *Dashboard) handlerPlayerRecentGames(w http.ResponseWriter, r *http.Request) {
	playerKey := normalizePlayerKey(mux.Vars(r)["playerKey"])
	if playerKey == "" {
		http.Error(w, "player key missing", http.StatusBadRequest)
		return
	}
	games, err := d.buildWorkflowPlayerRecentGames(playerKey)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"player_key":      playerKey,
		"recent_games":    games,
		"summary_version": workflowSummaryVersion,
	})
}

func (d *Dashboard) handlerPlayerChatSummary(w http.ResponseWriter, r *http.Request) {
	playerKey := normalizePlayerKey(mux.Vars(r)["playerKey"])
	if playerKey == "" {
		http.Error(w, "player key missing", http.StatusBadRequest)
		return
	}
	chatSummary, err := d.buildPlayerChatSummary(playerKey)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"player_key":      playerKey,
		"chat_summary":    chatSummary,
		"summary_version": workflowSummaryVersion,
	})
}

func (d *Dashboard) handlerPlayerOutliers(w http.ResponseWriter, r *http.Request) {
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

func (d *Dashboard) handlerPlayerMetrics(w http.ResponseWriter, r *http.Request) {
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

func (d *Dashboard) handlerPlayerInsight(w http.ResponseWriter, r *http.Request) {
	playerKey := normalizePlayerKey(mux.Vars(r)["playerKey"])
	if playerKey == "" {
		http.Error(w, "player key missing", http.StatusBadRequest)
		return
	}
	insightType := workflowPlayerInsightType(strings.TrimSpace(r.URL.Query().Get("type")))
	result, err := d.buildWorkflowPlayerAsyncInsight(playerKey, insightType)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		if errors.Is(err, errUnsupportedWorkflowPlayerInsightType) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (d *Dashboard) handlerPlayerApmHistogram(w http.ResponseWriter, r *http.Request) {
	playerKey := normalizePlayerKey(mux.Vars(r)["playerKey"])
	if playerKey == "" {
		http.Error(w, "player key missing", http.StatusBadRequest)
		return
	}
	histogram, err := d.buildWorkflowPlayerApmHistogram(playerKey)
	if err != nil {
		http.Error(w, "failed to compute histogram: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(histogram)
}

func (d *Dashboard) handlerPlayersApmHistogram(w http.ResponseWriter, _ *http.Request) {
	histogram, err := d.buildWorkflowPlayerApmHistogram("")
	if err != nil {
		http.Error(w, "failed to compute histogram: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(histogram)
}

func (d *Dashboard) handlerPlayersDelayHistogram(w http.ResponseWriter, _ *http.Request) {
	histogram, err := d.buildWorkflowPlayerDelayHistogram()
	if err != nil {
		http.Error(w, "failed to compute delay histogram: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(histogram)
}

func (d *Dashboard) handlerPlayerDelayInsight(w http.ResponseWriter, r *http.Request) {
	playerKey := normalizePlayerKey(mux.Vars(r)["playerKey"])
	if playerKey == "" {
		http.Error(w, "player key missing", http.StatusBadRequest)
		return
	}
	result, err := d.buildWorkflowPlayerDelayInsight(playerKey)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (d *Dashboard) handlerPlayersUnitCadence(w http.ResponseWriter, r *http.Request) {
	filterMode, err := parseWorkflowUnitCadenceFilterMode(r.URL.Query().Get("filter"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	minGames := workflowUnitCadenceMinGames
	if parsed, ok := parseOptionalInt64Query(r, "min_games"); ok && parsed > 0 {
		minGames = parsed
	}
	limit := workflowUnitCadenceDefaultLimit
	if parsed, ok := parseOptionalInt64Query(r, "limit"); ok {
		if parsed < 0 {
			http.Error(w, "limit must be >= 0", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	if limit > workflowUnitCadenceMaxLimit {
		limit = workflowUnitCadenceMaxLimit
	}
	result, err := d.buildWorkflowPlayerUnitCadenceLeaderboard(filterMode, minGames, limit)
	if err != nil {
		http.Error(w, "failed to compute unit cadence leaderboard: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (d *Dashboard) handlerPlayerUnitCadence(w http.ResponseWriter, r *http.Request) {
	playerKey := normalizePlayerKey(mux.Vars(r)["playerKey"])
	if playerKey == "" {
		http.Error(w, "player key missing", http.StatusBadRequest)
		return
	}
	filterMode, err := parseWorkflowUnitCadenceFilterMode(r.URL.Query().Get("filter"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := d.buildWorkflowPlayerUnitCadenceInsight(playerKey, filterMode)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (d *Dashboard) handlerPlayerColors(w http.ResponseWriter, _ *http.Request) {
	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
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

func (d *Dashboard) handlerGameAsk(w http.ResponseWriter, r *http.Request) {
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

func (d *Dashboard) handlerPlayerAsk(w http.ResponseWriter, r *http.Request) {
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

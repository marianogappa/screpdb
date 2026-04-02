package dashboard

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/marianogappa/screpdb/internal/models"
)

const workflowViewportMultitaskingMinGames int64 = 4

type workflowPlayerViewportMultitaskingPoint struct {
	PlayerKey                 string  `json:"player_key"`
	PlayerName                string  `json:"player_name"`
	GamesPlayed               int64   `json:"games_played"`
	AverageViewportSwitchRate float64 `json:"average_viewport_switch_rate"`
}

type workflowPlayerViewportMultitaskingDistribution struct {
	SummaryVersion  string                                    `json:"summary_version"`
	PatternName     string                                    `json:"pattern_name"`
	MinGames        int64                                     `json:"min_games"`
	PlayersIncluded int64                                     `json:"players_included"`
	Players         []workflowPlayerViewportMultitaskingPoint `json:"players"`
}

type workflowGameViewportMultitaskingPlayer struct {
	PlayerID           int64   `json:"player_id"`
	PlayerKey          string  `json:"player_key"`
	PlayerName         string  `json:"player_name"`
	Team               int64   `json:"team"`
	IsWinner           bool    `json:"is_winner"`
	Eligible           bool    `json:"eligible"`
	IneligibleReason   string  `json:"ineligible_reason,omitempty"`
	ViewportSwitchRate float64 `json:"viewport_switch_rate"`
}

type workflowViewportMultitaskingAggregate struct {
	PlayerKey                 string
	PlayerName                string
	GamesPlayed               int64
	averageViewportSwitchRate float64
}

func (d *Dashboard) handlerWorkflowPlayersViewportMultitasking(w http.ResponseWriter, _ *http.Request) {
	result, err := d.buildWorkflowPlayerViewportMultitaskingDistribution()
	if err != nil {
		http.Error(w, "failed to compute viewport multitasking distribution: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (d *Dashboard) buildWorkflowPlayerViewportMultitaskingDistribution() (workflowPlayerViewportMultitaskingDistribution, error) {
	allPlayers, err := d.loadWorkflowViewportMultitaskingAggregates()
	if err != nil {
		return workflowPlayerViewportMultitaskingDistribution{}, err
	}
	eligible := filterWorkflowViewportMultitaskingAggregates(allPlayers)
	result := workflowPlayerViewportMultitaskingDistribution{
		SummaryVersion:  workflowSummaryVersion,
		PatternName:     models.PatternNameViewportMultitasking,
		MinGames:        workflowViewportMultitaskingMinGames,
		PlayersIncluded: int64(len(eligible)),
		Players:         make([]workflowPlayerViewportMultitaskingPoint, 0, len(eligible)),
	}
	for _, player := range eligible {
		result.Players = append(result.Players, workflowPlayerViewportMultitaskingPoint{
			PlayerKey:                 player.PlayerKey,
			PlayerName:                player.PlayerName,
			GamesPlayed:               player.GamesPlayed,
			AverageViewportSwitchRate: player.averageViewportSwitchRate,
		})
	}
	return result, nil
}

func (d *Dashboard) buildWorkflowPlayerViewportAsyncInsight(playerKey string) (workflowPlayerAsyncInsight, error) {
	allPlayers, err := d.loadWorkflowViewportMultitaskingAggregates()
	if err != nil {
		return workflowPlayerAsyncInsight{}, err
	}
	eligible := filterWorkflowViewportMultitaskingAggregates(allPlayers)
	playerName, err := d.playerNameForKey(playerKey)
	if err != nil {
		return workflowPlayerAsyncInsight{}, err
	}
	result := workflowPlayerAsyncInsight{
		SummaryVersion:  workflowSummaryVersion,
		PlayerKey:       playerKey,
		PlayerName:      playerName,
		InsightType:     workflowPlayerInsightTypeViewportSwitchRate,
		Title:           "Viewport switch rate",
		BetterDirection: "higher",
		PopulationSize:  int64(len(eligible)),
		Description:     "This tracks how often a player's coordinate-bearing commands jump outside the prior viewport-sized area from 7:00 until 80% of game length. Higher suggests more frequent attention shifts across the map, though it is still a proxy rather than literal camera tracking.",
	}

	playerSummary, ok := findWorkflowViewportMultitaskingAggregate(allPlayers, playerKey)
	populationMean, populationStddev := workflowViewportSwitchPopulationStats(eligible)
	result.Details = append(result.Details,
		workflowPlayerInsightDetail{Label: "Eligible players", Value: fmt.Sprintf("%d (minimum %d games)", len(eligible), workflowViewportMultitaskingMinGames)},
		workflowPlayerInsightDetail{Label: "Population mean", Value: fmt.Sprintf("%.2f switches/min", populationMean)},
		workflowPlayerInsightDetail{Label: "Population stddev", Value: fmt.Sprintf("%.2f", populationStddev)},
	)
	if !ok {
		result.IneligibleReason = "No viewport multitasking data was found for this player yet."
		return result, nil
	}

	result.Details = append(result.Details, workflowPlayerInsightDetail{Label: "Player games", Value: strconv.FormatInt(playerSummary.GamesPlayed, 10)})
	if !playerSummary.isEligible() {
		result.IneligibleReason = fmt.Sprintf("Not enough viewport samples yet for a stable comparison. This view currently requires at least %d games.", workflowViewportMultitaskingMinGames)
		return result, nil
	}

	values := make([]float64, 0, len(eligible))
	for _, player := range eligible {
		values = append(values, player.averageViewportSwitchRate)
	}
	sort.Float64s(values)
	value := playerSummary.averageViewportSwitchRate
	percentile := performancePercentileFromSortedValues(values, value, false)
	result.Eligible = true
	result.PerformancePercentile = &percentile
	result.PlayerValue = &value
	result.PlayerValueLabel = fmt.Sprintf("%.2f switches/min", value)
	return result, nil
}

func (d *Dashboard) loadWorkflowViewportMultitaskingAggregates() ([]workflowViewportMultitaskingAggregate, error) {
	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT
			lower(trim(p.name)) AS player_key,
			MIN(p.name) AS player_name,
			dp.value_string
		FROM detected_patterns_replay_player dp
		JOIN players p
			ON p.id = dp.player_id
		WHERE dp.pattern_name = ?
			AND p.is_observer = 0
			AND lower(trim(coalesce(p.type, ''))) = 'human'
			AND dp.value_string IS NOT NULL
			AND trim(dp.value_string) <> ''
		GROUP BY player_key, dp.replay_id, dp.player_id, dp.value_string
		ORDER BY player_key ASC, player_name ASC
	`, models.PatternNameViewportMultitasking)
	if err != nil {
		return nil, fmt.Errorf("failed to load viewport multitasking patterns: %w", err)
	}
	defer rows.Close()

	aggregates := map[string]*workflowViewportMultitaskingAggregate{}
	for rows.Next() {
		var playerKey string
		var playerName string
		var rawValue string
		if err := rows.Scan(&playerKey, &playerName, &rawValue); err != nil {
			return nil, fmt.Errorf("failed to parse viewport multitasking aggregate: %w", err)
		}
		rate, ok := parseViewportSwitchRate(rawValue)
		if !ok {
			continue
		}
		aggregate := aggregates[playerKey]
		if aggregate == nil {
			aggregate = &workflowViewportMultitaskingAggregate{
				PlayerKey:  playerKey,
				PlayerName: playerName,
			}
			aggregates[playerKey] = aggregate
		}
		aggregate.GamesPlayed++
		aggregate.averageViewportSwitchRate += rate
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating viewport multitasking patterns: %w", err)
	}

	out := make([]workflowViewportMultitaskingAggregate, 0, len(aggregates))
	for _, aggregate := range aggregates {
		if aggregate.GamesPlayed > 0 {
			aggregate.averageViewportSwitchRate /= float64(aggregate.GamesPlayed)
		}
		out = append(out, *aggregate)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].averageViewportSwitchRate == out[j].averageViewportSwitchRate {
			if out[i].GamesPlayed == out[j].GamesPlayed {
				return out[i].PlayerKey < out[j].PlayerKey
			}
			return out[i].GamesPlayed > out[j].GamesPlayed
		}
		return out[i].averageViewportSwitchRate > out[j].averageViewportSwitchRate
	})
	return out, nil
}

func filterWorkflowViewportMultitaskingAggregates(all []workflowViewportMultitaskingAggregate) []workflowViewportMultitaskingAggregate {
	filtered := make([]workflowViewportMultitaskingAggregate, 0, len(all))
	for _, player := range all {
		if player.isEligible() {
			filtered = append(filtered, player)
		}
	}
	return filtered
}

func findWorkflowViewportMultitaskingAggregate(all []workflowViewportMultitaskingAggregate, playerKey string) (workflowViewportMultitaskingAggregate, bool) {
	for _, player := range all {
		if player.PlayerKey == playerKey {
			return player, true
		}
	}
	return workflowViewportMultitaskingAggregate{}, false
}

func workflowViewportSwitchPopulationStats(players []workflowViewportMultitaskingAggregate) (float64, float64) {
	if len(players) == 0 {
		return 0, 0
	}
	values := make([]float64, 0, len(players))
	for _, player := range players {
		values = append(values, player.averageViewportSwitchRate)
	}
	mean := meanFloatSlice(values)
	return mean, stddevFloatSlice(values, mean)
}

func meanFloatSlice(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func stddevFloatSlice(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		diff := value - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(values)))
}

func parseViewportSwitchRate(raw string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func (d *Dashboard) populateViewportMultitaskingForGameDetail(detail *workflowGameDetail) error {
	if detail == nil {
		return nil
	}
	detail.ViewportMultitasking = []workflowGameViewportMultitaskingPlayer{}
	playerByID := map[int64]workflowGamePlayer{}
	for _, player := range detail.Players {
		playerByID[player.PlayerID] = player
		detail.ViewportMultitasking = append(detail.ViewportMultitasking, workflowGameViewportMultitaskingPlayer{
			PlayerID:         player.PlayerID,
			PlayerKey:        player.PlayerKey,
			PlayerName:       player.Name,
			Team:             player.Team,
			IsWinner:         player.IsWinner,
			Eligible:         false,
			IneligibleReason: "no viewport switch rate found for this player",
		})
	}
	if len(detail.Players) == 0 {
		return nil
	}

	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT player_id, value_string
		FROM detected_patterns_replay_player
		WHERE replay_id = ?
			AND pattern_name = ?
			AND value_string IS NOT NULL
			AND trim(value_string) <> ''
	`, detail.ReplayID, models.PatternNameViewportMultitasking)
	if err != nil {
		return fmt.Errorf("failed to load game viewport multitasking patterns: %w", err)
	}
	defer rows.Close()

	entriesByPlayerID := map[int64]workflowGameViewportMultitaskingPlayer{}
	for rows.Next() {
		var playerID int64
		var rawValue string
		if err := rows.Scan(&playerID, &rawValue); err != nil {
			return fmt.Errorf("failed to parse game viewport multitasking row: %w", err)
		}
		rate, ok := parseViewportSwitchRate(rawValue)
		if !ok {
			continue
		}
		player, ok := playerByID[playerID]
		if !ok {
			continue
		}
		entriesByPlayerID[playerID] = workflowGameViewportMultitaskingPlayer{
			PlayerID:           player.PlayerID,
			PlayerKey:          player.PlayerKey,
			PlayerName:         player.Name,
			Team:               player.Team,
			IsWinner:           player.IsWinner,
			Eligible:           true,
			ViewportSwitchRate: rate,
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed iterating game viewport multitasking rows: %w", err)
	}

	for i := range detail.ViewportMultitasking {
		playerID := detail.ViewportMultitasking[i].PlayerID
		if entry, ok := entriesByPlayerID[playerID]; ok {
			detail.ViewportMultitasking[i] = entry
		}
	}
	sort.Slice(detail.ViewportMultitasking, func(i, j int) bool {
		a := detail.ViewportMultitasking[i]
		b := detail.ViewportMultitasking[j]
		if a.Eligible != b.Eligible {
			return a.Eligible
		}
		if a.ViewportSwitchRate == b.ViewportSwitchRate {
			return a.PlayerName < b.PlayerName
		}
		return a.ViewportSwitchRate > b.ViewportSwitchRate
	})
	return nil
}

func (a workflowViewportMultitaskingAggregate) isEligible() bool {
	return a.GamesPlayed >= workflowViewportMultitaskingMinGames
}

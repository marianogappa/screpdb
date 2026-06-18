package dashboard

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	db "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/samber/lo"
)

func (d *Dashboard) buildWorkflowGameDetail(replayID int64) (workflowGameDetail, error) {
	detail := workflowGameDetail{SummaryVersion: workflowSummaryVersion}
	summary, err := d.dbStore.GetReplaySummary(d.ctx, replayID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return detail, sql.ErrNoRows
		}
		return detail, fmt.Errorf("failed to load replay: %w", err)
	}
	detail.ReplayID = summary.ReplayID
	detail.ReplayDate = summary.ReplayDate
	detail.FileName = summary.FileName
	detail.FilePath = summary.FilePath
	detail.MapName = summary.MapName
	detail.MapKind = summary.MapKind
	detail.MapVisual = d.resolveWorkflowMapVisual(detail.ReplayID, summary.MapName, summary.FilePath, summary.FileChecksum)
	detail.DurationSeconds = summary.DurationSeconds
	detail.GameType = summary.GameType
	detail.TeamStacking = summary.TeamStacking
	detail.TeamInfoIncomplete = summary.TeamInfoIncomplete

	rows, err := d.dbStore.ListReplayPlayersForDetail(d.ctx, replayID)
	if err != nil {
		return detail, fmt.Errorf("failed to load players: %w", err)
	}
	playerNames := make([]string, 0, len(rows))
	for _, row := range rows {
		playerNames = append(playerNames, row.Name)
	}
	displayByName, err := d.aliasDisplayNames(playerNames)
	if err != nil {
		return detail, fmt.Errorf("failed to resolve player aliases: %w", err)
	}

	startClockByPlayerID := map[int64]int{}
	for _, row := range rows {
		var p workflowGamePlayer
		p.PlayerID = row.PlayerID
		p.Name = row.Name
		if displayName, ok := displayByName[row.Name]; ok {
			p.Name = displayName
		}
		p.Color = row.Color
		p.Race = row.Race
		p.Team = row.Team
		p.IsWinner = row.IsWinner
		p.APM = row.APM
		p.EAPM = row.EAPM
		p.PlayerKey = normalizePlayerKey(row.Name)
		p.DetectedPatterns = []workflowPatternValue{}
		detail.Players = append(detail.Players, p)
		if row.StartLocationOclock != nil && *row.StartLocationOclock >= 1 && *row.StartLocationOclock <= 12 {
			startClockByPlayerID[row.PlayerID] = int(*row.StartLocationOclock)
		}
	}

	var mapLayout *models.MapContextLayout
	if strings.TrimSpace(summary.FilePath) != "" {
		layout, layoutErr := buildDashboardMapContextLayoutFromReplay(summary.FilePath)
		if layoutErr == nil {
			mapLayout = layout
		}
	}
	if mapLayout != nil && mapLayout.WidthTiles > 0 && mapLayout.HeightTiles > 0 {
		detail.MapWidthPixels = int64(mapLayout.WidthTiles) * 32
		detail.MapHeightPixels = int64(mapLayout.HeightTiles) * 32
	}

	if err := d.populateDetectedPatternsForGameDetail(&detail, mapLayout, startClockByPlayerID, displayByName); err != nil {
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
	if err := d.populateUnitCadenceForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populateViewportMultitaskingForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populateMarkersForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populateMutaliskTimingForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populatePhaseMarkersForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populateUnitCompositionMarkersForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populateAllianceTimelineForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populatePlayerDepartureForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populateAllianceTabChatForGameDetail(&detail); err != nil {
		return detail, err
	}

	return detail, nil
}

// populateUnitCompositionMarkersForGameDetail computes attacker-composition
// pills at request time from the persisted phase boundaries
// (mid_game_starts / late_game_starts replay-level markers) and the
// Train / Unit Morph / Cast command stream for the replay. The result
// is one row per (player, phase) where the player produced ≥1
// non-excluded attacking unit. See dashboard/unit_composition.go.
//
// Side effect: also populates detail.TrainedUnitsTimeline from the same
// row set so the event-map overlay can render per-player composition at
// any selected event without a second DB query.
func (d *Dashboard) populateUnitCompositionMarkersForGameDetail(detail *workflowGameDetail) error {
	boundaries, err := d.dbStore.GetPhaseBoundariesForReplay(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load phase boundaries: %w", err)
	}
	rows, err := d.dbStore.ListGameUnitProductionAndCasts(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load production/casts: %w", err)
	}
	detail.UnitCompositionMarkers = computeCompositionForReplay(rows, boundaries)
	detail.TrainedUnitsTimeline = trainedUnitsTimelineFromRows(rows)
	return nil
}

// trainedUnitsTimelineFromRows converts the production/cast rows into a flat
// per-player "this unit became alive at this second" stream. Filters out
// workers + Overlord (the existing compositionExcluded set). Each Train /
// Unit Morph row contributes one sample per unit name (multi-unit morphs
// like a Zergling pair contribute two samples). Cast rows are ignored —
// they don't represent a new unit on the field.
//
// The sample's Second is the command's seconds_from_game_start shifted
// forward by the unit's build/morph duration (Fastest game speed) via
// buildTimeFor. Approximation: morph source units are NOT deducted (mirrors
// computeCompositionForReplay's approach) and deaths are not tracked.
func trainedUnitsTimelineFromRows(rows []db.UnitProductionOrCastRow) []workflowTrainedUnitSample {
	out := make([]workflowTrainedUnitSample, 0, len(rows))
	for _, row := range rows {
		if row.ActionType != "Train" && row.ActionType != "Unit Morph" {
			continue
		}
		for _, name := range commandUnitNamesFromPtrs(row.UnitType, row.UnitTypes) {
			if _, excluded := compositionExcluded[name]; excluded {
				continue
			}
			delay := int64(buildTimeFor(name) + 0.5)
			out = append(out, workflowTrainedUnitSample{
				PlayerID: row.PlayerID,
				Second:   row.SecondsFromGameStart + delay,
				UnitType: name,
			})
		}
	}
	return out
}

func (d *Dashboard) populateDetectedPatternsForGameDetail(detail *workflowGameDetail, mapLayout *models.MapContextLayout, startClockByPlayerID map[int64]int, displayByName map[string]string) error {
	detail.ReplayPatterns = []workflowPatternValue{}
	detail.GameEvents = []workflowGameEvent{}

	rowsReplay, err := d.dbStore.ListReplayPatterns(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to query replay patterns: %w", err)
	}
	for _, row := range rowsReplay {
		pattern := buildWorkflowPatternValue(row.PatternName, row.Value, row.DetectedSecond, row.Payload)
		detail.ReplayPatterns = append(detail.ReplayPatterns, pattern)
	}
	eventRows, err := d.dbStore.ListReplayEvents(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to query replay events: %w", err)
	}
	detail.GameEvents = replayEventsFromRows(eventRows, mapLayout, startClockByPlayerID)
	detail.GameEvents = resolveNaturalOwnerLabels(detail.GameEvents, detail.Players, startClockByPlayerID)
	detail.GameEvents = resolveRecallTargetOwners(detail.GameEvents, detail.Players)
	detail.GameEvents = resolveAllianceTeamPlayers(detail.GameEvents, detail.Players, displayByName)
	for i := range detail.GameEvents {
		event := &detail.GameEvents[i]
		if event.Actor != nil {
			if displayName, ok := displayByName[event.Actor.Name]; ok {
				event.Actor.Name = displayName
			}
		}
		if event.Target != nil {
			if displayName, ok := displayByName[event.Target.Name]; ok {
				event.Target.Name = displayName
			}
		}
		if event.TargetOwner != nil {
			if displayName, ok := displayByName[event.TargetOwner.Name]; ok {
				event.TargetOwner.Name = displayName
			}
		}
		for ti := range event.AllianceTeams {
			team := event.AllianceTeams[ti]
			for pi := range team {
				if displayName, ok := displayByName[team[pi].Name]; ok {
					team[pi].Name = displayName
				}
			}
		}
	}

	playerByID := map[int64]*workflowGamePlayer{}
	for i := range detail.Players {
		player := &detail.Players[i]
		playerByID[player.PlayerID] = player
	}

	rowsPlayer, err := d.dbStore.ListPlayerPatterns(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to query player patterns: %w", err)
	}
	for _, row := range rowsPlayer {
		playerID := row.PlayerID
		pattern := buildWorkflowPatternValue(row.PatternName, row.Value, row.DetectedSecond, row.Payload)
		if player, ok := playerByID[playerID]; ok {
			player.DetectedPatterns = append(player.DetectedPatterns, pattern)
		}
	}
	return nil
}

// buildWorkflowPatternValue constructs the detected_patterns[] entry the frontend
// consumes. Shape is registry-driven: event_type (FeatureKey), detected_second,
// and an optional JSON payload for markers carrying extras.
func buildWorkflowPatternValue(featureKey string, _ string, detectedSecond int64, rawPayload string) workflowPatternValue {
	pv := workflowPatternValue{
		EventType:      featureKey,
		DetectedSecond: int(detectedSecond),
	}
	if rawPayload != "" && rawPayload != "true" {
		pv.Payload = json.RawMessage(rawPayload)
	}
	return pv
}

func (d *Dashboard) buildWorkflowPlayerOverview(playerKey string) (workflowPlayerOverview, error) {
	result := workflowPlayerOverview{
		SummaryVersion: workflowSummaryVersion,
		PlayerKey:      playerKey,
	}

	summary, err := d.dbStore.GetPlayerOverviewSummary(d.ctx, playerKey)
	if err != nil {
		return result, fmt.Errorf("failed to load player summary: %w", err)
	}
	result.PlayerName = summary.PlayerName
	displayByName, err := d.aliasDisplayNames([]string{summary.PlayerName})
	if err != nil {
		return result, fmt.Errorf("failed to resolve player aliases: %w", err)
	}
	if displayName, ok := displayByName[summary.PlayerName]; ok {
		result.PlayerName = displayName
	}
	result.GamesPlayed = summary.GamesPlayed
	result.Wins = summary.Wins
	result.AverageAPM = summary.AverageAPM
	result.AverageEAPM = summary.AverageEAPM
	if result.GamesPlayed == 0 {
		return result, sql.ErrNoRows
	}
	result.WinRate = float64(result.Wins) / float64(result.GamesPlayed)
	if err := d.populateAdvancedPlayerOverview(playerKey, &result); err != nil {
		return result, fmt.Errorf("failed to populate advanced player overview: %w", err)
	}

	result.NarrativeHints = buildPlayerNarrativeHints(result)
	return result, nil
}

func (d *Dashboard) buildWorkflowPlayerRecentGames(playerKey string) ([]workflowGameListItem, error) {
	playerName, err := d.playerNameForKey(playerKey)
	if err != nil {
		return nil, err
	}
	recentRows, err := d.dbStore.ListPlayerRecentGames(d.ctx, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load recent games for %s: %w", playerName, err)
	}
	result := []workflowGameListItem{}
	for _, row := range recentRows {
		g := workflowGameListItem{
			ReplayID:           row.ReplayID,
			ReplayDate:         row.ReplayDate,
			FileName:           row.FileName,
			MapName:            row.MapName,
			MapKind:            row.MapKind,
			DurationSeconds:    row.DurationSeconds,
			GameType:           row.GameType,
			Matchup:            row.Matchup,
			TeamStacking:       row.TeamStacking,
			TeamInfoIncomplete: row.TeamInfoIncomplete,
			PlayersLabel:       row.PlayersLabel,
			WinnersLabel:       row.WinnersLabel,
		}
		result = append(result, g)
	}
	if err := d.populateWorkflowGameListPlayers(result); err != nil {
		return nil, fmt.Errorf("failed to populate players for %s: %w", playerName, err)
	}
	if err := d.populateWorkflowRecentGamesCurrentPlayer(playerKey, result); err != nil {
		return nil, fmt.Errorf("failed to populate recent game context for %s: %w", playerName, err)
	}
	return result, nil
}

func (d *Dashboard) buildWorkflowPlayerApmHistogram(playerKey string) (workflowPlayerApmHistogram, error) {
	const minGames int64 = 5
	result := workflowPlayerApmHistogram{
		SummaryVersion: workflowSummaryVersion,
		PlayerKey:      playerKey,
		MinGames:       minGames,
		Bins:           []workflowPlayerApmHistogramBin{},
		Players:        []workflowPlayerApmHistogramPoint{},
		PlayerEligible: false,
	}

	rows, err := d.dbStore.ListPlayerApmAggregates(d.ctx, minGames)
	if err != nil {
		return result, err
	}
	playerNames := make([]string, 0, len(rows))
	for _, row := range rows {
		playerNames = append(playerNames, row.PlayerName)
	}
	displayByName, err := d.aliasDisplayNames(playerNames)
	if err != nil {
		return result, err
	}

	values := []float64{}
	playerValue := 0.0
	for _, row := range rows {
		key := row.PlayerKey
		name := row.PlayerName
		avgAPM := row.AverageAPM
		gamesPlayed := row.GamesPlayed
		if avgAPM <= 0 {
			continue
		}
		values = append(values, avgAPM)
		result.Players = append(result.Players, workflowPlayerApmHistogramPoint{
			PlayerKey:   key,
			PlayerName:  lo.Ternary(displayByName[name] != "", displayByName[name], name),
			AverageAPM:  avgAPM,
			GamesPlayed: gamesPlayed,
		})
		if key == playerKey {
			playerValue = avgAPM
			result.PlayerEligible = true
		}
	}
	if len(values) == 0 {
		return result, nil
	}

	sort.Float64s(values)
	result.PlayersIncluded = int64(len(values))

	sum := 0.0
	for _, value := range values {
		sum += value
	}
	mean := sum / float64(len(values))
	result.MeanAPM = mean

	varianceSum := 0.0
	for _, value := range values {
		diff := value - mean
		varianceSum += diff * diff
	}
	result.StddevAPM = math.Sqrt(varianceSum / float64(len(values)))

	binCount := int(math.Round(math.Sqrt(float64(len(values)))))
	if binCount < 8 {
		binCount = 8
	}
	if binCount > 24 {
		binCount = 24
	}
	minValue := values[0]
	maxValue := values[len(values)-1]
	if maxValue <= minValue {
		result.Bins = []workflowPlayerApmHistogramBin{{
			X0:    minValue,
			X1:    minValue + 1,
			Count: int64(len(values)),
		}}
	} else {
		width := (maxValue - minValue) / float64(binCount)
		if width <= 0 {
			width = 1
		}
		bins := make([]workflowPlayerApmHistogramBin, binCount)
		for i := 0; i < binCount; i++ {
			start := minValue + float64(i)*width
			end := minValue + float64(i+1)*width
			if i == binCount-1 {
				end = maxValue
			}
			bins[i] = workflowPlayerApmHistogramBin{X0: start, X1: end, Count: 0}
		}
		for _, value := range values {
			idx := int(math.Floor((value - minValue) / width))
			if idx < 0 {
				idx = 0
			}
			if idx >= binCount {
				idx = binCount - 1
			}
			bins[idx].Count++
		}
		result.Bins = bins
	}

	sort.Slice(result.Players, func(i, j int) bool {
		if result.Players[i].AverageAPM == result.Players[j].AverageAPM {
			return result.Players[i].PlayerName < result.Players[j].PlayerName
		}
		return result.Players[i].AverageAPM < result.Players[j].AverageAPM
	})

	if result.PlayerEligible {
		value := playerValue
		result.PlayerAverageAPM = &value
		position := sort.SearchFloat64s(values, value)
		percentile := (float64(position) / float64(len(values))) * 100
		result.PlayerPercentile = &percentile
	}
	return result, nil
}

type workflowPlayerUnitCadenceReplayMetric struct {
	ReplayID        int64
	PlayerKey       string
	PlayerName      string
	FileName        string
	DurationSeconds int64
	WindowSeconds   int64
	UnitsProduced   int64
	GapCount        int64
	RatePerMinute   float64
	CVGap           float64
	Burstiness      float64
	Idle20Ratio     float64
	CadenceScore    float64
}

func workflowUnitCadenceExcludedUnits(filterMode workflowUnitCadenceFilterMode) []string {
	if filterMode == workflowUnitCadenceFilterBroad {
		return []string{"SCV", "Probe", "Drone", "Overlord"}
	}
	return []string{
		"SCV",
		"Probe",
		"Drone",
		"Overlord",
		"Observer",
		"Shuttle",
		"Science Vessel",
		"Medic",
		"Dropship",
		"Defiler",
		"Queen",
		"Nuclear Missile",
	}
}

func (d *Dashboard) queryWorkflowUnitCadenceReplayMetrics(filterMode workflowUnitCadenceFilterMode, onlyPlayerKey string) ([]workflowPlayerUnitCadenceReplayMetric, error) {
	excludedUnits := workflowUnitCadenceExcludedUnits(filterMode)
	if len(excludedUnits) == 0 {
		return nil, errors.New("workflow unit cadence requires at least one excluded unit")
	}
	rows, err := d.dbStore.ListUnitCadenceReplayMetrics(
		d.ctx,
		excludedUnits,
		onlyPlayerKey,
		workflowUnitCadenceStartSeconds,
		workflowUnitCadenceEndFraction,
		workflowUnitCadenceIdleGapSeconds,
		workflowUnitCadenceMinUnitsPerReplay,
		workflowUnitCadenceMinGapsPerReplay,
	)
	if err != nil {
		return nil, err
	}
	result := []workflowPlayerUnitCadenceReplayMetric{}
	for _, row := range rows {
		result = append(result, workflowPlayerUnitCadenceReplayMetric{
			ReplayID:        row.ReplayID,
			PlayerKey:       row.PlayerKey,
			PlayerName:      row.PlayerName,
			FileName:        row.FileName,
			DurationSeconds: row.DurationSeconds,
			WindowSeconds:   row.WindowSeconds,
			UnitsProduced:   row.UnitsProduced,
			GapCount:        row.GapCount,
			RatePerMinute:   row.RatePerMinute,
			CVGap:           row.CVGap,
			Burstiness:      row.Burstiness,
			Idle20Ratio:     row.Idle20Ratio,
			CadenceScore:    row.CadenceScore,
		})
	}
	return result, nil
}

func (d *Dashboard) buildWorkflowPlayerUnitCadenceLeaderboard(filterMode workflowUnitCadenceFilterMode, minGames int64, limit int64) (workflowPlayerUnitCadenceLeaderboard, error) {
	result := workflowPlayerUnitCadenceLeaderboard{
		SummaryVersion:    workflowSummaryVersion,
		FilterMode:        filterMode,
		StartSecond:       workflowUnitCadenceStartSeconds,
		EndFraction:       workflowUnitCadenceEndFraction,
		IdleGapSeconds:    workflowUnitCadenceIdleGapSeconds,
		MinUnitsPerReplay: workflowUnitCadenceMinUnitsPerReplay,
		MinGapsPerReplay:  workflowUnitCadenceMinGapsPerReplay,
		MinGames:          minGames,
		Bins:              []workflowPlayerUnitCadenceHistogramBin{},
		Players:           []workflowPlayerUnitCadencePoint{},
	}
	if minGames <= 0 {
		return result, errors.New("min games must be > 0")
	}
	if limit < 0 {
		return result, errors.New("limit must be >= 0")
	}
	if limit > workflowUnitCadenceMaxLimit {
		limit = workflowUnitCadenceMaxLimit
	}
	replays, err := d.queryWorkflowUnitCadenceReplayMetrics(filterMode, "")
	if err != nil {
		return result, err
	}
	type agg struct {
		name       string
		games      int64
		sumRate    float64
		sumCV      float64
		sumBurst   float64
		sumIdle    float64
		sumCadence float64
	}
	byPlayer := map[string]*agg{}
	for _, replay := range replays {
		entry, ok := byPlayer[replay.PlayerKey]
		if !ok {
			entry = &agg{name: replay.PlayerName}
			byPlayer[replay.PlayerKey] = entry
		}
		entry.games++
		entry.sumRate += replay.RatePerMinute
		entry.sumCV += replay.CVGap
		entry.sumBurst += replay.Burstiness
		entry.sumIdle += replay.Idle20Ratio
		entry.sumCadence += replay.CadenceScore
		if strings.TrimSpace(entry.name) == "" {
			entry.name = replay.PlayerName
		}
	}
	for playerKey, entry := range byPlayer {
		if entry.games < minGames {
			continue
		}
		denom := float64(entry.games)
		result.Players = append(result.Players, workflowPlayerUnitCadencePoint{
			PlayerKey:         playerKey,
			PlayerName:        entry.name,
			GamesUsed:         entry.games,
			AverageRatePerMin: entry.sumRate / denom,
			AverageCVGap:      entry.sumCV / denom,
			AverageBurstiness: entry.sumBurst / denom,
			AverageIdle20:     entry.sumIdle / denom,
			AverageCadence:    entry.sumCadence / denom,
		})
	}
	sort.Slice(result.Players, func(i, j int) bool {
		if result.Players[i].AverageCadence == result.Players[j].AverageCadence {
			if result.Players[i].GamesUsed == result.Players[j].GamesUsed {
				return result.Players[i].PlayerName < result.Players[j].PlayerName
			}
			return result.Players[i].GamesUsed > result.Players[j].GamesUsed
		}
		return result.Players[i].AverageCadence > result.Players[j].AverageCadence
	})
	if limit > 0 && int64(len(result.Players)) > limit {
		result.Players = result.Players[:limit]
	}
	result.PlayersIncluded = int64(len(result.Players))
	if len(result.Players) == 0 {
		return result, nil
	}
	values := make([]float64, 0, len(result.Players))
	for _, player := range result.Players {
		values = append(values, player.AverageCadence)
	}
	sort.Float64s(values)
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	mean := sum / float64(len(values))
	result.MeanCadence = mean
	varianceSum := 0.0
	for _, value := range values {
		diff := value - mean
		varianceSum += diff * diff
	}
	result.StddevCadence = math.Sqrt(varianceSum / float64(len(values)))

	binCount := int(math.Round(math.Sqrt(float64(len(values)))))
	if binCount < 8 {
		binCount = 8
	}
	if binCount > 24 {
		binCount = 24
	}
	minValue := values[0]
	maxValue := values[len(values)-1]
	if maxValue <= minValue {
		result.Bins = []workflowPlayerUnitCadenceHistogramBin{{
			X0:    minValue,
			X1:    minValue + 1,
			Count: int64(len(values)),
		}}
		return result, nil
	}
	width := (maxValue - minValue) / float64(binCount)
	if width <= 0 {
		width = 1
	}
	bins := make([]workflowPlayerUnitCadenceHistogramBin, binCount)
	for i := 0; i < binCount; i++ {
		start := minValue + float64(i)*width
		end := minValue + float64(i+1)*width
		if i == binCount-1 {
			end = maxValue
		}
		bins[i] = workflowPlayerUnitCadenceHistogramBin{X0: start, X1: end, Count: 0}
	}
	for _, value := range values {
		idx := int(math.Floor((value - minValue) / width))
		if idx < 0 {
			idx = 0
		}
		if idx >= binCount {
			idx = binCount - 1
		}
		bins[idx].Count++
	}
	result.Bins = bins
	return result, nil
}

func (d *Dashboard) buildWorkflowPlayerUnitCadenceInsight(playerKey string, filterMode workflowUnitCadenceFilterMode) (workflowPlayerUnitCadenceInsight, error) {
	result := workflowPlayerUnitCadenceInsight{
		SummaryVersion:    workflowSummaryVersion,
		PlayerKey:         playerKey,
		FilterMode:        filterMode,
		StartSecond:       workflowUnitCadenceStartSeconds,
		EndFraction:       workflowUnitCadenceEndFraction,
		IdleGapSeconds:    workflowUnitCadenceIdleGapSeconds,
		MinUnitsPerReplay: workflowUnitCadenceMinUnitsPerReplay,
		MinGapsPerReplay:  workflowUnitCadenceMinGapsPerReplay,
		Replays:           []workflowPlayerUnitCadenceReplay{},
	}
	playerName, err := d.dbStore.GetPlayerNameByKey(d.ctx, playerKey)
	if err != nil {
		return result, err
	}
	result.PlayerName = playerName
	if result.PlayerName == "" {
		return result, sql.ErrNoRows
	}
	replays, err := d.queryWorkflowUnitCadenceReplayMetrics(filterMode, playerKey)
	if err != nil {
		return result, err
	}
	if len(replays) == 0 {
		return result, nil
	}
	for _, replay := range replays {
		result.Replays = append(result.Replays, workflowPlayerUnitCadenceReplay{
			ReplayID:        replay.ReplayID,
			FileName:        replay.FileName,
			DurationSeconds: replay.DurationSeconds,
			WindowSeconds:   replay.WindowSeconds,
			UnitsProduced:   replay.UnitsProduced,
			GapCount:        replay.GapCount,
			RatePerMinute:   replay.RatePerMinute,
			CVGap:           replay.CVGap,
			Burstiness:      replay.Burstiness,
			Idle20Ratio:     replay.Idle20Ratio,
			CadenceScore:    replay.CadenceScore,
		})
		result.GamesUsed++
		result.AverageRatePerMin += replay.RatePerMinute
		result.AverageCVGap += replay.CVGap
		result.AverageBurstiness += replay.Burstiness
		result.AverageIdle20 += replay.Idle20Ratio
		result.AverageCadenceScore += replay.CadenceScore
	}
	if result.GamesUsed > 0 {
		denom := float64(result.GamesUsed)
		result.AverageRatePerMin /= denom
		result.AverageCVGap /= denom
		result.AverageBurstiness /= denom
		result.AverageIdle20 /= denom
		result.AverageCadenceScore /= denom
	}
	sort.Slice(result.Replays, func(i, j int) bool {
		if result.Replays[i].CadenceScore == result.Replays[j].CadenceScore {
			return result.Replays[i].ReplayID < result.Replays[j].ReplayID
		}
		return result.Replays[i].CadenceScore > result.Replays[j].CadenceScore
	})
	return result, nil
}

var errUnsupportedWorkflowPlayerInsightType = errors.New("unsupported workflow player insight type")

func (d *Dashboard) buildWorkflowPlayerAsyncInsight(playerKey string, insightType workflowPlayerInsightType) (workflowPlayerAsyncInsight, error) {
	switch insightType {
	case workflowPlayerInsightTypeAPM:
		return d.buildWorkflowPlayerApmAsyncInsight(playerKey)
	case workflowPlayerInsightTypeUnitCadence:
		return d.buildWorkflowPlayerCadenceAsyncInsight(playerKey)
	case workflowPlayerInsightTypeViewportSwitchRate:
		return d.buildWorkflowPlayerViewportAsyncInsight(playerKey)
	default:
		return workflowPlayerAsyncInsight{}, errUnsupportedWorkflowPlayerInsightType
	}
}

func (d *Dashboard) buildWorkflowPlayerApmAsyncInsight(playerKey string) (workflowPlayerAsyncInsight, error) {
	histogram, err := d.buildWorkflowPlayerApmHistogram(playerKey)
	if err != nil {
		return workflowPlayerAsyncInsight{}, err
	}
	playerName, err := d.playerNameForKey(playerKey)
	if err != nil {
		return workflowPlayerAsyncInsight{}, err
	}
	result := workflowPlayerAsyncInsight{
		SummaryVersion:  workflowSummaryVersion,
		PlayerKey:       playerKey,
		PlayerName:      playerName,
		InsightType:     workflowPlayerInsightTypeAPM,
		Title:           "APM",
		BetterDirection: "higher",
		PopulationSize:  histogram.PlayersIncluded,
		Description:     "Average actions per minute across this player's non-observer human games. Higher tends to mean more activity, but it is still contextual rather than a direct skill rating.",
		Details: []workflowPlayerInsightDetail{
			{Label: "Eligible players", Value: fmt.Sprintf("%d (minimum %d games)", histogram.PlayersIncluded, histogram.MinGames)},
			{Label: "Population mean", Value: fmt.Sprintf("%.1f APM", histogram.MeanAPM)},
			{Label: "Population stddev", Value: fmt.Sprintf("%.1f", histogram.StddevAPM)},
		},
	}
	if !histogram.PlayerEligible || histogram.PlayerAverageAPM == nil {
		result.IneligibleReason = fmt.Sprintf("Not enough games yet for a stable APM comparison. This view currently requires at least %d games.", histogram.MinGames)
		return result, nil
	}
	percentile := performancePercentileFromSortedValues(extractApmValues(histogram.Players), *histogram.PlayerAverageAPM, false)
	value := *histogram.PlayerAverageAPM
	result.Eligible = true
	result.PerformancePercentile = &percentile
	result.PlayerValue = &value
	result.PlayerValueLabel = fmt.Sprintf("%.1f APM", value)
	playerGames := int64(0)
	for _, player := range histogram.Players {
		if player.PlayerKey == playerKey {
			playerGames = player.GamesPlayed
			break
		}
	}
	result.Details = append(result.Details,
		workflowPlayerInsightDetail{Label: "Player games", Value: strconv.FormatInt(playerGames, 10)},
		workflowPlayerInsightDetail{Label: "Interpretation", Value: "Green means this player sits nearer the high-APM end of the eligible population."},
	)
	return result, nil
}

func (d *Dashboard) buildWorkflowPlayerCadenceAsyncInsight(playerKey string) (workflowPlayerAsyncInsight, error) {
	leaderboard, err := d.buildWorkflowPlayerUnitCadenceLeaderboard(workflowUnitCadenceFilterStrict, workflowUnitCadenceMinGames, 0)
	if err != nil {
		return workflowPlayerAsyncInsight{}, err
	}
	insight, err := d.buildWorkflowPlayerUnitCadenceInsight(playerKey, workflowUnitCadenceFilterStrict)
	if err != nil {
		return workflowPlayerAsyncInsight{}, err
	}
	result := workflowPlayerAsyncInsight{
		SummaryVersion:  workflowSummaryVersion,
		PlayerKey:       playerKey,
		PlayerName:      insight.PlayerName,
		InsightType:     workflowPlayerInsightTypeUnitCadence,
		Title:           "Unit production cadence",
		BetterDirection: "higher",
		PopulationSize:  leaderboard.PlayersIncluded,
		Description:     "Cadence looks at attacking-unit production rhythm from 7:00 until 80% game length. For each eligible game we combine unit rate and evenness using cadence = ratePerMin / (1 + cvGap), where cvGap is gap stddev divided by gap mean. Higher is better.",
		Details: []workflowPlayerInsightDetail{
			{Label: "Eligible players", Value: fmt.Sprintf("%d (minimum %d games)", leaderboard.PlayersIncluded, leaderboard.MinGames)},
			{Label: "Population mean", Value: fmt.Sprintf("%.3f", leaderboard.MeanCadence)},
			{Label: "Population stddev", Value: fmt.Sprintf("%.3f", leaderboard.StddevCadence)},
		},
	}
	if insight.GamesUsed < leaderboard.MinGames {
		result.IneligibleReason = fmt.Sprintf("Not enough eligible games yet. This view currently requires at least %d games with enough production events.", leaderboard.MinGames)
		return result, nil
	}
	values := extractCadenceValues(leaderboard.Players)
	percentile := performancePercentileFromSortedValues(values, insight.AverageCadenceScore, false)
	value := insight.AverageCadenceScore
	result.Eligible = true
	result.PerformancePercentile = &percentile
	result.PlayerValue = &value
	result.PlayerValueLabel = fmt.Sprintf("%.3f cadence", value)
	result.Details = append(result.Details,
		workflowPlayerInsightDetail{Label: "Games used", Value: strconv.FormatInt(insight.GamesUsed, 10)},
		workflowPlayerInsightDetail{Label: "Average rate/min", Value: fmt.Sprintf("%.2f", insight.AverageRatePerMin)},
		workflowPlayerInsightDetail{Label: "Average gap CV", Value: fmt.Sprintf("%.2f", insight.AverageCVGap)},
		workflowPlayerInsightDetail{Label: "Average idle-gap ratio (>=20s)", Value: fmt.Sprintf("%.1f%%", insight.AverageIdle20*100)},
	)
	return result, nil
}

func extractApmValues(players []workflowPlayerApmHistogramPoint) []float64 {
	values := make([]float64, 0, len(players))
	for _, player := range players {
		values = append(values, player.AverageAPM)
	}
	sort.Float64s(values)
	return values
}

func extractCadenceValues(players []workflowPlayerUnitCadencePoint) []float64 {
	values := make([]float64, 0, len(players))
	for _, player := range players {
		values = append(values, player.AverageCadence)
	}
	sort.Float64s(values)
	return values
}

func performancePercentileFromSortedValues(sortedValues []float64, playerValue float64, lowerIsBetter bool) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	if len(sortedValues) == 1 {
		return 100
	}
	first := sort.Search(len(sortedValues), func(i int) bool {
		return sortedValues[i] >= playerValue
	})
	last := sort.Search(len(sortedValues), func(i int) bool {
		return sortedValues[i] > playerValue
	}) - 1
	if first >= len(sortedValues) {
		first = len(sortedValues) - 1
	}
	if last < first {
		last = first
	}
	midRank := float64(first+last) / 2.0
	denom := float64(len(sortedValues) - 1)
	if lowerIsBetter {
		return 100 * ((denom - midRank) / denom)
	}
	return 100 * (midRank / denom)
}

func (d *Dashboard) topActionTypesForPlayer(playerID int64, limit int) ([]string, error) {
	return d.dbStore.ListTopActionTypes(d.ctx, playerID, limit)
}

type overlayBaseMeta struct {
	Base       workflowGameEventBase
	IsStarting bool
}

func replayEventsFromRows(rows []db.ReplayEventRow, mapLayout *models.MapContextLayout, startClockByPlayerID map[int64]int) []workflowGameEvent {
	baseMetas := overlayBaseMetasFromLayout(mapLayout)
	baseByKey := map[string]workflowGameEventBase{}
	ownershipByBaseKey := map[string]*workflowGameEventPlayer{}
	events := make([]workflowGameEvent, 0, len(rows))
	for _, row := range rows {
		event := workflowGameEvent{
			Type:             row.EventType,
			Second:           row.Second,
			Ownership:        []workflowGameOwnership{},
			AttackUnitTypes:  parseAttackUnitTypes(row.AttackUnitTypes),
			AttackCastCounts: parseAttackCastCounts(row.AttackCastCounts),
		}
		if row.SourcePlayerID != nil {
			event.Actor = &workflowGameEventPlayer{
				PlayerID: *row.SourcePlayerID,
				Name:     row.SourcePlayerName,
				Color:    row.SourcePlayerColor,
			}
		}
		if row.TargetPlayerID != nil {
			event.Target = &workflowGameEventPlayer{
				PlayerID: *row.TargetPlayerID,
				Name:     row.TargetPlayerName,
				Color:    row.TargetPlayerColor,
			}
		}
		if row.LocationBaseType != nil || row.LocationBaseOclock != nil {
			if matchedBase, ok := lookupOverlayBase(baseMetas, row.LocationBaseType, row.LocationBaseOclock, row.LocationNaturalOfClock); ok {
				baseCopy := matchedBase
				if label := baseLabel(row.LocationBaseType, row.LocationBaseOclock, row.LocationNaturalOfClock); strings.TrimSpace(label) != "" {
					baseCopy.Name = label
				}
				baseCopy.NaturalOfClock = row.LocationNaturalOfClock
				if row.LocationMineralOnly != nil && *row.LocationMineralOnly {
					baseCopy.MineralOnly = row.LocationMineralOnly
				}
				event.Base = &baseCopy
				baseByKey[baseKeyForEvent(&event)] = baseCopy
			}
		}
		if event.Base == nil && (row.LocationBaseType != nil || row.LocationBaseOclock != nil) {
			base := workflowGameEventBase{
				Name:           baseLabel(row.LocationBaseType, row.LocationBaseOclock, row.LocationNaturalOfClock),
				NaturalOfClock: row.LocationNaturalOfClock,
				Center: workflowGameEventPoint{
					X: 0,
					Y: 0,
				},
			}
			if row.LocationBaseType != nil {
				base.Kind = *row.LocationBaseType
			}
			if row.LocationBaseOclock != nil {
				base.Clock = *row.LocationBaseOclock
			}
			if row.LocationMineralOnly != nil && *row.LocationMineralOnly {
				base.MineralOnly = row.LocationMineralOnly
			}
			event.Base = &base
			baseByKey[baseKeyForEvent(&event)] = base
		}
		// Open-field 1v1 attacks (issue #186) have no base columns but carry a
		// relational location ("in the middle", "near White's base") in the
		// payload — surface it as the event's location label.
		if event.Base == nil && row.EventType == "attack" && row.Payload != nil {
			if loc, x, y, ok := parseAttackRelativeLocation(*row.Payload); ok {
				event.Base = &workflowGameEventBase{
					Name:   loc,
					Kind:   "open_field",
					Center: workflowGameEventPoint{X: x, Y: y},
				}
			}
		}
		if event.Actor != nil {
			if startClock, ok := startClockByPlayerID[event.Actor.PlayerID]; ok {
				event.ActorStartClock = lo.ToPtr(int64(startClock))
				if startBase, startBaseOK := lookupOverlayBaseByClock(baseMetas, int64(startClock)); startBaseOK {
					startCenter := startBase.Center
					event.ActorOrigin = &startCenter
				}
			}
		}
		if event.Target != nil {
			if startClock, ok := startClockByPlayerID[event.Target.PlayerID]; ok {
				event.TargetStartClock = lo.ToPtr(int64(startClock))
			}
		}
		applyOwnershipTransition(&event, ownershipByBaseKey)
		event.Ownership = ownershipSnapshot(ownershipByBaseKey, baseByKey)
		if event.ActorOrigin == nil && event.Actor != nil {
			if fallbackBase, ok := fallbackActorOriginFromOwnership(event.Actor.PlayerID, ownershipByBaseKey, baseByKey); ok {
				center := fallbackBase.Center
				event.ActorOrigin = &center
			}
		}
		if event.Type == "recall" && row.Payload != nil && *row.Payload != "" {
			applyRecallPayload(&event, *row.Payload, baseMetas)
		}
		if isDropEventType(event.Type) && row.Payload != nil && *row.Payload != "" {
			applyDropPayload(&event, *row.Payload, baseMetas)
		}
		if event.Type == "late_alliance" && row.Payload != nil && *row.Payload != "" {
			applyAlliancePayload(&event, *row.Payload)
		}
		events = append(events, event)
	}
	return events
}

// applyAlliancePayload decodes the {"teams":[["A","B"],["C"]]} payload that
// parser.BuildAllianceDerivedEvents writes for late_alliance events and stamps
// event.AllianceTeams with name-only player entries. PlayerID and Color are
// filled in a second pass (resolveAllianceTeamPlayers) which has access to the
// player roster.
func applyAlliancePayload(event *workflowGameEvent, raw string) {
	var pl struct {
		Teams [][]string `json:"teams"`
	}
	if err := json.Unmarshal([]byte(raw), &pl); err != nil {
		return
	}
	teams := make([][]workflowGameEventPlayer, 0, len(pl.Teams))
	for _, team := range pl.Teams {
		if len(team) < 2 {
			continue
		}
		row := make([]workflowGameEventPlayer, 0, len(team))
		for _, name := range team {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			row = append(row, workflowGameEventPlayer{Name: name})
		}
		if len(row) >= 2 {
			teams = append(teams, row)
		}
	}
	if len(teams) > 0 {
		event.AllianceTeams = teams
	}
}

// resolveAllianceTeamPlayers fills PlayerID and Color on each entry of every
// event's AllianceTeams. Mirrors the post-hoc resolution pattern used by
// resolveRecallTargetOwners — applyAlliancePayload runs inside
// replayEventsFromRows where the player roster isn't available.
//
// Lookups are keyed by both the raw player name (as stored in the alliance
// payload) and the display name (post displayByName aliasing) to cover both
// cases — detail.Players carries the display name in .Name, so a raw-name
// payload entry like "chobo86" would otherwise miss the lookup for a player
// whose display name is "chobo86 (you)".
func resolveAllianceTeamPlayers(events []workflowGameEvent, players []workflowGamePlayer, displayByName map[string]string) []workflowGameEvent {
	if len(events) == 0 {
		return events
	}
	byName := make(map[string]*workflowGamePlayer, len(players)*2)
	for i := range players {
		byName[players[i].Name] = &players[i]
	}
	// Map raw payload names back to the patched display-name entry in detail.Players.
	for raw, display := range displayByName {
		if p, ok := byName[display]; ok {
			byName[raw] = p
		}
	}
	for i := range events {
		if events[i].Type != "late_alliance" || len(events[i].AllianceTeams) == 0 {
			continue
		}
		for ti := range events[i].AllianceTeams {
			team := events[i].AllianceTeams[ti]
			for pi := range team {
				if p, ok := byName[team[pi].Name]; ok {
					team[pi].PlayerID = p.PlayerID
					team[pi].Color = p.Color
				}
			}
		}
	}
	return events
}

// isDropEventType reports whether an event_type is one of the drop variants
// emitted by worldstate.emitDropEvents.
func isDropEventType(t string) bool {
	switch t {
	case "drop", "reaver_drop", "cliff_drop":
		return true
	}
	return false
}

// applyDropPayload decodes a drop event's payload (same shape as the recall
// payload, separate JSON type for clarity) and stamps the source/target
// points, target base, and count/last-second fields onto the event.
func applyDropPayload(event *workflowGameEvent, raw string, baseMetas []overlayBaseMeta) {
	// The drop payload shape mirrors recallEventPayload; reuse the type.
	var pl recallEventPayload
	if err := json.Unmarshal([]byte(raw), &pl); err != nil {
		return
	}
	if len(pl.S) == 2 {
		event.SourcePoint = &workflowGameEventPoint{X: float64(pl.S[0]), Y: float64(pl.S[1])}
	}
	if len(pl.T) == 2 {
		event.TargetPoint = &workflowGameEventPoint{X: float64(pl.T[0]), Y: float64(pl.T[1])}
	}
	if pl.N > 1 {
		event.DropCount = int64(pl.N)
	}
	if pl.LE > 0 {
		event.DropLastSecond = int64(pl.LE)
	}
	event.DropTargetVia = pl.TV
	if pl.SB != nil {
		var baseTypePtr *string
		if pl.SB.K != "" {
			k := pl.SB.K
			baseTypePtr = &k
		}
		oclockCopy := pl.SB.O
		var oclockPtr *int64
		if pl.SB.K != "" || oclockCopy != 0 {
			oclockPtr = &oclockCopy
		}
		if matched, ok := lookupOverlayBase(baseMetas, baseTypePtr, oclockPtr, pl.SB.NO); ok {
			baseCopy := matched
			if label := baseLabel(baseTypePtr, oclockPtr, pl.SB.NO); strings.TrimSpace(label) != "" {
				baseCopy.Name = label
			}
			baseCopy.NaturalOfClock = pl.SB.NO
			if pl.SB.MO != nil && *pl.SB.MO {
				baseCopy.MineralOnly = pl.SB.MO
			}
			event.SourceBase = &baseCopy
		}
	}
	if pl.TB != nil {
		var baseTypePtr *string
		if pl.TB.K != "" {
			k := pl.TB.K
			baseTypePtr = &k
		}
		oclockCopy := pl.TB.O
		var oclockPtr *int64
		if pl.TB.K != "" || oclockCopy != 0 {
			oclockPtr = &oclockCopy
		}
		if matched, ok := lookupOverlayBase(baseMetas, baseTypePtr, oclockPtr, pl.TB.NO); ok {
			baseCopy := matched
			if label := baseLabel(baseTypePtr, oclockPtr, pl.TB.NO); strings.TrimSpace(label) != "" {
				baseCopy.Name = label
			}
			baseCopy.NaturalOfClock = pl.TB.NO
			if pl.TB.MO != nil && *pl.TB.MO {
				baseCopy.MineralOnly = pl.TB.MO
			}
			event.TargetBase = &baseCopy
		}
	}
}

// recallEventPayload mirrors the on-disk JSON shape produced by
// worldstate.emitRecallEvents. Short keys keep storage cost down — the
// canonical mapping is documented at the worldstate side. The drop payload
// also reuses this shape with an extra SB field for source base (recall's
// source IS event.base; drops need a separate source ref).
type recallEventPayload struct {
	N  int                       `json:"n,omitempty"`
	LE int                       `json:"le,omitempty"`
	S  []int                     `json:"s,omitempty"`
	SB *recallEventTargetBaseRef `json:"sb,omitempty"`
	T  []int                     `json:"t,omitempty"`
	TB *recallEventTargetBaseRef `json:"tb,omitempty"`
	TP int64                     `json:"tp,omitempty"`
	TV string                    `json:"tv,omitempty"`
}

type recallEventTargetBaseRef struct {
	K  string `json:"k,omitempty"`
	O  int64  `json:"o,omitempty"`
	NO *int64 `json:"no,omitempty"`
	MO *bool  `json:"mo,omitempty"`
}

// applyRecallPayload decodes the recall event's Payload JSON and populates
// the recall-specific fields on workflowGameEvent (source/target points,
// target base, count, last-second). The target_owner field — which needs
// the per-replay player list to resolve PID → name/color — is handled in a
// second pass after replayEventsFromRows returns.
func applyRecallPayload(event *workflowGameEvent, raw string, baseMetas []overlayBaseMeta) {
	var pl recallEventPayload
	if err := json.Unmarshal([]byte(raw), &pl); err != nil {
		return
	}
	if len(pl.S) == 2 {
		event.SourcePoint = &workflowGameEventPoint{X: float64(pl.S[0]), Y: float64(pl.S[1])}
	}
	if len(pl.T) == 2 {
		event.TargetPoint = &workflowGameEventPoint{X: float64(pl.T[0]), Y: float64(pl.T[1])}
	}
	if pl.N > 1 {
		event.RecallCount = int64(pl.N)
	}
	if pl.LE > 0 {
		event.RecallLastSecond = int64(pl.LE)
	}
	event.RecallTargetVia = pl.TV
	if pl.TB != nil {
		var baseTypePtr *string
		if pl.TB.K != "" {
			k := pl.TB.K
			baseTypePtr = &k
		}
		oclockCopy := pl.TB.O
		var oclockPtr *int64
		if pl.TB.K != "" || oclockCopy != 0 {
			oclockPtr = &oclockCopy
		}
		if matched, ok := lookupOverlayBase(baseMetas, baseTypePtr, oclockPtr, pl.TB.NO); ok {
			baseCopy := matched
			if label := baseLabel(baseTypePtr, oclockPtr, pl.TB.NO); strings.TrimSpace(label) != "" {
				baseCopy.Name = label
			}
			baseCopy.NaturalOfClock = pl.TB.NO
			if pl.TB.MO != nil && *pl.TB.MO {
				baseCopy.MineralOnly = pl.TB.MO
			}
			event.TargetBase = &baseCopy
		}
	}
}

// resolveRecallTargetOwners is the post-pass that fills in event.TargetOwner
// for recall events using the supplied PID → workflowGameEventPlayer lookup.
// Split from applyRecallPayload because replayEventsFromRows has no access
// to the per-replay players list — that's owned by populateDetectedPatternsForGameDetail.
// resolveNaturalOwnerLabels relabels a natural-expansion location with its
// owner's player name ("Skins_'s natural") instead of the start-clock form
// ("7's natural"), when the natural's natural_of_clock matches a player's
// start location (issue #186 review). Falls through unchanged when no owner is
// found, preserving the clock-based baseLabel.
func resolveNaturalOwnerLabels(events []workflowGameEvent, players []workflowGamePlayer, startClockByPlayerID map[int64]int) []workflowGameEvent {
	if len(events) == 0 || len(startClockByPlayerID) == 0 {
		return events
	}
	nameByID := make(map[int64]string, len(players))
	for _, p := range players {
		nameByID[p.PlayerID] = p.Name
	}
	nameByClock := map[int64]string{}
	for id, clock := range startClockByPlayerID {
		if name, ok := nameByID[id]; ok {
			nameByClock[int64(clock)] = name
		}
	}
	for i := range events {
		b := events[i].Base
		if b == nil || strings.ToLower(b.Kind) != "natural" || b.NaturalOfClock == nil {
			continue
		}
		if name, ok := nameByClock[*b.NaturalOfClock]; ok && strings.TrimSpace(name) != "" {
			b.Name = name + "'s natural"
		}
	}
	return events
}

func resolveRecallTargetOwners(events []workflowGameEvent, players []workflowGamePlayer) []workflowGameEvent {
	if len(events) == 0 || len(players) == 0 {
		return events
	}
	playerByID := make(map[int64]workflowGamePlayer, len(players))
	for _, p := range players {
		playerByID[p.PlayerID] = p
	}
	for i := range events {
		if events[i].Type != "recall" && !isDropEventType(events[i].Type) {
			continue
		}
		if events[i].TargetBase == nil {
			continue
		}
		// The recall/drop payload is no longer present at this layer (events
		// came out of replayEventsFromRows). We re-derive target_owner via
		// the existing event.Target field — populated upstream from the row's
		// target_player_id when worldstate set it. For both recall and drop,
		// worldstate writes Target when the destination's owner is hostile,
		// so Target ⇔ TargetOwner here.
		if events[i].Target != nil {
			cp := *events[i].Target
			if p, ok := playerByID[cp.PlayerID]; ok {
				cp.Name = p.Name
				cp.Color = p.Color
			}
			events[i].TargetOwner = &cp
		}
	}
	return events
}

func overlayBaseMetasFromLayout(layout *models.MapContextLayout) []overlayBaseMeta {
	if layout == nil || len(layout.Bases) == 0 {
		return nil
	}
	// scmapanalyzer annotates each start base with the Name of its natural.
	// Build: natural_base_name -> start_clock, so we can stamp NaturalOfClock
	// onto the natural base's overlay metadata. This lets the render-time
	// lookup distinguish a natural from an unrelated expa that happens to
	// share the same o'clock position (previously they collapsed to the
	// same (kind, clock) key and painted the wrong polygon).
	startClockByNaturalName := map[string]int64{}
	for _, base := range layout.Bases {
		if !strings.EqualFold(strings.TrimSpace(base.Kind), "start") {
			continue
		}
		naturalName := strings.TrimSpace(base.NaturalExpansion)
		if naturalName == "" {
			continue
		}
		startClockByNaturalName[naturalName] = int64(base.Clock)
	}
	out := make([]overlayBaseMeta, 0, len(layout.Bases))
	for _, base := range layout.Bases {
		polygon := make([]workflowGameEventPoint, 0, len(base.Polygon))
		for _, vertex := range base.Polygon {
			polygon = append(polygon, workflowGameEventPoint{X: float64(vertex.X), Y: float64(vertex.Y)})
		}
		kind := strings.TrimSpace(base.Kind)
		prettyName := strings.TrimSpace(base.Name)
		if prettyName == "" {
			if base.Clock == 0 {
				prettyName = "center base"
			} else if base.Clock >= 1 && base.Clock <= 12 {
				prettyName = fmt.Sprintf("%d", base.Clock)
			}
		}
		isStarting := strings.EqualFold(kind, "start") || strings.EqualFold(kind, "starting")
		var naturalOfClock *int64
		if !isStarting {
			if clock, ok := startClockByNaturalName[strings.TrimSpace(base.Name)]; ok {
				clockCopy := clock
				naturalOfClock = &clockCopy
			}
		}
		out = append(out, overlayBaseMeta{
			Base: workflowGameEventBase{
				Name:           prettyName,
				Kind:           kind,
				Clock:          int64(base.Clock),
				NaturalOfClock: naturalOfClock,
				MineralOnly:    lo.Ternary(base.MineralOnly, lo.ToPtr(true), nil),
				Center:         workflowGameEventPoint{X: float64(base.Center.X), Y: float64(base.Center.Y)},
				Polygon:        polygon,
			},
			IsStarting: isStarting,
		})
	}
	return out
}

func lookupOverlayBase(baseMetas []overlayBaseMeta, baseType *string, baseOclock *int64, naturalOfOclock *int64) (workflowGameEventBase, bool) {
	if baseOclock == nil {
		return workflowGameEventBase{}, false
	}
	targetClock := *baseOclock
	targetType := strings.ToLower(strings.TrimSpace(nullableString(baseType)))
	// Primary pass: match by (kind, clock[, natural_of_clock]). The
	// natural_of_clock component is what disambiguates a natural from a
	// coincident expa at the same clock.
	for _, candidate := range baseMetas {
		if candidate.Base.Clock != targetClock {
			continue
		}
		switch targetType {
		case "starting":
			if !candidate.IsStarting {
				continue
			}
		case "natural":
			if candidate.IsStarting {
				continue
			}
			if candidate.Base.NaturalOfClock == nil {
				continue
			}
			if naturalOfOclock == nil || *candidate.Base.NaturalOfClock != *naturalOfOclock {
				continue
			}
		default: // "expansion" and anything else
			if candidate.IsStarting {
				continue
			}
			if candidate.Base.NaturalOfClock != nil {
				continue
			}
		}
		return candidate.Base, true
	}
	// Secondary fallback: kind-agnostic clock match, preserving prior behavior
	// when the primary pass fails (e.g. layout missing or natural-of-clock
	// unmapped). Keeps rendering best-effort rather than dropping the polygon.
	for _, candidate := range baseMetas {
		if candidate.Base.Clock != targetClock {
			continue
		}
		if targetType == "starting" && !candidate.IsStarting {
			continue
		}
		if targetType != "starting" && candidate.IsStarting {
			continue
		}
		return candidate.Base, true
	}
	for _, candidate := range baseMetas {
		if candidate.Base.Clock == targetClock {
			return candidate.Base, true
		}
	}
	return workflowGameEventBase{}, false
}

func lookupOverlayBaseByClock(baseMetas []overlayBaseMeta, clock int64) (workflowGameEventBase, bool) {
	for _, candidate := range baseMetas {
		if candidate.Base.Clock == clock && candidate.IsStarting {
			return candidate.Base, true
		}
	}
	for _, candidate := range baseMetas {
		if candidate.Base.Clock == clock {
			return candidate.Base, true
		}
	}
	return workflowGameEventBase{}, false
}

func baseKeyForEvent(event *workflowGameEvent) string {
	if event == nil || event.Base == nil {
		return ""
	}
	kind := strings.ToLower(strings.TrimSpace(event.Base.Kind))
	// Disambiguate naturals by the clock of the start they belong to — two
	// different players' naturals can sit at the same clock, and an expa
	// can share a clock with a natural. Without natural_of_clock in the
	// key, ownership bookkeeping collapses them.
	if kind == "natural" && event.Base.NaturalOfClock != nil {
		return fmt.Sprintf("natural|%d|%d", *event.Base.NaturalOfClock, event.Base.Clock)
	}
	if event.Base.Clock >= 0 && (event.Base.Clock > 0 || hasValidCenterBaseKind(kind)) {
		return fmt.Sprintf("%s|%d", kind, event.Base.Clock)
	}
	return fmt.Sprintf("%s|%s", kind, strings.ToLower(strings.TrimSpace(event.Base.Name)))
}

// hasValidCenterBaseKind returns true for event base kinds that can legitimately
// carry clock=0 (the "center base" emitted by scmapanalyzer for maps with a
// rich expansion in the middle). Without this, center bases would silently
// fall through to name-based keying.
func hasValidCenterBaseKind(kind string) bool {
	switch kind {
	case "start", "starting", "natural", "expansion", "expa":
		return true
	}
	return false
}

func applyOwnershipTransition(event *workflowGameEvent, ownership map[string]*workflowGameEventPlayer) {
	if event == nil {
		return
	}
	eventType := strings.ToLower(strings.TrimSpace(event.Type))
	switch eventType {
	case "player_start", "expansion", "takeover":
		baseKey := baseKeyForEvent(event)
		if baseKey != "" && event.Actor != nil {
			ownerCopy := *event.Actor
			ownership[baseKey] = &ownerCopy
		}
	case "location_inactive":
		baseKey := baseKeyForEvent(event)
		if baseKey != "" {
			delete(ownership, baseKey)
		}
	case "leave_game":
		if event.Actor == nil {
			return
		}
		for key, owner := range ownership {
			if owner != nil && owner.PlayerID == event.Actor.PlayerID {
				delete(ownership, key)
			}
		}
	}
}

func ownershipSnapshot(ownership map[string]*workflowGameEventPlayer, baseByKey map[string]workflowGameEventBase) []workflowGameOwnership {
	if len(ownership) == 0 {
		return []workflowGameOwnership{}
	}
	keys := make([]string, 0, len(ownership))
	for key := range ownership {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]workflowGameOwnership, 0, len(keys))
	for _, key := range keys {
		owner := ownership[key]
		base, ok := baseByKey[key]
		if !ok {
			continue
		}
		var ownerCopy *workflowGameEventPlayer
		if owner != nil {
			value := *owner
			ownerCopy = &value
		}
		out = append(out, workflowGameOwnership{Base: base, Owner: ownerCopy})
	}
	return out
}

func fallbackActorOriginFromOwnership(playerID int64, ownership map[string]*workflowGameEventPlayer, baseByKey map[string]workflowGameEventBase) (workflowGameEventBase, bool) {
	for key, owner := range ownership {
		if owner == nil || owner.PlayerID != playerID {
			continue
		}
		base, ok := baseByKey[key]
		if !ok {
			continue
		}
		if strings.EqualFold(base.Kind, "start") || strings.EqualFold(base.Kind, "starting") {
			return base, true
		}
	}
	for key, owner := range ownership {
		if owner == nil || owner.PlayerID != playerID {
			continue
		}
		base, ok := baseByKey[key]
		if ok {
			return base, true
		}
	}
	return workflowGameEventBase{}, false
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func baseLabel(baseType *string, baseOclock *int64, naturalOf *int64) string {
	if baseType == nil {
		return ""
	}
	// oclock==0 means scmapanalyzer's "center base". None of the templated
	// labels ("9", "12's natural near 6", "an expansion near 3") read right
	// when inserted with 0, so short-circuit to a clear literal.
	isCenter := func(v *int64) bool { return v != nil && *v == 0 }
	switch strings.ToLower(strings.TrimSpace(*baseType)) {
	case "starting":
		if isCenter(baseOclock) {
			return "center base"
		}
		if baseOclock != nil {
			return fmt.Sprintf("%d", *baseOclock)
		}
		return "starting base"
	case "natural":
		if isCenter(baseOclock) {
			if isCenter(naturalOf) {
				return "center base"
			}
			if naturalOf != nil {
				return fmt.Sprintf("%d's natural (center base)", *naturalOf)
			}
			return "center base"
		}
		if naturalOf != nil {
			if baseOclock != nil && *baseOclock != *naturalOf {
				return fmt.Sprintf("%d's natural near %d", *naturalOf, *baseOclock)
			}
			return fmt.Sprintf("%d's natural", *naturalOf)
		}
		if baseOclock != nil {
			return fmt.Sprintf("%d's natural", *baseOclock)
		}
		return "natural expansion"
	default:
		if isCenter(baseOclock) {
			return "center base"
		}
		if baseOclock != nil {
			return fmt.Sprintf("an expansion near %d", *baseOclock)
		}
		return "expansion"
	}
}

// parseAttackRelativeLocation extracts the open-field relational location
// label ("loc") and the fight centroid (pixels) from an attack event's payload
// (issue #186). ok is false for any payload that doesn't carry a label.
func parseAttackRelativeLocation(raw string) (loc string, x, y float64, ok bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", 0, 0, false
	}
	var p struct {
		Loc string  `json:"loc"`
		X   float64 `json:"x"`
		Y   float64 `json:"y"`
	}
	if err := json.Unmarshal([]byte(trimmed), &p); err != nil {
		return "", 0, 0, false
	}
	label := strings.TrimSpace(p.Loc)
	if label == "" {
		return "", 0, 0, false
	}
	return label, p.X, p.Y, true
}

func parseAttackUnitTypes(raw *string) []string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	var unitTypes []string
	if err := json.Unmarshal([]byte(trimmed), &unitTypes); err != nil {
		return nil
	}
	filtered := make([]string, 0, len(unitTypes))
	seen := map[string]struct{}{}
	for _, unitType := range unitTypes {
		name := strings.TrimSpace(unitType)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		filtered = append(filtered, name)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func parseAttackCastCounts(raw *string) map[string]int64 {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	var counts map[string]int64
	if err := json.Unmarshal([]byte(trimmed), &counts); err != nil {
		return nil
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

func formatClockFromSeconds(second int64) string {
	if second < 0 {
		second = 0
	}
	minute := second / 60
	sec := second % 60
	return fmt.Sprintf("%d:%02d", minute, sec)
}

// workflowSliceBoundaries returns the start-second of each post-4-minute slice
// row for the production table. The first 4 minutes (0-239s) are rendered
// separately as a time-scaled vertical chart (UnitsEarlyEvents), not bucketed.
func workflowSliceBoundaries(durationSeconds int64) []int64 {
	base := []int64{240, 300, 360, 420, 600, 900, 1200, 1500, 1800, 2400, 3000, 3600}
	boundaries := []int64{}
	for _, point := range base {
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
	detail.UnitsEarlyEvents = []workflowUnitEarlyEventPlayer{}
	detail.ProductionTimeline = []workflowProductionTimelinePlayer{}
	playerOrder := make([]int64, 0, len(detail.Players))
	playerByID := map[int64]workflowGamePlayer{}
	for _, player := range detail.Players {
		playerOrder = append(playerOrder, player.PlayerID)
		playerByID[player.PlayerID] = player
	}

	rows, err := d.dbStore.ListUnitSliceCommandRows(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load unit slices: %w", err)
	}

	buildingSet := make(map[string]struct{}, len(models.Buildings))
	for _, b := range models.Buildings {
		buildingSet[b] = struct{}{}
	}

	perSlice := map[int64]map[int64]map[string]int64{}
	boundaries := workflowSliceBoundaries(detail.DurationSeconds)
	earlyEventsByPlayer := map[int64][]workflowUnitEarlyEvent{}
	timelineByPlayer := map[int64][]workflowProductionEvent{}
	workerOrdinalByPlayer := map[int64]map[string]int64{}
	for _, row := range rows {
		playerID := row.PlayerID
		second := row.Second
		unitType := row.UnitType
		if second < 0 {
			second = 0
		}
		// One Zergling Unit Morph command produces a pair of Zerglings
		// from a single larva. Source rows have Count=1 per command, so
		// double the increment for Zerglings.
		inc := int64(1)
		if unitType == models.GeneralUnitZergling {
			inc = 2
		}

		_, eventIsBuilding := buildingSet[unitType]
		timelineByPlayer[playerID] = append(timelineByPlayer[playerID], workflowProductionEvent{
			Second:     second,
			UnitType:   unitType,
			IsBuilding: eventIsBuilding,
			Count:      inc,
		})

		if second < 240 {
			_, isBuilding := buildingSet[unitType]
			label := ""
			if unitType == models.GeneralUnitSCV || unitType == models.GeneralUnitDrone || unitType == models.GeneralUnitProbe {
				if _, ok := workerOrdinalByPlayer[playerID]; !ok {
					workerOrdinalByPlayer[playerID] = map[string]int64{}
				}
				workerOrdinalByPlayer[playerID][unitType]++
				// Players begin every game with 4 starting workers, so the
				// first worker we observe being trained is the 5th overall.
				n := workerOrdinalByPlayer[playerID][unitType] + 4
				label = fmt.Sprintf("%d%s %s", n, ordinalSuffix(int(n)), unitType)
			}
			earlyEventsByPlayer[playerID] = append(earlyEventsByPlayer[playerID], workflowUnitEarlyEvent{
				Second:     second,
				UnitType:   unitType,
				IsBuilding: isBuilding,
				Label:      label,
				Count:      inc,
			})
			continue
		}

		sliceStart := sliceStartForSecond(second, boundaries)
		if _, ok := perSlice[sliceStart]; !ok {
			perSlice[sliceStart] = map[int64]map[string]int64{}
		}
		if _, ok := perSlice[sliceStart][playerID]; !ok {
			perSlice[sliceStart][playerID] = map[string]int64{}
		}
		perSlice[sliceStart][playerID][unitType] += inc
	}
	for _, playerID := range playerOrder {
		player := playerByID[playerID]
		events := earlyEventsByPlayer[playerID]
		if events == nil {
			events = []workflowUnitEarlyEvent{}
		}
		sort.SliceStable(events, func(i, j int) bool { return events[i].Second < events[j].Second })
		detail.UnitsEarlyEvents = append(detail.UnitsEarlyEvents, workflowUnitEarlyEventPlayer{
			PlayerID:  player.PlayerID,
			PlayerKey: player.PlayerKey,
			Name:      player.Name,
			Events:    events,
		})

		timeline := timelineByPlayer[playerID]
		if timeline == nil {
			timeline = []workflowProductionEvent{}
		}
		sort.SliceStable(timeline, func(i, j int) bool { return timeline[i].Second < timeline[j].Second })
		detail.ProductionTimeline = append(detail.ProductionTimeline, workflowProductionTimelinePlayer{
			PlayerID:  player.PlayerID,
			PlayerKey: player.PlayerKey,
			Name:      player.Name,
			Events:    timeline,
		})
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
	gasRows, err := d.dbStore.ListGasTimingRows(d.ctx, detail.ReplayID)
	if err != nil {
		return err
	}
	gas, err := d.playerTimingsFromReplayCommands(detail.Players, gasRows)
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

	upgradeRows, err := d.dbStore.ListUpgradeTimingRows(d.ctx, detail.ReplayID)
	if err != nil {
		return err
	}
	upgrades, err := d.playerLabeledTimingsFromReplayCommands(detail.Players, upgradeRows)
	if err != nil {
		return err
	}
	timings.Upgrades = upgrades

	techRows, err := d.dbStore.ListTechTimingRows(d.ctx, detail.ReplayID)
	if err != nil {
		return err
	}
	tech, err := d.playerLabeledTimingsFromReplayCommands(detail.Players, techRows)
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

	rows, err := d.dbStore.ListFirstUnitCommandRows(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load first unit efficiency commands: %w", err)
	}

	for _, row := range rows {
		playerID := row.PlayerID
		second := row.Second
		actionType := row.ActionType
		unitType := row.UnitType
		unitTypes := row.UnitTypes
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

// populatePhaseMarkersForGameDetail copies the persisted Early/Mid
// game-end seconds onto the response so the frontend can split the Game
// Events list into three sections. The boundaries come from the
// mid_game_starts / late_game_starts markers persisted at ingest by
// detectors.PhaseBoundaryDetector (which runs phases.Compute over the
// enriched stream). Reading the persisted markers here keeps this
// surface in lockstep with attacker-composition (which also reads them
// via GetPhaseBoundariesForReplay) and avoids the action-type drift
// that broke a request-time recomputation against raw rows.
//
// Algorithm (lives in internal/patterns/phases — see PhaseBoundaryDetector):
//
//   - earlyEnd: earliest first-occurrence across all players of
//     Mutalisk / Lurker / Wraith / max(Siege Tank, Tank Siege Mode) /
//     max(Dragoon, Singularity Charge) / Reaver / Dark Templar.
//   - midEnd:   earliest of Defiler / Arbiter / Carrier / Battlecruiser /
//     Ultralisk / any Terran ground or armor +2.
//
// Either field stays 0 when the corresponding marker isn't present
// (game ended before that signal); the frontend collapses empty
// sections.
func (d *Dashboard) populatePhaseMarkersForGameDetail(detail *workflowGameDetail) error {
	boundaries, err := d.dbStore.GetPhaseBoundariesForReplay(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load phase-boundary markers: %w", err)
	}
	detail.EarlyGameEndsAtSecond = boundaries.EarlyEndsAtSecond
	detail.MidGameEndsAtSecond = boundaries.MidEndsAtSecond
	return nil
}

// zergBOEventSchema describes the per-BO event list shown in the Build
// Orders detail tab for the simplified count-based Zerg BOs. Drones is
// the ordered drone-morph count to render (1st, 2nd, ..., Nth Drone);
// the boolean fields control whether to emit a Pool / Overlord / Hatchery
// row. Pool/Overlord/Hatch ticks come from each player's command stream
// at game-detail time (not persisted per detection).
type zergBOEventSchema struct {
	Drones      int
	HasOverlord bool
	HasPool     bool
	HasHatchery bool
}

// buildZergBOEvents builds the per-event timeline rows for one of the
// simplified Zerg BOs. Drone events are numbered (1st, 2nd, ..., Nth)
// from the player's command stream; the optional Overlord / Pool /
// Hatchery rows append the first observed time. Expert (golden) ranges
// from the marker definition are attached when available, else NoExpert
// is set so the frontend renders the actual tick alone.
func buildZergBOEvents(schema zergBOEventSchema, bo *markers.Marker, t db.EarlyZergTimingsRow) []workflowMarkerEvent {
	expertBySubject := map[string]*markers.ExpertEvent{}
	for i := range bo.Expert {
		e := &bo.Expert[i]
		expertBySubject[e.Match.Subject] = e
	}
	events := make([]workflowMarkerEvent, 0, schema.Drones+3)

	// Labels start at "5th" — the player begins with 4 starting drones, so
	// the first morphed drone is the 5th overall.
	const startingDrones = 4
	for i := 0; i < schema.Drones; i++ {
		ordinal := i + 1 + startingDrones
		ev := workflowMarkerEvent{
			Key:      fmt.Sprintf("%d%s Drone", ordinal, ordinalSuffix(ordinal)),
			Subject:  models.GeneralUnitDrone,
			NoExpert: true,
		}
		if i < len(t.DroneMorphSecs) {
			ev.Found = true
			ev.ActualSecond = int64(t.DroneMorphSecs[i])
		}
		events = append(events, ev)
	}
	if schema.HasOverlord {
		ev := workflowMarkerEvent{Key: "Overlord", Subject: models.GeneralUnitOverlord, NoExpert: true}
		if t.FirstOverlordSec != nil {
			ev.Found = true
			ev.ActualSecond = int64(*t.FirstOverlordSec)
		}
		events = append(events, ev)
	}
	if schema.HasPool {
		ev := workflowMarkerEvent{Key: "Spawning Pool", Subject: models.GeneralUnitSpawningPool}
		if t.FirstPoolSec != nil {
			ev.Found = true
			ev.ActualSecond = int64(*t.FirstPoolSec)
		}
		if exp, ok := expertBySubject[models.GeneralUnitSpawningPool]; ok {
			ev.TargetSecond = int64(exp.TargetSecond)
			ev.ToleranceEarlySeconds = int64(exp.Tolerance.EarlySeconds)
			ev.ToleranceLateSeconds = int64(exp.Tolerance.LateSeconds)
			if ev.Found {
				ev.DeltaSeconds = ev.ActualSecond - ev.TargetSecond
				ev.WithinTolerance = (ev.DeltaSeconds >= -ev.ToleranceEarlySeconds) &&
					(ev.DeltaSeconds <= ev.ToleranceLateSeconds)
			}
		} else {
			ev.NoExpert = true
		}
		events = append(events, ev)
	}
	if schema.HasHatchery {
		ev := workflowMarkerEvent{Key: "Hatchery", Subject: models.GeneralUnitHatchery}
		if t.FirstHatcherySec != nil {
			ev.Found = true
			ev.ActualSecond = int64(*t.FirstHatcherySec)
		}
		if exp, ok := expertBySubject[models.GeneralUnitHatchery]; ok {
			ev.TargetSecond = int64(exp.TargetSecond)
			ev.ToleranceEarlySeconds = int64(exp.Tolerance.EarlySeconds)
			ev.ToleranceLateSeconds = int64(exp.Tolerance.LateSeconds)
			if ev.Found {
				ev.DeltaSeconds = ev.ActualSecond - ev.TargetSecond
				ev.WithinTolerance = (ev.DeltaSeconds >= -ev.ToleranceEarlySeconds) &&
					(ev.DeltaSeconds <= ev.ToleranceLateSeconds)
			}
		} else {
			ev.NoExpert = true
		}
		events = append(events, ev)
	}
	return events
}

var zergBOEventSchemas = map[string]zergBOEventSchema{
	"bo_4_pool":     {Drones: 0, HasPool: true},
	"bo_9_pool":     {Drones: 5, HasPool: true},
	"bo_9_overpool": {Drones: 5, HasOverlord: true, HasPool: true},
	"bo_12_pool":    {Drones: 8, HasOverlord: true, HasPool: true},
	"bo_9_hatch":    {Drones: 5, HasHatchery: true},
	"bo_10_hatch":   {Drones: 6, HasOverlord: true, HasHatchery: true},
	"bo_11_hatch":   {Drones: 7, HasOverlord: true, HasHatchery: true},
	"bo_12_hatch":   {Drones: 8, HasOverlord: true, HasHatchery: true},
}

// ordinalSuffix returns the English ordinal suffix for an integer.
func ordinalSuffix(n int) string {
	if n%100 >= 11 && n%100 <= 13 {
		return "th"
	}
	switch n % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	}
	return "th"
}

// populateMarkersForGameDetail walks each player's detected build-order
// patterns and attaches one expert-vs-actual comparison entry per (player ×
// detected BO) to the detail's Markers field. Actual milestone timings are
// read from each marker's persisted payload (resolved once at detection
// time), so this path doesn't re-parse or re-resolve commands.
//
// BO broad definitions overlap on purpose (e.g. a "9 pool into hatchery" game
// also matches "9 pool"), so multiple entries can surface for the same player.
// Registry ordering in internal/patterns/markers drives the display order.
func (d *Dashboard) populateMarkersForGameDetail(detail *workflowGameDetail) error {
	detail.Markers = []workflowMarkerPlayer{}
	if len(detail.Players) == 0 {
		return nil
	}

	// Per-player Zerg morph / build timings (queried once for the replay).
	// Used to render simplified-Zerg BO events (drone-numbered ticks +
	// pool / overlord / hatchery) without re-parsing the replay.
	zergTimings := map[int64]db.EarlyZergTimingsRow{}
	if rows, err := d.dbStore.LoadEarlyZergTimings(d.ctx, detail.ReplayID); err == nil {
		for _, r := range rows {
			zergTimings[r.PlayerID] = r
		}
	}

	// Read pattern rows including their payload — payload carries the
	// resolved Expert milestone seconds (set at detection time). Post
	// markers-migration row.PatternName is the marker FeatureKey (e.g.
	// "bo_9_pool"); resolve through the registry rather than matching
	// "Build Order: <Name>" prefixes.
	patternRows, err := d.dbStore.ListPlayerPatterns(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load player patterns for build orders: %w", err)
	}
	type detectedBO struct {
		FeatureKey string
		Payload    string
	}
	detectedByPlayer := map[int64]map[string]detectedBO{}
	for _, row := range patternRows {
		featureKey := strings.TrimSpace(row.PatternName)
		marker := markers.ByFeatureKey(featureKey)
		if marker == nil || marker.Kind != markers.KindInitialBuildOrder {
			continue
		}
		if detectedByPlayer[row.PlayerID] == nil {
			detectedByPlayer[row.PlayerID] = map[string]detectedBO{}
		}
		detectedByPlayer[row.PlayerID][strings.ToLower(marker.FeatureKey)] = detectedBO{
			FeatureKey: marker.FeatureKey,
			Payload:    row.Payload,
		}
	}

	// One chart per (player × detected BO). Broad definitions overlap on
	// purpose (e.g. "9 pool" and "9 pool into hatchery" can both match the
	// same game) — render every match so the user can interpret them.
	// Registry order drives display order so specific variants sit next to
	// their general cousins.
	allMarkers := markers.Markers()
	// Start-location label per player, read from the player_start events
	// already populated on the timeline. Lets the FE render "X starts at L and
	// opens with BO" on the consolidated openers row.
	startLocationByPlayer := map[int64]string{}
	for i := range detail.GameEvents {
		ev := &detail.GameEvents[i]
		if strings.EqualFold(ev.Type, "player_start") && ev.Actor != nil && ev.Base != nil {
			startLocationByPlayer[ev.Actor.PlayerID] = ev.Base.Name
		}
	}
	// boOpeners accumulates one entry per (player × detected opener BO). They
	// are surfaced as a single consolidated "bo_openers" game event at second 0
	// (see below) rather than one timed event per BO — the per-BO timing was
	// uninformative and the per-event rows read as noise. Every player gets at
	// least one entry (with an empty BuildOrder when none was resolved) so the
	// FE always shows the player's name, residual/uncalculated BO or not.
	var boOpeners []workflowGameEventBuildOrder
	anyBO := false
	for _, player := range detail.Players {
		detected := detectedByPlayer[player.PlayerID]
		playerHadOpener := false
		for i := range allMarkers {
			if len(detected) == 0 {
				break
			}
			bo := &allMarkers[i]
			if bo.Kind != markers.KindInitialBuildOrder {
				continue
			}
			row, ok := detected[strings.ToLower(bo.FeatureKey)]
			if !ok {
				continue
			}
			var events []workflowMarkerEvent
			if schema, isSimplifiedZerg := zergBOEventSchemas[bo.FeatureKey]; isSimplifiedZerg {
				events = buildZergBOEvents(schema, bo, zergTimings[player.PlayerID])
			} else {
				actuals := markers.DecodeExpertActuals([]byte(row.Payload))
				events = make([]workflowMarkerEvent, 0, len(bo.Expert))
				for idx, expert := range bo.Expert {
					event := workflowMarkerEvent{
						Key:                   expert.Key,
						Subject:               expert.Match.Subject,
						TargetSecond:          int64(expert.TargetSecond),
						ToleranceEarlySeconds: int64(expert.Tolerance.EarlySeconds),
						ToleranceLateSeconds:  int64(expert.Tolerance.LateSeconds),
					}
					if idx < len(actuals) && actuals[idx].Found {
						event.Found = true
						event.ActualSecond = int64(actuals[idx].Second)
						event.DeltaSeconds = event.ActualSecond - event.TargetSecond
						event.WithinTolerance = (event.DeltaSeconds >= -event.ToleranceEarlySeconds) &&
							(event.DeltaSeconds <= event.ToleranceLateSeconds)
					}
					events = append(events, event)
				}
			}
			detail.Markers = append(detail.Markers, workflowMarkerPlayer{
				PlayerID:   player.PlayerID,
				PlayerKey:  player.PlayerKey,
				Name:       player.Name,
				Race:       player.Race,
				Marker:     bo.Name,
				FeatureKey: bo.FeatureKey,
				Events:     events,
			})

			// Surface the BO on the consolidated openers event. We still
			// gate on a resolved first building (per user spec — units
			// don't count): a BO that only matched via produce facts has
			// no concrete opening to show. The actual second is no longer
			// used (the consolidated event sits at 0:00), only the gate.
			if _, ok := firstBuildingActualSecond(bo, events); ok {
				boOpeners = append(boOpeners, workflowGameEventBuildOrder{
					PlayerID:      player.PlayerID,
					Name:          player.Name,
					Color:         player.Color,
					Race:          player.Race,
					IsWinner:      player.IsWinner,
					Team:          player.Team,
					StartLocation: startLocationByPlayer[player.PlayerID],
					BuildOrder:    bo.Name,
					FeatureKey:    bo.FeatureKey,
				})
				playerHadOpener = true
				anyBO = true
			}
		}
		// No resolved opener for this player — still emit a name-only entry so
		// the FE shows the player on the openers row and labels their start.
		if !playerHadOpener {
			boOpeners = append(boOpeners, workflowGameEventBuildOrder{
				PlayerID:      player.PlayerID,
				Name:          player.Name,
				Color:         player.Color,
				Race:          player.Race,
				IsWinner:      player.IsWinner,
				Team:          player.Team,
				StartLocation: startLocationByPlayer[player.PlayerID],
			})
		}
	}
	// Emit the openers as one consolidated event at second 0 — one entry per
	// (player × BO), plus a name-only entry for players without one. The FE
	// groups by player (one line each) and labels each starting location on the
	// map. Type starts with "bo_" so the ownership back-propagation below hands
	// it the starting-position snapshot. Skip entirely when no BO was resolved
	// for anyone (nothing meaningful to open the timeline with).
	if anyBO {
		detail.GameEvents = append(detail.GameEvents, workflowGameEvent{
			Type:        "bo_openers",
			Second:      0,
			BuildOrders: boOpeners,
		})
	}
	// detail.GameEvents was already populated by populateDetectedPatterns
	// in time-sorted order — re-sort after appending BO entries so the
	// timeline stays monotonic.
	sort.SliceStable(detail.GameEvents, func(i, j int) bool {
		return detail.GameEvents[i].Second < detail.GameEvents[j].Second
	})
	// Propagate ownership snapshots forward to BO events. replayEventsFromRows
	// computed Ownership for every replay_events row by walking transitions in
	// time order, but synthesized BO events are appended here without that
	// data. BOs don't change ownership themselves, so each one inherits the
	// snapshot from the most recent prior event — which gives the FE the
	// same set of base polygons it would draw if a real game_event fired at
	// the BO's second.
	var prevOwnership []workflowGameOwnership
	for i := range detail.GameEvents {
		ev := &detail.GameEvents[i]
		if strings.HasPrefix(ev.Type, "bo_") {
			ev.Ownership = prevOwnership
			continue
		}
		prevOwnership = ev.Ownership
	}
	return nil
}

// populateMutaliskTimingForGameDetail emits the chart payload for the
// "Mutalisk Timing" tab — one entry per side (Z + T) when both the
// mutalisk_timing and turret_timing markers fired. The payload schema mirrors
// workflowMarkerPlayer so the existing BuildOrderTimelineRows component
// renders it unchanged. Actual timings are read from each marker's persisted
// payload (set by the Custom evaluator at detection time); Expert ranges come
// from the marker definition.
func (d *Dashboard) populateMutaliskTimingForGameDetail(detail *workflowGameDetail) error {
	if detail == nil {
		return errors.New("nil game detail")
	}
	detail.MutaliskTiming = []workflowMarkerPlayer{}
	if len(detail.Players) == 0 {
		return nil
	}

	patternRows, err := d.dbStore.ListPlayerPatterns(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load player patterns for mutalisk timing: %w", err)
	}

	type sidePayload struct {
		spireCmd       int
		firstMutaCmd   int
		ebayCmd        int
		firstTurretCmd int
		hasZ           bool
		hasT           bool
	}
	byPlayer := map[int64]*sidePayload{}
	zergPlayerID, terranPlayerID := int64(-1), int64(-1)
	for _, row := range patternRows {
		key := strings.TrimSpace(row.PatternName)
		if key != "mutalisk_timing" && key != "turret_timing" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(row.Payload), &raw); err != nil {
			continue
		}
		payload, ok := byPlayer[row.PlayerID]
		if !ok {
			payload = &sidePayload{}
			byPlayer[row.PlayerID] = payload
		}
		switch key {
		case "mutalisk_timing":
			payload.hasZ = true
			zergPlayerID = row.PlayerID
			if v, ok := raw["spire_cmd"].(float64); ok {
				payload.spireCmd = int(v)
			}
			if v, ok := raw["first_muta_cmd"].(float64); ok {
				payload.firstMutaCmd = int(v)
			}
		case "turret_timing":
			payload.hasT = true
			terranPlayerID = row.PlayerID
			if v, ok := raw["ebay_cmd"].(float64); ok {
				payload.ebayCmd = int(v)
			}
			if v, ok := raw["first_turret_cmd"].(float64); ok {
				payload.firstTurretCmd = int(v)
			}
		}
	}

	if zergPlayerID < 0 || terranPlayerID < 0 {
		return nil
	}
	zMarker := markers.ByFeatureKey("mutalisk_timing")
	tMarker := markers.ByFeatureKey("turret_timing")
	if zMarker == nil || tMarker == nil {
		return nil
	}

	// Each side has 2 events: the prerequisite building, then the unit/turret
	// it gates. The chart renders trigger time + dotted build span + "built"
	// time per event. The "built" second clamps to max(prereq_built, trigger)
	// + build_time so queued commands don't fictitiously land before their
	// prerequisite. No Expert tolerance bands — the only progamer reference
	// is the muta-vs-turret completion gap (see summary below).
	type sideEventSpec struct {
		key       string
		subject   string
		buildTime int64
		actual    int
	}
	makeEvents := func(specs []sideEventSpec) []workflowMarkerEvent {
		events := make([]workflowMarkerEvent, 0, len(specs))
		// Track the previous (= prerequisite) event's actual built second so
		// the next event's built time clamps against it.
		var prevActualBuilt int64 = 0
		for _, sp := range specs {
			ev := workflowMarkerEvent{
				Key:              sp.key,
				Subject:          sp.subject,
				BuildTimeSeconds: sp.buildTime,
			}
			if sp.actual > 0 {
				ev.Found = true
				ev.ActualSecond = int64(sp.actual)
				effStart := ev.ActualSecond
				if prevActualBuilt > effStart {
					effStart = prevActualBuilt
				}
				ev.ActualBuiltSecond = effStart + sp.buildTime
				prevActualBuilt = ev.ActualBuiltSecond
			}
			events = append(events, ev)
		}
		return events
	}

	for _, player := range detail.Players {
		payload, ok := byPlayer[player.PlayerID]
		if !ok || payload == nil {
			continue
		}
		switch {
		case payload.hasZ:
			detail.MutaliskTiming = append(detail.MutaliskTiming, workflowMarkerPlayer{
				PlayerID:   player.PlayerID,
				PlayerKey:  player.PlayerKey,
				Name:       player.Name,
				Race:       player.Race,
				Marker:     zMarker.Name,
				FeatureKey: zMarker.FeatureKey,
				Events: makeEvents([]sideEventSpec{
					{key: "Spire", subject: models.GeneralUnitSpire, buildTime: int64(models.BuildTimeSpire), actual: payload.spireCmd},
					{key: "First Mutalisk", subject: models.GeneralUnitMutalisk, buildTime: int64(models.BuildTimeMutalisk), actual: payload.firstMutaCmd},
				}),
			})
		case payload.hasT:
			detail.MutaliskTiming = append(detail.MutaliskTiming, workflowMarkerPlayer{
				PlayerID:   player.PlayerID,
				PlayerKey:  player.PlayerKey,
				Name:       player.Name,
				Race:       player.Race,
				Marker:     tMarker.Name,
				FeatureKey: tMarker.FeatureKey,
				Events: makeEvents([]sideEventSpec{
					{key: "Engineering Bay", subject: models.GeneralUnitEngineeringBay, buildTime: int64(math.Round(models.BuildTimeEngineeringBay)), actual: payload.ebayCmd},
					{key: "First Missile Turret", subject: models.GeneralUnitMissileTurret, buildTime: int64(math.Round(models.BuildTimeMissileTurret)), actual: payload.firstTurretCmd},
				}),
			})
		}
	}

	// Populate the gap summary. ExpertGap* values are corpus-derived from the
	// 240-game cwal-dl 1v1 TvZ match set with prerequisite-clamped finish
	// times: Mutalisk hatch = max(spire_built, morph_cmd) + 25,
	// Missile Turret finish = max(ebay_built, build_cmd) + 19.
	// Median gap (turret_finish - muta_finish) = +8s, p25 = -5, p75 = +19.
	// Positive median = turret completes shortly after muta hatches — mutas
	// then eat ~10-20s of travel time across the map, so turrets land
	// just-in-time for muta arrival.
	summary := &workflowMutaliskTimingSummary{
		ExpertGapSeconds:    8,
		ExpertGapMinSeconds: -5,
		ExpertGapMaxSeconds: 19,
	}
	zPayload, tPayload := byPlayer[zergPlayerID], byPlayer[terranPlayerID]
	if zPayload != nil && tPayload != nil &&
		zPayload.firstMutaCmd > 0 && tPayload.firstTurretCmd > 0 {
		spireFinish := zPayload.spireCmd + int(models.BuildTimeSpire)
		mutaStart := zPayload.firstMutaCmd
		if zPayload.spireCmd > 0 && mutaStart < spireFinish {
			mutaStart = spireFinish
		}
		mutaFinish := mutaStart + int(models.BuildTimeMutalisk)

		ebayFinish := tPayload.ebayCmd + int(models.BuildTimeEngineeringBay)
		turretStart := tPayload.firstTurretCmd
		if tPayload.ebayCmd > 0 && turretStart < ebayFinish {
			turretStart = ebayFinish
		}
		turretFinish := turretStart + int(math.Round(models.BuildTimeMissileTurret))

		summary.ActualGapSeconds = int64(turretFinish - mutaFinish)
		summary.HasActual = true
	}
	detail.MutaliskTimingSummary = summary
	return nil
}

// firstBuildingActualSecond returns the actual_second of the first building
// milestone observed for a BO. For non-Zerg openers the events array is
// position-aligned with bo.Expert, so we walk Expert entries and pick the
// first KindMakeBuilding match. For simplified-Zerg BOs the events array is
// reshaped (drone-numbered ticks + pool/hatch); we scan it for the first
// known building Subject (Spawning Pool / Hatchery).
func firstBuildingActualSecond(bo *markers.Marker, events []workflowMarkerEvent) (int, bool) {
	if bo == nil {
		return 0, false
	}
	if _, isSimplifiedZerg := zergBOEventSchemas[bo.FeatureKey]; isSimplifiedZerg {
		for _, ev := range events {
			if !ev.Found {
				continue
			}
			if ev.Subject == models.GeneralUnitSpawningPool || ev.Subject == models.GeneralUnitHatchery {
				return int(ev.ActualSecond), true
			}
		}
		return 0, false
	}
	for idx, expert := range bo.Expert {
		if expert.Match.Kind != cmdenrich.KindMakeBuilding {
			continue
		}
		if idx >= len(events) || !events[idx].Found {
			continue
		}
		return int(events[idx].ActualSecond), true
	}
	return 0, false
}

func (d *Dashboard) populateUnitCadenceForGameDetail(detail *workflowGameDetail) error {
	if detail == nil {
		return errors.New("nil game detail")
	}
	detail.UnitCadence = []workflowGameUnitCadencePlayer{}
	playerByID := map[int64]workflowGamePlayer{}
	for _, player := range detail.Players {
		playerByID[player.PlayerID] = player
		detail.UnitCadence = append(detail.UnitCadence, workflowGameUnitCadencePlayer{
			PlayerID:         player.PlayerID,
			PlayerKey:        player.PlayerKey,
			PlayerName:       player.Name,
			Team:             player.Team,
			IsWinner:         player.IsWinner,
			Eligible:         false,
			IneligibleReason: "not enough attacking-unit production samples in analysis window",
		})
	}
	if len(detail.Players) == 0 {
		return nil
	}
	excludedUnits := workflowUnitCadenceExcludedUnits(workflowUnitCadenceFilterStrict)
	if len(excludedUnits) == 0 {
		return errors.New("missing excluded units for cadence computation")
	}
	rows, err := d.dbStore.ListGameUnitCadenceRows(
		d.ctx,
		detail.ReplayID,
		detail.DurationSeconds,
		excludedUnits,
		workflowUnitCadenceStartSeconds,
		workflowUnitCadenceEndFraction,
		workflowUnitCadenceIdleGapSeconds,
	)
	if err != nil {
		return fmt.Errorf("failed to query game unit cadence: %w", err)
	}

	scoredByPlayerID := map[int64]workflowGameUnitCadencePlayer{}
	for _, row := range rows {
		playerID := row.PlayerID
		windowSeconds := row.WindowSeconds
		unitsProduced := row.UnitsProduced
		gapCount := row.GapCount
		ratePerMinute := row.RatePerMinute
		cvGap := row.CVGap
		burstiness := row.Burstiness
		idle20Ratio := row.Idle20Ratio
		cadenceScore := row.CadenceScore
		player, ok := playerByID[playerID]
		if !ok {
			continue
		}
		entry := workflowGameUnitCadencePlayer{
			PlayerID:         player.PlayerID,
			PlayerKey:        player.PlayerKey,
			PlayerName:       player.Name,
			Team:             player.Team,
			IsWinner:         player.IsWinner,
			Eligible:         unitsProduced >= workflowUnitCadenceMinUnitsPerReplay && gapCount >= workflowUnitCadenceMinGapsPerReplay,
			WindowSeconds:    windowSeconds,
			UnitsProduced:    unitsProduced,
			GapCount:         gapCount,
			IneligibleReason: "not enough attacking-unit production samples in analysis window",
		}
		if ratePerMinute.Valid {
			entry.RatePerMinute = ratePerMinute.Float64
		}
		if cvGap.Valid {
			entry.CVGap = cvGap.Float64
		}
		if burstiness.Valid {
			entry.Burstiness = burstiness.Float64
		}
		if idle20Ratio.Valid {
			entry.Idle20Ratio = idle20Ratio.Float64
		}
		if cadenceScore.Valid {
			entry.CadenceScore = cadenceScore.Float64
		}
		if entry.Eligible {
			entry.IneligibleReason = ""
		}
		scoredByPlayerID[playerID] = entry
	}

	for i := range detail.UnitCadence {
		playerID := detail.UnitCadence[i].PlayerID
		if scored, ok := scoredByPlayerID[playerID]; ok {
			detail.UnitCadence[i] = scored
		}
	}
	sort.Slice(detail.UnitCadence, func(i, j int) bool {
		a := detail.UnitCadence[i]
		b := detail.UnitCadence[j]
		if a.Eligible != b.Eligible {
			return a.Eligible
		}
		if a.CadenceScore == b.CadenceScore {
			return a.PlayerName < b.PlayerName
		}
		return a.CadenceScore > b.CadenceScore
	})
	return nil
}

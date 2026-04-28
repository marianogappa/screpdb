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

	"github.com/marianogappa/screpdb/internal/patterns/markers"
	db "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/models"
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
	detail.MapVisual = d.resolveWorkflowMapVisual(detail.ReplayID, summary.MapName, summary.FilePath, summary.FileChecksum)
	detail.DurationSeconds = summary.DurationSeconds
	detail.GameType = summary.GameType

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

	return detail, nil
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
			ReplayID:        row.ReplayID,
			ReplayDate:      row.ReplayDate,
			FileName:        row.FileName,
			MapName:         row.MapName,
			DurationSeconds: row.DurationSeconds,
			GameType:        row.GameType,
			Matchup:         row.Matchup,
			PlayersLabel:    row.PlayersLabel,
			WinnersLabel:    row.WinnersLabel,
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

func newWorkflowFirstUnitEfficiencyState() *workflowFirstUnitEfficiencyState {
	return &workflowFirstUnitEfficiencyState{
		buildTimesByUnit: map[string][]int64{},
		unitTimesByUnit:  map[string][]int64{},
	}
}

func applyCommandToFirstUnitEfficiencyState(state *workflowFirstUnitEfficiencyState, actionType string, second int64, unitType sql.NullString, unitTypes sql.NullString) {
	commandUnits := parseCommandUnitNames(unitType, unitTypes)
	if len(commandUnits) == 0 {
		return
	}
	for _, name := range commandUnits {
		aliases := unitNameAliases(name)
		if len(aliases) == 0 {
			continue
		}
		if actionType == "Build" {
			for _, alias := range aliases {
				state.buildTimesByUnit[alias] = append(state.buildTimesByUnit[alias], second)
			}
			continue
		}
		for _, alias := range aliases {
			state.unitTimesByUnit[alias] = append(state.unitTimesByUnit[alias], second)
		}
	}
}

func firstUnitEfficiencyEntriesForRace(playerRace string, state *workflowFirstUnitEfficiencyState, maxGapSeconds int64) []workflowFirstUnitEfficiencyEntry {
	race := strings.ToLower(strings.TrimSpace(playerRace))
	entries := []workflowFirstUnitEfficiencyEntry{}
	for _, cfg := range firstUnitEfficiencyConfigs {
		if cfg.Race != race {
			continue
		}
		buildingKey := normalizeUnitName(cfg.BuildingName)
		buildStarts := state.buildTimesByUnit[buildingKey]
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
				timings := state.unitTimesByUnit[matchKey]
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
		if gapAfterReadySeconds < 0 || gapAfterReadySeconds > maxGapSeconds {
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
	return entries
}

func (d *Dashboard) collectWorkflowPlayerDelaySamples(onlyPlayerKey string) ([]workflowPlayerDelaySample, error) {
	rows, err := d.dbStore.ListDelayCommandRows(d.ctx, workflowPlayerDelayCutoffSeconds, onlyPlayerKey)
	if err != nil {
		return nil, err
	}

	samples := []workflowPlayerDelaySample{}
	var currentReplayID int64 = -1
	var currentPlayerID int64 = -1
	currentPlayerName := ""
	currentPlayerRace := ""
	currentPlayerKey := ""
	state := newWorkflowFirstUnitEfficiencyState()

	flushCurrent := func() {
		if currentReplayID < 0 || currentPlayerID < 0 {
			return
		}
		entries := firstUnitEfficiencyEntriesForRace(currentPlayerRace, state, workflowPlayerDelayMaxGapSeconds)
		for _, entry := range entries {
			samples = append(samples, workflowPlayerDelaySample{
				PlayerKey:            currentPlayerKey,
				PlayerName:           currentPlayerName,
				BuildingName:         entry.BuildingName,
				UnitName:             entry.UnitName,
				GapAfterReadySeconds: entry.GapAfterReadySeconds,
			})
		}
	}

	for _, row := range rows {
		replayID := row.ReplayID
		playerID := row.PlayerID
		playerName := row.PlayerName
		playerRace := row.PlayerRace
		second := row.Second
		actionType := row.ActionType
		unitType := row.UnitType
		unitTypes := row.UnitTypes
		if replayID != currentReplayID || playerID != currentPlayerID {
			flushCurrent()
			currentReplayID = replayID
			currentPlayerID = playerID
			currentPlayerName = playerName
			currentPlayerRace = playerRace
			currentPlayerKey = normalizePlayerKey(playerName)
			state = newWorkflowFirstUnitEfficiencyState()
		}
		applyCommandToFirstUnitEfficiencyState(state, actionType, second, unitType, unitTypes)
	}
	flushCurrent()
	return samples, nil
}

func (d *Dashboard) buildWorkflowPlayerDelayHistogram() (workflowPlayerDelayHistogram, error) {
	result := workflowPlayerDelayHistogram{
		SummaryVersion: workflowSummaryVersion,
		MinSamples:     workflowPlayerDelayMinSamples,
		Bins:           []workflowPlayerDelayHistogramBin{},
		Players:        []workflowPlayerDelayHistogramPoint{},
		CaseOptions:    []workflowPlayerDelayCaseOption{},
	}
	samples, err := d.collectWorkflowPlayerDelaySamples("")
	if err != nil {
		return result, err
	}
	type caseAgg struct {
		buildingName string
		unitName     string
		sum          float64
		count        int64
	}
	type playerAgg struct {
		playerName string
		sum        float64
		count      int64
		cases      map[string]*caseAgg
	}
	type caseOptionAgg struct {
		buildingName string
		unitName     string
		sampleCount  int64
		players      map[string]struct{}
	}
	aggregates := map[string]*playerAgg{}
	caseOptions := map[string]*caseOptionAgg{}
	for _, sample := range samples {
		caseKey := normalizeUnitName(sample.BuildingName) + "->" + normalizeUnitName(sample.UnitName)
		entry, ok := aggregates[sample.PlayerKey]
		if !ok {
			entry = &playerAgg{
				playerName: sample.PlayerName,
				sum:        0,
				count:      0,
				cases:      map[string]*caseAgg{},
			}
			aggregates[sample.PlayerKey] = entry
		}
		entry.sum += float64(sample.GapAfterReadySeconds)
		entry.count++
		if strings.TrimSpace(entry.playerName) == "" {
			entry.playerName = sample.PlayerName
		}
		caseEntry, exists := entry.cases[caseKey]
		if !exists {
			caseEntry = &caseAgg{
				buildingName: sample.BuildingName,
				unitName:     sample.UnitName,
				sum:          0,
				count:        0,
			}
			entry.cases[caseKey] = caseEntry
		}
		caseEntry.sum += float64(sample.GapAfterReadySeconds)
		caseEntry.count++

		caseOptionEntry, exists := caseOptions[caseKey]
		if !exists {
			caseOptionEntry = &caseOptionAgg{
				buildingName: sample.BuildingName,
				unitName:     sample.UnitName,
				sampleCount:  0,
				players:      map[string]struct{}{},
			}
			caseOptions[caseKey] = caseOptionEntry
		}
		caseOptionEntry.sampleCount++
		caseOptionEntry.players[sample.PlayerKey] = struct{}{}
	}

	values := []float64{}
	for playerKey, entry := range aggregates {
		if entry.count < workflowPlayerDelayMinSamples {
			continue
		}
		avg := entry.sum / float64(entry.count)
		caseAverages := []workflowPlayerDelayCaseAverage{}
		for caseKey, caseEntry := range entry.cases {
			if caseEntry.count <= 0 {
				continue
			}
			caseAverages = append(caseAverages, workflowPlayerDelayCaseAverage{
				CaseKey:             caseKey,
				BuildingName:        caseEntry.buildingName,
				UnitName:            caseEntry.unitName,
				AverageDelaySeconds: caseEntry.sum / float64(caseEntry.count),
				SampleCount:         caseEntry.count,
			})
		}
		sort.Slice(caseAverages, func(i, j int) bool {
			if caseAverages[i].SampleCount == caseAverages[j].SampleCount {
				return caseAverages[i].CaseKey < caseAverages[j].CaseKey
			}
			return caseAverages[i].SampleCount > caseAverages[j].SampleCount
		})
		result.Players = append(result.Players, workflowPlayerDelayHistogramPoint{
			PlayerKey:           playerKey,
			PlayerName:          entry.playerName,
			AverageDelaySeconds: avg,
			SampleCount:         entry.count,
			CaseAverages:        caseAverages,
		})
		values = append(values, avg)
	}
	for caseKey, option := range caseOptions {
		result.CaseOptions = append(result.CaseOptions, workflowPlayerDelayCaseOption{
			CaseKey:      caseKey,
			BuildingName: option.buildingName,
			UnitName:     option.unitName,
			SampleCount:  option.sampleCount,
			PlayerCount:  int64(len(option.players)),
		})
	}
	sort.Slice(result.CaseOptions, func(i, j int) bool {
		if result.CaseOptions[i].SampleCount == result.CaseOptions[j].SampleCount {
			return result.CaseOptions[i].CaseKey < result.CaseOptions[j].CaseKey
		}
		return result.CaseOptions[i].SampleCount > result.CaseOptions[j].SampleCount
	})
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
	result.MeanDelaySeconds = mean

	varianceSum := 0.0
	for _, value := range values {
		diff := value - mean
		varianceSum += diff * diff
	}
	result.StddevDelaySeconds = math.Sqrt(varianceSum / float64(len(values)))

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
		result.Bins = []workflowPlayerDelayHistogramBin{{
			X0:    minValue,
			X1:    minValue + 1,
			Count: int64(len(values)),
		}}
	} else {
		width := (maxValue - minValue) / float64(binCount)
		if width <= 0 {
			width = 1
		}
		bins := make([]workflowPlayerDelayHistogramBin, binCount)
		for i := 0; i < binCount; i++ {
			start := minValue + float64(i)*width
			end := minValue + float64(i+1)*width
			if i == binCount-1 {
				end = maxValue
			}
			bins[i] = workflowPlayerDelayHistogramBin{X0: start, X1: end, Count: 0}
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
		if result.Players[i].AverageDelaySeconds == result.Players[j].AverageDelaySeconds {
			return result.Players[i].PlayerName < result.Players[j].PlayerName
		}
		return result.Players[i].AverageDelaySeconds < result.Players[j].AverageDelaySeconds
	})
	return result, nil
}

func (d *Dashboard) buildWorkflowPlayerDelayInsight(playerKey string) (workflowPlayerDelayInsight, error) {
	result := workflowPlayerDelayInsight{
		SummaryVersion: workflowSummaryVersion,
		PlayerKey:      playerKey,
		Pairs:          []workflowPlayerDelayPair{},
	}
	playerName, err := d.dbStore.GetPlayerNameByKey(d.ctx, playerKey)
	if err != nil {
		return result, err
	}
	result.PlayerName = playerName
	if result.PlayerName == "" {
		return result, sql.ErrNoRows
	}
	samples, err := d.collectWorkflowPlayerDelaySamples(playerKey)
	if err != nil {
		return result, err
	}
	if len(samples) == 0 {
		return result, nil
	}
	type pairAgg struct {
		building string
		unit     string
		sum      float64
		count    int64
		minGap   int64
		maxGap   int64
	}
	pairs := map[string]*pairAgg{}
	total := 0.0
	var minDelay int64 = math.MaxInt64
	var maxDelay int64
	for _, sample := range samples {
		delay := sample.GapAfterReadySeconds
		total += float64(delay)
		result.SampleCount++
		if delay < minDelay {
			minDelay = delay
		}
		if delay > maxDelay {
			maxDelay = delay
		}
		pairKey := normalizeUnitName(sample.BuildingName) + "->" + normalizeUnitName(sample.UnitName)
		entry, ok := pairs[pairKey]
		if !ok {
			pairs[pairKey] = &pairAgg{
				building: sample.BuildingName,
				unit:     sample.UnitName,
				sum:      float64(delay),
				count:    1,
				minGap:   delay,
				maxGap:   delay,
			}
			continue
		}
		entry.sum += float64(delay)
		entry.count++
		if delay < entry.minGap {
			entry.minGap = delay
		}
		if delay > entry.maxGap {
			entry.maxGap = delay
		}
	}
	result.AverageDelaySeconds = total / float64(result.SampleCount)
	result.MinDelaySeconds = minDelay
	result.MaxDelaySeconds = maxDelay
	for _, entry := range pairs {
		result.Pairs = append(result.Pairs, workflowPlayerDelayPair{
			BuildingName:        entry.building,
			UnitName:            entry.unit,
			SampleCount:         entry.count,
			AverageDelaySeconds: entry.sum / float64(entry.count),
			MinDelaySeconds:     entry.minGap,
			MaxDelaySeconds:     entry.maxGap,
		})
	}
	sort.Slice(result.Pairs, func(i, j int) bool {
		if result.Pairs[i].SampleCount == result.Pairs[j].SampleCount {
			return result.Pairs[i].AverageDelaySeconds < result.Pairs[j].AverageDelaySeconds
		}
		return result.Pairs[i].SampleCount > result.Pairs[j].SampleCount
	})
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
	case workflowPlayerInsightTypeFirstDelay:
		return d.buildWorkflowPlayerDelayAsyncInsight(playerKey)
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

func (d *Dashboard) buildWorkflowPlayerDelayAsyncInsight(playerKey string) (workflowPlayerAsyncInsight, error) {
	histogram, err := d.buildWorkflowPlayerDelayHistogram()
	if err != nil {
		return workflowPlayerAsyncInsight{}, err
	}
	insight, err := d.buildWorkflowPlayerDelayInsight(playerKey)
	if err != nil {
		return workflowPlayerAsyncInsight{}, err
	}
	result := workflowPlayerAsyncInsight{
		SummaryVersion:  workflowSummaryVersion,
		PlayerKey:       playerKey,
		PlayerName:      insight.PlayerName,
		InsightType:     workflowPlayerInsightTypeFirstDelay,
		Title:           "First-unit delay",
		BetterDirection: "lower",
		PopulationSize:  histogram.PlayersIncluded,
		Description:     "Average delay from a production building becoming ready to the first matching unit command. We only count eligible build/train/morph observations up to 7:00 game time, and only when the unit follows within 20 seconds. Lower is better.",
		Details: []workflowPlayerInsightDetail{
			{Label: "Eligible players", Value: fmt.Sprintf("%d (minimum %d samples)", histogram.PlayersIncluded, histogram.MinSamples)},
			{Label: "Population mean", Value: fmt.Sprintf("%.2fs", histogram.MeanDelaySeconds)},
			{Label: "Population stddev", Value: fmt.Sprintf("%.2f", histogram.StddevDelaySeconds)},
		},
	}
	if insight.SampleCount < histogram.MinSamples {
		result.IneligibleReason = fmt.Sprintf("Not enough valid first-unit observations yet. This view currently requires at least %d samples.", histogram.MinSamples)
		return result, nil
	}
	values := extractDelayValues(histogram.Players)
	percentile := performancePercentileFromSortedValues(values, insight.AverageDelaySeconds, true)
	value := insight.AverageDelaySeconds
	result.Eligible = true
	result.PerformancePercentile = &percentile
	result.PlayerValue = &value
	result.PlayerValueLabel = fmt.Sprintf("%.2fs", value)
	result.Details = append(result.Details,
		workflowPlayerInsightDetail{Label: "Samples", Value: strconv.FormatInt(insight.SampleCount, 10)},
		workflowPlayerInsightDetail{Label: "Observed range", Value: fmt.Sprintf("%ds to %ds", insight.MinDelaySeconds, insight.MaxDelaySeconds)},
	)
	if len(insight.Pairs) > 0 {
		result.Details = append(result.Details, workflowPlayerInsightDetail{
			Label: "Typical cases",
			Value: summarizeDelayPairs(insight.Pairs, 3),
		})
	}
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

func extractDelayValues(players []workflowPlayerDelayHistogramPoint) []float64 {
	values := make([]float64, 0, len(players))
	for _, player := range players {
		values = append(values, player.AverageDelaySeconds)
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

func summarizeDelayPairs(pairs []workflowPlayerDelayPair, maxItems int) string {
	if len(pairs) == 0 || maxItems <= 0 {
		return ""
	}
	parts := make([]string, 0, minInt(len(pairs), maxItems))
	for i := 0; i < len(pairs) && i < maxItems; i++ {
		pair := pairs[i]
		parts = append(parts, fmt.Sprintf("%s -> %s %.2fs (%d)", pair.BuildingName, pair.UnitName, pair.AverageDelaySeconds, pair.SampleCount))
	}
	return strings.Join(parts, "; ")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (d *Dashboard) buildWorkflowPlayerMetrics(playerKey string) (workflowPlayerMetrics, error) {
	gamesPlayed, err := d.dbStore.CountPlayerGames(d.ctx, playerKey)
	if err != nil {
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
	raceRows, err := d.dbStore.ListRaceSections(d.ctx, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load race behaviour sections: %w", err)
	}

	sections := []workflowRaceBehaviourSection{}
	byRace := map[string]*workflowRaceBehaviourSection{}
	for _, row := range raceRows {
		race := row.Race
		gameCount := row.GameCount
		wins := row.Wins
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
	patternRows, err := d.dbStore.ListRacePatterns(d.ctx, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load race common behaviours: %w", err)
	}
	for _, row := range patternRows {
		race := row.Race
		patternName := row.PatternName
		replayCount := row.ReplayCount
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
			Type:            row.EventType,
			Second:          row.Second,
			Ownership:       []workflowGameOwnership{},
			AttackUnitTypes: parseAttackUnitTypes(row.AttackUnitTypes),
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
		if event.Actor != nil {
			if startClock, ok := startClockByPlayerID[event.Actor.PlayerID]; ok {
				event.ActorStartClock = lo.ToPtr(int64(startClock))
				if startBase, startBaseOK := lookupOverlayBaseByClock(baseMetas, int64(startClock)); startBaseOK {
					startCenter := startBase.Center
					event.ActorOrigin = &startCenter
				}
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
		events = append(events, event)
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
				prettyName = fmt.Sprintf("at %d", base.Clock)
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
	// labels ("at 9", "12's natural near 6", "an expa near 3") read right
	// when inserted with 0, so short-circuit to a clear literal.
	isCenter := func(v *int64) bool { return v != nil && *v == 0 }
	switch strings.ToLower(strings.TrimSpace(*baseType)) {
	case "starting":
		if isCenter(baseOclock) {
			return "center base"
		}
		if baseOclock != nil {
			return fmt.Sprintf("at %d", *baseOclock)
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
			return fmt.Sprintf("an expa near %d", *baseOclock)
		}
		return "expansion"
	}
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

	rows, err := d.dbStore.ListUnitSliceCommandRows(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load unit slices: %w", err)
	}

	perSlice := map[int64]map[int64]map[string]int64{}
	boundaries := workflowSliceBoundaries(detail.DurationSeconds)
	for _, row := range rows {
		playerID := row.PlayerID
		second := row.Second
		unitType := row.UnitType
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
		// One Zergling Unit Morph command produces a pair of Zerglings
		// from a single larva. Source rows have Count=1 per command, so
		// double the increment for Zerglings.
		inc := int64(1)
		if unitType == models.GeneralUnitZergling {
			inc = 2
		}
		perSlice[sliceStart][playerID][unitType] += inc
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

	for i := 1; i <= schema.Drones; i++ {
		ev := workflowMarkerEvent{
			Key:      fmt.Sprintf("%d%s Drone", i, ordinalSuffix(i)),
			Subject:  models.GeneralUnitDrone,
			NoExpert: true,
		}
		if i-1 < len(t.DroneMorphSecs) {
			ev.Found = true
			ev.ActualSecond = int64(t.DroneMorphSecs[i-1])
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
	for _, player := range detail.Players {
		detected := detectedByPlayer[player.PlayerID]
		if len(detected) == 0 {
			continue
		}
		for i := range allMarkers {
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
		}
	}
	return nil
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

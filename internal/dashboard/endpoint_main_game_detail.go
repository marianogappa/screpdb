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

	"github.com/marianogappa/screpdb/internal/models"
)

func (d *Dashboard) buildWorkflowGameDetail(replayID int64) (workflowGameDetail, error) {
	detail := workflowGameDetail{SummaryVersion: workflowSummaryVersion}
	err := d.currentReplayScopedDB().QueryRowContext(d.ctx, `
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

	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
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
	if err := d.populateUnitCadenceForGameDetail(&detail); err != nil {
		return detail, err
	}
	if err := d.populateViewportMultitaskingForGameDetail(&detail); err != nil {
		return detail, err
	}

	return detail, nil
}

func (d *Dashboard) populateDetectedPatternsForGameDetail(detail *workflowGameDetail) error {
	detail.ReplayPatterns = []workflowPatternValue{}
	detail.TeamPatterns = []workflowTeamPattern{}
	detail.GameEvents = []workflowGameEvent{}

	rowsReplay, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
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

	rowsTeam, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
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

	rowsPlayer, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
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

	err := d.currentReplayScopedDB().QueryRowContext(d.ctx, `
		SELECT
			MIN(p.name) AS player_name,
			COUNT(*) AS games_played,
			SUM(CASE WHEN p.is_winner = 1 THEN 1 ELSE 0 END) AS wins,
			AVG(p.apm) AS avg_apm,
			AVG(p.eapm) AS avg_eapm
		FROM players p
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
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
	recentRows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
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
					WHERE p2.replay_id = r.id AND p2.is_observer = 0 AND lower(trim(coalesce(p2.type, ''))) = 'human'
					ORDER BY p2.team ASC, p2.id ASC
				)
			), ''),
			COALESCE((
				SELECT group_concat(p3.name, ', ')
				FROM players p3
				WHERE p3.replay_id = r.id AND p3.is_winner = 1 AND p3.is_observer = 0 AND lower(trim(coalesce(p3.type, ''))) = 'human'
			), '')
		FROM replays r
		JOIN players p ON p.replay_id = r.id
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
		ORDER BY r.replay_date DESC, r.id DESC
		LIMIT 12
	`, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load recent games for %s: %w", playerName, err)
	}
	defer recentRows.Close()
	result := []workflowGameListItem{}
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
			return nil, fmt.Errorf("failed to parse recent games for %s: %w", playerName, err)
		}
		result = append(result, g)
	}
	if err := recentRows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating recent games for %s: %w", playerName, err)
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

	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT player_key, player_name, average_apm, games_played
		FROM (
			SELECT
				lower(trim(p.name)) AS player_key,
				MIN(p.name) AS player_name,
				COALESCE(AVG(CASE WHEN p.apm > 0 THEN p.apm END), 0) AS average_apm,
				COUNT(*) AS games_played
			FROM players p
			WHERE p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
			GROUP BY lower(trim(p.name))
		) per_player
		WHERE games_played >= ?
			AND average_apm > 0
	`, minGames)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	values := []float64{}
	playerValue := 0.0
	for rows.Next() {
		var key string
		var name string
		var avgAPM float64
		var gamesPlayed int64
		if err := rows.Scan(&key, &name, &avgAPM, &gamesPlayed); err != nil {
			return result, err
		}
		if avgAPM <= 0 {
			continue
		}
		values = append(values, avgAPM)
		result.Players = append(result.Players, workflowPlayerApmHistogramPoint{
			PlayerKey:   key,
			PlayerName:  name,
			AverageAPM:  avgAPM,
			GamesPlayed: gamesPlayed,
		})
		if key == playerKey {
			playerValue = avgAPM
			result.PlayerEligible = true
		}
	}
	if err := rows.Err(); err != nil {
		return result, err
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
	args := []any{workflowPlayerDelayCutoffSeconds}
	filterSQL := ""
	if onlyPlayerKey != "" {
		filterSQL = "AND lower(trim(p.name)) = ?"
		args = append(args, onlyPlayerKey)
	}
	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT
			p.replay_id,
			p.id,
			p.name,
			p.race,
			c.seconds_from_game_start,
			c.action_type,
			c.unit_type,
			c.unit_types
		FROM players p
		JOIN commands c
			ON c.replay_id = p.replay_id
			AND c.player_id = p.id
		WHERE
			p.is_observer = 0
			AND lower(trim(coalesce(p.type, ''))) = 'human'
			AND c.action_type IN ('Build', 'Train', 'Unit Morph')
			AND c.seconds_from_game_start <= ?
			`+filterSQL+`
		ORDER BY p.replay_id ASC, p.id ASC, c.seconds_from_game_start ASC, c.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

	for rows.Next() {
		var replayID int64
		var playerID int64
		var playerName string
		var playerRace string
		var second int64
		var actionType string
		var unitType sql.NullString
		var unitTypes sql.NullString
		if err := rows.Scan(&replayID, &playerID, &playerName, &playerRace, &second, &actionType, &unitType, &unitTypes); err != nil {
			return nil, err
		}
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
	if err := rows.Err(); err != nil {
		return nil, err
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
	if err := d.currentReplayScopedDB().QueryRowContext(d.ctx, `
		SELECT MIN(name)
		FROM players
		WHERE lower(trim(name)) = ? AND is_observer = 0 AND lower(trim(coalesce(type, ''))) = 'human'
	`, playerKey).Scan(&result.PlayerName); err != nil {
		return result, err
	}
	if strings.TrimSpace(result.PlayerName) == "" {
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
	inClause := buildInClausePlaceholders(len(excludedUnits))
	filterSQL := ""
	args := []any{}
	for _, name := range excludedUnits {
		args = append(args, name)
	}
	if onlyPlayerKey != "" {
		filterSQL = "AND lower(trim(p.name)) = ?"
		args = append(args, onlyPlayerKey)
	}
	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		WITH base AS (
			SELECT
				c.replay_id,
				lower(trim(p.name)) AS player_key,
				MIN(p.name) AS player_name,
				r.file_name,
				r.duration_seconds,
				c.seconds_from_game_start AS t,
				c.id AS cmd_id
			FROM commands c
			JOIN players p
				ON p.id = c.player_id
			JOIN replays r
				ON r.id = c.replay_id
			WHERE
				p.is_observer = 0
				AND lower(trim(coalesce(p.type, ''))) = 'human'
				AND c.action_type IN ('Train', 'Unit Morph')
				AND c.unit_type IS NOT NULL
				AND trim(c.unit_type) <> ''
				AND c.unit_type NOT IN (`+inClause+`)
				AND c.seconds_from_game_start >= `+strconv.FormatInt(workflowUnitCadenceStartSeconds, 10)+`
				AND c.seconds_from_game_start <= CAST(`+strconv.FormatFloat(workflowUnitCadenceEndFraction, 'f', 4, 64)+` * r.duration_seconds AS INTEGER)
				AND CAST(`+strconv.FormatFloat(workflowUnitCadenceEndFraction, 'f', 4, 64)+` * r.duration_seconds AS INTEGER) > `+strconv.FormatInt(workflowUnitCadenceStartSeconds, 10)+`
				`+filterSQL+`
			GROUP BY
				c.replay_id,
				player_key,
				r.file_name,
				r.duration_seconds,
				c.seconds_from_game_start,
				c.id
		),
		ordered AS (
			SELECT
				replay_id,
				player_key,
				player_name,
				file_name,
				duration_seconds,
				t,
				cmd_id,
				LAG(t) OVER (PARTITION BY replay_id, player_key ORDER BY t, cmd_id) AS prev_t
			FROM base
		),
		gaps AS (
			SELECT
				replay_id,
				player_key,
				player_name,
				file_name,
				duration_seconds,
				t,
				(t - prev_t) AS gap_s
			FROM ordered
		),
		replay_metrics AS (
			SELECT
				replay_id,
				player_key,
				player_name,
				file_name,
				duration_seconds,
				CAST(`+strconv.FormatFloat(workflowUnitCadenceEndFraction, 'f', 4, 64)+` * duration_seconds AS INTEGER) - `+strconv.FormatInt(workflowUnitCadenceStartSeconds, 10)+` AS window_s,
				COUNT(*) AS n_units,
				COUNT(gap_s) AS n_gaps,
				AVG(gap_s * 1.0) AS mean_gap_s,
				sqrt(AVG(gap_s * gap_s * 1.0) - AVG(gap_s * 1.0) * AVG(gap_s * 1.0)) AS std_gap_s,
				SUM(CASE WHEN gap_s >= `+strconv.FormatInt(workflowUnitCadenceIdleGapSeconds, 10)+` THEN 1 ELSE 0 END) * 1.0 / NULLIF(COUNT(gap_s), 0) AS idle20_ratio
			FROM gaps
			GROUP BY replay_id, player_key, player_name, file_name, duration_seconds
			HAVING
				COUNT(*) >= `+strconv.FormatInt(workflowUnitCadenceMinUnitsPerReplay, 10)+`
				AND COUNT(gap_s) >= `+strconv.FormatInt(workflowUnitCadenceMinGapsPerReplay, 10)+`
				AND window_s > 0
		),
		scored AS (
			SELECT
				replay_id,
				player_key,
				player_name,
				file_name,
				duration_seconds,
				window_s,
				n_units,
				n_gaps,
				(n_units * 60.0) / window_s AS rate_per_min,
				(std_gap_s / NULLIF(mean_gap_s, 0)) AS cv_gap,
				(((std_gap_s / NULLIF(mean_gap_s, 0)) - 1.0) / ((std_gap_s / NULLIF(mean_gap_s, 0)) + 1.0)) AS burstiness,
				idle20_ratio,
				((n_units * 60.0) / window_s) / (1.0 + COALESCE((std_gap_s / NULLIF(mean_gap_s, 0)), 9999.0)) AS cadence_score
			FROM replay_metrics
		)
		SELECT
			replay_id,
			player_key,
			player_name,
			file_name,
			duration_seconds,
			window_s,
			n_units,
			n_gaps,
			rate_per_min,
			cv_gap,
			burstiness,
			idle20_ratio,
			cadence_score
		FROM scored
		ORDER BY player_key ASC, replay_id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []workflowPlayerUnitCadenceReplayMetric{}
	for rows.Next() {
		var row workflowPlayerUnitCadenceReplayMetric
		var cvGap sql.NullFloat64
		var burstiness sql.NullFloat64
		var idle20 sql.NullFloat64
		var cadence sql.NullFloat64
		if err := rows.Scan(
			&row.ReplayID,
			&row.PlayerKey,
			&row.PlayerName,
			&row.FileName,
			&row.DurationSeconds,
			&row.WindowSeconds,
			&row.UnitsProduced,
			&row.GapCount,
			&row.RatePerMinute,
			&cvGap,
			&burstiness,
			&idle20,
			&cadence,
		); err != nil {
			return nil, err
		}
		if cvGap.Valid {
			row.CVGap = cvGap.Float64
		}
		if burstiness.Valid {
			row.Burstiness = burstiness.Float64
		}
		if idle20.Valid {
			row.Idle20Ratio = idle20.Float64
		}
		if cadence.Valid {
			row.CadenceScore = cadence.Float64
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
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
	if err := d.currentReplayScopedDB().QueryRowContext(d.ctx, `
		SELECT MIN(name)
		FROM players
		WHERE lower(trim(name)) = ? AND is_observer = 0 AND lower(trim(coalesce(type, ''))) = 'human'
	`, playerKey).Scan(&result.PlayerName); err != nil {
		return result, err
	}
	if strings.TrimSpace(result.PlayerName) == "" {
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
	var gamesPlayed int64
	if err := d.currentReplayScopedDB().QueryRowContext(d.ctx, `
		SELECT COUNT(*)
		FROM players p
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
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
	raceRows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT p.race, COUNT(*) AS game_count, SUM(CASE WHEN p.is_winner = 1 THEN 1 ELSE 0 END) AS wins
		FROM players p
		WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
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

	patternRows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT p.race, dp.pattern_name, COUNT(DISTINCT dp.replay_id) AS replay_count
		FROM detected_patterns_replay_player dp
		JOIN players p ON p.id = dp.player_id
		WHERE lower(trim(p.name)) = ?
			AND p.is_observer = 0
			AND lower(trim(coalesce(p.type, ''))) = 'human'
			AND dp.pattern_name IS NOT NULL
			AND dp.pattern_name <> ''
			AND lower(replace(replace(dp.pattern_name, ' ', ''), '_', '')) NOT IN ('usedhotkeygroups', 'viewportmultitasking')
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
	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
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
	if lowerName == strings.ToLower(models.PatternNameViewportMultitasking) {
		switchRate, ok := parseViewportSwitchRate(v)
		if !ok {
			return "-"
		}
		return fmt.Sprintf("%.2f switches/min", switchRate)
	}
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

	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
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

	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
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
	placeholders := buildInClausePlaceholders(len(excludedUnits))
	args := []any{detail.ReplayID}
	for _, name := range excludedUnits {
		args = append(args, name)
	}
	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		WITH base AS (
			SELECT
				c.player_id,
				c.seconds_from_game_start AS t,
				c.id AS cmd_id
			FROM commands c
			JOIN players p
				ON p.id = c.player_id
			JOIN replays r
				ON r.id = c.replay_id
			WHERE
				c.replay_id = ?
				AND p.is_observer = 0
				AND lower(trim(coalesce(p.type, ''))) = 'human'
				AND c.action_type IN ('Train', 'Unit Morph')
				AND c.unit_type IS NOT NULL
				AND trim(c.unit_type) <> ''
				AND c.unit_type NOT IN (`+placeholders+`)
				AND c.seconds_from_game_start >= `+strconv.FormatInt(workflowUnitCadenceStartSeconds, 10)+`
				AND c.seconds_from_game_start <= CAST(`+strconv.FormatFloat(workflowUnitCadenceEndFraction, 'f', 4, 64)+` * r.duration_seconds AS INTEGER)
				AND CAST(`+strconv.FormatFloat(workflowUnitCadenceEndFraction, 'f', 4, 64)+` * r.duration_seconds AS INTEGER) > `+strconv.FormatInt(workflowUnitCadenceStartSeconds, 10)+`
		),
		ordered AS (
			SELECT
				player_id,
				t,
				cmd_id,
				LAG(t) OVER (PARTITION BY player_id ORDER BY t, cmd_id) AS prev_t
			FROM base
		),
		gaps AS (
			SELECT
				player_id,
				t,
				(t - prev_t) AS gap_s
			FROM ordered
		),
		replay_metrics AS (
			SELECT
				player_id,
				CAST(`+strconv.FormatFloat(workflowUnitCadenceEndFraction, 'f', 4, 64)+` * ? AS INTEGER) - `+strconv.FormatInt(workflowUnitCadenceStartSeconds, 10)+` AS window_s,
				COUNT(*) AS n_units,
				COUNT(gap_s) AS n_gaps,
				AVG(gap_s * 1.0) AS mean_gap_s,
				sqrt(AVG(gap_s * gap_s * 1.0) - AVG(gap_s * 1.0) * AVG(gap_s * 1.0)) AS std_gap_s,
				SUM(CASE WHEN gap_s >= `+strconv.FormatInt(workflowUnitCadenceIdleGapSeconds, 10)+` THEN 1 ELSE 0 END) * 1.0 / NULLIF(COUNT(gap_s), 0) AS idle20_ratio
			FROM gaps
			GROUP BY player_id
			HAVING window_s > 0
		),
		scored AS (
			SELECT
				player_id,
				window_s,
				n_units,
				n_gaps,
				(n_units * 60.0) / window_s AS rate_per_min,
				(std_gap_s / NULLIF(mean_gap_s, 0)) AS cv_gap,
				(((std_gap_s / NULLIF(mean_gap_s, 0)) - 1.0) / ((std_gap_s / NULLIF(mean_gap_s, 0)) + 1.0)) AS burstiness,
				idle20_ratio,
				((n_units * 60.0) / window_s) / (1.0 + COALESCE((std_gap_s / NULLIF(mean_gap_s, 0)), 9999.0)) AS cadence_score
			FROM replay_metrics
		)
		SELECT
			player_id,
			window_s,
			n_units,
			n_gaps,
			rate_per_min,
			cv_gap,
			burstiness,
			idle20_ratio,
			cadence_score
		FROM scored
	`, append(args, detail.DurationSeconds)...)
	if err != nil {
		return fmt.Errorf("failed to query game unit cadence: %w", err)
	}
	defer rows.Close()

	scoredByPlayerID := map[int64]workflowGameUnitCadencePlayer{}
	for rows.Next() {
		var playerID int64
		var windowSeconds int64
		var unitsProduced int64
		var gapCount int64
		var ratePerMinute sql.NullFloat64
		var cvGap sql.NullFloat64
		var burstiness sql.NullFloat64
		var idle20Ratio sql.NullFloat64
		var cadenceScore sql.NullFloat64
		if err := rows.Scan(
			&playerID,
			&windowSeconds,
			&unitsProduced,
			&gapCount,
			&ratePerMinute,
			&cvGap,
			&burstiness,
			&idle20Ratio,
			&cadenceScore,
		); err != nil {
			return fmt.Errorf("failed to parse game unit cadence: %w", err)
		}
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
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed iterating game unit cadence: %w", err)
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

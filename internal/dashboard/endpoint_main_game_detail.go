package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/marianogappa/scfingerprint"
	"github.com/marianogappa/screpdb/internal/cmdenrich"
	db "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/samber/lo"
)

func (d *Dashboard) buildWorkflowGameDetail(replayID int64) (workflowGameDetail, error) {
	detail := workflowGameDetail{SummaryVersion: workflowSummaryVersion}
	summary, err := d.dbStore.GetReplaySummary(d.ctx, replayID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return detail, db.ErrNotFound
		}
		return detail, fmt.Errorf("failed to load replay: %w", err)
	}
	detail.ReplayID = summary.ReplayID
	detail.workflowEngineFields = d.engineFields(replayID)
	detail.ReplayDate = summary.ReplayDate
	detail.FileName = summary.FileName
	detail.FilePath = summary.FilePath
	// A complete-copy game (issue #341) keeps the user's own recording beside
	// it; naming it lets the frontend offer both for watching.
	if ownPath, completePath, err := d.dbStore.GetReplayWatchFiles(d.ctx, replayID); err == nil && completePath != "" && ownPath != "" {
		detail.OwnCopyFileName = filepath.Base(ownPath)
	}
	if d.library != nil {
		if r, ok := d.library.lib.Snapshot().ByID(replayID); ok {
			detail.CompleteCopy = r.Flags.Has(library.FlagIsCompleteCopy)
		}
	}
	detail.MapName = summary.MapName
	detail.MapKind = summary.MapKind
	detail.GameSource = summary.GameSource
	detail.LobbyKind = summary.LobbyKind
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
	displayByName := d.youDisplayNames(playerNames)

	detailPlayerKeys := make([]string, 0, len(rows))
	for _, row := range rows {
		detailPlayerKeys = append(detailPlayerKeys, normalizePlayerKey(row.Name))
	}
	countryCodes, _ := d.countryCodesByPlayerKeys(detailPlayerKeys)

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
		p.TeamWon = row.TeamWon
		p.APM = row.APM
		p.EAPM = row.EAPM
		p.PlayerKey = normalizePlayerKey(row.Name)
		p.CountryCode = countryCodes[p.PlayerKey]
		p.DetectedPatterns = []workflowPatternValue{}
		if match, _ := d.matchFingerprint(p.PlayerKey, scfingerprint.FeatureVersion()); match != nil {
			p.FingerprintMatch = match
		}
		p.PrimaryBadge, p.SecondaryBadge = d.resolvePlayerIdentityBadges(p.PlayerKey, p.FingerprintMatch)
		detail.Players = append(detail.Players, p)
		if row.StartLocationOclock != nil && *row.StartLocationOclock >= 1 && *row.StartLocationOclock <= 12 {
			startClockByPlayerID[row.PlayerID] = int(*row.StartLocationOclock)
		}
	}
	d.triggerBnetProfileFetchesForPlayers(playerNames, detail.GameSource)

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
	if err := d.populateGameEvents(&detail, mapLayout, startClockByPlayerID, displayByName); err != nil {
		return detail, err
	}
	if err := d.populateUnitsBySliceForGameDetail(&detail); err != nil {
		return detail, err
	}
	d.applyEngineToGameDetail(&detail)
	d.populateSkill(&detail)
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
	curateGameEvents(&detail)

	return detail, nil
}

// Computed at request time from the persisted phase boundaries and the
// Train / Unit Morph / Cast stream; see dashboard/unit_composition.go.
//
// Side effect: also populates detail.TrainedUnitsTimeline from the same rows,
// so the event-map overlay needs no second DB query.
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

// trainedUnitsTimelineFromRows flattens production/cast rows into a per-player
// "this unit became alive at this second" stream. Cast rows are ignored — they
// don't put a new unit on the field — and multi-unit morphs contribute one
// sample each. Second is shifted forward by the unit's build duration (Fastest
// speed). Approximation: morph source units are not deducted (mirroring
// computeCompositionForReplay) and deaths are not tracked.
func trainedUnitsTimelineFromRows(rows []db.UnitProductionOrCastRow) []workflowTrainedUnitSample {
	out := make([]workflowTrainedUnitSample, 0, len(rows))
	for _, row := range rows {
		if row.ActionType != "Train" && row.ActionType != "Unit Morph" {
			continue
		}
		for _, name := range parseCommandUnitNames(row.UnitType, row.UnitTypes) {
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

	rowsReplay, err := d.dbStore.ListReplayPatterns(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to query replay patterns: %w", err)
	}
	for _, row := range rowsReplay {
		pattern := buildWorkflowPatternValue(row.PatternName, row.Value, row.DetectedSecond, row.Payload)
		detail.ReplayPatterns = append(detail.ReplayPatterns, pattern)
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

// Shape is registry-driven: event_type (FeatureKey), detected_second, and an
// optional JSON payload for markers carrying extras.
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
	displayByName := d.youDisplayNames([]string{summary.PlayerName})
	if displayName, ok := displayByName[summary.PlayerName]; ok {
		result.PlayerName = displayName
	}
	if cc, _ := d.countryCodesByPlayerKeys([]string{playerKey}); len(cc) > 0 {
		result.CountryCode = cc[playerKey]
	}
	// Cache-only, so this costs no bridge budget.
	if details := d.bnetProfileDetailsByPlayerKeys(d.ctx, []string{playerKey}); len(details) > 0 {
		result.BnetProfile = details[playerKey]
		if result.CountryCode == "" && result.BnetProfile != nil {
			result.CountryCode = result.BnetProfile.CountryCode
		}
		if result.BnetProfile != nil {
			result.BnetProfile.Habits = d.bnetPlayHabitsFor(d.ctx, result.BnetProfile.AuroraID, result.BnetProfile.CountryCode, time.Now())
			d.fillLocalPlayerKeys(d.ctx, result.BnetProfile)
		}
	}
	if bnetGames, err := d.dbStore.CountPlayerBnetGames(d.ctx, playerKey); err == nil {
		result.BnetGames = bnetGames
	}
	if result.BnetProfile != nil || result.BnetGames > 0 {
		d.backfillBnetProfiles([]string{summary.PlayerName})
	}
	result.GamesPlayed = summary.GamesPlayed
	result.Wins = summary.Wins
	result.Undecided = summary.Undecided
	result.AverageAPM = summary.AverageAPM
	result.AverageEAPM = summary.AverageEAPM
	if result.GamesPlayed == 0 {
		return result, db.ErrNotFound
	}
	if decided := result.GamesPlayed - result.Undecided; decided > 0 {
		result.WinRate = float64(result.Wins) / float64(decided)
	}
	gamesWithVectors, err := d.dbStore.GetPlayerFingerprintCoverage(d.ctx, playerKey, int64(scfingerprint.FeatureVersion()))
	if err != nil {
		return result, fmt.Errorf("failed to load fingerprint coverage: %w", err)
	}
	result.FingerprintCoverage = workflowFingerprintCoverage{
		GamesWithVectors: gamesWithVectors,
		FeatureVersion:   scfingerprint.FeatureVersion(),
		MinVectors:       fingerprintMatchMinVectors,
	}
	outcome, err := d.fingerprintOutcomeFor(playerKey, scfingerprint.FeatureVersion())
	if err != nil {
		return result, fmt.Errorf("failed to match progamer: %w", err)
	}
	result.FingerprintMatch = outcome.match
	result.FingerprintCoverage.NoMatchReason = outcome.reason
	result.PrimaryBadge, result.SecondaryBadge = d.resolvePlayerIdentityBadges(playerKey, result.FingerprintMatch)

	if err := d.populateAdvancedPlayerOverview(playerKey, &result); err != nil {
		return result, fmt.Errorf("failed to populate advanced player overview: %w", err)
	}

	result.NarrativeHints = buildPlayerNarrativeHints(result)
	return result, nil
}

func (d *Dashboard) buildWorkflowPlayerLastGames(playerKey string) ([]workflowGameListItem, error) {
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
			GameSource:         row.GameSource,
			LobbyKind:          row.LobbyKind,
			DurationSeconds:    row.DurationSeconds,
			GameType:           row.GameType,
			TeamFormat:         row.TeamFormat,
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
	// Ten games means ten cheap re-computations (see unit_composition.go).
	for i := range result {
		current := result[i].CurrentPlayer
		if current == nil {
			continue
		}
		boundaries, err := d.dbStore.GetPhaseBoundariesForReplay(d.ctx, result[i].ReplayID)
		if err != nil {
			return nil, fmt.Errorf("failed to load phase boundaries for %s: %w", playerName, err)
		}
		rows, err := d.dbStore.ListGameUnitProductionAndCasts(d.ctx, result[i].ReplayID)
		if err != nil {
			return nil, fmt.Errorf("failed to load production for %s: %w", playerName, err)
		}
		for _, comp := range computeCompositionForReplay(rows, boundaries) {
			if comp.PlayerID == current.PlayerID {
				current.Composition = append(current.Composition, comp)
			}
		}
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
	displayByName := d.youDisplayNames(playerNames)

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
		return result, db.ErrNotFound
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
		Description:     "Average actions per minute across this player's non-observer human games. Higher tends to mean more activity.",
		Details: []workflowPlayerInsightDetail{
			{Label: "Eligible players", Value: fmt.Sprintf("%d (minimum %d games)", histogram.PlayersIncluded, histogram.MinGames)},
			{Label: "Population mean", Value: fmt.Sprintf("%.1f APM", histogram.MeanAPM)},
			{Label: "Population stddev", Value: fmt.Sprintf("%.1f", histogram.StddevAPM)},
		},
	}
	if !histogram.PlayerEligible || histogram.PlayerAverageAPM == nil {
		result.IneligibleReason = fmt.Sprintf("Not enough games yet for a stable APM comparison. This view currently requires at least %d games.", histogram.MinGames)
		result.IneligibleReasonKey = "not_enough_games_apm"
		result.IneligibleReasonArgs = []string{fmt.Sprint(histogram.MinGames)}
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
		Description:     "Cadence looks at attacking-unit production rhythm from 7:00 until 80% game length, combining how fast you produce with how evenly you space it. Higher is better.",
		Details: []workflowPlayerInsightDetail{
			{Label: "Eligible players", Value: fmt.Sprintf("%d (minimum %d games)", leaderboard.PlayersIncluded, leaderboard.MinGames)},
			{Label: "Population mean", Value: fmt.Sprintf("%.3f", leaderboard.MeanCadence)},
			{Label: "Population stddev", Value: fmt.Sprintf("%.3f", leaderboard.StddevCadence)},
		},
	}
	if insight.GamesUsed < leaderboard.MinGames {
		result.IneligibleReason = fmt.Sprintf("Not enough eligible games yet. This view currently requires at least %d games with enough production events.", leaderboard.MinGames)
		result.IneligibleReasonKey = "not_enough_eligible_games_cadence"
		result.IneligibleReasonArgs = []string{fmt.Sprint(leaderboard.MinGames)}
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

func formatClockFromSeconds(second int64) string {
	if second < 0 {
		second = 0
	}
	minute := second / 60
	sec := second % 60
	return fmt.Sprintf("%d:%02d", minute, sec)
}

// The first 4 minutes are rendered separately as a time-scaled vertical chart
// (UnitsEarlyEvents), not bucketed.
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

// unitProductionRow is one unit or building a player started. Command rows guess it from what was
// clicked; engine rows are what the game made, with completion and cancel.
type unitProductionRow struct {
	PlayerID        int64
	Second          int64
	UnitType        string
	Count           int64
	CompletedSecond *int64
	Cancelled       bool
}

func (d *Dashboard) populateUnitsBySliceForGameDetail(detail *workflowGameDetail) error {
	rows, err := d.dbStore.ListUnitSliceCommandRows(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load unit slices: %w", err)
	}
	productions := make([]unitProductionRow, 0, len(rows))
	for _, row := range rows {
		// One Zergling Unit Morph produces a pair from a single larva, and source rows
		// carry Count=1 per command, so double the increment.
		inc := int64(1)
		if row.UnitType == models.GeneralUnitZergling {
			inc = 2
		}
		productions = append(productions, unitProductionRow{PlayerID: row.PlayerID, Second: row.Second, UnitType: row.UnitType, Count: inc})
	}
	buildUnitsBySlice(detail, productions)
	return nil
}

// buildUnitsBySlice fills the early build order, the full production timeline and the per-slice unit
// counts. A cancelled production shows in the early build order only, marked, and counts nowhere.
func buildUnitsBySlice(detail *workflowGameDetail, rows []unitProductionRow) {
	detail.UnitsBySlice = []workflowUnitSlice{}
	detail.UnitsEarlyEvents = []workflowUnitEarlyEventPlayer{}
	detail.ProductionTimeline = []workflowProductionTimelinePlayer{}
	playerOrder := make([]int64, 0, len(detail.Players))
	playerByID := map[int64]workflowGamePlayer{}
	for _, player := range detail.Players {
		playerOrder = append(playerOrder, player.PlayerID)
		playerByID[player.PlayerID] = player
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
		second := max(row.Second, 0)
		unitType := row.UnitType
		inc := row.Count
		_, isBuilding := buildingSet[unitType]

		if !row.Cancelled {
			timelineByPlayer[playerID] = append(timelineByPlayer[playerID], workflowProductionEvent{
				Second:     second,
				UnitType:   unitType,
				IsBuilding: isBuilding,
				Count:      inc,
			})
		}

		if second < 240 {
			label := ""
			if !row.Cancelled && (unitType == models.GeneralUnitSCV || unitType == models.GeneralUnitDrone || unitType == models.GeneralUnitProbe) {
				if _, ok := workerOrdinalByPlayer[playerID]; !ok {
					workerOrdinalByPlayer[playerID] = map[string]int64{}
				}
				workerOrdinalByPlayer[playerID][unitType]++
				// Players start with 4 workers, so the first one observed training is the 5th.
				n := workerOrdinalByPlayer[playerID][unitType] + 4
				label = fmt.Sprintf("%d%s %s", n, ordinalSuffix(int(n)), unitType)
			}
			earlyEventsByPlayer[playerID] = append(earlyEventsByPlayer[playerID], workflowUnitEarlyEvent{
				Second:          second,
				UnitType:        unitType,
				IsBuilding:      isBuilding,
				Label:           label,
				Count:           inc,
				CompletedSecond: row.CompletedSecond,
				Cancelled:       row.Cancelled,
			})
			continue
		}
		if row.Cancelled {
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

// populatePhaseMarkersForGameDetail copies the persisted Early/Mid game-end
// seconds onto the response so the frontend can split Game Events into three
// sections. Reading the persisted mid_game_starts / late_game_starts markers
// keeps this in lockstep with attacker-composition and avoids the action-type
// drift that broke recomputing them at request time from raw rows. Either field
// stays 0 when its marker is absent, and the frontend collapses that section.
// The algorithm lives in internal/patterns/phases.
func (d *Dashboard) populatePhaseMarkersForGameDetail(detail *workflowGameDetail) error {
	boundaries, err := d.dbStore.GetPhaseBoundariesForReplay(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to load phase-boundary markers: %w", err)
	}
	detail.EarlyGameEndsAtSecond = boundaries.EarlyEndsAtSecond
	detail.MidGameEndsAtSecond = boundaries.MidEndsAtSecond
	return nil
}

// zergBOEventSchema describes the per-BO event list for the count-based Zerg
// BOs: Drones is the ordered drone-morph count to render, and the booleans
// control the Pool / Overlord / Hatchery rows. Their ticks come from the command
// stream at game-detail time, not from the persisted detection.
type zergBOEventSchema struct {
	Drones      int
	HasOverlord bool
	HasPool     bool
	HasHatchery bool
}

// Expert (golden) ranges from the marker definition are attached when
// available, else NoExpert is set so the frontend renders the tick alone.
func buildZergBOEvents(schema zergBOEventSchema, expert []markers.ExpertEvent, t db.EarlyZergTimingsRow) []workflowMarkerEvent {
	expertBySubject := map[string]*markers.ExpertEvent{}
	for i := range expert {
		e := &expert[i]
		expertBySubject[e.Match.Subject] = e
	}
	events := make([]workflowMarkerEvent, 0, schema.Drones+3)

	// Labels start at "5th": the player begins with 4 drones.
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

// The fuzzy marker has no static schema — its rung is only known at detection
// time — so the drone count and defining-building row come from the resolved
// payload label instead.
func fuzzyZergSchemaFromLabel(featureKey, label string) (zergBOEventSchema, bool) {
	if featureKey != "bo_z_fuzzy" {
		return zergBOEventSchema{}, false
	}
	var n int
	var kind string
	if _, err := fmt.Sscanf(label, "~%d %s", &n, &kind); err != nil || n < 4 {
		return zergBOEventSchema{}, false
	}
	s := zergBOEventSchema{Drones: n - 4}
	switch kind {
	case "Pool":
		s.HasPool = true
	case "Overpool":
		s.HasPool = true
		s.HasOverlord = true
	case "Hatch":
		s.HasHatchery = true
		s.HasOverlord = n >= 10
	default:
		return zergBOEventSchema{}, false
	}
	return s, true
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

// populateMarkersForGameDetail attaches one expert-vs-actual entry per
// (player × detected BO). Actual timings are read from each marker's persisted
// payload, resolved once at detection time, so this path never re-parses
// commands. BO definitions overlap on purpose (a "9 pool into hatchery" game
// also matches "9 pool"), so several entries can surface for one player;
// registry ordering drives display order.
func (d *Dashboard) populateMarkersForGameDetail(detail *workflowGameDetail) error {
	detail.Markers = []workflowMarkerPlayer{}
	if len(detail.Players) == 0 {
		return nil
	}

	// Queried once per replay, so simplified-Zerg BO events render without
	// re-parsing.
	zergTimings := map[int64]db.EarlyZergTimingsRow{}
	if rows, err := d.dbStore.LoadEarlyZergTimings(d.ctx, detail.ReplayID); err == nil {
		for _, r := range rows {
			zergTimings[r.PlayerID] = r
		}
	}

	// Payload carries the Expert milestone seconds resolved at detection time.
	// Post-migration row.PatternName is the marker FeatureKey, so resolve through
	// the registry rather than matching "Build Order: <Name>" prefixes.
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

	// Broad definitions overlap on purpose, so render every match and let the user
	// interpret them; registry order puts specific variants next to their cousins.
	allMarkers := markers.Markers()
	// Read from the player_start events already on the timeline, so the FE can
	// render "X starts at L and opens with BO" on the consolidated openers row.
	startLocationByPlayer := map[int64]string{}
	for i := range detail.GameEvents {
		ev := &detail.GameEvents[i]
		if strings.EqualFold(ev.Type, "player_start") && ev.Actor != nil && ev.Base != nil {
			startLocationByPlayer[ev.Actor.PlayerID] = ev.Base.Name
		}
	}
	// Surfaced as ONE consolidated "bo_openers" event at second 0 rather than one
	// timed event per BO: the per-BO timing was uninformative and read as noise.
	// Every player gets an entry (empty BuildOrder when none resolved) so the FE
	// always shows their name.
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
			boModifiers := markers.DecodeModifiers([]byte(row.Payload))
			// Dynamic openers persist their resolved value ("~9 Overpool") in the payload;
			// bo.Name is only a placeholder.
			boLabel := bo.Name
			if resolved, ok := markers.DecodePayloadLabel([]byte(row.Payload)); ok {
				boLabel = resolved
			}
			var events []workflowMarkerEvent
			if schema, isSimplifiedZerg := zergBOEventSchemas[bo.FeatureKey]; isSimplifiedZerg {
				events = buildZergBOEvents(schema, bo.Expert, zergTimings[player.PlayerID])
			} else if schema, isFuzzy := fuzzyZergSchemaFromLabel(bo.FeatureKey, boLabel); isFuzzy {
				events = buildZergBOEvents(schema, markers.FuzzyZergExpertEvents(boLabel), zergTimings[player.PlayerID])
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
				Marker:     boLabel,
				FeatureKey: bo.FeatureKey,
				Events:     events,
				Modifiers:  boModifiers,
			})

			// Still gate on a resolved first building (units don't count): a BO that
			// matched only via produce facts has no concrete opening to show. The actual
			// second is unused now that the consolidated event sits at 0:00.
			//
			// The fuzzy opener persists only its label and no expert milestones, so
			// firstBuildingActualSecond can't see its building — but it fires only on a
			// clean pool/hatch open, so a resolved label is itself proof of one.
			_, hasFirstBuilding := firstBuildingActualSecond(bo, events)
			_, hasPayloadLabel := markers.DecodePayloadLabel([]byte(row.Payload))
			if hasFirstBuilding || hasPayloadLabel {
				boOpeners = append(boOpeners, workflowGameEventBuildOrder{
					PlayerID:      player.PlayerID,
					Name:          player.Name,
					Color:         player.Color,
					Race:          player.Race,
					TeamWon:       player.TeamWon,
					Team:          player.Team,
					StartLocation: startLocationByPlayer[player.PlayerID],
					BuildOrder:    boLabel,
					FeatureKey:    bo.FeatureKey,
					Modifiers:     boModifiers,
				})
				playerHadOpener = true
				anyBO = true
			}
		}
		// Emit a name-only entry so the FE still shows the player and their start.
		if !playerHadOpener {
			boOpeners = append(boOpeners, workflowGameEventBuildOrder{
				PlayerID:      player.PlayerID,
				Name:          player.Name,
				Color:         player.Color,
				Race:          player.Race,
				TeamWon:       player.TeamWon,
				Team:          player.Team,
				StartLocation: startLocationByPlayer[player.PlayerID],
			})
		}
	}
	// Type starts with "bo_" so the ownership back-propagation below hands it the
	// starting-position snapshot. Skipped entirely when nobody resolved a BO.
	if anyBO {
		detail.GameEvents = append(detail.GameEvents, workflowGameEvent{
			Type:        "bo_openers",
			Second:      0,
			BuildOrders: boOpeners,
		})
	}
	// populateDetectedPatterns left GameEvents time-sorted; re-sort after appending
	// BO entries so the timeline stays monotonic.
	sort.SliceStable(detail.GameEvents, func(i, j int) bool {
		return detail.GameEvents[i].Second < detail.GameEvents[j].Second
	})
	// replayEventsFromRows computed Ownership for real rows by walking transitions
	// in time order, but synthesized BO events are appended without it. BOs don't
	// change ownership, so each inherits the most recent prior snapshot — giving the
	// FE the same polygons it would draw for a real event at that second.
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

// One entry per side, when both mutalisk_timing and turret_timing fired. The
// payload schema mirrors workflowMarkerPlayer so BuildOrderTimelineRows renders
// it unchanged. Actual timings come from each marker's persisted payload.
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

	// Each side has 2 events: the prerequisite building, then what it gates. The
	// "built" second clamps to max(prereq_built, trigger) + build_time so queued
	// commands can't fictitiously land before their prerequisite. No Expert bands —
	// the only progamer reference is the muta-vs-turret gap in the summary below.
	type sideEventSpec struct {
		key       string
		subject   string
		buildTime int64
		actual    int
	}
	makeEvents := func(specs []sideEventSpec) []workflowMarkerEvent {
		events := make([]workflowMarkerEvent, 0, len(specs))
		// Clamp the next event's built time against the prerequisite's.
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

	// ExpertGap* are corpus-derived from the 723 labelled progamer 1v1 TvZ matches
	// where both markers fire (MEASUREMENT.md), with prerequisite-clamped finishes:
	// Mutalisk hatch = max(spire_built, morph_cmd) + 25, Missile Turret =
	// max(ebay_built, build_cmd) + 19. Median gap +10s, p25 +3, p75 +19: the turret
	// completes just after the muta hatches, which then spends ~10-20s crossing the
	// map, so turrets land just-in-time.
	summary := &workflowMutaliskTimingSummary{
		ExpertGapSeconds:    10,
		ExpertGapMinSeconds: 3,
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

// For non-Zerg openers the events array is position-aligned with bo.Expert, so
// walk Expert and take the first KindMakeBuilding. Simplified-Zerg BOs reshape
// the array, so scan it for the first known building Subject instead.
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
			PlayerID:            player.PlayerID,
			PlayerKey:           player.PlayerKey,
			PlayerName:          player.Name,
			Team:                player.Team,
			TeamWon:             player.TeamWon,
			Eligible:            false,
			IneligibleReason:    "not enough attacking-unit production samples in analysis window",
			IneligibleReasonKey: "not_enough_attacking_unit_samples",
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
			PlayerID:            player.PlayerID,
			PlayerKey:           player.PlayerKey,
			PlayerName:          player.Name,
			Team:                player.Team,
			TeamWon:             player.TeamWon,
			Eligible:            unitsProduced >= workflowUnitCadenceMinUnitsPerReplay && gapCount >= workflowUnitCadenceMinGapsPerReplay,
			WindowSeconds:       windowSeconds,
			UnitsProduced:       unitsProduced,
			GapCount:            gapCount,
			IneligibleReason:    "not enough attacking-unit production samples in analysis window",
			IneligibleReasonKey: "not_enough_attacking_unit_samples",
		}
		if ratePerMinute != nil {
			entry.RatePerMinute = *ratePerMinute
		}
		if cvGap != nil {
			entry.CVGap = *cvGap
		}
		if burstiness != nil {
			entry.Burstiness = *burstiness
		}
		if idle20Ratio != nil {
			entry.Idle20Ratio = *idle20Ratio
		}
		if cadenceScore != nil {
			entry.CadenceScore = *cadenceScore
		}
		if entry.Eligible {
			entry.IneligibleReason = ""
			entry.IneligibleReasonKey = ""
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

const (
	fingerprintTierConfirmed = "confirmed"
	fingerprintTierHigh      = "high"

	fingerprintMatchMinVectors = 3

	// Why a player carries no fingerprint match. These are different answers
	// and read differently: "we could not try" is not "we tried and it is not
	// you". Empty means a match was found.
	fingerprintNoMatchNotEnoughGames = "not_enough_games"
	fingerprintNoMatchNoCandidate    = "no_candidate"
	fingerprintNoMatchBelowBar       = "below_bar"

	fingerprintOperatingPointStrict   = "fpr_1e4"
	fingerprintOperatingPointModerate = "fpr_1e3"
)

// fingerprintTier reports the claim a match earns, or "" for no claim at all.
//
// There are only two claims. A hit that clears just fpr_1e2 is, family-wise
// across a catalog this size, roughly a coin flip, and a hit that misses its
// own identity bar says the probe looks like a crowded corner of style space
// rather than like that person. Neither is worth a name: attaching one to a
// player tells them they play like a progamer they have never been compared
// to, and the whole feature is only worth having if a badge can be trusted.
func fingerprintTier(m scfingerprint.MatchResult) string {
	if m.ModelIsSynthetic || !m.ClearsIdentityBar {
		return ""
	}
	clearsStrict := m.OperatingPoints[fingerprintOperatingPointStrict]
	clearsModerate := m.OperatingPoints[fingerprintOperatingPointModerate]

	switch {
	case clearsStrict && m.EvidenceN >= 3:
		return fingerprintTierConfirmed
	case clearsStrict || clearsModerate:
		return fingerprintTierHigh
	}
	return ""
}

func (d *Dashboard) fingerprintDataset() (*scfingerprint.Dataset, error) {
	d.fpDatasetOnce.Do(func() {
		d.fpDataset, d.fpDatasetErr = scfingerprint.BuiltinDataset(scfingerprint.ConfidenceHigh)
	})
	return d.fpDataset, d.fpDatasetErr
}

func (d *Dashboard) fingerprintRegistry() *scfingerprint.Registry {
	d.fpRegistryOnce.Do(func() {
		reg, err := scfingerprint.BuiltinRegistry()
		if err != nil {
			log.Printf("[fingerprint] loading built-in registry: %v", err)
			return
		}
		d.fpRegistry = reg
	})
	return d.fpRegistry
}

// fingerprintOutcome is a match or the reason there is none. The reason is the
// point: four separate conditions used to collapse into one nil, so a player
// with two games and a player who was compared against the whole catalogue and
// matched nobody got the same blank space.
type fingerprintOutcome struct {
	match  *workflowFingerprintMatch
	reason string
}

func (d *Dashboard) matchFingerprint(playerKey string, featureVersion int) (*workflowFingerprintMatch, error) {
	outcome, err := d.fingerprintOutcomeFor(playerKey, featureVersion)
	return outcome.match, err
}

func (d *Dashboard) fingerprintOutcomeFor(playerKey string, featureVersion int) (fingerprintOutcome, error) {
	view := d.dbStore.CorpusView()
	d.fpMatchCacheMu.RLock()
	if d.fpMatchCache != nil && d.fpMatchCacheView == view {
		if entry, ok := d.fpMatchCache[playerKey]; ok {
			d.fpMatchCacheMu.RUnlock()
			return entry, nil
		}
	}
	d.fpMatchCacheMu.RUnlock()

	outcome, err := d.computeFingerprintMatch(playerKey, featureVersion)
	if err != nil {
		return fingerprintOutcome{}, err
	}

	// An outcome computed while the corpus moved on describes neither view,
	// so it is returned but not cached.
	if d.dbStore.CorpusView() != view {
		return outcome, nil
	}
	d.fpMatchCacheMu.Lock()
	if d.fpMatchCache == nil || d.fpMatchCacheView != view {
		d.fpMatchCache = make(map[string]fingerprintOutcome)
		d.fpMatchCacheView = view
	}
	d.fpMatchCache[playerKey] = outcome
	d.fpMatchCacheMu.Unlock()

	return outcome, nil
}

func (d *Dashboard) computeFingerprintMatch(playerKey string, featureVersion int) (fingerprintOutcome, error) {
	notEnough := fingerprintOutcome{reason: fingerprintNoMatchNotEnoughGames}
	rows, err := d.dbStore.ListPlayerFingerprintVectors(d.ctx, playerKey, int64(featureVersion))
	if err != nil {
		return fingerprintOutcome{}, fmt.Errorf("listing fingerprint vectors: %w", err)
	}
	if len(rows) < fingerprintMatchMinVectors {
		return notEnough, nil
	}

	games := make([]scfingerprint.PlayerGame, 0, len(rows))
	for _, row := range rows {
		if len(row.Vector) == 0 {
			continue
		}
		games = append(games, scfingerprint.PlayerGame{
			Vector: row.Vector,
			Race:   row.Race,
			Toon:   row.Name,
		})
	}
	if len(games) < fingerprintMatchMinVectors {
		return notEnough, nil
	}

	dataset, err := d.fingerprintDataset()
	if err != nil {
		return fingerprintOutcome{}, fmt.Errorf("loading fingerprint dataset: %w", err)
	}

	reg := d.fingerprintRegistry()
	var opts []scfingerprint.Option
	if reg != nil {
		opts = append(opts, scfingerprint.WithRegistry(reg))
	}

	results, err := scfingerprint.MatchMany(games, dataset, opts...)
	if err != nil {
		return fingerprintOutcome{}, fmt.Errorf("matching fingerprint: %w", err)
	}
	if len(results) == 0 {
		return fingerprintOutcome{reason: fingerprintNoMatchNoCandidate}, nil
	}

	top := results[0]
	tier := fingerprintTier(top)
	if tier == "" {
		return fingerprintOutcome{reason: fingerprintNoMatchBelowBar}, nil
	}

	match := &workflowFingerprintMatch{
		Label:             top.Label,
		Liquipedia:        top.Liquipedia,
		Z:                 top.Z,
		EvidenceN:         top.EvidenceN,
		SearchFPR:         top.SearchFPR,
		Tier:              tier,
		IdentityBar:       top.IdentityBar,
		ClearsIdentityBar: top.ClearsIdentityBar,
		ModelSynthetic:    top.ModelIsSynthetic,
	}
	if top.Registry != nil {
		match.Registry = &workflowRegistryOpinion{
			Name:     top.Registry.Name,
			Toon:     top.Registry.Toon,
			AuroraID: top.Registry.AuroraID,
			Agrees:   top.Registry.Agrees,
		}
	}
	// Backward compat: map new tiers to old confidence values.
	if tier == fingerprintTierConfirmed || tier == fingerprintTierHigh {
		match.Confidence = fingerprintTierHigh
	}
	return fingerprintOutcome{match: match}, nil
}

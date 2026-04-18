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

	db "github.com/marianogappa/screpdb/internal/dashboard/db"
)

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

func (d *Dashboard) playerTimingsFromReplayCommands(players []workflowGamePlayer, rows []db.TimingRow) ([]workflowPlayerTimingSeries, error) {
	seriesByPlayer, playerOrder := initPlayerTimingSeries(players)
	orderByPlayer := map[int64]int64{}
	for _, row := range rows {
		playerID := row.PlayerID
		second := row.Second
		current := orderByPlayer[playerID] + 1
		orderByPlayer[playerID] = current
		if s, ok := seriesByPlayer[playerID]; ok {
			s.Points = append(s.Points, workflowTimingPoint{Second: second, Order: current})
		}
	}
	return orderedTimingSeries(seriesByPlayer, playerOrder), nil
}

func (d *Dashboard) playerLabeledTimingsFromReplayCommands(players []workflowGamePlayer, rows []db.TimingRow) ([]workflowPlayerTimingSeries, error) {
	seriesByPlayer, playerOrder := initPlayerTimingSeries(players)
	orderByPlayerAndLabel := map[int64]map[string]int64{}
	for _, row := range rows {
		playerID := row.PlayerID
		second := row.Second
		label := row.Label
		if _, ok := orderByPlayerAndLabel[playerID]; !ok {
			orderByPlayerAndLabel[playerID] = map[string]int64{}
		}
		current := orderByPlayerAndLabel[playerID][label] + 1
		orderByPlayerAndLabel[playerID][label] = current
		if s, ok := seriesByPlayer[playerID]; ok {
			s.Points = append(s.Points, workflowTimingPoint{Second: second, Order: current, Label: label})
		}
	}
	return orderedTimingSeries(seriesByPlayer, playerOrder), nil
}

func playerExpansionTimingsFromGameEvents(events []workflowGameEvent, players []workflowGamePlayer) []workflowPlayerTimingSeries {
	seriesByPlayer, playerOrder := initPlayerTimingSeries(players)
	orderByPlayer := map[int64]int64{}
	for _, event := range events {
		typeLower := strings.ToLower(event.Type)
		if typeLower != "expansion" {
			continue
		}
		playerID := int64(0)
		if event.Actor != nil {
			playerID = event.Actor.PlayerID
		}
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

	hotkeyGamesRate, err := d.dbStore.ListHotkeyGamesRateByPlayer(d.ctx)
	if err != nil {
		return err
	}
	queuedGames, err := d.countQueuedGamesForPlayer(playerKey)
	if err != nil {
		return err
	}
	result.HotkeyUsageRate = hotkeyGamesRate[playerKey] / 100.0
	result.QueuedGames = queuedGames
	if result.GamesPlayed > 0 {
		result.QueuedGameRate = float64(queuedGames) / float64(result.GamesPlayed)
	}
	result.FingerprintMetrics = []workflowComparativeMetric{}

	return nil
}

func (d *Dashboard) commonBehavioursForPlayer(playerKey string, gamesPlayed int64) ([]workflowCommonBehaviour, error) {
	if gamesPlayed <= 0 {
		return []workflowCommonBehaviour{}, nil
	}
	rows, err := d.dbStore.ListCommonBehaviours(d.ctx, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load common behaviours: %w", err)
	}
	out := []workflowCommonBehaviour{}
	for _, row := range rows {
		patternName := row.PatternName
		replayCount := row.ReplayCount
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
	CategoryLabel    string
	ActionTypes      []string
	NameColumn       string
	UseInstanceShare bool
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
	playerSummary, err := d.dbStore.GetOutlierPlayerSummary(d.ctx, playerKey)
	if err != nil {
		return result, fmt.Errorf("failed to load player for outliers: %w", err)
	}
	if playerSummary.Count <= 0 || playerSummary.Name == nil || strings.TrimSpace(*playerSummary.Name) == "" {
		return result, sql.ErrNoRows
	}
	result.PlayerName = *playerSummary.Name

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
		{CategoryLabel: "Order", ActionTypes: []string{"Targeted Order"}, NameColumn: "order_name", UseInstanceShare: true},
		{CategoryLabel: "Build", ActionTypes: []string{"Build", "Building Morph"}, NameColumn: "unit_type"},
		{CategoryLabel: "Train", ActionTypes: []string{"Train"}, NameColumn: "unit_type"},
		{CategoryLabel: "Morph", ActionTypes: []string{"Unit Morph"}, NameColumn: "unit_type"},
		{CategoryLabel: "Tech", ActionTypes: []string{"Tech"}, NameColumn: "tech_name"},
		{CategoryLabel: "Upgrade", ActionTypes: []string{"Upgrade"}, NameColumn: "upgrade_name"},
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
	rows, err := d.dbStore.ListPlayerGamesByRace(d.ctx, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load player games by race: %w", err)
	}
	out := map[string]int64{}
	for _, row := range rows {
		race := row.Race
		games := row.Count
		out[strings.TrimSpace(race)] = games
	}
	return out, nil
}

func (d *Dashboard) populationGamesByRace() (map[string]int64, error) {
	rows, err := d.dbStore.ListPopulationGamesByRace(d.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load population games by race: %w", err)
	}
	out := map[string]int64{}
	for _, row := range rows {
		race := row.Race
		games := row.Count
		out[strings.TrimSpace(race)] = games
	}
	return out, nil
}

func (d *Dashboard) populationDistinctPlayersByRace() (map[string]float64, error) {
	rows, err := d.dbStore.ListPopulationDistinctPlayersByRace(d.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load distinct players by race: %w", err)
	}
	out := map[string]float64{}
	for _, row := range rows {
		race := row.Race
		players := row.Value
		out[strings.TrimSpace(race)] = players
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
	playerRows, err := d.dbStore.ListOutlierPlayerCounts(d.ctx, playerKey, primaryRace, spec.NameColumn, spec.UseInstanceShare, spec.ActionTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to query player outliers for %s: %w", spec.CategoryLabel, err)
	}

	type pair struct {
		race string
		name string
	}
	playerCounts := map[pair]int64{}
	for _, row := range playerRows {
		race := row.Race
		name := row.Name
		games := row.Count
		playerCounts[pair{race: strings.TrimSpace(race), name: strings.TrimSpace(name)}] = games
	}

	globalRows, err := d.dbStore.ListOutlierGlobalRows(d.ctx, primaryRace, spec.NameColumn, spec.UseInstanceShare, spec.ActionTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to query baseline outliers for %s: %w", spec.CategoryLabel, err)
	}
	globalGames := map[pair]int64{}
	globalPlayers := map[pair]float64{}
	for _, row := range globalRows {
		race := row.Race
		name := row.Name
		games := row.Games
		players := row.Players
		key := pair{race: strings.TrimSpace(race), name: strings.TrimSpace(name)}
		globalGames[key] = games
		globalPlayers[key] = players
	}

	// For targeted orders we compare usage share in terms of raw order instances,
	// not replay incidence. These totals are intentionally built from the filtered
	// item universe so numerator and denominator stay aligned.
	playerTargetedTotalsByRace := map[string]int64{}
	globalTargetedTotalsByRace := map[string]int64{}
	if spec.UseInstanceShare {
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
		if spec.UseInstanceShare {
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
		if spec.UseInstanceShare {
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
	switch spec.CategoryLabel {
	case "Tech":
		_, ok := workflowProtossAllowedTechs[canonical]
		return ok
	case "Upgrade":
		_, ok := workflowProtossAllowedUpgrades[canonical]
		return ok
	case "Order":
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
	total, err := d.dbStore.CountDistinctPlayers(d.ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count distinct players: %w", err)
	}
	return total, nil
}

func (d *Dashboard) totalDistinctPlayersByRace(race string) (float64, error) {
	total, err := d.dbStore.CountDistinctPlayersByRace(d.ctx, race)
	if err != nil {
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

	playerRows, err := d.dbStore.ReplayQueryContext(d.ctx, playerQuery, playerKey, race)
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

	popRows, err := d.dbStore.ReplayQueryContext(d.ctx, populationQuery, race)
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

func (d *Dashboard) firstExpansionAverageByPlayer() (map[string]float64, error) {
	rows, err := d.dbStore.ListExpansionEvents(d.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load game events for expansion averages: %w", err)
	}
	playersByReplay, err := d.playersByReplay()
	if err != nil {
		return nil, err
	}
	valuesByPlayer := map[string][]int64{}
	firstByReplayAndPlayer := map[int64]map[int64]int64{}
	for _, row := range rows {
		replayID := row.ReplayID
		if row.PlayerID == nil {
			continue
		}
		playerID := *row.PlayerID
		if playerID == 0 {
			continue
		}
		if _, ok := firstByReplayAndPlayer[replayID]; !ok {
			firstByReplayAndPlayer[replayID] = map[int64]int64{}
		}
		current, exists := firstByReplayAndPlayer[replayID][playerID]
		if !exists || row.Second < current {
			firstByReplayAndPlayer[replayID][playerID] = row.Second
		}
	}
	for replayID, firstByPlayer := range firstByReplayAndPlayer {
		players := playersByReplay[replayID]
		if len(players) == 0 {
			continue
		}
		for playerID, second := range firstByPlayer {
			playerKey := normalizePlayerKey(playerNameByID(playerID, players))
			if playerKey == "" {
				continue
			}
			valuesByPlayer[playerKey] = append(valuesByPlayer[playerKey], second)
		}
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
	rows, err := d.dbStore.ListPlayersByReplayRows(d.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load players by replay: %w", err)
	}
	out := map[int64][]workflowGamePlayer{}
	for _, row := range rows {
		replayID := row.ReplayID
		playerID := row.PlayerID
		name := row.Name
		out[replayID] = append(out[replayID], workflowGamePlayer{
			PlayerID:  playerID,
			PlayerKey: normalizePlayerKey(name),
			Name:      name,
		})
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

func (d *Dashboard) playerNameForKey(playerKey string) (string, error) {
	playerName, err := d.dbStore.GetPlayerNameByKey(d.ctx, playerKey)
	if err != nil {
		return "", err
	}
	if playerName == "" {
		return "", sql.ErrNoRows
	}
	return playerName, nil
}

func (d *Dashboard) loadRaceOrderSummaryForPlayer(playerKey string) ([]workflowRaceOrderSummary, error) {
	rows, err := d.dbStore.ListRaceOrderRows(d.ctx, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load race order summary: %w", err)
	}

	type gameOrders struct {
		race     string
		techs    []string
		upgrades []string
	}
	byGame := map[int64]*gameOrders{}
	for _, row := range rows {
		playerID := row.PlayerID
		race := row.Race
		actionType := row.ActionType
		if _, ok := byGame[playerID]; !ok {
			byGame[playerID] = &gameOrders{race: race, techs: []string{}, upgrades: []string{}}
		}
		entry := byGame[playerID]
		if actionType == "Tech" && row.TechName != nil && len(entry.techs) < 6 {
			entry.techs = append(entry.techs, *row.TechName)
		}
		if actionType == "Upgrade" && row.UpgradeName != nil && len(entry.upgrades) < 6 {
			entry.upgrades = append(entry.upgrades, *row.UpgradeName)
		}
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
	count, err := d.dbStore.CountQueuedGamesByPlayer(d.ctx, playerKey)
	if err != nil {
		return 0, fmt.Errorf("failed to count queued games: %w", err)
	}
	return count, nil
}

func (d *Dashboard) countCarrierGamesForPlayer(playerKey string) (int64, error) {
	count, err := d.dbStore.CountCarrierGamesByPlayer(d.ctx, playerKey)
	if err != nil {
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
	"gl": {}, "hf": {}, "wp": {}, "pls": {}, "plz": {}, "ok": {}, "yes": {}, "no": {}, "nah": {}, "lol": {},
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
		ExampleMessages: []string{},
	}

	rows, err := d.dbStore.ListPlayerChatRows(d.ctx, playerKey)
	if err != nil {
		return summary, err
	}

	termCounts := map[string]int64{}
	gamesWithChat := map[int64]struct{}{}
	rawMessages := []string{}

	for _, row := range rows {
		replayID := row.ReplayID
		raw := row.Message
		msg := strings.TrimSpace(raw)
		if msg == "" {
			continue
		}
		rawMessages = append(rawMessages, msg)
		gamesWithChat[replayID] = struct{}{}

		tokens := summarizeChatTokens(msg)
		for _, token := range tokens {
			termCounts[token]++
		}
	}
	summary.TotalMessages = int64(len(rawMessages))
	summary.GamesWithChat = int64(len(gamesWithChat))
	summary.DistinctTerms = int64(len(termCounts))
	summary.TopTerms = summarizeChatCounts(termCounts, 10)
	summary.ExampleMessages = summarizeChatExamples(rawMessages, 5)

	return summary, nil
}

func summarizeChatTokens(message string) []string {
	lowered := strings.ToLower(message)
	rawTokens := workflowChatWordSplitter.FindAllString(lowered, -1)
	result := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		token = strings.Trim(token, "'")
		if token == "gg" {
			result = append(result, token)
			continue
		}
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
		hints = append(hints, fmt.Sprintf("Hotkeys appear in %.1f%% of this player's games.", player.HotkeyUsageRate*100))
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

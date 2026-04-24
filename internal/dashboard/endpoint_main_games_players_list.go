package dashboard

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"

	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

func (d *Dashboard) listWorkflowPlayers(limit, offset int, filters workflowPlayersListFilters, sortSpec workflowPlayersListSort) ([]workflowPlayersListItem, int64, workflowPlayersListFilterOptions, error) {
	baseSQL, baseArgs := buildWorkflowPlayersListBaseSQL(filters)
	whereSQL, whereArgs := buildWorkflowPlayersListWhere(filters)
	allArgs := append(append([]any{}, baseArgs...), whereArgs...)

	total, err := d.dbStore.CountWorkflowPlayers(d.ctx, baseSQL, whereSQL, allArgs)
	if err != nil {
		return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
	}

	sortColumn := sortSpec.Column
	sortDir := "ASC"
	if sortSpec.Desc {
		sortDir = "DESC"
	}

	listRows, err := d.dbStore.ListWorkflowPlayers(d.ctx, baseSQL, whereSQL, sortColumn, sortDir, allArgs, limit, offset)
	if err != nil {
		return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
	}
	playerNames := make([]string, 0, len(listRows))
	for _, row := range listRows {
		playerNames = append(playerNames, row.PlayerName)
	}
	displayByName, err := d.aliasDisplayNames(playerNames)
	if err != nil {
		return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
	}

	items := []workflowPlayersListItem{}
	for _, row := range listRows {
		item := workflowPlayersListItem{}
		item.PlayerKey = row.PlayerKey
		item.PlayerName = row.PlayerName
		if displayName, ok := displayByName[row.PlayerName]; ok {
			item.PlayerName = displayName
		}
		item.Race = row.Race
		item.GamesPlayed = row.GamesPlayed
		item.AverageAPM = row.AverageAPM
		item.LastPlayed = row.LastPlayed
		item.LastPlayedDaysAgo = row.LastPlayedDaysAgo
		if item.LastPlayedDaysAgo < 0 {
			item.LastPlayedDaysAgo = 0
		}
		items = append(items, item)
	}

	filterOptions, err := d.workflowPlayersListFilterOptions(baseSQL, baseArgs, whereSQL, whereArgs)
	if err != nil {
		return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
	}
	return items, total, filterOptions, nil
}

func buildWorkflowPlayersListBaseSQL(filters workflowPlayersListFilters) (string, []any) {
	return dashboarddb.BuildWorkflowPlayersListBaseSQL(normalizePlayerKey(filters.NameContains))
}

func buildWorkflowPlayersListWhere(filters workflowPlayersListFilters) (string, []any) {
	return dashboarddb.BuildWorkflowPlayersListWhere(filters.OnlyFivePlus, filters.LastPlayedBuckets)
}

func parseWorkflowPlayersListFilters(r *http.Request) workflowPlayersListFilters {
	filters := workflowPlayersListFilters{
		NameContains:      strings.TrimSpace(r.URL.Query().Get("name")),
		LastPlayedBuckets: parseCSVQueryValues(r.URL.Query()["last_played"], true),
	}
	onlyFivePlus := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("only_5_plus")))
	if onlyFivePlus == "1" || onlyFivePlus == "true" || onlyFivePlus == "on" || onlyFivePlus == "yes" {
		filters.OnlyFivePlus = true
	}
	return filters
}

func parseWorkflowPlayersListSort(r *http.Request) workflowPlayersListSort {
	sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_by")))
	sortDir := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_dir")))
	columnBySortBy := map[string]string{
		"name":        "player_name",
		"race":        "race",
		"games":       "games_played",
		"apm":         "average_apm",
		"last_played": "last_played_days_ago",
	}
	column, ok := columnBySortBy[sortBy]
	if !ok {
		column = "games_played"
	}
	desc := sortDir != "asc"
	return workflowPlayersListSort{Column: column, Desc: desc}
}

func (d *Dashboard) workflowPlayersListFilterOptions(baseSQL string, baseArgs []any, whereSQL string, whereArgs []any) (workflowPlayersListFilterOptions, error) {
	result := workflowPlayersListFilterOptions{
		Races: []workflowPlayersListFilterOption{},
		LastPlayed: []workflowPlayersListFilterOption{
			{Key: "1m", Label: "Last month"},
			{Key: "3m", Label: "Last 3 months"},
		},
	}

	countRowArgs := append(append([]any{}, baseArgs...), whereArgs...)
	count1m, count3m, err := d.dbStore.CountWorkflowLastPlayedBuckets(d.ctx, baseSQL, whereSQL, countRowArgs)
	if err != nil {
		return result, err
	}
	result.LastPlayed = []workflowPlayersListFilterOption{
		{Key: "1m", Label: "Last month", Count: count1m},
		{Key: "3m", Label: "Last 3 months", Count: count3m},
	}
	return result, nil
}

func parseOptionalInt64Query(r *http.Request, key string) (int64, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseOptionalFloatQuery(r *http.Request, key string) (float64, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseWorkflowUnitCadenceFilterMode(raw string) (workflowUnitCadenceFilterMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(workflowUnitCadenceFilterStrict):
		return workflowUnitCadenceFilterStrict, nil
	case string(workflowUnitCadenceFilterBroad):
		return workflowUnitCadenceFilterBroad, nil
	default:
		return "", fmt.Errorf("invalid filter mode: %s", raw)
	}
}

func prettyWorkflowRaceLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "protoss":
		return "Protoss"
	case "terran":
		return "Terran"
	case "zerg":
		return "Zerg"
	default:
		return "Random"
	}
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
	return dashboarddb.BuildWorkflowGamesListWhere(
		filters.PlayerKeys,
		filters.MapNames,
		filters.DurationBuckets,
		filters.FeaturingKeys,
		dashboarddb.WorkflowDurationSQLByKey(),
	)
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
	rows, err := d.dbStore.ListReplayPlayers(d.ctx, replayIDs)
	if err != nil {
		return err
	}
	playerNames := make([]string, 0, len(rows))
	for _, row := range rows {
		playerNames = append(playerNames, row.Name)
	}
	displayByName, err := d.aliasDisplayNames(playerNames)
	if err != nil {
		return err
	}
	for _, row := range rows {
		var player workflowGameListPlayer
		replayID := row.ReplayID
		player.PlayerID = row.PlayerID
		player.Name = row.Name
		if displayName, ok := displayByName[row.Name]; ok {
			player.Name = displayName
		}
		player.Team = row.Team
		player.IsWinner = row.IsWinner
		player.PlayerKey = normalizePlayerKey(row.Name)
		idx, ok := itemIndexByReplayID[replayID]
		if !ok {
			continue
		}
		items[idx].Players = append(items[idx].Players, player)
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
	rowsPlayerPatterns, err := d.dbStore.ListFeaturingPlayerPatternRows(d.ctx, replayIDs)
	if err != nil {
		return err
	}
	for _, row := range rowsPlayerPatterns {
		replayID := row.ReplayID
		// Post-markers-migration: row.PatternName carries the marker FeatureKey
		// (row presence alone = match; no value-column truthiness check needed).
		featureKey := strings.TrimSpace(strings.ToLower(row.PatternName))
		switch featureKey {
		case "carriers":
			featureSets[replayID]["carriers"] = struct{}{}
		case "battlecruisers":
			featureSets[replayID]["battlecruisers"] = struct{}{}
		case "made_recalls":
			featureSets[replayID]["recalls"] = struct{}{}
		case "threw_nukes":
			featureSets[replayID]["nukes"] = struct{}{}
		case "became_terran", "became_zerg":
			featureSets[replayID]["mind_control"] = struct{}{}
		default:
			// Build-order markers route directly to their featuring key.
			if bo := markers.ByFeatureKey(featureKey); bo != nil {
				featureSets[replayID][bo.FeatureKey] = struct{}{}
			}
		}
	}
	rowsReplayEvents, err := d.dbStore.ListFeaturingReplayEventRows(d.ctx, replayIDs)
	if err != nil {
		return err
	}
	for _, row := range rowsReplayEvents {
		replayID := row.ReplayID
		switch strings.ToLower(strings.TrimSpace(row.EventType)) {
		case "zergling_rush":
			featureSets[replayID]["zergling_rush"] = struct{}{}
		case "cannon_rush":
			featureSets[replayID]["cannon_rush"] = struct{}{}
		case "bunker_rush":
			featureSets[replayID]["bunker_rush"] = struct{}{}
		}
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

func (d *Dashboard) populateWorkflowRecentGamesCurrentPlayer(playerKey string, items []workflowGameListItem) error {
	replayIDs := make([]int64, 0, len(items))
	itemByReplayID := map[int64]*workflowGameListItem{}
	for i := range items {
		replayIDs = append(replayIDs, items[i].ReplayID)
		itemByReplayID[items[i].ReplayID] = &items[i]
	}
	if len(replayIDs) == 0 {
		return nil
	}
	playerRows, err := d.dbStore.ListCurrentPlayersForReplayIDs(d.ctx, playerKey, replayIDs)
	if err != nil {
		return err
	}
	playerNames := make([]string, 0, len(playerRows))
	for _, row := range playerRows {
		playerNames = append(playerNames, row.Name)
	}
	displayByName, err := d.aliasDisplayNames(playerNames)
	if err != nil {
		return err
	}
	playerIDs := []int64{}
	currentByPlayerID := map[int64]*workflowRecentGamePlayer{}
	for _, row := range playerRows {
		replayID := row.ReplayID
		currentPlayer := &workflowRecentGamePlayer{DetectedPatterns: []workflowPatternValue{}}
		currentPlayer.PlayerID = row.PlayerID
		currentPlayer.Name = row.Name
		if displayName, ok := displayByName[row.Name]; ok {
			currentPlayer.Name = displayName
		}
		currentPlayer.Race = row.Race
		currentPlayer.IsWinner = row.IsWinner
		currentPlayer.PlayerKey = normalizePlayerKey(row.Name)
		item := itemByReplayID[replayID]
		if item == nil {
			continue
		}
		item.CurrentPlayer = currentPlayer
		playerIDs = append(playerIDs, currentPlayer.PlayerID)
		currentByPlayerID[currentPlayer.PlayerID] = currentPlayer
	}
	if len(playerIDs) == 0 {
		return nil
	}
	patternRows, err := d.dbStore.ListPatternValuesForPlayerIDs(d.ctx, playerIDs)
	if err != nil {
		return err
	}
	for _, row := range patternRows {
		playerID := row.PlayerID
		pattern := workflowPatternValue{PatternName: row.PatternName, Value: row.PatternValue}
		pattern.Value = formatPatternValueForUI(pattern.PatternName, pattern.Value)
		currentPlayer := currentByPlayerID[playerID]
		if currentPlayer == nil {
			continue
		}
		currentPlayer.DetectedPatterns = append(currentPlayer.DetectedPatterns, pattern)
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
	if len(teamOrder) <= 1 {
		names := make([]string, 0, len(players))
		for _, p := range players {
			names = append(names, p.Name)
		}
		return strings.Join(names, ", ")
	}
	sides := make([]string, 0, len(teamOrder))
	for _, team := range teamOrder {
		teamPlayers := playersByTeam[team]
		switch len(teamPlayers) {
		case 0:
			continue
		case 1:
			sides = append(sides, teamPlayers[0])
		default:
			sides = append(sides, strings.Join(teamPlayers, ", "))
		}
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

	rowsPlayers, err := d.dbStore.ListWorkflowFilterPlayers(d.ctx)
	if err != nil {
		return result, err
	}
	playerNames := make([]string, 0, len(rowsPlayers))
	for _, row := range rowsPlayers {
		playerNames = append(playerNames, row.Label)
	}
	displayByName, err := d.aliasDisplayNames(playerNames)
	if err != nil {
		return result, err
	}
	for _, row := range rowsPlayers {
		var option workflowGamesListFilterOption
		option.Key = row.Key
		option.Label = row.Label
		if displayName, ok := displayByName[row.Label]; ok {
			option.Label = displayName
		}
		option.Games = row.Games
		result.Players = append(result.Players, option)
	}

	rowsMaps, err := d.dbStore.ListWorkflowFilterMaps(d.ctx)
	if err != nil {
		return result, err
	}
	for _, row := range rowsMaps {
		var option workflowGamesListFilterOption
		option.Label = row.Label
		option.Games = row.Games
		option.Key = strings.ToLower(strings.TrimSpace(option.Label))
		result.Maps = append(result.Maps, option)
	}
	under10m, m10to20, m20to30, m30to45, m45Plus, err := d.dbStore.CountWorkflowDurationBuckets(d.ctx)
	if err != nil {
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

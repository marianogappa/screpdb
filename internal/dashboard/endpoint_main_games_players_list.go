package dashboard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (d *Dashboard) handlerGamesList(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r, 20, 200)
	filters := parseWorkflowGamesListFilters(r)
	whereSQL, whereArgs := buildWorkflowGamesListWhere(filters)

	countQuery := "SELECT COUNT(*) FROM replays r " + whereSQL
	var total int64
	if err := d.currentReplayScopedDB().QueryRowContext(d.ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		http.Error(w, "failed to count games: "+err.Error(), http.StatusInternalServerError)
		return
	}

	listArgs := append([]any{}, whereArgs...)
	listArgs = append(listArgs, limit, offset)
	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT
			r.id,
			r.replay_date,
			r.file_name,
			r.map_name,
			r.duration_seconds,
			r.game_type
		FROM replays r
	`+whereSQL+`
		ORDER BY r.replay_date DESC, r.id DESC
		LIMIT ? OFFSET ?
	`, listArgs...)
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
		); err != nil {
			http.Error(w, "failed to parse games list: "+err.Error(), http.StatusInternalServerError)
			return
		}
		item.Players = []workflowGameListPlayer{}
		item.Featuring = []string{}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "failed to iterate games list: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := d.populateWorkflowGameListPlayers(items); err != nil {
		http.Error(w, "failed to enrich games list players: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := d.populateWorkflowGameListFeaturing(items); err != nil {
		http.Error(w, "failed to enrich games list featuring: "+err.Error(), http.StatusInternalServerError)
		return
	}
	filterOptions, err := d.workflowGamesListFilterOptions()
	if err != nil {
		http.Error(w, "failed to build games list filters: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"summary_version": workflowSummaryVersion,
		"items":           items,
		"limit":           limit,
		"offset":          offset,
		"total":           total,
		"filter_options":  filterOptions,
	})
}

func (d *Dashboard) handlerPlayersList(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r, 20, 200)
	filters := parseWorkflowPlayersListFilters(r)
	sortSpec := parseWorkflowPlayersListSort(r)

	items, total, filterOptions, err := d.listWorkflowPlayers(limit, offset, filters, sortSpec)
	if err != nil {
		http.Error(w, "failed to list players: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"summary_version": workflowSummaryVersion,
		"items":           items,
		"limit":           limit,
		"offset":          offset,
		"total":           total,
		"filter_options":  filterOptions,
	})
}

func (d *Dashboard) listWorkflowPlayers(limit, offset int, filters workflowPlayersListFilters, sortSpec workflowPlayersListSort) ([]workflowPlayersListItem, int64, workflowPlayersListFilterOptions, error) {
	baseSQL, baseArgs := buildWorkflowPlayersListBaseSQL(filters)
	whereSQL, whereArgs := buildWorkflowPlayersListWhere(filters)
	allArgs := append(append([]any{}, baseArgs...), whereArgs...)

	countQuery := `WITH player_agg AS (` + baseSQL + `) SELECT COUNT(*) FROM player_agg ` + whereSQL
	var total int64
	if err := d.currentReplayScopedDB().QueryRowContext(d.ctx, countQuery, allArgs...).Scan(&total); err != nil {
		return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
	}

	sortColumn := sortSpec.Column
	sortDir := "ASC"
	if sortSpec.Desc {
		sortDir = "DESC"
	}

	listArgs := append(append([]any{}, allArgs...), limit, offset)
	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		WITH player_agg AS (`+baseSQL+`)
		SELECT
			player_key,
			player_name,
			race,
			games_played,
			average_apm,
			last_played,
			last_played_days_ago
		FROM player_agg
	`+whereSQL+`
		ORDER BY `+sortColumn+` `+sortDir+`, player_name ASC
		LIMIT ? OFFSET ?
	`, listArgs...)
	if err != nil {
		return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
	}
	defer rows.Close()

	items := []workflowPlayersListItem{}
	for rows.Next() {
		item := workflowPlayersListItem{}
		if err := rows.Scan(
			&item.PlayerKey,
			&item.PlayerName,
			&item.Race,
			&item.GamesPlayed,
			&item.AverageAPM,
			&item.LastPlayed,
			&item.LastPlayedDaysAgo,
		); err != nil {
			return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
		}
		if item.LastPlayedDaysAgo < 0 {
			item.LastPlayedDaysAgo = 0
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
	}

	filterOptions, err := d.workflowPlayersListFilterOptions(baseSQL, baseArgs, whereSQL, whereArgs)
	if err != nil {
		return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
	}
	return items, total, filterOptions, nil
}

func buildWorkflowPlayersListBaseSQL(filters workflowPlayersListFilters) (string, []any) {
	baseWhere := []string{"p.is_observer = 0", "lower(trim(coalesce(p.type, ''))) = 'human'"}
	args := []any{}
	if filters.NameContains != "" {
		baseWhere = append(baseWhere, "lower(trim(p.name)) LIKE ?")
		args = append(args, "%"+normalizePlayerKey(filters.NameContains)+"%")
	}
	sqlText := `
		SELECT
			player_key,
			player_name,
			games_played,
			average_apm,
			last_played,
			CASE
				WHEN games_played <= 0 THEN 'Random'
				WHEN protoss_games * 1.0 / games_played > 0.67 THEN 'Protoss'
				WHEN terran_games * 1.0 / games_played > 0.67 THEN 'Terran'
				WHEN zerg_games * 1.0 / games_played > 0.67 THEN 'Zerg'
				ELSE 'Random'
			END AS race,
			COALESCE(CAST(julianday('now') - julianday(substr(last_played, 1, 19)) AS INTEGER), 0) AS last_played_days_ago
		FROM (
			SELECT
				lower(trim(p.name)) AS player_key,
				MIN(p.name) AS player_name,
				COUNT(*) AS games_played,
				COALESCE(AVG(CASE WHEN p.apm > 0 THEN p.apm END), 0) AS average_apm,
				MAX(r.replay_date) AS last_played,
				SUM(CASE WHEN lower(trim(p.race)) = 'protoss' THEN 1 ELSE 0 END) AS protoss_games,
				SUM(CASE WHEN lower(trim(p.race)) = 'terran' THEN 1 ELSE 0 END) AS terran_games,
				SUM(CASE WHEN lower(trim(p.race)) = 'zerg' THEN 1 ELSE 0 END) AS zerg_games
			FROM players p
			JOIN replays r ON r.id = p.replay_id
			WHERE ` + strings.Join(baseWhere, " AND ") + `
			GROUP BY lower(trim(p.name))
		) grouped
	`
	return sqlText, args
}

func buildWorkflowPlayersListWhere(filters workflowPlayersListFilters) (string, []any) {
	clauses := []string{}
	args := []any{}
	if filters.OnlyFivePlus {
		clauses = append(clauses, "games_played >= 5")
	}
	if len(filters.LastPlayedBuckets) > 0 {
		bucketClauses := []string{}
		for _, bucket := range filters.LastPlayedBuckets {
			switch strings.ToLower(strings.TrimSpace(bucket)) {
			case "1m", "30d":
				bucketClauses = append(bucketClauses, "last_played_days_ago <= 30")
			case "3m", "90d":
				bucketClauses = append(bucketClauses, "last_played_days_ago <= 90")
			}
		}
		if len(bucketClauses) > 0 {
			clauses = append(clauses, "("+strings.Join(bucketClauses, " OR ")+")")
		}
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
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
	var count1m, count3m int64
	if err := d.currentReplayScopedDB().QueryRowContext(d.ctx, `
		WITH player_agg AS (`+baseSQL+`)
		SELECT
			COALESCE(SUM(CASE WHEN last_played_days_ago <= 30 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN last_played_days_ago <= 90 THEN 1 ELSE 0 END), 0)
		FROM player_agg
	`+whereSQL+`
	`, countRowArgs...).Scan(&count1m, &count3m); err != nil {
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
	clauses := []string{}
	args := []any{}

	if len(filters.PlayerKeys) > 0 {
		playerPlaceholders := buildInClausePlaceholders(len(filters.PlayerKeys))
		clauses = append(clauses, "EXISTS (SELECT 1 FROM players p WHERE p.replay_id = r.id AND p.is_observer = 0 AND lower(trim(p.name)) IN ("+playerPlaceholders+"))")
		for _, key := range filters.PlayerKeys {
			args = append(args, key)
		}
	}

	if len(filters.MapNames) > 0 {
		mapPlaceholders := buildInClausePlaceholders(len(filters.MapNames))
		clauses = append(clauses, "lower(trim(r.map_name)) IN ("+mapPlaceholders+")")
		for _, mapName := range filters.MapNames {
			args = append(args, strings.ToLower(strings.TrimSpace(mapName)))
		}
	}

	if len(filters.DurationBuckets) > 0 {
		durationClauses := []string{}
		for _, key := range filters.DurationBuckets {
			for _, bucket := range workflowDurationFilterBuckets {
				if key == bucket.Key {
					durationClauses = append(durationClauses, "("+bucket.SQL+")")
					break
				}
			}
		}
		if len(durationClauses) > 0 {
			clauses = append(clauses, "("+strings.Join(durationClauses, " OR ")+")")
		}
	}

	if len(filters.FeaturingKeys) > 0 {
		featureClauses := []string{}
		for _, featureKey := range filters.FeaturingKeys {
			existsSQL, ok := workflowFeaturingExistsSQL(featureKey)
			if !ok {
				continue
			}
			featureClauses = append(featureClauses, existsSQL)
		}
		if len(featureClauses) > 0 {
			clauses = append(clauses, "("+strings.Join(featureClauses, " OR ")+")")
		}
	}

	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func workflowFeaturingExistsSQL(featureKey string) (string, bool) {
	switch strings.TrimSpace(strings.ToLower(featureKey)) {
	case "carriers":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay_player dprp
			WHERE dprp.replay_id = r.id
				AND lower(trim(dprp.pattern_name)) = 'carriers'
				AND dprp.value_bool = 1
		)`, true
	case "battlecruisers":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay_player dprp
			WHERE dprp.replay_id = r.id
				AND lower(trim(dprp.pattern_name)) = 'battlecruisers'
				AND dprp.value_bool = 1
		)`, true
	case "mind_control":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay_player dprp
			WHERE dprp.replay_id = r.id
				AND lower(trim(dprp.pattern_name)) IN ('became terran', 'became zerg')
				AND (dprp.value_timestamp IS NOT NULL OR dprp.value_int IS NOT NULL OR dprp.value_string IS NOT NULL)
		)`, true
	case "nukes":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay_player dprp
			WHERE dprp.replay_id = r.id
				AND lower(trim(dprp.pattern_name)) = 'threw nukes'
				AND (dprp.value_timestamp IS NOT NULL OR dprp.value_int IS NOT NULL OR dprp.value_string IS NOT NULL OR dprp.value_bool = 1)
		)`, true
	case "recalls":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay_player dprp
			WHERE dprp.replay_id = r.id
				AND lower(trim(dprp.pattern_name)) = 'made recalls'
				AND (dprp.value_timestamp IS NOT NULL OR dprp.value_int IS NOT NULL OR dprp.value_string IS NOT NULL OR dprp.value_bool = 1)
		)`, true
	case "cannon_rush", "bunker_rush":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay dpr
			WHERE dpr.replay_id = r.id
				AND lower(trim(dpr.pattern_name)) = 'game events'
				AND lower(coalesce(dpr.value_string, '')) LIKE '%cannon/bunker rushes%'
		)`, true
	case "zergling_rush":
		return `EXISTS (
			SELECT 1
			FROM detected_patterns_replay dpr
			WHERE dpr.replay_id = r.id
				AND lower(trim(dpr.pattern_name)) = 'game events'
				AND lower(coalesce(dpr.value_string, '')) LIKE '%zergling rushes%'
		)`, true
	default:
		return "", false
	}
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
	placeholders := buildInClausePlaceholders(len(replayIDs))
	args := make([]any, 0, len(replayIDs))
	for _, replayID := range replayIDs {
		args = append(args, replayID)
	}
	rows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT replay_id, id, name, team, is_winner
		FROM players
		WHERE is_observer = 0
			AND replay_id IN (`+placeholders+`)
		ORDER BY replay_id ASC, team ASC, id ASC
	`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var replayID int64
		var player workflowGameListPlayer
		if err := rows.Scan(&replayID, &player.PlayerID, &player.Name, &player.Team, &player.IsWinner); err != nil {
			return err
		}
		player.PlayerKey = normalizePlayerKey(player.Name)
		idx, ok := itemIndexByReplayID[replayID]
		if !ok {
			continue
		}
		items[idx].Players = append(items[idx].Players, player)
	}
	if err := rows.Err(); err != nil {
		return err
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
	placeholders := buildInClausePlaceholders(len(replayIDs))
	args := make([]any, 0, len(replayIDs))
	for _, replayID := range replayIDs {
		args = append(args, replayID)
	}

	rowsPlayerPatterns, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT replay_id, pattern_name, value_bool, value_int, value_string, value_timestamp
		FROM detected_patterns_replay_player
		WHERE replay_id IN (`+placeholders+`)
			AND lower(trim(pattern_name)) IN ('carriers', 'battlecruisers', 'made recalls', 'threw nukes', 'became terran', 'became zerg')
	`, args...)
	if err != nil {
		return err
	}
	defer rowsPlayerPatterns.Close()
	for rowsPlayerPatterns.Next() {
		var replayID int64
		var patternName string
		var valueBool sql.NullBool
		var valueInt sql.NullInt64
		var valueString sql.NullString
		var valueTimestamp sql.NullInt64
		if err := rowsPlayerPatterns.Scan(&replayID, &patternName, &valueBool, &valueInt, &valueString, &valueTimestamp); err != nil {
			return err
		}
		if !workflowTruthyPatternValue(valueBool, valueInt, valueString, valueTimestamp) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(patternName)) {
		case "carriers":
			featureSets[replayID]["carriers"] = struct{}{}
		case "battlecruisers":
			featureSets[replayID]["battlecruisers"] = struct{}{}
		case "made recalls":
			featureSets[replayID]["recalls"] = struct{}{}
		case "threw nukes":
			featureSets[replayID]["nukes"] = struct{}{}
		case "became terran", "became zerg":
			featureSets[replayID]["mind_control"] = struct{}{}
		}
	}
	if err := rowsPlayerPatterns.Err(); err != nil {
		return err
	}

	rowsReplayPatterns, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT replay_id, value_string
		FROM detected_patterns_replay
		WHERE replay_id IN (`+placeholders+`)
			AND lower(trim(pattern_name)) = 'game events'
	`, args...)
	if err != nil {
		return err
	}
	defer rowsReplayPatterns.Close()
	for rowsReplayPatterns.Next() {
		var replayID int64
		var gameEventsRaw sql.NullString
		if err := rowsReplayPatterns.Scan(&replayID, &gameEventsRaw); err != nil {
			return err
		}
		if !gameEventsRaw.Valid {
			continue
		}
		events := parseGameEvents(gameEventsRaw.String)
		for _, event := range events {
			description := strings.ToLower(strings.TrimSpace(event.Description))
			if strings.Contains(description, "zergling rushes") {
				featureSets[replayID]["zergling_rush"] = struct{}{}
			}
			if strings.Contains(description, "cannon/bunker rushes") {
				featureSets[replayID]["cannon_rush"] = struct{}{}
				featureSets[replayID]["bunker_rush"] = struct{}{}
			}
		}
	}
	if err := rowsReplayPatterns.Err(); err != nil {
		return err
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
	placeholders := buildInClausePlaceholders(len(replayIDs))
	args := make([]any, 0, len(replayIDs)+1)
	args = append(args, playerKey)
	for _, replayID := range replayIDs {
		args = append(args, replayID)
	}

	playerRows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT replay_id, id, name, race, is_winner
		FROM players
		WHERE lower(trim(name)) = ?
			AND is_observer = 0
			AND replay_id IN (`+placeholders+`)
	`, args...)
	if err != nil {
		return err
	}
	defer playerRows.Close()
	playerIDs := []int64{}
	currentByPlayerID := map[int64]*workflowRecentGamePlayer{}
	for playerRows.Next() {
		var replayID int64
		currentPlayer := &workflowRecentGamePlayer{DetectedPatterns: []workflowPatternValue{}}
		if err := playerRows.Scan(&replayID, &currentPlayer.PlayerID, &currentPlayer.Name, &currentPlayer.Race, &currentPlayer.IsWinner); err != nil {
			return err
		}
		currentPlayer.PlayerKey = normalizePlayerKey(currentPlayer.Name)
		item := itemByReplayID[replayID]
		if item == nil {
			continue
		}
		item.CurrentPlayer = currentPlayer
		playerIDs = append(playerIDs, currentPlayer.PlayerID)
		currentByPlayerID[currentPlayer.PlayerID] = currentPlayer
	}
	if err := playerRows.Err(); err != nil {
		return err
	}
	if len(playerIDs) == 0 {
		return nil
	}

	patternPlaceholders := buildInClausePlaceholders(len(playerIDs))
	patternArgs := make([]any, 0, len(playerIDs))
	for _, playerID := range playerIDs {
		patternArgs = append(patternArgs, playerID)
	}
	patternRows, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
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
		WHERE player_id IN (`+patternPlaceholders+`)
		ORDER BY player_id ASC, pattern_name ASC
	`, patternArgs...)
	if err != nil {
		return err
	}
	defer patternRows.Close()
	for patternRows.Next() {
		var playerID int64
		var pattern workflowPatternValue
		if err := patternRows.Scan(&playerID, &pattern.PatternName, &pattern.Value); err != nil {
			return err
		}
		pattern.Value = formatPatternValueForUI(pattern.PatternName, pattern.Value)
		currentPlayer := currentByPlayerID[playerID]
		if currentPlayer == nil {
			continue
		}
		currentPlayer.DetectedPatterns = append(currentPlayer.DetectedPatterns, pattern)
	}
	return patternRows.Err()
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
	usesTeams := false
	for _, team := range teamOrder {
		if len(playersByTeam[team]) > 1 {
			usesTeams = true
			break
		}
	}
	sides := make([]string, 0, len(teamOrder))
	for _, team := range teamOrder {
		teamPlayers := playersByTeam[team]
		if usesTeams && len(teamPlayers) > 1 {
			sides = append(sides, "("+strings.Join(teamPlayers, " & ")+")")
			continue
		}
		sides = append(sides, strings.Join(teamPlayers, ", "))
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

	rowsPlayers, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT lower(trim(name)) AS player_key, MIN(name) AS player_name, COUNT(*) AS games
		FROM players
		WHERE is_observer = 0
		GROUP BY lower(trim(name))
		HAVING COUNT(*) >= 5
		ORDER BY games DESC, player_name ASC
		LIMIT 200
	`)
	if err != nil {
		return result, err
	}
	defer rowsPlayers.Close()
	for rowsPlayers.Next() {
		var option workflowGamesListFilterOption
		if err := rowsPlayers.Scan(&option.Key, &option.Label, &option.Games); err != nil {
			return result, err
		}
		result.Players = append(result.Players, option)
	}
	if err := rowsPlayers.Err(); err != nil {
		return result, err
	}

	rowsMaps, err := d.currentReplayScopedDB().QueryContext(d.ctx, `
		SELECT MIN(map_name) AS map_name, COUNT(*) AS games
		FROM replays
		GROUP BY lower(trim(map_name))
		ORDER BY games DESC, map_name ASC
		LIMIT 15
	`)
	if err != nil {
		return result, err
	}
	defer rowsMaps.Close()
	for rowsMaps.Next() {
		var option workflowGamesListFilterOption
		if err := rowsMaps.Scan(&option.Label, &option.Games); err != nil {
			return result, err
		}
		option.Key = strings.ToLower(strings.TrimSpace(option.Label))
		result.Maps = append(result.Maps, option)
	}
	if err := rowsMaps.Err(); err != nil {
		return result, err
	}

	durationCountQuery := `
		SELECT
			COALESCE(SUM(CASE WHEN duration_seconds < 600 THEN 1 ELSE 0 END), 0) AS under_10m,
			COALESCE(SUM(CASE WHEN duration_seconds >= 600 AND duration_seconds < 1200 THEN 1 ELSE 0 END), 0) AS m10_20,
			COALESCE(SUM(CASE WHEN duration_seconds >= 1200 AND duration_seconds < 1800 THEN 1 ELSE 0 END), 0) AS m20_30,
			COALESCE(SUM(CASE WHEN duration_seconds >= 1800 AND duration_seconds < 2700 THEN 1 ELSE 0 END), 0) AS m30_45,
			COALESCE(SUM(CASE WHEN duration_seconds >= 2700 THEN 1 ELSE 0 END), 0) AS m45_plus
		FROM replays
	`
	var under10m, m10to20, m20to30, m30to45, m45Plus int64
	if err := d.currentReplayScopedDB().QueryRowContext(d.ctx, durationCountQuery).Scan(&under10m, &m10to20, &m20to30, &m30to45, &m45Plus); err != nil {
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

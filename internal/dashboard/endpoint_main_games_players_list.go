package dashboard

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"

	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

func (d *Dashboard) listWorkflowPlayers(limit, offset int, filters workflowPlayersListFilters, sortSpec workflowPlayersListSort) ([]workflowPlayersListItem, int64, workflowPlayersListFilterOptions, error) {
	query := buildPlayersQuery(filters, sortSpec)

	total, err := d.dbStore.CountPlayers(d.ctx, query)
	if err != nil {
		return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
	}

	listRows, err := d.dbStore.ListPlayers(d.ctx, query, limit, offset)
	if err != nil {
		return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
	}
	playerNames := make([]string, 0, len(listRows))
	for _, row := range listRows {
		playerNames = append(playerNames, row.PlayerName)
	}
	displayByName := d.youDisplayNames(playerNames)
	playerKeys := make([]string, 0, len(listRows))
	for _, row := range listRows {
		playerKeys = append(playerKeys, row.PlayerKey)
	}
	countryCodes, _ := d.countryCodesByPlayerKeys(playerKeys)

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
		item.CountryCode = countryCodes[item.PlayerKey]
		item.PrimaryBadge = d.primaryIdentityBadge(item.PlayerKey)
		items = append(items, item)
	}

	// Kick off flag lookups for the players on this page. The rows arrive in the
	// user's chosen sort order, so the most significant ones are already first
	// and the backfill cap keeps those; the page polls
	// /api/custom/bnet/country-codes to paint flags in as they land.
	backfillNames := make([]string, 0, len(items))
	for _, item := range items {
		if item.CountryCode == "" {
			backfillNames = append(backfillNames, item.PlayerKey)
		}
	}
	d.backfillBnetProfiles(backfillNames)

	filterOptions, err := d.workflowPlayersListFilterOptions(query)
	if err != nil {
		return []workflowPlayersListItem{}, 0, workflowPlayersListFilterOptions{}, err
	}
	return items, total, filterOptions, nil
}

func buildPlayersQuery(filters workflowPlayersListFilters, sortSpec workflowPlayersListSort) dashboarddb.PlayersQuery {
	sortDir := "ASC"
	if sortSpec.Desc {
		sortDir = "DESC"
	}
	return dashboarddb.PlayersQuery{
		NameFilter:   normalizePlayerKey(filters.NameContains),
		OnlyFivePlus: filters.OnlyFivePlus,
		LastPlayed:   filters.LastPlayedBuckets,
		Races:        filters.Races,
		ApmBuckets:   filters.ApmBuckets,
		GamesBuckets: filters.GamesBuckets,
		SortColumn:   sortSpec.Column,
		SortDir:      sortDir,
	}
}

func parseWorkflowPlayersListFilters(r *http.Request) workflowPlayersListFilters {
	filters := workflowPlayersListFilters{
		NameContains:      strings.TrimSpace(r.URL.Query().Get("name")),
		LastPlayedBuckets: parseCSVQueryValues(r.URL.Query()["last_played"], true),
		Races:             parseCSVQueryValues(r.URL.Query()["races"], true),
		ApmBuckets:        parseCSVQueryValues(r.URL.Query()["apm"], true),
		GamesBuckets:      parseCSVQueryValues(r.URL.Query()["games"], true),
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

func (d *Dashboard) workflowPlayersListFilterOptions(query dashboarddb.PlayersQuery) (workflowPlayersListFilterOptions, error) {
	result := workflowPlayersListFilterOptions{
		Races: []workflowPlayersListFilterOption{},
		LastPlayed: []workflowPlayersListFilterOption{
			{Key: "1m", Label: "Last month"},
			{Key: "3m", Label: "Last 3 months"},
		},
	}

	count1m, count3m, err := d.dbStore.CountPlayersLastPlayedBuckets(d.ctx, query)
	if err != nil {
		return result, err
	}
	result.LastPlayed = []workflowPlayersListFilterOption{
		{Key: "1m", Label: "Last month", Count: count1m},
		{Key: "3m", Label: "Last 3 months", Count: count3m},
	}

	facets, err := d.dbStore.CountPlayerFacets(d.ctx, query)
	if err != nil {
		return result, err
	}
	// Races are offered only where somebody actually plays them, so a corpus
	// with no Random players carries no dead Random chip.
	for _, race := range []string{"zerg", "terran", "protoss", "random"} {
		if count := facets.Races[race]; count > 0 {
			result.Races = append(result.Races, workflowPlayersListFilterOption{Key: race, Label: playersRaceLabel(race), Count: count})
		}
	}
	for _, key := range dashboarddb.PlayerApmBuckets {
		result.Apm = append(result.Apm, workflowPlayersListFilterOption{Key: key, Label: playersApmLabel(key), Count: facets.Apm[key]})
	}
	for _, key := range dashboarddb.PlayerGamesBuckets {
		result.Games = append(result.Games, workflowPlayersListFilterOption{Key: key, Label: playersGamesLabel(key), Count: facets.Games[key]})
	}
	return result, nil
}

// The labels below are the English source the frontend catalogs key off; the
// UI translates by the option key, so these never reach a non-English screen.
func playersRaceLabel(race string) string {
	switch race {
	case "zerg":
		return "Zerg"
	case "terran":
		return "Terran"
	case "protoss":
		return "Protoss"
	case "random":
		return "Random"
	}
	return race
}

func playersApmLabel(key string) string {
	switch key {
	case dashboarddb.PlayerApmUnder60:
		return "Under 60 APM"
	case dashboarddb.PlayerApm60To120:
		return "60 to 120 APM"
	case dashboarddb.PlayerApm120To200:
		return "120 to 200 APM"
	case dashboarddb.PlayerApm200Plus:
		return "200+ APM"
	}
	return key
}

func playersGamesLabel(key string) string {
	switch key {
	case dashboarddb.PlayerGames1To4:
		return "1 to 4 games"
	case dashboarddb.PlayerGames5To19:
		return "5 to 19 games"
	case dashboarddb.PlayerGames20Plus:
		return "20+ games"
	}
	return key
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
		MatchupKeys:     parseCSVQueryValues(r.URL.Query()["matchup"], true),
		MapKindKeys:     parseCSVQueryValues(r.URL.Query()["map_kind"], true),
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

func buildGamesQuery(filters workflowGamesListFilters) dashboarddb.GamesQuery {
	return dashboarddb.GamesQuery{
		PlayerKeys:      filters.PlayerKeys,
		MapNames:        filters.MapNames,
		DurationBuckets: filters.DurationBuckets,
		Featuring:       filters.FeaturingKeys,
		MatchupKeys:     filters.MatchupKeys,
		MapKindKeys:     filters.MapKindKeys,
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
	rows, err := d.dbStore.ListReplayPlayers(d.ctx, replayIDs)
	if err != nil {
		return err
	}
	playerNames := make([]string, 0, len(rows))
	for _, row := range rows {
		playerNames = append(playerNames, row.Name)
	}
	displayByName := d.youDisplayNames(playerNames)
	playerKeys := make([]string, 0, len(rows))
	for _, row := range rows {
		playerKeys = append(playerKeys, normalizePlayerKey(row.Name))
	}
	countryCodes, _ := d.countryCodesByPlayerKeys(playerKeys)
	for _, row := range rows {
		var player workflowGameListPlayer
		replayID := row.ReplayID
		player.PlayerID = row.PlayerID
		player.Name = row.Name
		if displayName, ok := displayByName[row.Name]; ok {
			player.Name = displayName
		}
		player.Race = row.Race
		player.Team = row.Team
		player.TeamWon = row.TeamWon
		player.PlayerOutcome = row.PlayerOutcome.String()
		player.Dropped = row.Dropped
		if d.debug {
			player.OutcomeTrace = outcomeTrace(row)
		}
		player.PlayerKey = normalizePlayerKey(row.Name)
		player.CountryCode = countryCodes[player.PlayerKey]
		player.PrimaryBadge = d.primaryIdentityBadge(player.PlayerKey)
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
	// Resolved payload labels per (replay, featureKey) for dynamic markers
	// (e.g. "3 Hatch Muta", "~9 Overpool"). A game can feature more than one
	// value of the same marker, so each distinct label becomes its own pill.
	featureLabels := map[int64]map[string][]string{}
	for i, item := range items {
		replayIDs = append(replayIDs, item.ReplayID)
		itemIndexByReplayID[item.ReplayID] = i
		featureSets[item.ReplayID] = map[string]struct{}{}
		featureLabels[item.ReplayID] = map[string][]string{}
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
		case "carriers", "battlecruisers", "ten_plus_scouts", "cliff_drop":
			featureSets[replayID][featureKey] = struct{}{}
		case "made_recalls":
			featureSets[replayID]["recalls"] = struct{}{}
		case "threw_nukes":
			featureSets[replayID]["nukes"] = struct{}{}
		case "became_terran", "became_zerg":
			featureSets[replayID]["mind_control"] = struct{}{}
		default:
			// Build-order markers never reach the games-list Featuring
			// column. An opener is a per-player property, but this column is
			// per-game, so a row full of "~10 Hatch" / "~9 Overpool" chips
			// says nothing about who opened that way and crowds out the
			// game-level features the column exists to show. The BO tab and
			// the per-player summary pill still surface openers inside the
			// game detail page, and the BO filter dropdown still selects on
			// them.
			if bo := markers.ByFeatureKey(featureKey); bo != nil {
				if bo.Kind == markers.KindInitialBuildOrder {
					continue
				}
				featureSets[replayID][bo.FeatureKey] = struct{}{}
				// row.ValueString carries the marker payload (see the query).
				// Dynamic markers persist their resolved value there; collect
				// each distinct one so the pill shows the number.
				if label, ok := markers.DecodePayloadLabel([]byte(nullableStringValue(row.ValueString))); ok {
					existing := featureLabels[replayID][bo.FeatureKey]
					if !slices.Contains(existing, label) {
						featureLabels[replayID][bo.FeatureKey] = append(existing, label)
					}
				}
			}
		}
	}
	rowsReplayEvents, err := d.dbStore.ListFeaturingReplayEventRows(d.ctx, replayIDs)
	if err != nil {
		return err
	}
	for _, row := range rowsReplayEvents {
		replayID := row.ReplayID
		eventType := strings.ToLower(strings.TrimSpace(row.EventType))
		switch eventType {
		case "zergling_rush", "cannon_rush", "bunker_rush",
			"proxy_gate", "proxy_rax", "proxy_factory", "proxy_starport", "manner_pylon":
			featureSets[replayID][eventType] = struct{}{}
		case "drop", "cliff_drop":
			// Every drop variant lights up the generic "drop" chip; the
			// specific subtype (cliff_drop) also lights its own chip.
			featureSets[replayID]["drop"] = struct{}{}
			featureSets[replayID][eventType] = struct{}{}
		}
	}
	for replayID, set := range featureSets {
		idx, ok := itemIndexByReplayID[replayID]
		if !ok {
			continue
		}
		labels := make([]string, 0, len(set))
		keys := make([]string, 0, len(set))
		seenLabel := make(map[string]struct{}, len(set))
		for _, cfg := range workflowFeaturingFilters {
			if _, has := set[cfg.Key]; !has {
				continue
			}
			// Prefer resolved payload labels ("3 Hatch Muta") over the static
			// placeholder ("N Hatch Muta"); a game may feature several distinct
			// values of the same marker. Fall back to cfg.Label when the marker
			// has no payload label.
			pillLabels := featureLabels[replayID][cfg.Key]
			if len(pillLabels) == 0 {
				pillLabels = []string{cfg.Label}
			}
			for _, label := range pillLabels {
				// Distinct keys can share a label (e.g. a unit marker and a
				// build-order opener that read the same word) — collapse to one pill.
				if _, dup := seenLabel[label]; dup {
					continue
				}
				seenLabel[label] = struct{}{}
				labels = append(labels, label)
				if label == cfg.Label {
					keys = append(keys, cfg.Key)
				} else {
					keys = append(keys, "")
				}
			}
		}
		items[idx].Featuring = labels
		items[idx].FeaturingKeys = keys
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
	displayByName := d.youDisplayNames(playerNames)
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
		currentPlayer.TeamWon = row.TeamWon
		currentPlayer.APM = row.APM
		currentPlayer.EAPM = row.EAPM
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
	dropped, err := d.dbStore.ListDroppedPlayerIDs(d.ctx, playerIDs)
	if err != nil {
		return err
	}
	for playerID, currentPlayer := range currentByPlayerID {
		currentPlayer.Disconnected = dropped[playerID]
	}
	patternRows, err := d.dbStore.ListPatternValuesForPlayerIDs(d.ctx, playerIDs)
	if err != nil {
		return err
	}
	for _, row := range patternRows {
		playerID := row.PlayerID
		pattern := buildWorkflowPatternValue(row.PatternName, row.PatternValue, row.DetectedSecond, row.Payload)
		currentPlayer := currentByPlayerID[playerID]
		if currentPlayer == nil {
			continue
		}
		currentPlayer.DetectedPatterns = append(currentPlayer.DetectedPatterns, pattern)
	}
	return nil
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
		Matchups:  []workflowGamesListFilterOption{},
		MapKinds:  []workflowGamesListFilterOption{},
	}

	rowsPlayers, err := d.dbStore.ListWorkflowFilterPlayers(d.ctx)
	if err != nil {
		return result, err
	}
	playerNames := make([]string, 0, len(rowsPlayers))
	for _, row := range rowsPlayers {
		playerNames = append(playerNames, row.Label)
	}
	displayByName := d.youDisplayNames(playerNames)
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
	// Two-bucket UI: <10m and ≥10m. The underlying SQL count query still
	// returns the legacy 5-bucket split; sum the four ≥10m buckets here.
	durationCounts := map[string]int64{
		"under_10m": under10m,
		"10m_plus":  m10to20 + m20to30 + m30to45 + m45Plus,
	}
	for _, bucket := range workflowDurationFilterBuckets {
		result.Durations = append(result.Durations, workflowGamesListFilterOption{
			Key:   bucket.Key,
			Label: bucket.Label,
			Games: durationCounts[bucket.Key],
		})
	}

	// The fuzzy Zerg opener has no single meaningful value — its identity is the
	// resolved rung ("~9 Overpool", "~10 Hatch"). Replace the placeholder
	// "Approx. opener" bucket with one filterable pill per distinct value present
	// in the data (none if there are no fuzzy openers).
	fuzzyLabels, err := d.dbStore.ListDistinctMarkerLabels(d.ctx, "bo_z_fuzzy")
	if err != nil {
		return result, err
	}
	for _, feature := range workflowFeaturingFilters {
		if feature.Key == "bo_z_fuzzy" {
			for _, label := range fuzzyLabels {
				result.Featuring = append(result.Featuring, workflowGamesListFilterOption{
					Key:   dashboarddb.PerValueFeatureKey("bo_z_fuzzy", label),
					Label: label,
					Group: feature.Group,
					Race:  feature.Race,
				})
			}
			continue
		}
		result.Featuring = append(result.Featuring, workflowGamesListFilterOption{
			Key:       feature.Key,
			Label:     feature.Label,
			Group:     feature.Group,
			IconKey:   feature.IconKey,
			IconKeys:  feature.IconKeys,
			IconLabel: feature.IconLabel,
			Emoji:     feature.Emoji,
			Race:      feature.Race,
		})
	}
	// Populate the per-option game counts the chips have always had a field for.
	// A count is the difference between a filter menu you can browse and one you
	// have to guess at, and it is the only way 67 build orders become navigable.
	featureKeys := make([]string, 0, len(result.Featuring))
	for _, option := range result.Featuring {
		featureKeys = append(featureKeys, option.Key)
	}
	featureCounts, err := d.dbStore.CountWorkflowFeaturingGames(d.ctx, featureKeys)
	if err != nil {
		return result, err
	}
	for i := range result.Featuring {
		result.Featuring[i].Games = featureCounts[result.Featuring[i].Key]
	}

	matchupCounts, err := d.dbStore.CountWorkflowMatchupGames(d.ctx)
	if err != nil {
		return result, err
	}
	for _, matchup := range workflowMatchupFilters {
		result.Matchups = append(result.Matchups, workflowGamesListFilterOption{
			Key:   matchup.Key,
			Label: matchup.Label,
			Games: matchupCounts[matchup.Key],
		})
	}
	mapKindCounts, err := d.dbStore.CountWorkflowMapKindGames(d.ctx)
	if err != nil {
		return result, err
	}
	for _, mapKind := range workflowMapKindFilters {
		result.MapKinds = append(result.MapKinds, workflowGamesListFilterOption{
			Key:   mapKind.Key,
			Label: mapKind.Label,
			Games: mapKindCounts[mapKind.Key],
		})
	}
	return result, nil
}


// outcomeTrace spells out how a player's two results were reached, for the
// hover overlay the --debug flag turns on. Deliberately English only: it is a
// diagnostic, and keeping it out of the locale catalogues is the point.
func outcomeTrace(row dashboarddb.WorkflowGamePlayerRow) []string {
	lines := []string{"YOU — " + row.PlayerOutcome.String()}
	if row.PlayerOutcomeReason != "" {
		lines = append(lines, "    "+row.PlayerOutcomeReason)
	}
	if row.TeamOutcomeReason != "" {
		lines = append(lines, "", "YOUR SIDE", "    "+row.TeamOutcomeReason)
	}
	// Battle.net contributes in two different ways and the trace has to
	// distinguish them, because a silent contribution is what hides a bug.
	// Here it did not supply a result — it supplied the recording the result
	// was read from, which is a stronger form of help and was invisible before.
	switch {
	case row.CompleteCopy:
		lines = append(lines, "", "BATTLE.NET — supplied this recording",
			"    your own copy ended before the game did, so a co-player's longer",
			"    recording was downloaded and the result read from that")
	case row.BnetOutcomeSource != "":
		lines = append(lines, "", "BATTLE.NET — "+row.BnetOutcomeSource)
	}
	return lines
}

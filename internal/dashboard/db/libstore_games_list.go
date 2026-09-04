package db

import (
	"context"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

// libDurationBuckets is the Go form of workflowDurationSQLByKey: the two
// duration keys the games-list filter offers, OR-ed together when several are
// selected. Unrecognised keys impose nothing, as the SQL builder dropped them.
var libDurationBuckets = map[string]func(seconds uint16) bool{
	"under_10m": func(seconds uint16) bool { return seconds < 600 },
	"10m_plus":  func(seconds uint16) bool { return seconds >= 600 },
}

// libFeatureShape is what a UI featuring key MEANS over a replay record. It
// mirrors workflowFeaturingCountShape, and both the games-list filter and the
// filter-menu counts evaluate it, so a count can never advertise games the
// filter cannot deliver.
type libFeatureShape struct {
	markerFeatures []string
	eventTypes     []string
	label          string
	teamStacking   bool
}

// libFeatureShapeFor resolves a UI featuring key, using the same aliasing and
// compositing rules as workflowFeaturingExistsSQL.
func libFeatureShapeFor(featureKey string) (libFeatureShape, bool) {
	normalized := strings.TrimSpace(strings.ToLower(featureKey))
	if fk, label, ok := splitPerValueFeatureKey(normalized); ok {
		return libFeatureShape{markerFeatures: []string{fk}, label: label}, true
	}
	switch normalized {
	case "team_stacking":
		return libFeatureShape{teamStacking: true}, true
	case "cannon_rush", "bunker_rush", "zergling_rush",
		"proxy_gate", "proxy_rax", "proxy_factory", "proxy_starport", "manner_pylon":
		return libFeatureShape{eventTypes: []string{normalized}}, true
	case "drop":
		return libFeatureShape{eventTypes: []string{"drop", "cliff_drop"}}, true
	case "mind_control":
		return libFeatureShape{markerFeatures: []string{"became_terran", "became_zerg"}}, true
	}
	lookup := normalized
	if alias, ok := uiFeatureKeyToMarkerFeatureKey[normalized]; ok {
		lookup = alias
	}
	if marker := markers.ByFeatureKey(lookup); marker != nil {
		return libFeatureShape{markerFeatures: []string{marker.FeatureKey}}, true
	}
	return libFeatureShape{}, false
}

// replayFeatures is one replay's featuring vocabulary: the marker feature keys
// and game-event types it carries, the resolved payload labels per marker, and
// the team-stacking flag that lives on the replay itself rather than an event.
type replayFeatures struct {
	markerFeatures map[string]struct{}
	eventTypes     map[string]struct{}
	labels         map[string]map[string]struct{}
	teamStacking   bool
}

func featuresOf(r *library.Replay) replayFeatures {
	f := replayFeatures{
		markerFeatures: make(map[string]struct{}, len(r.Markers)),
		eventTypes:     make(map[string]struct{}, len(r.Events)),
		teamStacking:   r.Flags.Has(library.FlagTeamStacking),
	}
	for i := range r.Markers {
		m := &r.Markers[i]
		key := library.Features.Name(m.Feature)
		if key == "" {
			continue
		}
		f.markerFeatures[key] = struct{}{}
		label, ok := markerLabel(m)
		if !ok || label == "" {
			continue
		}
		if f.labels == nil {
			f.labels = map[string]map[string]struct{}{}
		}
		if f.labels[key] == nil {
			f.labels[key] = map[string]struct{}{}
		}
		f.labels[key][strings.ToLower(label)] = struct{}{}
	}
	for i := range r.Events {
		eventType := library.EventTypes.Name(r.Events[i].Type)
		if eventType == "" {
			continue
		}
		f.eventTypes[eventType] = struct{}{}
	}
	return f
}

func (sh libFeatureShape) matches(f replayFeatures) bool {
	if sh.teamStacking {
		return f.teamStacking
	}
	if sh.label != "" {
		for _, key := range sh.markerFeatures {
			if _, ok := f.labels[key][sh.label]; ok {
				return true
			}
		}
		return false
	}
	for _, key := range sh.markerFeatures {
		if _, ok := f.markerFeatures[key]; ok {
			return true
		}
	}
	for _, eventType := range sh.eventTypes {
		if _, ok := f.eventTypes[eventType]; ok {
			return true
		}
	}
	return false
}

// replayHasFeature is the whole of workflowFeaturingExistsSQL as one Go
// predicate: does this replay carry the thing the UI key names.
func replayHasFeature(r *library.Replay, key string) bool {
	if r == nil {
		return false
	}
	shape, ok := libFeatureShapeFor(key)
	if !ok {
		return false
	}
	return shape.matches(featuresOf(r))
}

// featureIndex is featuresOf for the whole filtered corpus, built once per
// (snapshot, filter) so counting 120 filter chips stays one pass.
func (s *LibStore) featureIndex() map[int64]replayFeatures {
	v := s.view()
	return memo(v, "featuring:index", func() map[int64]replayFeatures {
		out := make(map[int64]replayFeatures, v.Len())
		for _, r := range v.Replays() {
			out[r.ID] = featuresOf(r)
		}
		return out
	})
}

// libGamesFilter is BuildWorkflowGamesListWhere as a predicate: values OR
// within an axis, axes AND together, and an axis whose values are all
// unrecognised imposes nothing.
type libGamesFilter struct {
	playerKeys map[string]struct{}
	mapNames   map[string]struct{}
	durations  []func(uint16) bool
	features   []libFeatureShape
	matchups   map[string]struct{}
	mapKind    libMapKindClause
}

type libMapKindClause uint8

const (
	libMapKindAny libMapKindClause = iota
	libMapKindMoney
	libMapKindRegular
)

func newLibGamesFilter(query GamesQuery) libGamesFilter {
	f := libGamesFilter{}
	if len(query.PlayerKeys) > 0 {
		f.playerKeys = make(map[string]struct{}, len(query.PlayerKeys))
		for _, key := range query.PlayerKeys {
			f.playerKeys[normalizeKey(key)] = struct{}{}
		}
	}
	if len(query.MapNames) > 0 {
		f.mapNames = make(map[string]struct{}, len(query.MapNames))
		for _, name := range query.MapNames {
			f.mapNames[normalizeKey(name)] = struct{}{}
		}
	}
	for _, key := range query.DurationBuckets {
		if bucket, ok := libDurationBuckets[key]; ok {
			f.durations = append(f.durations, bucket)
		}
	}
	for _, key := range query.Featuring {
		if shape, ok := libFeatureShapeFor(key); ok {
			f.features = append(f.features, shape)
		}
	}
	validMatchups := map[string]struct{}{
		"pvp": {}, "tvt": {}, "zvz": {},
		"pvt": {}, "pvz": {}, "tvz": {},
	}
	for _, key := range query.MatchupKeys {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if _, ok := validMatchups[normalized]; !ok {
			continue
		}
		if f.matchups == nil {
			f.matchups = map[string]struct{}{}
		}
		f.matchups[normalized] = struct{}{}
	}
	f.mapKind = libMapKindClauseFor(query.MapKindKeys)
	return f
}

// libMapKindClauseFor mirrors buildMapKindClause: "regular" covers both
// Regular and UseMapSettings, and selecting both keys (or neither) filters
// nothing.
func libMapKindClauseFor(mapKindKeys []string) libMapKindClause {
	wantMoney, wantRegular := false, false
	for _, key := range mapKindKeys {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "money":
			wantMoney = true
		case "regular":
			wantRegular = true
		}
	}
	switch {
	case wantMoney && !wantRegular:
		return libMapKindMoney
	case wantRegular && !wantMoney:
		return libMapKindRegular
	}
	return libMapKindAny
}

func (f libGamesFilter) matches(r *library.Replay) bool {
	if r == nil {
		return false
	}
	if f.playerKeys != nil && !f.hasAnyPlayer(r) {
		return false
	}
	if f.mapNames != nil {
		if _, ok := f.mapNames[normalizeKey(library.Strings.Name(r.Map))]; !ok {
			return false
		}
	}
	if len(f.durations) > 0 && !anyDurationBucket(f.durations, r.Duration) {
		return false
	}
	if len(f.features) > 0 {
		sets := featuresOf(r)
		matched := false
		for _, shape := range f.features {
			if shape.matches(sets) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if f.matchups != nil {
		if _, ok := f.matchups[strings.ToLower(library.Strings.Name(r.Matchup))]; !ok {
			return false
		}
	}
	switch f.mapKind {
	case libMapKindMoney:
		if r.MapKind != library.MapKindMoney {
			return false
		}
	case libMapKindRegular:
		if r.MapKind != library.MapKindRegular && r.MapKind != library.MapKindUseMapSettings {
			return false
		}
	}
	return true
}

func anyDurationBucket(buckets []func(uint16) bool, seconds uint16) bool {
	for _, bucket := range buckets {
		if bucket(seconds) {
			return true
		}
	}
	return false
}

// hasAnyPlayer is the EXISTS player clause: any non-observer slot whose key is
// selected, computer slots included.
func (f libGamesFilter) hasAnyPlayer(r *library.Replay) bool {
	for i := range r.Players {
		p := &r.Players[i]
		if p.IsObserver() {
			continue
		}
		if _, ok := f.playerKeys[p.Key]; ok {
			return true
		}
	}
	return false
}

func (s *LibStore) CountGames(ctx context.Context, query GamesQuery) (int64, error) {
	filter := newLibGamesFilter(query)
	total := int64(0)
	for _, r := range s.view().Replays() {
		if filter.matches(r) {
			total++
		}
	}
	return total, nil
}

func (s *LibStore) ListGames(ctx context.Context, query GamesQuery, limit, offset int) ([]WorkflowGameListRow, error) {
	filter := newLibGamesFilter(query)
	items := []WorkflowGameListRow{}
	if limit == 0 {
		return items, nil
	}
	skipped := 0
	for _, r := range s.view().Replays() {
		if !filter.matches(r) {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		items = append(items, gameListRow(r))
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items, nil
}

func gameListRow(r *library.Replay) WorkflowGameListRow {
	return WorkflowGameListRow{
		ReplayID:           r.ID,
		ReplayDate:         r.Date.String(),
		FileName:           r.FileName(),
		MapName:            library.Strings.Name(r.Map),
		MapKind:            r.MapKind.String(),
		GameSource:         library.Strings.Name(r.GameSource),
		LobbyKind:          library.Strings.Name(r.LobbyKind),
		DurationSeconds:    int64(r.Duration),
		GameType:           library.Strings.Name(r.GameType),
		Matchup:            library.Strings.Name(r.Matchup),
		TeamStacking:       r.Flags.Has(library.FlagTeamStacking),
		TeamInfoIncomplete: r.Flags.Has(library.FlagTeamInfoIncomplete),
	}
}

func (s *LibStore) ListReplayPlayers(ctx context.Context, replayIDs []int64) ([]WorkflowGamePlayerRow, error) {
	result := []WorkflowGamePlayerRow{}
	for _, r := range s.replaysByIDs(replayIDs) {
		for i := range r.Players {
			p := &r.Players[i]
			if p.IsObserver() {
				continue
			}
			result = append(result, WorkflowGamePlayerRow{
				ReplayID: r.ID,
				PlayerID: rowPlayerID(r, uint8(i)),
				Name:     p.Name,
				Race:     p.Race.String(),
				Team:     int64(p.Team),
				IsWinner: p.IsWinner(),
			})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].ReplayID != result[j].ReplayID {
			return result[i].ReplayID < result[j].ReplayID
		}
		if result[i].Team != result[j].Team {
			return result[i].Team < result[j].Team
		}
		return result[i].PlayerID < result[j].PlayerID
	})
	return result, nil
}

// libFeaturingMarkerKeys is the marker allowlist the games-list Featuring
// column reads: the fixed narrative markers plus every registered marker.
func libFeaturingMarkerKeys() map[string]struct{} {
	keys := map[string]struct{}{
		"carriers": {}, "battlecruisers": {}, "made_recalls": {},
		"threw_nukes": {}, "became_terran": {}, "became_zerg": {},
	}
	for _, m := range markers.Markers() {
		keys[m.FeatureKey] = struct{}{}
	}
	return keys
}

func (s *LibStore) ListFeaturingPlayerPatternRows(ctx context.Context, replayIDs []int64) ([]WorkflowPlayerPatternRow, error) {
	result := []WorkflowPlayerPatternRow{}
	if len(replayIDs) == 0 {
		return result, nil
	}
	allowed := libFeaturingMarkerKeys()
	for _, r := range s.replaysByIDs(replayIDs) {
		for i := range r.Markers {
			m := &r.Markers[i]
			key := library.Features.Name(m.Feature)
			if _, ok := allowed[key]; !ok {
				continue
			}
			row := WorkflowPlayerPatternRow{
				ReplayID:       r.ID,
				PatternName:    key,
				ValueBool:      ptrBool(true),
				DetectedSecond: int64(m.Sec),
			}
			if len(m.Payload) > 0 {
				row.ValueString = ptrString(string(m.Payload))
			}
			result = append(result, row)
		}
	}
	return result, nil
}

// libFeaturingEventTypes is the game-event allowlist of the Featuring column.
// manner_pylon is deliberately absent: the SQL this replaces never selected
// it, so including it here would light a chip the SQL store never lit.
var libFeaturingEventTypes = map[string]struct{}{
	"zergling_rush": {}, "cannon_rush": {}, "bunker_rush": {},
	"proxy_gate": {}, "proxy_rax": {}, "proxy_factory": {}, "proxy_starport": {},
	"drop": {}, "cliff_drop": {},
}

func (s *LibStore) ListFeaturingReplayEventRows(ctx context.Context, replayIDs []int64) ([]WorkflowReplayEventRow, error) {
	result := []WorkflowReplayEventRow{}
	if len(replayIDs) == 0 {
		return result, nil
	}
	for _, r := range s.replaysByIDs(replayIDs) {
		for i := range r.Events {
			eventType := library.EventTypes.Name(r.Events[i].Type)
			if _, ok := libFeaturingEventTypes[eventType]; !ok {
				continue
			}
			result = append(result, WorkflowReplayEventRow{ReplayID: r.ID, EventType: eventType})
		}
	}
	return result, nil
}

func (s *LibStore) ListCurrentPlayersForReplayIDs(ctx context.Context, playerKey string, replayIDs []int64) ([]WorkflowCurrentPlayerRow, error) {
	result := []WorkflowCurrentPlayerRow{}
	if len(replayIDs) == 0 {
		return result, nil
	}
	key := normalizeKey(playerKey)
	for _, r := range s.replaysByIDs(replayIDs) {
		for i := range r.Players {
			p := &r.Players[i]
			if p.IsObserver() || p.Key != key {
				continue
			}
			result = append(result, WorkflowCurrentPlayerRow{
				ReplayID: r.ID,
				PlayerID: rowPlayerID(r, uint8(i)),
				Name:     p.Name,
				Race:     p.Race.String(),
				IsWinner: p.IsWinner(),
				APM:      int64(p.APM),
				EAPM:     int64(p.EAPM),
			})
		}
	}
	return result, nil
}

func (s *LibStore) ListPatternValuesForPlayerIDs(ctx context.Context, playerIDs []int64) ([]WorkflowCurrentPlayerPatternRow, error) {
	result := []WorkflowCurrentPlayerPatternRow{}
	if len(playerIDs) == 0 {
		return result, nil
	}
	wanted := make(map[int64]struct{}, len(playerIDs))
	for _, id := range playerIDs {
		wanted[id] = struct{}{}
	}
	for _, r := range s.replaysForPlayerIDs(playerIDs) {
		for i := range r.Markers {
			m := &r.Markers[i]
			if m.Player == library.NoPlayer || int(m.Player) >= len(r.Players) {
				continue
			}
			id := rowPlayerID(r, m.Player)
			if _, ok := wanted[id]; !ok {
				continue
			}
			row := WorkflowCurrentPlayerPatternRow{
				PlayerID:       id,
				PatternName:    library.Features.Name(m.Feature),
				PatternValue:   "true",
				DetectedSecond: int64(m.Sec),
			}
			if len(m.Payload) > 0 {
				row.PatternValue = string(m.Payload)
				row.Payload = string(m.Payload)
			}
			result = append(result, row)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].PlayerID != result[j].PlayerID {
			return result[i].PlayerID < result[j].PlayerID
		}
		return result[i].PatternName < result[j].PatternName
	})
	return result, nil
}

func (s *LibStore) ListDroppedPlayerIDs(ctx context.Context, playerIDs []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if len(playerIDs) == 0 {
		return out, nil
	}
	wanted := make(map[int64]struct{}, len(playerIDs))
	for _, id := range playerIDs {
		wanted[id] = struct{}{}
	}
	for _, r := range s.replaysForPlayerIDs(playerIDs) {
		for i := range r.Events {
			ev := &r.Events[i]
			if library.EventTypes.Name(ev.Type) != "player_dropped" {
				continue
			}
			if ev.Source == library.NoPlayer || int(ev.Source) >= len(r.Players) {
				continue
			}
			id := rowPlayerID(r, ev.Source)
			if _, ok := wanted[id]; ok {
				out[id] = true
			}
		}
	}
	return out, nil
}

// replaysForPlayerIDs resolves the distinct replays behind a set of player
// ids, keeping first-seen order.
func (s *LibStore) replaysForPlayerIDs(playerIDs []int64) []*library.Replay {
	seen := make(map[int64]struct{}, len(playerIDs))
	ids := make([]int64, 0, len(playerIDs))
	for _, playerID := range playerIDs {
		replayID, _ := library.SplitPlayerID(playerID)
		if _, ok := seen[replayID]; ok {
			continue
		}
		seen[replayID] = struct{}{}
		ids = append(ids, replayID)
	}
	return s.replaysByIDs(ids)
}

func (s *LibStore) ListWorkflowFilterPlayers(ctx context.Context) ([]WorkflowFilterOptionRow, error) {
	aggregates := s.playerAggregates(libPlayerScopeNonObserver)
	result := make([]WorkflowFilterOptionRow, 0, len(aggregates))
	for i := range aggregates {
		if aggregates[i].Games < 5 {
			continue
		}
		result = append(result, WorkflowFilterOptionRow{
			Key:   aggregates[i].Key,
			Label: aggregates[i].Name,
			Games: aggregates[i].Games,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Games != result[j].Games {
			return result[i].Games > result[j].Games
		}
		return result[i].Label < result[j].Label
	})
	if len(result) > 200 {
		result = result[:200]
	}
	return result, nil
}

func (s *LibStore) ListWorkflowFilterMaps(ctx context.Context) ([]WorkflowFilterOptionRow, error) {
	type mapGroup struct {
		name  string
		games int64
	}
	groups := map[string]*mapGroup{}
	for _, r := range s.view().Replays() {
		name := library.Strings.Name(r.Map)
		key := normalizeKey(name)
		group := groups[key]
		if group == nil {
			group = &mapGroup{name: name}
			groups[key] = group
		}
		if name < group.name {
			group.name = name
		}
		group.games++
	}
	result := make([]WorkflowFilterOptionRow, 0, len(groups))
	for _, group := range groups {
		result = append(result, WorkflowFilterOptionRow{Label: group.name, Games: group.games})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Games != result[j].Games {
			return result[i].Games > result[j].Games
		}
		return result[i].Label < result[j].Label
	})
	if len(result) > 15 {
		result = result[:15]
	}
	return result, nil
}

func (s *LibStore) ListDistinctMarkerLabels(ctx context.Context, featureKey string) ([]string, error) {
	seen := map[string]struct{}{}
	labels := []string{}
	for _, r := range s.view().Replays() {
		for i := range r.Markers {
			m := &r.Markers[i]
			if library.Features.Name(m.Feature) != featureKey {
				continue
			}
			label, ok := markerLabel(m)
			if !ok || label == "" {
				continue
			}
			if _, dup := seen[label]; dup {
				continue
			}
			seen[label] = struct{}{}
			labels = append(labels, label)
		}
	}
	sort.SliceStable(labels, func(i, j int) bool {
		ni, nj := leadingSupplyNumber(labels[i]), leadingSupplyNumber(labels[j])
		if ni != nj {
			return ni < nj
		}
		return labels[i] < labels[j]
	})
	return labels, nil
}

func (s *LibStore) CountWorkflowDurationBuckets(ctx context.Context) (int64, int64, int64, int64, int64, error) {
	var under10m, m1020, m2030, m3045, m45Plus int64
	for _, r := range s.view().Replays() {
		switch seconds := r.Duration; {
		case seconds < 600:
			under10m++
		case seconds < 1200:
			m1020++
		case seconds < 1800:
			m2030++
		case seconds < 2700:
			m3045++
		default:
			m45Plus++
		}
	}
	return under10m, m1020, m2030, m3045, m45Plus, nil
}

func (s *LibStore) CountWorkflowFeaturingGames(ctx context.Context, featureKeys []string) (map[string]int64, error) {
	out := make(map[string]int64, len(featureKeys))
	if len(featureKeys) == 0 {
		return out, nil
	}
	shapes := make(map[string]libFeatureShape, len(featureKeys))
	for _, key := range featureKeys {
		if shape, ok := libFeatureShapeFor(key); ok {
			shapes[key] = shape
		}
	}
	if len(shapes) == 0 {
		return out, nil
	}
	index := s.featureIndex()
	for key, shape := range shapes {
		count := int64(0)
		for _, features := range index {
			if shape.matches(features) {
				count++
			}
		}
		out[key] = count
	}
	return out, nil
}

func (s *LibStore) CountWorkflowMatchupGames(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	for _, r := range s.view().Replays() {
		matchup := normalizeKey(library.Strings.Name(r.Matchup))
		if matchup == "" {
			continue
		}
		out[matchup]++
	}
	return out, nil
}

func (s *LibStore) CountWorkflowMapKindGames(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	for _, r := range s.view().Replays() {
		switch r.MapKind {
		case library.MapKindMoney:
			out["money"]++
		case library.MapKindRegular, library.MapKindUseMapSettings:
			out["regular"]++
		}
	}
	return out, nil
}

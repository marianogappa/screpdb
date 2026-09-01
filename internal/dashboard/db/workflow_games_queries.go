package db

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

type WorkflowGameListRow struct {
	ReplayID           int64
	ReplayDate         string
	FileName           string
	MapName            string
	MapKind            string
	GameSource         string
	LobbyKind          string
	DurationSeconds    int64
	GameType           string
	Matchup            string
	TeamStacking       bool
	TeamInfoIncomplete bool
}

type WorkflowGamePlayerRow struct {
	ReplayID int64
	PlayerID int64
	Name     string
	Race     string
	Team     int64
	IsWinner bool
}

type WorkflowPlayerPatternRow struct {
	ReplayID       int64
	PatternName    string
	ValueBool      sql.NullBool
	ValueInt       sql.NullInt64
	ValueString    sql.NullString
	ValueTimestamp sql.NullInt64
	DetectedSecond int64
}

type WorkflowReplayEventRow struct {
	ReplayID  int64
	EventType string
}

type WorkflowCurrentPlayerRow struct {
	ReplayID int64
	PlayerID int64
	Name     string
	Race     string
	IsWinner bool
}

type WorkflowCurrentPlayerPatternRow struct {
	PlayerID       int64
	PatternName    string
	PatternValue   string
	DetectedSecond int64
	Payload        string
}

type WorkflowFilterOptionRow struct {
	Key   string
	Label string
	Games int64
}

func (s *Store) CountGamesWithWhere(ctx context.Context, whereSQL string, whereArgs []any) (int64, error) {
	countQuery := "SELECT COUNT(*) FROM replays r " + whereSQL
	var total int64
	if err := s.ReplayQueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) ListGamesWithWhere(ctx context.Context, whereSQL string, whereArgs []any, limit, offset int) ([]WorkflowGameListRow, error) {
	listArgs := append([]any{}, whereArgs...)
	listArgs = append(listArgs, limit, offset)
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT
			r.id,
			r.replay_date,
			r.file_name,
			r.map_name,
			r.map_kind,
			r.game_source,
			r.lobby_kind,
			r.duration_seconds,
			r.game_type,
			r.matchup,
			COALESCE(r.team_stacking, 0),
			COALESCE(r.team_info_incomplete, 0)
		FROM replays r
	`+whereSQL+`
		ORDER BY r.replay_date DESC, r.id DESC
		LIMIT ? OFFSET ?
	`, listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []WorkflowGameListRow{}
	for rows.Next() {
		var item WorkflowGameListRow
		if err := rows.Scan(
			&item.ReplayID,
			&item.ReplayDate,
			&item.FileName,
			&item.MapName,
			&item.MapKind,
			&item.GameSource,
			&item.LobbyKind,
			&item.DurationSeconds,
			&item.GameType,
			&item.Matchup,
			&item.TeamStacking,
			&item.TeamInfoIncomplete,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountWorkflowPlayers(ctx context.Context, baseSQL, whereSQL string, allArgs []any) (int64, error) {
	countQuery := `WITH player_agg AS (` + baseSQL + `) SELECT COUNT(*) FROM player_agg ` + whereSQL
	var total int64
	if err := s.ReplayQueryRowContext(ctx, countQuery, allArgs...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

type WorkflowPlayersListRow struct {
	PlayerKey         string
	PlayerName        string
	Race              string
	GamesPlayed       int64
	AverageAPM        float64
	LastPlayed        string
	LastPlayedDaysAgo int64
}

func (s *Store) ListWorkflowPlayers(ctx context.Context, baseSQL, whereSQL, sortColumn, sortDir string, allArgs []any, limit, offset int) ([]WorkflowPlayersListRow, error) {
	listArgs := append(append([]any{}, allArgs...), limit, offset)
	rows, err := s.ReplayQueryContext(ctx, `
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
		return nil, err
	}
	defer rows.Close()
	items := []WorkflowPlayersListRow{}
	for rows.Next() {
		item := WorkflowPlayersListRow{}
		if err := rows.Scan(
			&item.PlayerKey,
			&item.PlayerName,
			&item.Race,
			&item.GamesPlayed,
			&item.AverageAPM,
			&item.LastPlayed,
			&item.LastPlayedDaysAgo,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountWorkflowLastPlayedBuckets(ctx context.Context, baseSQL, whereSQL string, countRowArgs []any) (int64, int64, error) {
	var count1m, count3m int64
	if err := s.ReplayQueryRowContext(ctx, `
		WITH player_agg AS (`+baseSQL+`)
		SELECT
			COALESCE(SUM(CASE WHEN last_played_days_ago <= 30 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN last_played_days_ago <= 90 THEN 1 ELSE 0 END), 0)
		FROM player_agg
	`+whereSQL+`
	`, countRowArgs...).Scan(&count1m, &count3m); err != nil {
		return 0, 0, err
	}
	return count1m, count3m, nil
}

func (s *Store) ListReplayPlayers(ctx context.Context, replayIDs []int64) ([]WorkflowGamePlayerRow, error) {
	if len(replayIDs) == 0 {
		return []WorkflowGamePlayerRow{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(replayIDs)), ",")
	args := make([]any, 0, len(replayIDs))
	for _, replayID := range replayIDs {
		args = append(args, replayID)
	}
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT replay_id, id, name, COALESCE(race, ''), team, is_winner
		FROM players
		WHERE is_observer = 0
			AND replay_id IN (`+placeholders+`)
		ORDER BY replay_id ASC, team ASC, id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkflowGamePlayerRow{}
	for rows.Next() {
		var row WorkflowGamePlayerRow
		if err := rows.Scan(&row.ReplayID, &row.PlayerID, &row.Name, &row.Race, &row.Team, &row.IsWinner); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) ListFeaturingPlayerPatternRows(ctx context.Context, replayIDs []int64) ([]WorkflowPlayerPatternRow, error) {
	if len(replayIDs) == 0 {
		return []WorkflowPlayerPatternRow{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(replayIDs)), ",")
	args := make([]any, 0, len(replayIDs))
	for _, replayID := range replayIDs {
		args = append(args, replayID)
	}
	// Feature-keys of interest: fixed set of "featuring"-capable markers + every
	// registered build order. Assembled dynamically so adding a BO needs no SQL
	// edit. Post-markers-migration each row's pattern_name is the marker FeatureKey.
	featureKeys := []string{"carriers", "battlecruisers", "made_recalls", "threw_nukes", "became_terran", "became_zerg"}
	for _, m := range markers.Markers() {
		featureKeys = append(featureKeys, m.FeatureKey)
	}
	quoted := make([]string, 0, len(featureKeys))
	for _, key := range featureKeys {
		quoted = append(quoted, "'"+strings.ReplaceAll(key, "'", "''")+"'")
	}
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT replay_id, event_type, 1, NULL, payload, NULL, seconds_from_game_start
		FROM replay_events
		WHERE replay_id IN (`+placeholders+`)
			AND event_kind = 'marker'
			AND event_type IN (`+strings.Join(quoted, ", ")+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkflowPlayerPatternRow{}
	for rows.Next() {
		var row WorkflowPlayerPatternRow
		if err := rows.Scan(&row.ReplayID, &row.PatternName, &row.ValueBool, &row.ValueInt, &row.ValueString, &row.ValueTimestamp, &row.DetectedSecond); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ListDistinctMarkerLabels returns the distinct resolved payload labels
// ({"label":...}) persisted by a dynamic-label marker (e.g. bo_z_fuzzy's
// "~9 Overpool" / "~10 Hatch"), so the games-list filter bar can offer one
// filterable pill per value instead of a single placeholder bucket. Sorted by
// the numeric supply rung then the label so "~9" precedes "~10".
func (s *Store) ListDistinctMarkerLabels(ctx context.Context, featureKey string) ([]string, error) {
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT DISTINCT json_extract(payload, '$.label') AS label
		FROM replay_events
		WHERE event_kind = 'marker'
			AND event_type = ?
			AND json_extract(payload, '$.label') IS NOT NULL
			AND json_extract(payload, '$.label') != ''
	`, featureKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	labels := []string{}
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	if err := rows.Err(); err != nil {
		return nil, err
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

// leadingSupplyNumber extracts the integer from a "~N ..." fuzzy-opener label so
// filter pills sort by supply rung rather than lexically. Returns a large
// sentinel when no number is present, sinking such labels to the end.
func leadingSupplyNumber(label string) int {
	digits := strings.TrimLeft(label, "~")
	n := 0
	found := false
	for _, r := range digits {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		found = true
	}
	if !found {
		return 1 << 30
	}
	return n
}

func (s *Store) ListFeaturingReplayEventRows(ctx context.Context, replayIDs []int64) ([]WorkflowReplayEventRow, error) {
	if len(replayIDs) == 0 {
		return []WorkflowReplayEventRow{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(replayIDs)), ",")
	args := make([]any, 0, len(replayIDs))
	for _, replayID := range replayIDs {
		args = append(args, replayID)
	}
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT replay_id, event_type
		FROM replay_events
		WHERE replay_id IN (`+placeholders+`)
			AND event_kind = 'game_event'
			AND event_type IN (
				'zergling_rush', 'cannon_rush', 'bunker_rush',
				'proxy_gate', 'proxy_rax', 'proxy_factory', 'proxy_starport',
				'drop', 'cliff_drop'
			)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkflowReplayEventRow{}
	for rows.Next() {
		var row WorkflowReplayEventRow
		if err := rows.Scan(&row.ReplayID, &row.EventType); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) ListCurrentPlayersForReplayIDs(ctx context.Context, playerKey string, replayIDs []int64) ([]WorkflowCurrentPlayerRow, error) {
	if len(replayIDs) == 0 {
		return []WorkflowCurrentPlayerRow{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(replayIDs)), ",")
	args := make([]any, 0, len(replayIDs)+1)
	args = append(args, playerKey)
	for _, replayID := range replayIDs {
		args = append(args, replayID)
	}
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT replay_id, id, name, race, is_winner
		FROM players
		WHERE lower(trim(name)) = ?
			AND is_observer = 0
			AND replay_id IN (`+placeholders+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkflowCurrentPlayerRow{}
	for rows.Next() {
		var row WorkflowCurrentPlayerRow
		if err := rows.Scan(&row.ReplayID, &row.PlayerID, &row.Name, &row.Race, &row.IsWinner); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) ListPatternValuesForPlayerIDs(ctx context.Context, playerIDs []int64) ([]WorkflowCurrentPlayerPatternRow, error) {
	if len(playerIDs) == 0 {
		return []WorkflowCurrentPlayerPatternRow{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(playerIDs)), ",")
	args := make([]any, 0, len(playerIDs))
	for _, playerID := range playerIDs {
		args = append(args, playerID)
	}
	// Per-player marker presence. Post-migration, presence of the row is the
	// match — detected_second + payload are the authoritative per-row data;
	// pattern_value stays as a transitional alias for now.
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT
			source_player_id AS player_id,
			event_type AS pattern_name,
			COALESCE(payload, 'true') AS pattern_value,
			seconds_from_game_start AS detected_second,
			COALESCE(payload, '') AS payload
		FROM replay_events
		WHERE source_player_id IN (`+placeholders+`)
			AND event_kind = 'marker'
		ORDER BY source_player_id ASC, event_type ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkflowCurrentPlayerPatternRow{}
	for rows.Next() {
		var row WorkflowCurrentPlayerPatternRow
		if err := rows.Scan(&row.PlayerID, &row.PatternName, &row.PatternValue, &row.DetectedSecond, &row.Payload); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) ListWorkflowFilterPlayers(ctx context.Context) ([]WorkflowFilterOptionRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListWorkflowFilterPlayers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]WorkflowFilterOptionRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, WorkflowFilterOptionRow{
			Key:   row.PlayerKey,
			Label: row.PlayerName,
			Games: row.Games,
		})
	}
	return result, nil
}

func (s *Store) ListWorkflowFilterMaps(ctx context.Context) ([]WorkflowFilterOptionRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListWorkflowFilterMaps(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]WorkflowFilterOptionRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, WorkflowFilterOptionRow{
			Label: row.MapName,
			Games: row.Games,
		})
	}
	return result, nil
}

func (s *Store) CountWorkflowDurationBuckets(ctx context.Context) (int64, int64, int64, int64, int64, error) {
	row, err := sqlcgen.New(Trace(s.replayScoped())).CountWorkflowDurationBuckets(ctx)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	return row.Under10m, row.M1020, row.M2030, row.M3045, row.M45Plus, nil
}

// CountWorkflowFeaturingGames returns, per UI featuring key, how many replays
// in the corpus carry that marker or game event.
//
// The filter chips have always had a Games field; for featuring it was left at
// zero, which is 93 of the 122 filter options. Counts are what make a filter
// menu worth browsing (they say what is worth clicking before you click it),
// so the omnibar needs them.
//
// Cost is a fixed three aggregate queries regardless of how many keys are
// asked for, rather than one EXISTS per key. Counts are corpus-wide and
// deliberately ignore the active filters: they describe the vocabulary, not
// the current result set, so they stay stable while the user narrows.
func (s *Store) CountWorkflowFeaturingGames(ctx context.Context, featureKeys []string) (map[string]int64, error) {
	out := make(map[string]int64, len(featureKeys))
	if len(featureKeys) == 0 {
		return out, nil
	}

	shapes := make(map[string]workflowFeaturingCountShape, len(featureKeys))
	needPerValue := false
	needTeamStacking := false
	for _, key := range featureKeys {
		shape, ok := workflowFeaturingCountShapeFor(key)
		if !ok {
			continue
		}
		shapes[key] = shape
		if shape.perValueLabel != "" {
			needPerValue = true
		}
		if shape.teamStacking {
			needTeamStacking = true
		}
	}
	if len(shapes) == 0 {
		return out, nil
	}

	// One replay can carry the same event type many times, so every count is a
	// COUNT(DISTINCT replay_id).
	perType := map[string]map[string]int64{}
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT re.event_kind, re.event_type, COUNT(DISTINCT re.replay_id)
		FROM replay_events re
		WHERE re.event_kind IN ('marker', 'game_event')
		GROUP BY re.event_kind, re.event_type`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kind, eventType string
		var count int64
		if err := rows.Scan(&kind, &eventType, &count); err != nil {
			rows.Close()
			return nil, err
		}
		if perType[kind] == nil {
			perType[kind] = map[string]int64{}
		}
		perType[kind][eventType] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// The per-value openers ("bo_z_fuzzy::~10 hatch") need a label breakdown.
	perLabel := map[string]map[string]int64{}
	if needPerValue {
		labelRows, err := s.ReplayQueryContext(ctx, `
			SELECT re.event_type, lower(json_extract(re.payload, '$.label')), COUNT(DISTINCT re.replay_id)
			FROM replay_events re
			WHERE re.event_kind = 'marker'
				AND json_extract(re.payload, '$.label') IS NOT NULL
			GROUP BY re.event_type, lower(json_extract(re.payload, '$.label'))`)
		if err != nil {
			return nil, err
		}
		for labelRows.Next() {
			var eventType, label string
			var count int64
			if err := labelRows.Scan(&eventType, &label, &count); err != nil {
				labelRows.Close()
				return nil, err
			}
			if perLabel[eventType] == nil {
				perLabel[eventType] = map[string]int64{}
			}
			perLabel[eventType][label] = count
		}
		if err := labelRows.Err(); err != nil {
			labelRows.Close()
			return nil, err
		}
		labelRows.Close()
	}

	var teamStackingCount int64
	if needTeamStacking {
		if err := s.ReplayQueryRowContext(ctx,
			`SELECT COUNT(*) FROM replays r WHERE COALESCE(r.team_stacking, 0) = 1`,
		).Scan(&teamStackingCount); err != nil && err != sql.ErrNoRows {
			return nil, err
		}
	}

	// A composite key ("drop" = drop OR cliff_drop) cannot sum its parts: a
	// replay carrying both would be counted twice. Resolve those with one extra
	// DISTINCT query each rather than over-reporting.
	for key, shape := range shapes {
		switch {
		case shape.teamStacking:
			out[key] = teamStackingCount
		case shape.perValueLabel != "":
			if byLabel, ok := perLabel[shape.eventTypes[0]]; ok {
				out[key] = byLabel[shape.perValueLabel]
			}
		case len(shape.eventTypes) == 1:
			out[key] = perType[shape.eventKind][shape.eventTypes[0]]
		default:
			count, err := s.countReplaysWithAnyEvent(ctx, shape.eventKind, shape.eventTypes)
			if err != nil {
				return nil, err
			}
			out[key] = count
		}
	}
	return out, nil
}

func (s *Store) countReplaysWithAnyEvent(ctx context.Context, eventKind string, eventTypes []string) (int64, error) {
	placeholders := strings.TrimRight(strings.Repeat("?, ", len(eventTypes)), ", ")
	args := make([]any, 0, len(eventTypes)+1)
	args = append(args, eventKind)
	for _, eventType := range eventTypes {
		args = append(args, eventType)
	}
	var count int64
	err := s.ReplayQueryRowContext(ctx, `
		SELECT COUNT(DISTINCT re.replay_id)
		FROM replay_events re
		WHERE re.event_kind = ?
			AND re.event_type IN (`+placeholders+`)`, args...).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	return count, nil
}

// CountWorkflowMatchupGames returns replay counts keyed by lowercase matchup
// ("pvt", "tvz"). replays.matchup is already canonicalised (TvZ == ZvT), which
// is what buildMatchupClause filters on, so one GROUP BY covers every key.
func (s *Store) CountWorkflowMatchupGames(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT lower(trim(COALESCE(r.matchup, ''))), COUNT(*)
		FROM replays r
		WHERE COALESCE(r.matchup, '') <> ''
		GROUP BY lower(trim(COALESCE(r.matchup, '')))`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var matchup string
		var count int64
		if err := rows.Scan(&matchup, &count); err != nil {
			return nil, err
		}
		out[matchup] = count
	}
	return out, rows.Err()
}

// CountWorkflowMapKindGames returns replay counts for the two map-kind filter
// keys. It mirrors buildMapKindClause: "regular" deliberately covers both
// Regular and UseMapSettings.
func (s *Store) CountWorkflowMapKindGames(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT COALESCE(r.map_kind, ''), COUNT(*)
		FROM replays r
		GROUP BY COALESCE(r.map_kind, '')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mapKind string
		var count int64
		if err := rows.Scan(&mapKind, &count); err != nil {
			return nil, err
		}
		switch mapKind {
		case "Money":
			out["money"] += count
		case "Regular", "UseMapSettings":
			out["regular"] += count
		}
	}
	return out, rows.Err()
}

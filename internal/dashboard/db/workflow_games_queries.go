package db

import (
	"context"
	"database/sql"
	"strings"

	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type WorkflowGameListRow struct {
	ReplayID        int64
	ReplayDate      string
	FileName        string
	MapName         string
	DurationSeconds int64
	GameType        string
}

type WorkflowGamePlayerRow struct {
	ReplayID int64
	PlayerID int64
	Name     string
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
	PlayerID     int64
	PatternName  string
	PatternValue string
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
			r.duration_seconds,
			r.game_type
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
			&item.DurationSeconds,
			&item.GameType,
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
		SELECT replay_id, id, name, team, is_winner
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
		if err := rows.Scan(&row.ReplayID, &row.PlayerID, &row.Name, &row.Team, &row.IsWinner); err != nil {
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
		SELECT replay_id, event_type, 1, NULL, payload, NULL
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
		if err := rows.Scan(&row.ReplayID, &row.PatternName, &row.ValueBool, &row.ValueInt, &row.ValueString, &row.ValueTimestamp); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
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
			AND event_type IN ('zergling_rush', 'cannon_rush', 'bunker_rush')
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
	// match — there are no value columns, so we synthesize "true" / payload string
	// for downstream code that still expects a value field.
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT
			source_player_id AS player_id,
			event_type AS pattern_name,
			COALESCE(payload, 'true') AS pattern_value
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
		if err := rows.Scan(&row.PlayerID, &row.PatternName, &row.PatternValue); err != nil {
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
	rows, err := sqlcgen.New(s.replayScoped()).ListWorkflowFilterPlayers(ctx)
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
	rows, err := sqlcgen.New(s.replayScoped()).ListWorkflowFilterMaps(ctx)
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
	row, err := sqlcgen.New(s.replayScoped()).CountWorkflowDurationBuckets(ctx)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	return row.Under10m, row.M1020, row.M2030, row.M3045, row.M45Plus, nil
}

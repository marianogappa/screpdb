package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type CommonBehaviourRow struct {
	PatternName string
	ReplayCount int64
}

type NullableStringInt64Row struct {
	Name  *string
	Count int64
}

type RaceCountRow struct {
	Race  string
	Count int64
}

type RaceFloatRow struct {
	Race  string
	Value float64
}

type OutlierCountRow struct {
	Race  string
	Name  string
	Count int64
}

type OutlierGlobalRow struct {
	Race    string
	Name    string
	Games   int64
	Players float64
}

func (s *Store) ListCommonBehaviours(ctx context.Context, playerKey string) ([]CommonBehaviourRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListCommonBehaviours(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]CommonBehaviourRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, CommonBehaviourRow{
			PatternName: row.PatternName,
			ReplayCount: row.ReplayCount,
		})
	}
	return out, nil
}

func (s *Store) GetOutlierPlayerSummary(ctx context.Context, playerKey string) (*NullableStringInt64Row, error) {
	row, err := sqlcgen.New(Trace(s.replayScoped())).GetOutlierPlayerSummary(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	return &NullableStringInt64Row{Name: &row.Name, Count: row.Count}, nil
}

func (s *Store) ListPlayerGamesByRace(ctx context.Context, playerKey string) ([]RaceCountRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerGamesByRace(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]RaceCountRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, RaceCountRow{
			Race:  row.Race,
			Count: row.Games,
		})
	}
	return out, nil
}

func (s *Store) ListPopulationGamesByRace(ctx context.Context) ([]RaceCountRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPopulationGamesByRace(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RaceCountRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, RaceCountRow{
			Race:  row.Race,
			Count: row.Games,
		})
	}
	return out, nil
}

type RaceMapKindCountRow struct {
	Race    string
	MapKind string
	Count   int64
}

type RaceMapKindFloatRow struct {
	Race    string
	MapKind string
	Value   float64
}

func (s *Store) ListPlayerGamesByRaceMapKind(ctx context.Context, playerKey string) ([]RaceMapKindCountRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerGamesByRaceMapKind(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]RaceMapKindCountRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, RaceMapKindCountRow{
			Race:    row.Race,
			MapKind: row.MapKind,
			Count:   row.Games,
		})
	}
	return out, nil
}

func (s *Store) ListPopulationGamesByRaceMapKind(ctx context.Context) ([]RaceMapKindCountRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPopulationGamesByRaceMapKind(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RaceMapKindCountRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, RaceMapKindCountRow{
			Race:    row.Race,
			MapKind: row.MapKind,
			Count:   row.Games,
		})
	}
	return out, nil
}

func (s *Store) ListPopulationDistinctPlayersByRaceMapKind(ctx context.Context) ([]RaceMapKindFloatRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPopulationDistinctPlayersByRaceMapKind(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RaceMapKindFloatRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, RaceMapKindFloatRow{
			Race:    row.Race,
			MapKind: row.MapKind,
			Value:   row.Value,
		})
	}
	return out, nil
}

func (s *Store) ListPopulationDistinctPlayersByRace(ctx context.Context) ([]RaceFloatRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPopulationDistinctPlayersByRace(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RaceFloatRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, RaceFloatRow{
			Race:  row.Race,
			Value: row.Value,
		})
	}
	return out, nil
}

func (s *Store) ListOutlierPlayerCounts(ctx context.Context, playerKey, primaryRace, nameColumn string, useInstanceShare bool, actionTypes []string) ([]OutlierCountRow, error) {
	actionTypePlaceholders := strings.TrimRight(strings.Repeat("?,", len(actionTypes)), ",")
	playerQuery := fmt.Sprintf(`
		SELECT ? AS race, c.%s AS item_name,
			CASE
				WHEN ? THEN COUNT(c.id)
				ELSE COUNT(DISTINCT p.id)
			END AS player_games
		FROM players p
		JOIN commands c ON c.player_id = p.id
		WHERE lower(trim(p.name)) = ?
			AND p.is_observer = 0
			AND lower(trim(coalesce(p.type, ''))) = 'human'
			AND p.race = ?
			AND c.action_type IN (`+actionTypePlaceholders+`)
			AND c.%s IS NOT NULL
			AND c.%s <> ''
		GROUP BY c.%s
	`, nameColumn, nameColumn, nameColumn, nameColumn)
	args := make([]any, 0, len(actionTypes)+4)
	args = append(args, primaryRace, useInstanceShare, playerKey, primaryRace)
	for _, actionType := range actionTypes {
		args = append(args, actionType)
	}
	rows, err := s.ReplayQueryContext(ctx, playerQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OutlierCountRow{}
	for rows.Next() {
		var row OutlierCountRow
		if err := rows.Scan(&row.Race, &row.Name, &row.Count); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SegmentedOutlierCountRow is one row of the segmented variants. games_all
// totals across all map kinds (matching the legacy single-segment query);
// games_regular and games_money carry the per-segment slice. Same idea on
// the corpus side with players_* counters.
type SegmentedOutlierCountRow struct {
	Race         string
	Name         string
	GamesAll     int64
	GamesRegular int64
	GamesMoney   int64
}

type SegmentedOutlierGlobalRow struct {
	Race           string
	Name           string
	GamesAll       int64
	GamesRegular   int64
	GamesMoney     int64
	PlayersAll     float64
	PlayersRegular float64
	PlayersMoney   float64
}

// ListOutlierPlayerCountsSegmented runs a single query per spec that
// returns the all-maps total plus per-(Regular,Money) segment counts in
// one pass. Replaces three separate calls and so cuts the DB round-trips
// 3x without changing the underlying scan cost — the bottleneck for the
// segmented Summary-tab pill row.
func (s *Store) ListOutlierPlayerCountsSegmented(ctx context.Context, playerKey, primaryRace, nameColumn string, useInstanceShare bool, actionTypes []string) ([]SegmentedOutlierCountRow, error) {
	actionTypePlaceholders := strings.TrimRight(strings.Repeat("?,", len(actionTypes)), ",")
	// SQLite doesn't allow CASE WHEN ? THEN <agg1> ELSE <agg2> END portably,
	// so we branch the SQL string on useInstanceShare. games_* expressions
	// either count rows or distinct player ids; player counts are always
	// distinct.
	var allExpr, regularExpr, moneyExpr string
	if useInstanceShare {
		allExpr = "COUNT(c.id)"
		regularExpr = "SUM(CASE WHEN r.map_kind = 'Regular' THEN 1 ELSE 0 END)"
		moneyExpr = "SUM(CASE WHEN r.map_kind = 'Money' THEN 1 ELSE 0 END)"
	} else {
		allExpr = "COUNT(DISTINCT p.id)"
		regularExpr = "COUNT(DISTINCT CASE WHEN r.map_kind = 'Regular' THEN p.id END)"
		moneyExpr = "COUNT(DISTINCT CASE WHEN r.map_kind = 'Money' THEN p.id END)"
	}
	playerQuery := fmt.Sprintf(`
		SELECT ? AS race, c.%s AS item_name,
			%s AS games_all,
			%s AS games_regular,
			%s AS games_money
		FROM players p
		JOIN commands c ON c.player_id = p.id
		JOIN replays r ON r.id = p.replay_id
		WHERE lower(trim(p.name)) = ?
			AND p.is_observer = 0
			AND lower(trim(coalesce(p.type, ''))) = 'human'
			AND p.race = ?
			AND c.action_type IN (`+actionTypePlaceholders+`)
			AND c.%s IS NOT NULL
			AND c.%s <> ''
		GROUP BY c.%s
	`, nameColumn, allExpr, regularExpr, moneyExpr, nameColumn, nameColumn, nameColumn)
	args := make([]any, 0, len(actionTypes)+3)
	args = append(args, primaryRace, playerKey, primaryRace)
	for _, actionType := range actionTypes {
		args = append(args, actionType)
	}
	rows, err := s.ReplayQueryContext(ctx, playerQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SegmentedOutlierCountRow{}
	for rows.Next() {
		var row SegmentedOutlierCountRow
		if err := rows.Scan(&row.Race, &row.Name, &row.GamesAll, &row.GamesRegular, &row.GamesMoney); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListOutlierGlobalRowsSegmented(ctx context.Context, primaryRace, nameColumn string, useInstanceShare bool, actionTypes []string) ([]SegmentedOutlierGlobalRow, error) {
	actionTypePlaceholders := strings.TrimRight(strings.Repeat("?,", len(actionTypes)), ",")
	var allExpr, regularExpr, moneyExpr string
	if useInstanceShare {
		allExpr = "COUNT(c.id)"
		regularExpr = "SUM(CASE WHEN r.map_kind = 'Regular' THEN 1 ELSE 0 END)"
		moneyExpr = "SUM(CASE WHEN r.map_kind = 'Money' THEN 1 ELSE 0 END)"
	} else {
		allExpr = "COUNT(DISTINCT p.id)"
		regularExpr = "COUNT(DISTINCT CASE WHEN r.map_kind = 'Regular' THEN p.id END)"
		moneyExpr = "COUNT(DISTINCT CASE WHEN r.map_kind = 'Money' THEN p.id END)"
	}
	globalQuery := fmt.Sprintf(`
		SELECT
			? AS race,
			c.%s AS item_name,
			%s AS games_all,
			%s AS games_regular,
			%s AS games_money,
			COUNT(DISTINCT lower(trim(p.name))) AS players_all,
			COUNT(DISTINCT CASE WHEN r.map_kind = 'Regular' THEN lower(trim(p.name)) END) AS players_regular,
			COUNT(DISTINCT CASE WHEN r.map_kind = 'Money' THEN lower(trim(p.name)) END) AS players_money
		FROM players p
		JOIN commands c ON c.player_id = p.id
		JOIN replays r ON r.id = p.replay_id
		WHERE p.is_observer = 0
			AND lower(trim(coalesce(p.type, ''))) = 'human'
			AND p.race = ?
			AND c.action_type IN (`+actionTypePlaceholders+`)
			AND c.%s IS NOT NULL
			AND c.%s <> ''
		GROUP BY c.%s
	`, nameColumn, allExpr, regularExpr, moneyExpr, nameColumn, nameColumn, nameColumn)
	args := make([]any, 0, len(actionTypes)+2)
	args = append(args, primaryRace, primaryRace)
	for _, actionType := range actionTypes {
		args = append(args, actionType)
	}
	rows, err := s.ReplayQueryContext(ctx, globalQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SegmentedOutlierGlobalRow{}
	for rows.Next() {
		var row SegmentedOutlierGlobalRow
		if err := rows.Scan(&row.Race, &row.Name, &row.GamesAll, &row.GamesRegular, &row.GamesMoney, &row.PlayersAll, &row.PlayersRegular, &row.PlayersMoney); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListOutlierGlobalRows(ctx context.Context, primaryRace, nameColumn string, useInstanceShare bool, actionTypes []string) ([]OutlierGlobalRow, error) {
	actionTypePlaceholders := strings.TrimRight(strings.Repeat("?,", len(actionTypes)), ",")
	globalQuery := fmt.Sprintf(`
		SELECT
			? AS race,
			c.%s AS item_name,
			CASE
				WHEN ? THEN COUNT(c.id)
				ELSE COUNT(DISTINCT p.id)
			END AS global_games,
			COUNT(DISTINCT lower(trim(p.name))) AS global_players
		FROM players p
		JOIN commands c ON c.player_id = p.id
		WHERE p.is_observer = 0
			AND lower(trim(coalesce(p.type, ''))) = 'human'
			AND p.race = ?
			AND c.action_type IN (`+actionTypePlaceholders+`)
			AND c.%s IS NOT NULL
			AND c.%s <> ''
		GROUP BY c.%s
	`, nameColumn, nameColumn, nameColumn, nameColumn)
	args := make([]any, 0, len(actionTypes)+3)
	args = append(args, primaryRace, useInstanceShare, primaryRace)
	for _, actionType := range actionTypes {
		args = append(args, actionType)
	}
	rows, err := s.ReplayQueryContext(ctx, globalQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OutlierGlobalRow{}
	for rows.Next() {
		var row OutlierGlobalRow
		if err := rows.Scan(&row.Race, &row.Name, &row.Games, &row.Players); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

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
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListCommonBehaviours(ctx, playerKey)
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
	row, err := sqlcgen.New(s.replayScoped()).GetOutlierPlayerSummary(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	return &NullableStringInt64Row{Name: &row.Name, Count: row.Count}, nil
}

func (s *Store) ListPlayerGamesByRace(ctx context.Context, playerKey string) ([]RaceCountRow, error) {
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListPlayerGamesByRace(ctx, playerKey)
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
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListPopulationGamesByRace(ctx)
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

func (s *Store) ListPopulationDistinctPlayersByRace(ctx context.Context) ([]RaceFloatRow, error) {
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListPopulationDistinctPlayersByRace(ctx)
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

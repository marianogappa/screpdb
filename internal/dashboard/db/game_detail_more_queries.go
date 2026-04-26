package db

import (
	"context"
	"database/sql"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type DelayCommandRow struct {
	ReplayID    int64
	PlayerID    int64
	PlayerName  string
	PlayerRace  string
	Second      int64
	ActionType  string
	UnitType    sql.NullString
	UnitTypes   sql.NullString
}

func (s *Store) ListDelayCommandRows(ctx context.Context, cutoffSeconds int64, onlyPlayerKey string) ([]DelayCommandRow, error) {
	q := sqlcgen.New(Trace(s.replayScoped()))
	out := []DelayCommandRow{}
	if onlyPlayerKey != "" {
		rows, err := q.ListDelayCommandRowsForPlayer(ctx, sqlcgen.ListDelayCommandRowsForPlayerParams{
			SecondsFromGameStart: cutoffSeconds,
			Name:                 onlyPlayerKey,
		})
		if err != nil {
			return nil, err
		}
		out = make([]DelayCommandRow, 0, len(rows))
		for _, row := range rows {
			out = append(out, DelayCommandRow{
				ReplayID:   row.ReplayID,
				PlayerID:   row.ID,
				PlayerName: row.Name,
				PlayerRace: row.Race,
				Second:     row.SecondsFromGameStart,
				ActionType: row.ActionType,
				UnitType:   nullableStringPtrToNullString(row.UnitType),
				UnitTypes:  nullableStringPtrToNullString(row.UnitTypes),
			})
		}
		return out, nil
	}
	rows, err := q.ListDelayCommandRows(ctx, cutoffSeconds)
	if err != nil {
		return nil, err
	}
	out = make([]DelayCommandRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, DelayCommandRow{
			ReplayID:   row.ReplayID,
			PlayerID:   row.ID,
			PlayerName: row.Name,
			PlayerRace: row.Race,
			Second:     row.SecondsFromGameStart,
			ActionType: row.ActionType,
			UnitType:   nullableStringPtrToNullString(row.UnitType),
			UnitTypes:  nullableStringPtrToNullString(row.UnitTypes),
		})
	}
	return out, nil
}

func (s *Store) CountPlayerGames(ctx context.Context, playerKey string) (int64, error) {
	return sqlcgen.New(Trace(s.replayScoped())).CountPlayerGames(ctx, playerKey)
}

type RaceSectionRow struct {
	Race      string
	GameCount int64
	Wins      int64
}

func (s *Store) ListRaceSections(ctx context.Context, playerKey string) ([]RaceSectionRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListRaceSections(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]RaceSectionRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, RaceSectionRow{
			Race:      row.Race,
			GameCount: row.GameCount,
			Wins:      row.Wins,
		})
	}
	return out, nil
}

type RacePatternRow struct {
	Race        string
	PatternName string
	ReplayCount int64
}

func (s *Store) ListRacePatterns(ctx context.Context, playerKey string) ([]RacePatternRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListRacePatterns(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]RacePatternRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, RacePatternRow{
			Race:        row.Race,
			PatternName: row.PatternName,
			ReplayCount: row.ReplayCount,
		})
	}
	return out, nil
}

func (s *Store) ListTopActionTypes(ctx context.Context, playerID int64, limit int) ([]string, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListTopActionTypes(ctx, sqlcgen.ListTopActionTypesParams{
		PlayerID: playerID,
		Limit:    int64(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, strings.TrimSpace(row.ActionType))
	}
	return out, nil
}

func nullableStringPtrToNullString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

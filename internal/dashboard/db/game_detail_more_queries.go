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

type PlayerFirstExpansionTimingRow struct {
	Race                 string
	MapKind              string
	ReplayID             int64
	FirstExpansionSecond int64
}

func (s *Store) ListPlayerFirstExpansionTimings(ctx context.Context, playerKey string) ([]PlayerFirstExpansionTimingRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerFirstExpansionTimings(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerFirstExpansionTimingRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlayerFirstExpansionTimingRow{
			Race:                 strings.TrimSpace(row.Race),
			MapKind:              strings.TrimSpace(row.MapKind),
			ReplayID:             row.ReplayID,
			FirstExpansionSecond: row.FirstExpansionSecond,
		})
	}
	return out, nil
}

// PhaseBoundaries carries the early/mid game-end seconds persisted as
// replay-level markers at ingest. Either or both may be 0 when the
// replay never reached the corresponding boundary (the game ended
// inside Early, or inside Mid). Same "0 = not detected" convention used
// elsewhere in the codebase.
type PhaseBoundaries struct {
	EarlyEndsAtSecond int64 // = mid_game_starts marker's second
	MidEndsAtSecond   int64 // = late_game_starts marker's second
}

// GetPhaseBoundariesForReplay returns the early/mid boundary seconds
// for one replay. Source: persisted replay-level markers
// (mid_game_starts, late_game_starts) emitted at ingest by
// internal/patterns/detectors/phase_boundary_detector.go.
func (s *Store) GetPhaseBoundariesForReplay(ctx context.Context, replayID int64) (PhaseBoundaries, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).GetPhaseBoundariesForReplay(ctx, replayID)
	if err != nil {
		return PhaseBoundaries{}, err
	}
	out := PhaseBoundaries{}
	for _, row := range rows {
		switch row.EventType {
		case "mid_game_starts":
			out.EarlyEndsAtSecond = row.SecondsFromGameStart
		case "late_game_starts":
			out.MidEndsAtSecond = row.SecondsFromGameStart
		}
	}
	return out, nil
}

// UnitProductionOrCastRow is one Train / Unit Morph / spell-cast
// command for the per-game composition computation. Caster rows have
// ActionType empty and OrderName populated; production rows have
// ActionType in (Train, Unit Morph) and a non-nil UnitType.
type UnitProductionOrCastRow struct {
	PlayerID             int64
	ActionType           string
	UnitType             *string
	UnitTypes            *string
	OrderName            *string
	SecondsFromGameStart int64
}

// ListGameUnitProductionAndCasts returns the rows the per-game endpoint
// uses to compute attacker composition pills at request time. The
// composition itself is NOT persisted — see plan: derived from these
// rows + the persisted phase boundaries on every render so it stays in
// sync with edits to caster sets / excluded units / presentation rules.
func (s *Store) ListGameUnitProductionAndCasts(ctx context.Context, replayID int64) ([]UnitProductionOrCastRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListGameUnitProductionAndCasts(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]UnitProductionOrCastRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, UnitProductionOrCastRow{
			PlayerID:             row.PlayerID,
			ActionType:           row.ActionType,
			UnitType:             row.UnitType,
			UnitTypes:            row.UnitTypes,
			OrderName:            row.OrderName,
			SecondsFromGameStart: row.SecondsFromGameStart,
		})
	}
	return out, nil
}

type PlayerMatchupRow struct {
	OwnRace string
	OppRace string
	Games   int64
	Wins    int64
}

func (s *Store) ListPlayerMatchups(ctx context.Context, playerKey string) ([]PlayerMatchupRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerMatchups(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerMatchupRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlayerMatchupRow{
			OwnRace: strings.TrimSpace(row.OwnRace),
			OppRace: strings.TrimSpace(row.OppRace),
			Games:   row.Games,
			Wins:    row.Wins,
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

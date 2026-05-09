package db

import (
	"context"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type PlayerMatchupAggregateRow struct {
	OwnRace string
	OppRace string
	Games   int64
	Wins    int64
	AvgAPM  float64
	AvgEAPM float64
}

type PlayerMatchupMarkerCountRow struct {
	OwnRace     string
	OppRace     string
	PatternName string
	ReplayCount int64
}

type PlayerByFormatAggregateRow struct {
	OwnRace    string
	TeamFormat string
	MapKind    string
	Games      int64
	Wins       int64
	AvgAPM     float64
	AvgEAPM    float64
}

type PlayerByFormatMarkerCountRow struct {
	OwnRace     string
	TeamFormat  string
	MapKind     string
	PatternName string
	ReplayCount int64
}

func (s *Store) ListPlayerMatchupAggregates(ctx context.Context, playerKey string) ([]PlayerMatchupAggregateRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerMatchupAggregates(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerMatchupAggregateRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlayerMatchupAggregateRow{
			OwnRace: row.OwnRace,
			OppRace: row.OppRace,
			Games:   row.Games,
			Wins:    row.Wins,
			AvgAPM:  row.AvgApm,
			AvgEAPM: row.AvgEapm,
		})
	}
	return out, nil
}

func (s *Store) ListPlayerMatchupMarkerCounts(ctx context.Context, playerKey string) ([]PlayerMatchupMarkerCountRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerMatchupMarkerCounts(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerMatchupMarkerCountRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlayerMatchupMarkerCountRow{
			OwnRace:     row.OwnRace,
			OppRace:     row.OppRace,
			PatternName: row.PatternName,
			ReplayCount: row.ReplayCount,
		})
	}
	return out, nil
}

func (s *Store) ListPlayerByFormatAggregates(ctx context.Context, playerKey string) ([]PlayerByFormatAggregateRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerByFormatAggregates(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerByFormatAggregateRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlayerByFormatAggregateRow{
			OwnRace:    row.OwnRace,
			TeamFormat: row.TeamFormat,
			MapKind:    row.MapKind,
			Games:      row.Games,
			Wins:       row.Wins,
			AvgAPM:     row.AvgApm,
			AvgEAPM:    row.AvgEapm,
		})
	}
	return out, nil
}

func (s *Store) ListPlayerByFormatMarkerCounts(ctx context.Context, playerKey string) ([]PlayerByFormatMarkerCountRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerByFormatMarkerCounts(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerByFormatMarkerCountRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlayerByFormatMarkerCountRow{
			OwnRace:     row.OwnRace,
			TeamFormat:  row.TeamFormat,
			MapKind:     row.MapKind,
			PatternName: row.PatternName,
			ReplayCount: row.ReplayCount,
		})
	}
	return out, nil
}

func (s *Store) CountPlayerMultiTeamMeleeGames(ctx context.Context, playerKey string) (int64, error) {
	return sqlcgen.New(Trace(s.replayScoped())).CountPlayerMultiTeamMeleeGames(ctx, playerKey)
}

func (s *Store) CountPlayerAllianceCommandsInMultiTeamMelee(ctx context.Context, playerKey string) (int64, error) {
	return sqlcgen.New(Trace(s.replayScoped())).CountPlayerAllianceCommandsInMultiTeamMelee(ctx, playerKey)
}

package db

import (
	"context"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type ReplaySummaryRow struct {
	ReplayID        int64
	ReplayDate      string
	FileName        string
	MapName         string
	DurationSeconds int64
	GameType        string
}

type ReplayPlayerDetailRow struct {
	PlayerID             int64
	Name                 string
	Race                 string
	Team                 int64
	IsWinner             bool
	APM                  int64
	EAPM                 int64
	CommandCount         int64
	HotkeyCommandCount   int64
	LowValueCommandCount int64
}

type PatternValueRow struct {
	PatternName string
	Value       string
}

type TeamPatternValueRow struct {
	Team        int64
	PatternName string
	Value       string
}

type PlayerPatternValueRow struct {
	PlayerID    int64
	PatternName string
	Value       string
}

type PlayerOverviewSummaryRow struct {
	PlayerName  string
	GamesPlayed int64
	Wins        int64
	AverageAPM  float64
	AverageEAPM float64
}

type PlayerRecentGameRow struct {
	ReplayID        int64
	ReplayDate      string
	FileName        string
	MapName         string
	DurationSeconds int64
	GameType        string
	PlayersLabel    string
	WinnersLabel    string
}

type PlayerApmAggregateRow struct {
	PlayerKey   string
	PlayerName  string
	AverageAPM  float64
	GamesPlayed int64
}

func (s *Store) GetReplaySummary(ctx context.Context, replayID int64) (*ReplaySummaryRow, error) {
	sqlcRow, err := sqlcgen.New(s.replayScoped()).GetReplaySummary(ctx, replayID)
	if err != nil {
		return nil, err
	}
	return &ReplaySummaryRow{
		ReplayID:        sqlcRow.ID,
		ReplayDate:      sqlcRow.ReplayDate,
		FileName:        sqlcRow.FileName,
		MapName:         sqlcRow.MapName,
		DurationSeconds: sqlcRow.DurationSeconds,
		GameType:        sqlcRow.GameType,
	}, nil
}

func (s *Store) ListReplayPlayersForDetail(ctx context.Context, replayID int64) ([]ReplayPlayerDetailRow, error) {
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListReplayPlayersForDetail(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]ReplayPlayerDetailRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, ReplayPlayerDetailRow{
			PlayerID:             row.ID,
			Name:                 row.Name,
			Race:                 row.Race,
			Team:                 row.Team,
			IsWinner:             row.IsWinner,
			APM:                  row.Apm,
			EAPM:                 row.Eapm,
			CommandCount:         row.CommandCount,
			HotkeyCommandCount:   row.HotkeyCount,
			LowValueCommandCount: row.LowValueCommandCount,
		})
	}
	return out, nil
}

func (s *Store) ListReplayPatterns(ctx context.Context, replayID int64) ([]PatternValueRow, error) {
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListReplayPatterns(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]PatternValueRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, PatternValueRow{
			PatternName: row.PatternName,
			Value:       row.PatternValue,
		})
	}
	return out, nil
}

func (s *Store) ListTeamPatterns(ctx context.Context, replayID int64) ([]TeamPatternValueRow, error) {
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListTeamPatterns(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]TeamPatternValueRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, TeamPatternValueRow{
			Team:        row.Team,
			PatternName: row.PatternName,
			Value:       row.PatternValue,
		})
	}
	return out, nil
}

func (s *Store) ListPlayerPatterns(ctx context.Context, replayID int64) ([]PlayerPatternValueRow, error) {
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListPlayerPatterns(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerPatternValueRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, PlayerPatternValueRow{
			PlayerID:    row.PlayerID,
			PatternName: row.PatternName,
			Value:       row.PatternValue,
		})
	}
	return out, nil
}

func (s *Store) GetPlayerOverviewSummary(ctx context.Context, playerKey string) (*PlayerOverviewSummaryRow, error) {
	row, err := sqlcgen.New(s.replayScoped()).GetPlayerOverviewSummary(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	return &PlayerOverviewSummaryRow{
		PlayerName:  row.PlayerName,
		GamesPlayed: row.GamesPlayed,
		Wins:        row.Wins,
		AverageAPM:  row.AvgApm,
		AverageEAPM: row.AvgEapm,
	}, nil
}

func (s *Store) ListPlayerRecentGames(ctx context.Context, playerKey string) ([]PlayerRecentGameRow, error) {
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListPlayerRecentGames(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerRecentGameRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, PlayerRecentGameRow{
			ReplayID:        row.ID,
			ReplayDate:      row.ReplayDate,
			FileName:        row.FileName,
			MapName:         row.MapName,
			DurationSeconds: row.DurationSeconds,
			GameType:        row.GameType,
			PlayersLabel:    row.PlayersLabel,
			WinnersLabel:    row.WinnersLabel,
		})
	}
	return out, nil
}

func (s *Store) ListPlayerApmAggregates(ctx context.Context, minGames int64) ([]PlayerApmAggregateRow, error) {
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListPlayerApmAggregates(ctx, minGames)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerApmAggregateRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, PlayerApmAggregateRow{
			PlayerKey:  row.PlayerKey,
			PlayerName: row.PlayerName,
			AverageAPM: row.AverageApm,
			GamesPlayed: row.GamesPlayed,
		})
	}
	return out, nil
}

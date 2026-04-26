package db

import (
	"context"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type ReplaySummaryRow struct {
	ReplayID        int64
	ReplayDate      string
	FileName        string
	FilePath        string
	FileChecksum    string
	MapName         string
	DurationSeconds int64
	GameType        string
}

type ReplayPlayerDetailRow struct {
	PlayerID            int64
	Name                string
	Color               string
	Race                string
	Team                int64
	IsWinner            bool
	StartLocationOclock *int64
	APM                 int64
	EAPM                int64
}

type PatternValueRow struct {
	PatternName    string
	Value          string
	DetectedSecond int64
	Payload        string
}

type PlayerPatternValueRow struct {
	PlayerID       int64
	PatternName    string
	Value          string
	DetectedSecond int64
	Payload        string
}

type ReplayEventRow struct {
	EventType              string
	Second                 int64
	SourcePlayerID         *int64
	SourcePlayerName       string
	SourcePlayerColor      string
	TargetPlayerID         *int64
	TargetPlayerName       string
	TargetPlayerColor      string
	LocationBaseType       *string
	LocationBaseOclock     *int64
	LocationNaturalOfClock *int64
	LocationMineralOnly    *bool
	AttackUnitTypes        *string
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
	sqlcRow, err := sqlcgen.New(Trace(s.replayScoped())).GetReplaySummary(ctx, replayID)
	if err != nil {
		return nil, err
	}
	return &ReplaySummaryRow{
		ReplayID:        sqlcRow.ID,
		ReplayDate:      sqlcRow.ReplayDate,
		FileName:        sqlcRow.FileName,
		FilePath:        sqlcRow.FilePath,
		FileChecksum:    sqlcRow.FileChecksum,
		MapName:         sqlcRow.MapName,
		DurationSeconds: sqlcRow.DurationSeconds,
		GameType:        sqlcRow.GameType,
	}, nil
}

func (s *Store) ListReplayPlayersForDetail(ctx context.Context, replayID int64) ([]ReplayPlayerDetailRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListReplayPlayersForDetail(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]ReplayPlayerDetailRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, ReplayPlayerDetailRow{
			PlayerID:            row.ID,
			Name:                row.Name,
			Color:               row.Color,
			Race:                row.Race,
			Team:                row.Team,
			IsWinner:            row.IsWinner,
			StartLocationOclock: row.StartLocationOclock,
			APM:                 row.Apm,
			EAPM:                row.Eapm,
		})
	}
	return out, nil
}

func (s *Store) ListReplayPatterns(ctx context.Context, replayID int64) ([]PatternValueRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListReplayPatterns(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]PatternValueRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, PatternValueRow{
			PatternName:    row.PatternName,
			Value:          row.PatternValue,
			DetectedSecond: row.DetectedSecond,
			Payload:        row.Payload,
		})
	}
	return out, nil
}

func (s *Store) ListPlayerPatterns(ctx context.Context, replayID int64) ([]PlayerPatternValueRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerPatterns(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerPatternValueRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, PlayerPatternValueRow{
			PlayerID:       row.PlayerID,
			PatternName:    row.PatternName,
			Value:          row.PatternValue,
			DetectedSecond: row.DetectedSecond,
			Payload:        row.Payload,
		})
	}
	return out, nil
}

func (s *Store) ListReplayEvents(ctx context.Context, replayID int64) ([]ReplayEventRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListReplayEvents(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]ReplayEventRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, ReplayEventRow{
			EventType:              row.EventType,
			Second:                 row.SecondsFromGameStart,
			SourcePlayerID:         row.SourcePlayerID,
			SourcePlayerName:       row.SourcePlayerName,
			SourcePlayerColor:      row.SourcePlayerColor,
			TargetPlayerID:         row.TargetPlayerID,
			TargetPlayerName:       row.TargetPlayerName,
			TargetPlayerColor:      row.TargetPlayerColor,
			LocationBaseType:       row.LocationBaseType,
			LocationBaseOclock:     row.LocationBaseOclock,
			LocationNaturalOfClock: row.LocationNaturalOfOclock,
			LocationMineralOnly:    row.LocationMineralOnly,
			AttackUnitTypes:        row.AttackUnitTypes,
		})
	}
	return out, nil
}

func (s *Store) GetPlayerOverviewSummary(ctx context.Context, playerKey string) (*PlayerOverviewSummaryRow, error) {
	row, err := sqlcgen.New(Trace(s.replayScoped())).GetPlayerOverviewSummary(ctx, playerKey)
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
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerRecentGames(ctx, playerKey)
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
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerApmAggregates(ctx, minGames)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerApmAggregateRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, PlayerApmAggregateRow{
			PlayerKey:   row.PlayerKey,
			PlayerName:  row.PlayerName,
			AverageAPM:  row.AverageApm,
			GamesPlayed: row.GamesPlayed,
		})
	}
	return out, nil
}

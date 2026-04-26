package db

import (
	"context"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

func (s *Store) CountDistinctPlayers(ctx context.Context) (float64, error) {
	return sqlcgen.New(Trace(s.replayScoped())).CountDistinctPlayers(ctx)
}

func (s *Store) CountDistinctPlayersByRace(ctx context.Context, race string) (float64, error) {
	return sqlcgen.New(Trace(s.replayScoped())).CountDistinctPlayersByRace(ctx, race)
}

type ReplayExpansionEventRow struct {
	ReplayID int64
	PlayerID *int64
	Second   int64
}

func (s *Store) ListExpansionEvents(ctx context.Context) ([]ReplayExpansionEventRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListExpansionEvents(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ReplayExpansionEventRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, ReplayExpansionEventRow{
			ReplayID: row.ReplayID,
			PlayerID: row.SourcePlayerID,
			Second:   row.SecondsFromGameStart,
		})
	}
	return out, nil
}

type ReplayPlayerNameRow struct {
	ReplayID int64
	PlayerID int64
	Name     string
}

func (s *Store) ListPlayersByReplayRows(ctx context.Context) ([]ReplayPlayerNameRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayersByReplayRows(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ReplayPlayerNameRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, ReplayPlayerNameRow{
			ReplayID: row.ReplayID,
			PlayerID: row.PlayerID,
			Name:     row.Name,
		})
	}
	return out, nil
}

func (s *Store) GetPlayerNameByKey(ctx context.Context, playerKey string) (string, error) {
	playerName, err := sqlcgen.New(Trace(s.replayScoped())).GetPlayerNameByKey(ctx, playerKey)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(playerName), nil
}

type RaceOrderRow struct {
	PlayerID    int64
	Race        string
	ActionType  string
	TechName    *string
	UpgradeName *string
	Second      int64
}

func (s *Store) ListRaceOrderRows(ctx context.Context, playerKey string) ([]RaceOrderRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListRaceOrderRows(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]RaceOrderRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, RaceOrderRow{
			PlayerID:    row.ID,
			Race:        row.Race,
			ActionType:  row.ActionType,
			TechName:    row.TechName,
			UpgradeName: row.UpgradeName,
			Second:      row.SecondsFromGameStart,
		})
	}
	return out, nil
}

func (s *Store) CountQueuedGamesByPlayer(ctx context.Context, playerKey string) (int64, error) {
	return sqlcgen.New(Trace(s.replayScoped())).CountQueuedGamesByPlayer(ctx, playerKey)
}

func (s *Store) CountCarrierGamesByPlayer(ctx context.Context, playerKey string) (int64, error) {
	return sqlcgen.New(Trace(s.replayScoped())).CountCarrierGamesByPlayer(ctx, playerKey)
}

type PlayerChatRow struct {
	ReplayID int64
	Message  string
}

func (s *Store) ListPlayerChatRows(ctx context.Context, playerKey string) ([]PlayerChatRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerChatRows(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerChatRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, PlayerChatRow{
			ReplayID: row.ReplayID,
			Message:  row.ChatMessage,
		})
	}
	return out, nil
}

type TimingRow struct {
	PlayerID int64
	Second   int64
	Label    string
}

func (s *Store) ListGasTimingRows(ctx context.Context, replayID int64) ([]TimingRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListGasTimingRows(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]TimingRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, TimingRow{
			PlayerID: row.PlayerID,
			Second:   row.SecondsFromGameStart,
			Label:    row.UnitType,
		})
	}
	return out, nil
}

func (s *Store) ListUpgradeTimingRows(ctx context.Context, replayID int64) ([]TimingRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListUpgradeTimingRows(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]TimingRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, TimingRow{
			PlayerID: row.PlayerID,
			Second:   row.SecondsFromGameStart,
			Label:    row.UpgradeName,
		})
	}
	return out, nil
}

func (s *Store) ListTechTimingRows(ctx context.Context, replayID int64) ([]TimingRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListTechTimingRows(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]TimingRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, TimingRow{
			PlayerID: row.PlayerID,
			Second:   row.SecondsFromGameStart,
			Label:    row.TechName,
		})
	}
	return out, nil
}

func (s *Store) ListHotkeyGamesRateByPlayer(ctx context.Context) (map[string]float64, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListHotkeyGamesRateByPlayer(ctx)
	if err != nil {
		return nil, err
	}
	valuesByPlayer := map[string]float64{}
	for _, row := range sqlcRows {
		if row.MetricValue == nil {
			valuesByPlayer[row.PlayerKey] = 0
			continue
		}
		valuesByPlayer[row.PlayerKey] = *row.MetricValue
	}
	return valuesByPlayer, nil
}

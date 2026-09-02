package db

import (
	"context"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

// ReplayPlayerHotkeyStreamRow is one player's encoded hotkey stream in a game.
type ReplayPlayerHotkeyStreamRow struct {
	PlayerID     int64
	Name         string
	Race         string
	Team         int64
	HotkeyStream []byte
}

func (s *Store) ListReplayPlayerHotkeyStreams(ctx context.Context, replayID int64) ([]ReplayPlayerHotkeyStreamRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListReplayPlayerHotkeyStreams(ctx, replayID)
	if err != nil {
		return nil, err
	}
	rows := make([]ReplayPlayerHotkeyStreamRow, 0, len(sqlcRows))
	for _, r := range sqlcRows {
		rows = append(rows, ReplayPlayerHotkeyStreamRow{
			PlayerID:     r.ID,
			Name:         r.Name,
			Race:         r.Race,
			Team:         r.Team,
			HotkeyStream: r.HotkeyStream,
		})
	}
	return rows, nil
}

// PlayerHotkeyStreamRow is one game's stream for a player, for signature
// aggregation.
type PlayerHotkeyStreamRow struct {
	Name            string
	Race            string
	HotkeyStream    []byte
	DurationSeconds int64
}

func (s *Store) ListPlayerHotkeyStreamsByKey(ctx context.Context, playerKey string) ([]PlayerHotkeyStreamRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerHotkeyStreamsByKey(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	rows := make([]PlayerHotkeyStreamRow, 0, len(sqlcRows))
	for _, r := range sqlcRows {
		rows = append(rows, PlayerHotkeyStreamRow{
			Name:            r.Name,
			Race:            r.Race,
			HotkeyStream:    r.HotkeyStream,
			DurationSeconds: r.DurationSeconds,
		})
	}
	return rows, nil
}

// ReplayPlayerHotkeyStream is one player's stream fetched for the map
// composite.
type ReplayPlayerHotkeyStream struct {
	PlayerID     int64
	Name         string
	Race         string
	SlotID       int64
	HotkeyStream []byte
}

func (s *Store) GetReplayPlayerHotkeyStream(ctx context.Context, replayID, playerID int64) (*ReplayPlayerHotkeyStream, error) {
	r, err := sqlcgen.New(Trace(s.replayScoped())).GetReplayPlayerHotkeyStream(ctx, sqlcgen.GetReplayPlayerHotkeyStreamParams{
		ReplayID: replayID,
		ID:       playerID,
	})
	if err != nil {
		return nil, err
	}
	return &ReplayPlayerHotkeyStream{
		PlayerID:     r.ID,
		Name:         r.Name,
		Race:         r.Race,
		SlotID:       r.SlotID,
		HotkeyStream: r.HotkeyStream,
	}, nil
}

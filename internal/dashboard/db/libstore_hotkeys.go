package db

import (
	"context"
	"sort"
)

func (s *LibStore) ListReplayPlayerHotkeyStreams(_ context.Context, replayID int64) ([]ReplayPlayerHotkeyStreamRow, error) {
	rows := []ReplayPlayerHotkeyStreamRow{}
	r, err := s.replay(replayID)
	if err != nil {
		return rows, nil
	}
	for i := range r.Players {
		p := &r.Players[i]
		if !humanNonObserver(p) {
			continue
		}
		rows = append(rows, ReplayPlayerHotkeyStreamRow{
			PlayerID:     rowPlayerID(r, uint8(i)),
			Name:         p.Name,
			Race:         p.Race.String(),
			Team:         int64(p.Team),
			HotkeyStream: p.HotkeyStream,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Team < rows[j].Team })
	return rows, nil
}

const (
	hotkeySignatureMinDuration = 360
	hotkeySignatureMaxGames    = 120
)

func (s *LibStore) ListPlayerHotkeyStreamsByKey(_ context.Context, playerKey string) ([]PlayerHotkeyStreamRow, error) {
	rows := make([]PlayerHotkeyStreamRow, 0, hotkeySignatureMaxGames)
	for _, ref := range s.playerGames(playerKey) {
		p := ref.Player()
		if p.IsObserver() || len(p.HotkeyStream) == 0 || int(ref.Replay.Duration) < hotkeySignatureMinDuration {
			continue
		}
		rows = append(rows, PlayerHotkeyStreamRow{
			Name:            p.Name,
			Race:            p.Race.String(),
			HotkeyStream:    p.HotkeyStream,
			DurationSeconds: int64(ref.Replay.Duration),
		})
		if len(rows) == hotkeySignatureMaxGames {
			break
		}
	}
	return rows, nil
}

func (s *LibStore) GetReplayPlayerHotkeyStream(_ context.Context, replayID, playerID int64) (*ReplayPlayerHotkeyStream, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	ordinal, ok := ordinalForPlayerID(r, playerID)
	if !ok {
		return nil, ErrNotFound
	}
	p := &r.Players[ordinal]
	return &ReplayPlayerHotkeyStream{
		PlayerID:     playerID,
		Name:         p.Name,
		Race:         p.Race.String(),
		SlotID:       int64(p.Slot),
		HotkeyStream: p.HotkeyStream,
	}, nil
}

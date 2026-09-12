package db

import (
	"context"
	"time"
)

// ListCoPlayerCounts tallies everyone who shared a game with any of playerKeys
// since the given time: how many such games, and when the last one was.
//
// Unlike the session candidate scan this deliberately counts every game, not
// only autosaved ones. A session is evidence the user sat down to play, which
// only their own Autosave folder can give; who they play with is a fact about
// the games themselves, and a downloaded copy of a game they were in is still
// a game they were in.
func (s *LibStore) ListCoPlayerCounts(_ context.Context, playerKeys []string, since time.Time) ([]CoPlayerRow, error) {
	out := []CoPlayerRow{}
	if len(playerKeys) == 0 {
		return out, nil
	}
	mine := make(map[string]struct{}, len(playerKeys))
	for _, key := range playerKeys {
		mine[normalizeKey(key)] = struct{}{}
	}
	byKey := map[string]*CoPlayerRow{}
	for _, r := range s.view().Replays() {
		if r.Date.Before(since) {
			continue
		}
		ours := false
		for i := range r.Players {
			p := &r.Players[i]
			if p.IsObserver() {
				continue
			}
			if _, ok := mine[p.Key]; ok {
				ours = true
				break
			}
		}
		if !ours {
			continue
		}
		for i := range r.Players {
			p := &r.Players[i]
			if p.IsObserver() || p.IsComputer() || p.Key == "" {
				continue
			}
			if _, skip := mine[p.Key]; skip {
				continue
			}
			row, ok := byKey[p.Key]
			if !ok {
				row = &CoPlayerRow{PlayerKey: p.Key, PlayerName: p.Name}
				byKey[p.Key] = row
			}
			row.Games++
			if r.Date.After(row.LastPlayed) {
				row.LastPlayed = r.Date
				// Names drift; the one they used most recently is the one the
				// user will recognise.
				row.PlayerName = p.Name
			}
		}
	}
	for _, row := range byKey {
		out = append(out, *row)
	}
	return out, nil
}

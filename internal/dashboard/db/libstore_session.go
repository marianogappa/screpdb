package db

import (
	"context"

	"github.com/marianogappa/screpdb/internal/library"
)

func (s *LibStore) ListRecentAutosaveGamesForPlayers(_ context.Context, playerKeys []string, limit int) ([]SessionCandidateRow, error) {
	out := []SessionCandidateRow{}
	if len(playerKeys) == 0 || limit <= 0 {
		return out, nil
	}
	wanted := make(map[string]struct{}, len(playerKeys))
	for _, key := range playerKeys {
		wanted[normalizeKey(key)] = struct{}{}
	}
	for _, r := range s.view().Replays() {
		path := sessionCandidatePath(r)
		for i := range r.Players {
			p := &r.Players[i]
			if p.IsObserver() {
				continue
			}
			if _, ok := wanted[p.Key]; !ok {
				continue
			}
			out = append(out, SessionCandidateRow{
				ReplayID:   r.ID,
				ReplayDate: r.Date.String(),
				FilePath:   path,
				PlayerKey:  p.Key,
				PlayerName: p.Name,
			})
			if len(out) == limit {
				return out, nil
			}
		}
	}
	return out, nil
}

// sessionCandidatePath reports an Autosave path when the record has one, so
// the caller's own path test agrees with the autosave flag compaction stamped
// from whichever file the replay was first loaded from.
func sessionCandidatePath(r *library.Replay) string {
	if !r.Flags.Has(library.FlagIsAutosave) {
		return r.Path()
	}
	for _, file := range r.Paths {
		if library.IsAutosavePath(file.Path) {
			return file.Path
		}
	}
	return r.Path()
}

func (s *LibStore) ListReplaysByIDs(_ context.Context, replayIDs []int64) ([]SessionReplayRow, error) {
	out := []SessionReplayRow{}
	if len(replayIDs) == 0 {
		return out, nil
	}
	wanted := make(map[int64]struct{}, len(replayIDs))
	for _, id := range replayIDs {
		wanted[id] = struct{}{}
	}
	for _, r := range s.view().Replays() {
		if _, ok := wanted[r.ID]; !ok {
			continue
		}
		out = append(out, SessionReplayRow{
			ReplayID:           r.ID,
			ReplayDate:         r.Date.String(),
			FileName:           r.FileName(),
			MapName:            library.Strings.Name(r.Map),
			MapKind:            r.MapKind.String(),
			GameSource:         library.Strings.Name(r.GameSource),
			LobbyKind:          library.Strings.Name(r.LobbyKind),
			DurationSeconds:    int64(r.Duration),
			GameType:           library.Strings.Name(r.GameType),
			Matchup:            library.Strings.Name(r.Matchup),
			TeamStacking:       r.Flags.Has(library.FlagTeamStacking),
			TeamInfoIncomplete: r.Flags.Has(library.FlagTeamInfoIncomplete),
		})
	}
	return out, nil
}

func (s *LibStore) ListPlayerAPMByReplayIDs(_ context.Context, replayIDs []int64) ([]SessionPlayerAPMRow, error) {
	out := []SessionPlayerAPMRow{}
	if len(replayIDs) == 0 {
		return out, nil
	}
	seen := make(map[int64]struct{}, len(replayIDs))
	for _, r := range s.replaysByIDs(replayIDs) {
		if _, done := seen[r.ID]; done {
			continue
		}
		seen[r.ID] = struct{}{}
		for i := range r.Players {
			p := &r.Players[i]
			if p.IsObserver() {
				continue
			}
			out = append(out, SessionPlayerAPMRow{
				ReplayID:  r.ID,
				PlayerKey: p.Key,
				APM:       int64(p.APM),
				EAPM:      int64(p.EAPM),
			})
		}
	}
	return out, nil
}

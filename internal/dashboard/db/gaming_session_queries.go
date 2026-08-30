package db

import (
	"context"
	"strings"
)

// SessionCandidateRow is one of the user's games, with just enough to decide
// whether it belongs to the current session.
type SessionCandidateRow struct {
	ReplayID   int64
	ReplayDate string
	FilePath   string
	PlayerKey  string
	PlayerName string
}

// ListRecentAutosaveGamesForPlayers returns the most recent games played by any
// of playerKeys, newest first. The Autosave path filter is applied in Go rather
// than SQL so the separator handling lives in one place, but the LIMIT keeps
// the scan bounded regardless.
func (s *Store) ListRecentAutosaveGamesForPlayers(ctx context.Context, playerKeys []string, limit int) ([]SessionCandidateRow, error) {
	if len(playerKeys) == 0 || limit <= 0 {
		return []SessionCandidateRow{}, nil
	}
	placeholders := make([]string, len(playerKeys))
	args := make([]any, 0, len(playerKeys)+1)
	for i, key := range playerKeys {
		placeholders[i] = "?"
		args = append(args, key)
	}
	args = append(args, limit)
	query := `SELECT r.id, r.replay_date, r.file_path, LOWER(TRIM(p.name)), p.name
		FROM replays r
		JOIN players p ON p.replay_id = r.id
		WHERE LOWER(TRIM(p.name)) IN (` + strings.Join(placeholders, ",") + `)
		  AND p.is_observer = 0
		ORDER BY r.replay_date DESC
		LIMIT ?`
	rows, err := Trace(s.replayScoped()).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SessionCandidateRow{}
	for rows.Next() {
		var row SessionCandidateRow
		if err := rows.Scan(&row.ReplayID, &row.ReplayDate, &row.FilePath, &row.PlayerKey, &row.PlayerName); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SessionReplayRow is the game-level half of a session game.
type SessionReplayRow struct {
	ReplayID           int64
	ReplayDate         string
	FileName           string
	MapName            string
	MapKind            string
	GameSource         string
	LobbyKind          string
	DurationSeconds    int64
	GameType           string
	Matchup            string
	TeamStacking       bool
	TeamInfoIncomplete bool
}

func (s *Store) ListReplaysByIDs(ctx context.Context, replayIDs []int64) ([]SessionReplayRow, error) {
	if len(replayIDs) == 0 {
		return []SessionReplayRow{}, nil
	}
	placeholders := make([]string, len(replayIDs))
	args := make([]any, len(replayIDs))
	for i, id := range replayIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT id, replay_date, file_name, map_name, map_kind, game_source, lobby_kind,
		duration_seconds, game_type, matchup, team_stacking, team_info_incomplete
		FROM replays
		WHERE id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY replay_date DESC`
	rows, err := Trace(s.replayScoped()).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SessionReplayRow{}
	for rows.Next() {
		var row SessionReplayRow
		if err := rows.Scan(&row.ReplayID, &row.ReplayDate, &row.FileName, &row.MapName, &row.MapKind,
			&row.GameSource, &row.LobbyKind, &row.DurationSeconds, &row.GameType, &row.Matchup,
			&row.TeamStacking, &row.TeamInfoIncomplete); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SessionPlayerAPMRow carries the per-game APM the session summary averages.
type SessionPlayerAPMRow struct {
	ReplayID  int64
	PlayerKey string
	APM       int64
	EAPM      int64
}

func (s *Store) ListPlayerAPMByReplayIDs(ctx context.Context, replayIDs []int64) ([]SessionPlayerAPMRow, error) {
	if len(replayIDs) == 0 {
		return []SessionPlayerAPMRow{}, nil
	}
	placeholders := make([]string, len(replayIDs))
	args := make([]any, len(replayIDs))
	for i, id := range replayIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT replay_id, LOWER(TRIM(name)), apm, eapm
		FROM players
		WHERE replay_id IN (` + strings.Join(placeholders, ",") + `) AND is_observer = 0`
	rows, err := Trace(s.replayScoped()).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SessionPlayerAPMRow{}
	for rows.Next() {
		var row SessionPlayerAPMRow
		if err := rows.Scan(&row.ReplayID, &row.PlayerKey, &row.APM, &row.EAPM); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

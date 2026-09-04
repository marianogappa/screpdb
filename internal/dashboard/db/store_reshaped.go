package db

import (
	"context"
	"fmt"
	"strings"
)

var _ Reader = (*Store)(nil)

func (s *Store) CountGames(ctx context.Context, query GamesQuery) (int64, error) {
	whereSQL, whereArgs := gamesQuerySQL(query)
	return s.CountGamesWithWhere(ctx, whereSQL, whereArgs)
}

func (s *Store) ListGames(ctx context.Context, query GamesQuery, limit, offset int) ([]WorkflowGameListRow, error) {
	whereSQL, whereArgs := gamesQuerySQL(query)
	return s.ListGamesWithWhere(ctx, whereSQL, whereArgs, limit, offset)
}

func gamesQuerySQL(query GamesQuery) (string, []any) {
	return BuildWorkflowGamesListWhere(
		query.PlayerKeys,
		query.MapNames,
		query.DurationBuckets,
		query.Featuring,
		query.MatchupKeys,
		query.MapKindKeys,
		WorkflowDurationSQLByKey(),
	)
}

func (s *Store) CountPlayers(ctx context.Context, query PlayersQuery) (int64, error) {
	baseSQL, whereSQL, allArgs := playersQuerySQL(query)
	return s.CountWorkflowPlayers(ctx, baseSQL, whereSQL, allArgs)
}

func (s *Store) ListPlayers(ctx context.Context, query PlayersQuery, limit, offset int) ([]WorkflowPlayersListRow, error) {
	baseSQL, whereSQL, allArgs := playersQuerySQL(query)
	return s.ListWorkflowPlayers(ctx, baseSQL, whereSQL, query.SortColumn, query.SortDir, allArgs, limit, offset)
}

func (s *Store) CountPlayersLastPlayedBuckets(ctx context.Context, query PlayersQuery) (int64, int64, error) {
	baseSQL, whereSQL, allArgs := playersQuerySQL(query)
	return s.CountWorkflowLastPlayedBuckets(ctx, baseSQL, whereSQL, allArgs)
}

func playersQuerySQL(query PlayersQuery) (baseSQL, whereSQL string, allArgs []any) {
	baseSQL, baseArgs := BuildWorkflowPlayersListBaseSQL(strings.ToLower(strings.TrimSpace(query.NameFilter)))
	whereSQL, whereArgs := BuildWorkflowPlayersListWhere(query.OnlyFivePlus, query.LastPlayed)
	allArgs = append(append([]any{}, baseArgs...), whereArgs...)
	return baseSQL, whereSQL, allArgs
}

func (s *Store) ListReplayLeaveReasons(ctx context.Context, replayID int64) ([]ReplayLeaveReasonRow, error) {
	rows, err := s.ReplayQueryContext(ctx,
		`SELECT c.player_id, COALESCE(c.leave_reason, '')
		 FROM commands c
		 WHERE c.replay_id = ?
		   AND c.action_type = 'Leave Game'
		   AND c.leave_reason IS NOT NULL
		 UNION ALL
		 SELECT c.player_id, COALESCE(c.leave_reason, '')
		 FROM commands_low_value c
		 WHERE c.replay_id = ?
		   AND c.action_type = 'Leave Game'
		   AND c.leave_reason IS NOT NULL`,
		replayID, replayID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query leave reasons: %w", err)
	}
	defer rows.Close()
	out := []ReplayLeaveReasonRow{}
	for rows.Next() {
		var row ReplayLeaveReasonRow
		if err := rows.Scan(&row.PlayerID, &row.Reason); err != nil {
			return nil, fmt.Errorf("failed to scan leave reason: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate leave reasons: %w", err)
	}
	return out, nil
}

func (s *Store) ListReplayChat(ctx context.Context, replayID int64) ([]ReplayChatRow, error) {
	rows, err := s.ReplayQueryContext(ctx,
		`SELECT c.seconds_from_game_start, c.player_id, COALESCE(c.chat_message, '')
		 FROM commands c
		 WHERE c.replay_id = ?
		   AND c.chat_message IS NOT NULL
		   AND trim(c.chat_message) <> ''
		 UNION ALL
		 SELECT c.seconds_from_game_start, c.player_id, COALESCE(c.chat_message, '')
		 FROM commands_low_value c
		 WHERE c.replay_id = ?
		   AND c.chat_message IS NOT NULL
		   AND trim(c.chat_message) <> ''
		 ORDER BY 1 ASC, 2 ASC`,
		replayID, replayID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query alliance-tab chat: %w", err)
	}
	defer rows.Close()
	out := []ReplayChatRow{}
	for rows.Next() {
		var row ReplayChatRow
		if err := rows.Scan(&row.Second, &row.PlayerID, &row.Message); err != nil {
			return nil, fmt.Errorf("failed to scan chat row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate chat rows: %w", err)
	}
	return out, nil
}

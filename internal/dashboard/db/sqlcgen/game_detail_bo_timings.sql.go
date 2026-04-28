// Hand-written to match sqlc-generated style. The query is static SQL —
// kept here (rather than under db/) to satisfy the package's
// "static SQL must live in sqlcgen" invariant guarded by
// TestStoreQueriesPreferSQLCForStaticSQL. The matching .sql definition
// lives in sqlc/queries/game_detail_more.sql.

package sqlcgen

import "context"

const ListEarlyZergMorphsForBOTimings = `-- name: ListEarlyZergMorphsForBOTimings :many
SELECT
  c.player_id,
  c.action_type,
  c.unit_type,
  c.seconds_from_game_start,
  c.frame
FROM commands c
JOIN players p ON p.id = c.player_id
WHERE c.replay_id = ?
  AND p.race = 'Zerg'
  AND p.is_observer = 0
  AND c.seconds_from_game_start < 600
  AND c.action_type IN ('Unit Morph', 'Build')
  AND c.unit_type IN ('Drone', 'Overlord', 'Spawning Pool', 'Hatchery')
ORDER BY c.player_id, c.frame
`

type ListEarlyZergMorphsForBOTimingsRow struct {
	PlayerID             int64
	ActionType           string
	UnitType             *string
	SecondsFromGameStart int64
	Frame                int64
}

func (q *Queries) ListEarlyZergMorphsForBOTimings(ctx context.Context, replayID int64) ([]ListEarlyZergMorphsForBOTimingsRow, error) {
	rows, err := q.db.QueryContext(ctx, ListEarlyZergMorphsForBOTimings, replayID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListEarlyZergMorphsForBOTimingsRow{}
	for rows.Next() {
		var i ListEarlyZergMorphsForBOTimingsRow
		if err := rows.Scan(
			&i.PlayerID,
			&i.ActionType,
			&i.UnitType,
			&i.SecondsFromGameStart,
			&i.Frame,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

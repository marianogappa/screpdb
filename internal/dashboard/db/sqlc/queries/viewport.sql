-- name: ListViewportAggregateRows :many
SELECT
  lower(trim(p.name)) AS player_key,
  CAST(MIN(p.name) AS TEXT) AS player_name,
  COALESCE(dp.value_string, '') AS raw_value
FROM detected_patterns_replay_player dp
JOIN players p
  ON p.id = dp.player_id
WHERE dp.pattern_name = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND dp.value_string IS NOT NULL
  AND trim(dp.value_string) <> ''
GROUP BY player_key, dp.replay_id, dp.player_id, dp.value_string
ORDER BY player_key ASC, player_name ASC;

-- name: ListViewportGameRows :many
SELECT player_id, COALESCE(value_string, '') AS raw_value
FROM detected_patterns_replay_player
WHERE replay_id = ?
  AND pattern_name = ?
  AND value_string IS NOT NULL
  AND trim(value_string) <> '';

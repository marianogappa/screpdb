-- name: ListUnitSliceCommandRows :many
SELECT c.player_id, c.seconds_from_game_start, c.unit_type
FROM commands c
WHERE c.replay_id = ?
  AND c.action_type IN ('Train', 'Unit Morph', 'Building Morph', 'Build')
  AND c.unit_type IS NOT NULL
  AND c.unit_type <> ''
ORDER BY c.seconds_from_game_start ASC, c.player_id ASC;

-- name: ListFirstUnitCommandRows :many
SELECT c.player_id, c.seconds_from_game_start, c.action_type, c.unit_type, c.unit_types
FROM commands c
WHERE c.replay_id = ?
  AND c.action_type IN ('Build', 'Train', 'Unit Morph')
ORDER BY c.player_id ASC, c.seconds_from_game_start ASC, c.id ASC;

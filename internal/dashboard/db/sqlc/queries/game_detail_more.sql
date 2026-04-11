-- name: ListDelayCommandRows :many
SELECT
  p.replay_id,
  p.id,
  p.name,
  p.race,
  c.seconds_from_game_start,
  c.action_type,
  c.unit_type,
  c.unit_types
FROM players p
JOIN commands c
  ON c.replay_id = p.replay_id
  AND c.player_id = p.id
WHERE
  p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND c.action_type IN ('Build', 'Train', 'Unit Morph')
  AND c.seconds_from_game_start <= ?
ORDER BY p.replay_id ASC, p.id ASC, c.seconds_from_game_start ASC, c.id ASC;

-- name: ListDelayCommandRowsForPlayer :many
SELECT
  p.replay_id,
  p.id,
  p.name,
  p.race,
  c.seconds_from_game_start,
  c.action_type,
  c.unit_type,
  c.unit_types
FROM players p
JOIN commands c
  ON c.replay_id = p.replay_id
  AND c.player_id = p.id
WHERE
  p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND c.action_type IN ('Build', 'Train', 'Unit Morph')
  AND c.seconds_from_game_start <= ?
  AND lower(trim(p.name)) = ?
ORDER BY p.replay_id ASC, p.id ASC, c.seconds_from_game_start ASC, c.id ASC;

-- name: CountPlayerGames :one
SELECT COUNT(*) AS games_played
FROM players p
WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human';

-- name: ListRaceSections :many
SELECT p.race, COUNT(*) AS game_count, CAST(COALESCE(SUM(CASE WHEN p.is_winner = 1 THEN 1 ELSE 0 END), 0) AS INTEGER) AS wins
FROM players p
WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
GROUP BY p.race
ORDER BY game_count DESC, p.race ASC;

-- name: ListRacePatterns :many
SELECT p.race, dp.pattern_name, COUNT(DISTINCT dp.replay_id) AS replay_count
FROM detected_patterns_replay_player dp
JOIN players p ON p.id = dp.player_id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND dp.pattern_name IS NOT NULL
  AND dp.pattern_name <> ''
  AND lower(replace(replace(dp.pattern_name, ' ', ''), '_', '')) NOT IN ('usedhotkeygroups', 'viewportmultitasking')
  AND (
    dp.value_bool = 1
    OR dp.value_int IS NOT NULL
    OR dp.value_timestamp IS NOT NULL
    OR (
      dp.value_string IS NOT NULL
      AND trim(dp.value_string) <> ''
      AND lower(trim(dp.value_string)) NOT IN ('0', 'false', 'no', '-')
    )
  )
GROUP BY p.race, dp.pattern_name;

-- name: ListTopActionTypes :many
SELECT c.action_type, COUNT(*) AS n
FROM commands c
WHERE c.player_id = ?
GROUP BY c.action_type
ORDER BY n DESC
LIMIT ?;

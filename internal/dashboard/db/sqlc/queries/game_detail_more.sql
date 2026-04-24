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
-- Post-markers-migration: presence of a replay_events row (event_kind='marker') *is* the match.
-- Filter out used_hotkey_groups/viewport_multitasking (meta markers that aren't race-characterising).
SELECT p.race, re.event_type AS pattern_name, COUNT(DISTINCT re.replay_id) AS replay_count
FROM replay_events re
JOIN players p ON p.id = re.source_player_id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND re.event_kind = 'marker'
  AND re.event_type NOT IN ('used_hotkey_groups', 'viewport_multitasking')
GROUP BY p.race, re.event_type;

-- name: ListTopActionTypes :many
SELECT c.action_type, COUNT(*) AS n
FROM commands c
WHERE c.player_id = ?
GROUP BY c.action_type
ORDER BY n DESC
LIMIT ?;

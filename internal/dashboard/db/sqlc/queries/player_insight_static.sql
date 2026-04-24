-- name: ListCommonBehaviours :many
SELECT re.event_type AS pattern_name, COUNT(DISTINCT re.replay_id) AS replay_count
FROM replay_events re
JOIN players p ON p.id = re.source_player_id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND re.event_kind = 'marker'
  AND re.event_type NOT IN ('used_hotkey_groups', 'viewport_multitasking')
GROUP BY re.event_type;

-- name: GetOutlierPlayerSummary :one
SELECT CAST(MIN(p.name) AS TEXT) AS name, COUNT(*) AS count
FROM players p
WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human';

-- name: ListPlayerGamesByRace :many
SELECT p.race, COUNT(*) AS games
FROM players p
WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
GROUP BY p.race;

-- name: ListPopulationGamesByRace :many
SELECT p.race, COUNT(*) AS games
FROM players p
WHERE p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
GROUP BY p.race;

-- name: ListPopulationDistinctPlayersByRace :many
SELECT p.race, CAST(COUNT(*) AS FLOAT) AS value
FROM (
  SELECT lower(trim(name)) AS player_key, race
  FROM players
  WHERE is_observer = 0 AND lower(trim(coalesce(type, ''))) = 'human'
  GROUP BY lower(trim(name)), race
) p
GROUP BY p.race;

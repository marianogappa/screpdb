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
SELECT CAST(COALESCE(MIN(p.name), '') AS TEXT) AS name, COUNT(*) AS count
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

-- name: ListPlayerGamesByRaceMapKind :many
-- Per-(race, map_kind) game count for a single player. Used as the
-- denominator when computing player rates for the segmented outlier pills.
-- map_kind=='UseMapSettings' is excluded from analytics elsewhere; we still
-- emit it here so downstream code has full visibility and can decide.
SELECT p.race AS race, r.map_kind AS map_kind, COUNT(*) AS games
FROM players p
JOIN replays r ON r.id = p.replay_id
WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
GROUP BY p.race, r.map_kind;

-- name: ListPopulationGamesByRaceMapKind :many
SELECT p.race AS race, r.map_kind AS map_kind, COUNT(*) AS games
FROM players p
JOIN replays r ON r.id = p.replay_id
WHERE p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
GROUP BY p.race, r.map_kind;

-- name: ListPopulationDistinctPlayersByRaceMapKind :many
SELECT t.race AS race, t.map_kind AS map_kind, CAST(COUNT(*) AS FLOAT) AS value
FROM (
  SELECT lower(trim(p.name)) AS player_key, p.race AS race, r.map_kind AS map_kind
  FROM players p
  JOIN replays r ON r.id = p.replay_id
  WHERE p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
  GROUP BY lower(trim(p.name)), p.race, r.map_kind
) t
GROUP BY t.race, t.map_kind;

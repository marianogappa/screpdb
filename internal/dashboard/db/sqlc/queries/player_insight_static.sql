-- name: ListCommonBehaviours :many
SELECT dp.pattern_name, COUNT(DISTINCT dp.replay_id) AS replay_count
FROM detected_patterns_replay_player dp
JOIN players p ON p.id = dp.player_id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
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
GROUP BY dp.pattern_name;

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

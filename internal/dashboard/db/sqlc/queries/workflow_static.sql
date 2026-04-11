-- name: ListWorkflowFilterPlayers :many
SELECT lower(trim(name)) AS player_key, CAST(MIN(name) AS TEXT) AS player_name, COUNT(*) AS games
FROM players
WHERE is_observer = 0
GROUP BY lower(trim(name))
HAVING COUNT(*) >= 5
ORDER BY games DESC, player_name ASC
LIMIT 200;

-- name: ListWorkflowFilterMaps :many
SELECT CAST(MIN(map_name) AS TEXT) AS map_name, COUNT(*) AS games
FROM replays
GROUP BY lower(trim(map_name))
ORDER BY games DESC, map_name ASC
LIMIT 15;

-- name: CountWorkflowDurationBuckets :one
SELECT
  CAST(COALESCE(SUM(CASE WHEN duration_seconds < 600 THEN 1 ELSE 0 END), 0) AS INTEGER) AS under_10m,
  CAST(COALESCE(SUM(CASE WHEN duration_seconds >= 600 AND duration_seconds < 1200 THEN 1 ELSE 0 END), 0) AS INTEGER) AS m10_20,
  CAST(COALESCE(SUM(CASE WHEN duration_seconds >= 1200 AND duration_seconds < 1800 THEN 1 ELSE 0 END), 0) AS INTEGER) AS m20_30,
  CAST(COALESCE(SUM(CASE WHEN duration_seconds >= 1800 AND duration_seconds < 2700 THEN 1 ELSE 0 END), 0) AS INTEGER) AS m30_45,
  CAST(COALESCE(SUM(CASE WHEN duration_seconds >= 2700 THEN 1 ELSE 0 END), 0) AS INTEGER) AS m45_plus
FROM replays;

-- name: CountDistinctPlayers :one
SELECT CAST(COUNT(*) AS FLOAT) AS total
FROM (
  SELECT lower(trim(name)) AS player_key
  FROM players
  WHERE is_observer = 0
  GROUP BY lower(trim(name))
);

-- name: CountDistinctPlayersByRace :one
SELECT CAST(COUNT(*) AS FLOAT) AS total
FROM (
  SELECT lower(trim(name)) AS player_key
  FROM players
  WHERE is_observer = 0
    AND race = ?
  GROUP BY lower(trim(name))
);

-- name: ListGameEventValues :many
SELECT replay_id, COALESCE(value_string, '') AS value
FROM detected_patterns_replay
WHERE pattern_name = 'Game Events';

-- name: ListPlayersByReplayRows :many
SELECT replay_id, id AS player_id, name
FROM players
WHERE is_observer = 0;

-- name: GetPlayerNameByKey :one
SELECT CAST(COALESCE(MIN(name), '') AS TEXT) AS player_name
FROM players
WHERE lower(trim(name)) = ?
  AND is_observer = 0
  AND lower(trim(coalesce(type, ''))) = 'human';

-- name: ListRaceOrderRows :many
SELECT p.id, p.race, c.action_type, c.tech_name, c.upgrade_name, c.seconds_from_game_start
FROM players p
JOIN commands c ON c.player_id = p.id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND (
    (c.action_type = 'Tech' AND c.tech_name IS NOT NULL AND c.tech_name <> '')
    OR
    (c.action_type = 'Upgrade' AND c.upgrade_name IS NOT NULL AND c.upgrade_name <> '')
  )
ORDER BY p.id ASC, c.seconds_from_game_start ASC;

-- name: CountQueuedGamesByPlayer :one
SELECT COUNT(DISTINCT p.id) AS count
FROM players p
JOIN commands c ON c.player_id = p.id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND c.is_queued = 1;

-- name: CountCarrierGamesByPlayer :one
SELECT COUNT(DISTINCT p.replay_id) AS count
FROM detected_patterns_replay_player dp
JOIN players p ON p.id = dp.player_id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND dp.pattern_name = 'Carriers'
  AND dp.value_bool = 1;

-- name: ListPlayerChatRows :many
SELECT c.replay_id, COALESCE(c.chat_message, '') AS chat_message
FROM commands c
JOIN players p ON p.id = c.player_id
JOIN replays r ON r.id = c.replay_id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND c.action_type = 'Chat'
  AND c.chat_message IS NOT NULL
  AND trim(c.chat_message) <> ''
ORDER BY r.replay_date DESC, c.replay_id DESC, c.seconds_from_game_start DESC;

-- name: ListGasTimingRows :many
SELECT c.player_id, c.seconds_from_game_start, COALESCE(c.unit_type, '') AS unit_type
FROM commands c
WHERE c.replay_id = ?
  AND c.action_type = 'Build'
  AND c.unit_type IN ('Assimilator', 'Extractor', 'Refinery')
ORDER BY c.player_id ASC, c.seconds_from_game_start ASC;

-- name: ListUpgradeTimingRows :many
SELECT c.player_id, c.seconds_from_game_start, COALESCE(c.upgrade_name, '') AS upgrade_name
FROM commands c
WHERE c.replay_id = ?
  AND c.action_type = 'Upgrade'
  AND c.upgrade_name IS NOT NULL
  AND c.upgrade_name <> ''
ORDER BY c.player_id ASC, c.seconds_from_game_start ASC;

-- name: ListTechTimingRows :many
SELECT c.player_id, c.seconds_from_game_start, COALESCE(c.tech_name, '') AS tech_name
FROM commands c
WHERE c.replay_id = ?
  AND c.action_type = 'Tech'
  AND c.tech_name IS NOT NULL
  AND c.tech_name <> ''
ORDER BY c.player_id ASC, c.seconds_from_game_start ASC;

-- name: ListHotkeyGamesRateByPlayer :many
WITH game_level AS (
  SELECT
    lower(trim(p.name)) AS player_key,
    CASE WHEN SUM(CASE WHEN c.action_type = 'Hotkey' AND c.hotkey_type IS NOT NULL THEN 1 ELSE 0 END) > 0 THEN 100.0 ELSE 0.0 END AS metric_value
  FROM players p
  LEFT JOIN commands_low_value c ON c.player_id = p.id
  WHERE p.is_observer = 0
    AND lower(trim(coalesce(p.type, ''))) = 'human'
  GROUP BY p.id
)
SELECT player_key, AVG(metric_value) AS metric_value
FROM game_level
GROUP BY player_key;

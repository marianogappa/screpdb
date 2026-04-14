-- name: GetReplaySummary :one
SELECT id, replay_date, file_name, map_name, duration_seconds, game_type
FROM replays
WHERE id = ?;

-- name: ListReplayPlayersForDetail :many
SELECT
  p.id,
  p.name,
  p.race,
  p.team,
  p.is_winner,
  p.apm,
  p.eapm,
  COUNT(c.id) AS command_count,
  (
    SELECT COUNT(*)
    FROM commands_low_value clv
    WHERE clv.player_id = p.id
      AND clv.action_type = 'Hotkey'
      AND clv.hotkey_type IS NOT NULL
  ) AS hotkey_count,
  (
    SELECT COUNT(*)
    FROM commands_low_value clv
    WHERE clv.player_id = p.id
  ) AS low_value_command_count
FROM players p
LEFT JOIN commands c ON c.player_id = p.id
WHERE p.replay_id = ? AND p.is_observer = 0
GROUP BY p.id, p.name, p.race, p.team, p.is_winner, p.apm, p.eapm
ORDER BY p.team ASC, p.id ASC;

-- name: ListReplayPatterns :many
SELECT
  pattern_name,
  CASE
    WHEN value_bool IS NOT NULL THEN CASE WHEN value_bool = 1 THEN 'true' ELSE 'false' END
    WHEN value_int IS NOT NULL THEN CAST(value_int AS TEXT)
    WHEN value_string IS NOT NULL THEN value_string
    WHEN value_timestamp IS NOT NULL THEN CAST(value_timestamp AS TEXT)
    ELSE ''
  END AS pattern_value
FROM detected_patterns_replay
WHERE replay_id = ?
ORDER BY pattern_name ASC;

-- name: ListTeamPatterns :many
SELECT
  team,
  pattern_name,
  CASE
    WHEN value_bool IS NOT NULL THEN CASE WHEN value_bool = 1 THEN 'true' ELSE 'false' END
    WHEN value_int IS NOT NULL THEN CAST(value_int AS TEXT)
    WHEN value_string IS NOT NULL THEN value_string
    WHEN value_timestamp IS NOT NULL THEN CAST(value_timestamp AS TEXT)
    ELSE ''
  END AS pattern_value
FROM detected_patterns_replay_team
WHERE replay_id = ?
ORDER BY team ASC, pattern_name ASC;

-- name: ListPlayerPatterns :many
SELECT
  player_id,
  pattern_name,
  CASE
    WHEN value_bool IS NOT NULL THEN CASE WHEN value_bool = 1 THEN 'true' ELSE 'false' END
    WHEN value_int IS NOT NULL THEN CAST(value_int AS TEXT)
    WHEN value_string IS NOT NULL THEN value_string
    WHEN value_timestamp IS NOT NULL THEN CAST(value_timestamp AS TEXT)
    ELSE ''
  END AS pattern_value
FROM detected_patterns_replay_player
WHERE replay_id = ?
ORDER BY player_id ASC, pattern_name ASC;

-- name: GetPlayerOverviewSummary :one
SELECT
  CAST(COALESCE(MIN(p.name), '') AS TEXT) AS player_name,
  COUNT(*) AS games_played,
  CAST(COALESCE(SUM(CASE WHEN p.is_winner = 1 THEN 1 ELSE 0 END), 0) AS INTEGER) AS wins,
  CAST(COALESCE(AVG(p.apm), 0) AS FLOAT) AS avg_apm,
  CAST(COALESCE(AVG(p.eapm), 0) AS FLOAT) AS avg_eapm
FROM players p
WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human';

-- name: ListPlayerRecentGames :many
SELECT
  r.id,
  r.replay_date,
  r.file_name,
  r.map_name,
  r.duration_seconds,
  r.game_type,
  CAST(COALESCE((
    SELECT group_concat(name, ' vs ')
    FROM (
      SELECT p2.name AS name
      FROM players p2
      WHERE p2.replay_id = r.id AND p2.is_observer = 0 AND lower(trim(coalesce(p2.type, ''))) = 'human'
      ORDER BY p2.team ASC, p2.id ASC
    )
  ), '') AS TEXT) AS players_label,
  CAST(COALESCE((
    SELECT group_concat(p3.name, ', ')
    FROM players p3
    WHERE p3.replay_id = r.id AND p3.is_winner = 1 AND p3.is_observer = 0 AND lower(trim(coalesce(p3.type, ''))) = 'human'
  ), '') AS TEXT) AS winners_label
FROM replays r
JOIN players p ON p.replay_id = r.id
WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
ORDER BY r.replay_date DESC, r.id DESC
LIMIT 12;

-- name: ListPlayerApmAggregates :many
SELECT
  lower(trim(p.name)) AS player_key,
  CAST(COALESCE(MIN(p.name), '') AS TEXT) AS player_name,
  CAST(COALESCE(AVG(CASE WHEN p.apm > 0 THEN p.apm END), 0) AS FLOAT) AS average_apm,
  COUNT(*) AS games_played
FROM players p
WHERE p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
GROUP BY lower(trim(p.name))
HAVING COUNT(*) >= ?
  AND COALESCE(AVG(CASE WHEN p.apm > 0 THEN p.apm END), 0) > 0;

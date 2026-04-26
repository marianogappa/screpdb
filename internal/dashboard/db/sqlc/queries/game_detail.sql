-- name: GetReplaySummary :one
SELECT id, replay_date, file_name, file_path, file_checksum, map_name, duration_seconds, game_type
FROM replays
WHERE id = ?;

-- name: ListReplayPlayersForDetail :many
-- Trimmed in Apr 2026: previously joined commands and ran two correlated
-- subqueries against commands_low_value (Hotkey count + total low-value)
-- per player to power a game-level hotkey-usage ratio. That ratio is no
-- longer surfaced — hotkey signal lives in the used_hotkey_groups /
-- never_used_hotkeys markers (computed at ingestion, read from
-- replay_events). Page-level metrics now only need player metadata + APM.
SELECT
  p.id,
  p.name,
  COALESCE(p.color, '') AS color,
  p.race,
  p.team,
  p.is_winner,
  p.start_location_oclock,
  p.apm,
  p.eapm
FROM players p
WHERE p.replay_id = ? AND p.is_observer = 0
ORDER BY p.team ASC, p.id ASC;

-- name: ListReplayPatterns :many
-- Replay-level markers (source_player_id IS NULL). event_type is the marker's FeatureKey.
-- detected_second + payload carry the post-migration per-row data; pattern_value
-- is kept as a transitional alias for frontend code that hasn't moved yet.
SELECT
  event_type AS pattern_name,
  COALESCE(payload, 'true') AS pattern_value,
  seconds_from_game_start AS detected_second,
  COALESCE(payload, '') AS payload
FROM replay_events
WHERE replay_id = ?
  AND event_kind = 'marker'
  AND source_player_id IS NULL
ORDER BY event_type ASC;

-- name: ListPlayerPatterns :many
SELECT
  source_player_id AS player_id,
  event_type AS pattern_name,
  COALESCE(payload, 'true') AS pattern_value,
  seconds_from_game_start AS detected_second,
  COALESCE(payload, '') AS payload
FROM replay_events
WHERE replay_id = ?
  AND event_kind = 'marker'
  AND source_player_id IS NOT NULL
ORDER BY source_player_id ASC, event_type ASC;

-- name: ListReplayEvents :many
SELECT
  re.event_type,
  re.seconds_from_game_start,
  re.source_player_id,
  COALESCE(sp.name, '') AS source_player_name,
  COALESCE(sp.color, '') AS source_player_color,
  re.target_player_id,
  COALESCE(tp.name, '') AS target_player_name,
  COALESCE(tp.color, '') AS target_player_color,
  re.location_base_type,
  re.location_base_oclock,
  re.location_natural_of_oclock,
  re.location_mineral_only,
  re.attack_unit_types
FROM replay_events re
LEFT JOIN players sp ON sp.id = re.source_player_id
LEFT JOIN players tp ON tp.id = re.target_player_id
WHERE re.replay_id = ?
  AND re.event_kind = 'game_event'
ORDER BY re.seconds_from_game_start ASC, re.event_type ASC, re.id ASC;

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
    SELECT group_concat(name, ', ')
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

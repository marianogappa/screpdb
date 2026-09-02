-- name: ListReplayPlayerHotkeyStreams :many
-- Per-player encoded hotkey streams for one game (hotkey timeline tab).
-- The blob is the internal/hotkeystream wire format; NULL when the player
-- issued no hotkey commands.
SELECT
  p.id,
  p.name,
  p.race,
  p.team,
  p.hotkey_stream
FROM players p
WHERE p.replay_id = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
ORDER BY p.team ASC, p.id ASC;

-- name: ListPlayerHotkeyStreamsByKey :many
-- All of a player's stored hotkey streams, newest first, for the hotkey
-- signature aggregation. Race comes per game: a Random player contributes
-- games under each actual race. Short games carry no phase signal.
SELECT
  p.name,
  p.race,
  p.hotkey_stream,
  r.duration_seconds
FROM players p
JOIN replays r ON r.id = p.replay_id
WHERE lower(trim(p.name)) = ?
  AND p.hotkey_stream IS NOT NULL
  AND p.is_observer = 0
  AND r.duration_seconds >= 360
ORDER BY r.replay_date DESC
LIMIT 120;

-- name: GetReplayPlayerHotkeyStream :one
-- One player's stream within one replay (hotkey map composite).
SELECT
  p.id,
  p.name,
  p.race,
  p.slot_id,
  p.hotkey_stream
FROM players p
WHERE p.replay_id = ? AND p.id = ?;

-- name: CountPlayerBnetGames :one
-- Whether (and how much) a player appears in Battle.net-sourced replays; the
-- player page shows its Battle.net section only when this is non-zero.
SELECT COUNT(*)
FROM replays r
JOIN players p ON p.replay_id = r.id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND r.game_source = 'AssumedBattleNet';

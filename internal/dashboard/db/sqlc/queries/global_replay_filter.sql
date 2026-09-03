-- name: GetGlobalReplayFilterConfigRaw :one
SELECT
  game_type,
  included_players,
  excluded_players,
  game_types_mode,
  game_types,
  exclude_short_games,
  exclude_computers,
  map_kind_filter_mode,
  map_kinds,
  player_filter_mode,
  players,
  compiled_replays_filter_sql
FROM settings
WHERE config_key = ?;

-- name: UpdateGlobalReplayFilterConfigRaw :exec
UPDATE settings
SET
  game_type = ?,
  included_maps = '[]',
  excluded_maps = '[]',
  included_players = '[]',
  excluded_players = '[]',
  game_types_mode = ?,
  game_types = ?,
  exclude_short_games = ?,
  exclude_computers = ?,
  map_kind_filter_mode = ?,
  map_kinds = ?,
  player_filter_mode = ?,
  players = ?,
  compiled_replays_filter_sql = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE config_key = ?;

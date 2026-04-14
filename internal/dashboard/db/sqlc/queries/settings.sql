-- name: GetIngestInputDir :one
SELECT ingest_input_dir
FROM settings
WHERE config_key = ?;

-- name: SetIngestInputDir :exec
UPDATE settings
SET ingest_input_dir = ?, updated_at = CURRENT_TIMESTAMP
WHERE config_key = ?;

-- name: CountReplays :one
SELECT COUNT(*) AS total_replays
FROM replays;

-- name: GetReplayFilePathByID :one
SELECT file_path
FROM replays
WHERE id = ?;

-- name: ListTopPlayerColorRows :many
SELECT lower(trim(name)) AS player_key, COUNT(*) AS games
FROM players
WHERE is_observer = 0
GROUP BY lower(trim(name))
ORDER BY games DESC, player_key ASC
LIMIT 15;

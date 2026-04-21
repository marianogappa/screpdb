-- name: ListPlayerAliases :many
SELECT
  id,
  canonical_alias,
  battle_tag_normalized,
  battle_tag_raw,
  aurora_id,
  source,
  updated_at
FROM player_aliases
ORDER BY battle_tag_normalized ASC, source ASC, canonical_alias ASC;

-- name: UpsertPlayerAlias :exec
INSERT INTO player_aliases (
  canonical_alias,
  battle_tag_normalized,
  battle_tag_raw,
  aurora_id,
  source
) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(source, battle_tag_normalized, canonical_alias) DO UPDATE SET
  battle_tag_raw = excluded.battle_tag_raw,
  aurora_id = excluded.aurora_id,
  updated_at = CURRENT_TIMESTAMP;

-- name: DeletePlayerAliasByID :exec
DELETE FROM player_aliases
WHERE id = ?;

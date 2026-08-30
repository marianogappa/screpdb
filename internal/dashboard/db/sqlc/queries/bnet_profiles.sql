-- name: GetBnetProfile :one
SELECT found, aurora_id, battle_tag, country_code, payload, fetched_at
FROM bnet_profiles
WHERE toon = ? AND gateway = ?;

-- name: UpsertBnetProfile :exec
INSERT INTO bnet_profiles (toon, gateway, found, aurora_id, battle_tag, country_code, payload, fetched_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (toon, gateway) DO UPDATE SET
	found = excluded.found,
	aurora_id = excluded.aurora_id,
	battle_tag = excluded.battle_tag,
	country_code = excluded.country_code,
	payload = excluded.payload,
	fetched_at = excluded.fetched_at;

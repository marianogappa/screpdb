-- name: ListViewportAggregateRows :many
-- Post-migration, the viewport_multitasking marker stores switches-per-minute in
-- replay_events.payload as JSON. Callers pass the marker FeatureKey as the bind.
SELECT
  lower(trim(p.name)) AS player_key,
  CAST(MIN(p.name) AS TEXT) AS player_name,
  COALESCE(re.payload, '') AS raw_value
FROM replay_events re
JOIN players p
  ON p.id = re.source_player_id
WHERE re.event_kind = 'marker'
  AND re.event_type = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND re.payload IS NOT NULL
  AND trim(re.payload) <> ''
GROUP BY player_key, re.replay_id, re.source_player_id, re.payload
ORDER BY player_key ASC, player_name ASC;

-- name: ListViewportGameRows :many
SELECT source_player_id AS player_id, COALESCE(payload, '') AS raw_value
FROM replay_events
WHERE replay_id = ?
  AND event_kind = 'marker'
  AND event_type = ?
  AND payload IS NOT NULL
  AND trim(payload) <> '';

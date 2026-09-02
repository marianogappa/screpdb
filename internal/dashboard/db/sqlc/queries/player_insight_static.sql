-- name: ListCommonBehaviours :many
SELECT re.event_type AS pattern_name, COUNT(DISTINCT re.replay_id) AS replay_count
FROM replay_events re
JOIN players p ON p.id = re.source_player_id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND re.event_kind = 'marker'
  AND re.event_type NOT IN ('used_hotkey_groups', 'viewport_multitasking')
GROUP BY re.event_type;


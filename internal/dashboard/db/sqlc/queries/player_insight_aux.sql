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

-- name: ListExpansionEvents :many
SELECT replay_id, source_player_id, seconds_from_game_start
FROM replay_events
WHERE event_kind = 'game_event'
  AND event_type = 'expansion'
  AND source_player_id IS NOT NULL;

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

-- name: ListMatchupOrderRows :many
-- Per-matchup tech/upgrade rows for a single player. 1v1 only - the opponent
-- race is well-defined only when there's exactly one opposing human; multi-
-- player games are excluded so the sequences aren't averaged across mismatched
-- opponents. Each row carries (own_race, opp_race, replay_id, action_type,
-- tech_name, upgrade_name, seconds) so the consumer can stitch sequences
-- per (player, replay) and bucket them per matchup.
SELECT
  self.id AS player_id,
  self.race AS own_race,
  opp.race AS opp_race,
  self.replay_id AS replay_id,
  c.action_type AS action_type,
  c.tech_name AS tech_name,
  c.upgrade_name AS upgrade_name,
  c.seconds_from_game_start AS seconds_from_game_start
FROM players self
JOIN players opp
  ON opp.replay_id = self.replay_id
  AND opp.id != self.id
  AND opp.is_observer = 0
  AND lower(trim(coalesce(opp.type, ''))) = 'human'
JOIN commands c
  ON c.player_id = self.id
WHERE lower(trim(self.name)) = ?
  AND self.is_observer = 0
  AND lower(trim(coalesce(self.type, ''))) = 'human'
  AND (
    (c.action_type = 'Tech' AND c.tech_name IS NOT NULL AND c.tech_name <> '')
    OR
    (c.action_type = 'Upgrade' AND c.upgrade_name IS NOT NULL AND c.upgrade_name <> '')
  )
  AND 2 = (
    SELECT COUNT(*) FROM players p2
    WHERE p2.replay_id = self.replay_id
      AND p2.is_observer = 0
      AND lower(trim(coalesce(p2.type, ''))) = 'human'
  )
ORDER BY self.id, c.seconds_from_game_start;

-- name: CountQueuedGamesByPlayer :one
SELECT COUNT(DISTINCT p.id) AS count
FROM players p
JOIN commands c ON c.player_id = p.id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND c.is_queued = 1;

-- name: CountCarrierGamesByPlayer :one
SELECT COUNT(DISTINCT p.replay_id) AS count
FROM players p
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND EXISTS (
    SELECT 1 FROM replay_events re
    WHERE re.source_player_id = p.id
      AND re.event_kind = 'marker'
      AND re.event_type = 'carriers'
  );

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
-- Hotkey usage as a per-player rate of "games where any hotkey-group command
-- fired." Sourced from the used_hotkey_groups marker (computed at ingestion;
-- one replay_events row per (replay x player) when groups exist), so this
-- avoids scanning commands_low_value at query time. EXISTS guard handles
-- duplicate marker rows defensively even though the streaming detector
-- emits at most one per (replay x player).
WITH game_level AS (
  SELECT
    lower(trim(p.name)) AS player_key,
    CASE WHEN EXISTS (
      SELECT 1 FROM replay_events re
      WHERE re.replay_id = p.replay_id
        AND re.source_player_id = p.id
        AND re.event_kind = 'marker'
        AND re.event_type = 'used_hotkey_groups'
    ) THEN 100.0 ELSE 0.0 END AS metric_value
  FROM players p
  WHERE p.is_observer = 0
    AND lower(trim(coalesce(p.type, ''))) = 'human'
)
SELECT player_key, AVG(metric_value) AS metric_value
FROM game_level
GROUP BY player_key;

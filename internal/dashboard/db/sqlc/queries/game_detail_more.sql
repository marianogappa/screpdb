-- name: CountPlayerGames :one
SELECT COUNT(*) AS games_played
FROM players p
WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human';

-- name: ListRaceSections :many
SELECT p.race, COUNT(*) AS game_count, CAST(COALESCE(SUM(CASE WHEN p.is_winner = 1 THEN 1 ELSE 0 END), 0) AS INTEGER) AS wins
FROM players p
WHERE lower(trim(p.name)) = ? AND p.is_observer = 0 AND lower(trim(coalesce(p.type, ''))) = 'human'
GROUP BY p.race
ORDER BY game_count DESC, p.race ASC;

-- name: ListRacePatterns :many
-- Post-markers-migration: presence of a replay_events row (event_kind='marker') *is* the match.
-- Filter out used_hotkey_groups/viewport_multitasking (meta markers that aren't race-characterising).
SELECT p.race, re.event_type AS pattern_name, COUNT(DISTINCT re.replay_id) AS replay_count
FROM replay_events re
JOIN players p ON p.id = re.source_player_id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND re.event_kind = 'marker'
  AND re.event_type NOT IN ('used_hotkey_groups', 'viewport_multitasking')
GROUP BY p.race, re.event_type;

-- name: ListTopActionTypes :many
SELECT c.action_type, COUNT(*) AS n
FROM commands c
WHERE c.player_id = ?
GROUP BY c.action_type
ORDER BY n DESC
LIMIT ?;

-- name: GetPhaseBoundariesForReplay :many
-- Returns the replay-level phase-boundary markers (mid_game_starts,
-- late_game_starts) persisted at ingest by
-- internal/patterns/detectors/phase_boundary_detector.go. Each row is a
-- single (event_type, second) pair; either or both may be absent when
-- the replay never reached the corresponding boundary. Caller treats
-- absent rows as "phase split not detected" -- the same semantics used
-- by the per-game events list (collapsed empty section).
SELECT
  re.event_type,
  re.seconds_from_game_start
FROM replay_events re
WHERE re.replay_id = ?
  AND re.event_kind = 'marker'
  AND re.event_type IN ('mid_game_starts', 'late_game_starts')
  AND re.source_player_id IS NULL;

-- name: ListGameUnitProductionAndCasts :many
-- Returns Train / Unit Morph / spell-cast commands for a single replay,
-- needed by the per-game endpoint to compute attacker composition pills
-- at request time (no ingest-time persistence -- see plan: composition is
-- computed on the fly so it stays in sync with edits to caster sets,
-- excluded units, and presentation rules without re-ingest).
--
-- Rows pre-joined to players so the caller has player_id without a
-- second roundtrip. Right-clicks/hotkeys etc. live in commands_low_value
-- and are intentionally excluded -- per the project memory rule the
-- dashboard never queries that table.
--
-- Casts/abilities are all 'Targeted Order' commands (Cast*, FireYamatoGun,
-- PlaceMine, ...). We pull the whole action type rather than matching
-- order_name prefixes here so the Go-side compositionSpells map is the
-- single source of truth for which orders surface -- adding a spell never
-- needs an SQL change. Unmapped Targeted Orders are dropped in Go.
SELECT
  c.player_id,
  c.action_type,
  c.unit_type,
  c.unit_types,
  c.order_name,
  c.seconds_from_game_start
FROM commands c
JOIN players p ON p.id = c.player_id
WHERE c.replay_id = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND c.action_type IN ('Train', 'Unit Morph', 'Targeted Order')
ORDER BY c.player_id, c.seconds_from_game_start, c.id;

-- name: ListPlayerFirstExpansionTimings :many
-- One row per (race, map_kind, replay) for a single player giving the
-- earliest expansion event time. Backs the early-game timing summary that
-- compares Regular vs Money maps. Note: relies on game_event 'expansion'
-- already being stored at ingest by the worldstate detector.
SELECT
  p.race AS race,
  r.map_kind AS map_kind,
  re.replay_id AS replay_id,
  CAST(MIN(re.seconds_from_game_start) AS INTEGER) AS first_expansion_second
FROM replay_events re
JOIN players p ON p.id = re.source_player_id
JOIN replays r ON r.id = re.replay_id
WHERE re.event_kind = 'game_event'
  AND re.event_type = 'expansion'
  AND lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND r.map_kind != 'UseMapSettings'
GROUP BY p.race, r.map_kind, re.replay_id
ORDER BY p.race, r.map_kind, first_expansion_second;

-- name: ListPlayerMatchups :many
-- Per-matchup breakdown for a single player. 1v1 only - multi-player games
-- have ambiguous opponent race so we exclude them. Returns one row per
-- (own_race, opp_race) pair with sample size and win count.
SELECT
  self.race AS own_race,
  opp.race AS opp_race,
  COUNT(DISTINCT self.replay_id) AS games,
  CAST(SUM(CASE WHEN self.is_winner = 1 THEN 1 ELSE 0 END) AS INTEGER) AS wins
FROM players self
JOIN players opp ON opp.replay_id = self.replay_id AND opp.id != self.id
WHERE lower(trim(self.name)) = ?
  AND self.is_observer = 0
  AND lower(trim(coalesce(self.type, ''))) = 'human'
  AND opp.is_observer = 0
  AND lower(trim(coalesce(opp.type, ''))) = 'human'
  AND 2 = (
    SELECT COUNT(*) FROM players p
    WHERE p.replay_id = self.replay_id
      AND p.is_observer = 0
      AND lower(trim(coalesce(p.type, ''))) = 'human'
  )
GROUP BY self.race, opp.race
ORDER BY games DESC, own_race, opp_race;

-- name: ListEarlyZergMorphsForBOTimings :many
SELECT
  c.player_id,
  c.action_type,
  c.unit_type,
  c.seconds_from_game_start,
  c.frame
FROM commands c
JOIN players p ON p.id = c.player_id
WHERE c.replay_id = ?
  AND p.race = 'Zerg'
  AND p.is_observer = 0
  AND c.seconds_from_game_start < 600
  AND c.action_type IN ('Unit Morph', 'Build')
  AND c.unit_type IN ('Drone', 'Overlord', 'Spawning Pool', 'Hatchery')
ORDER BY c.player_id, c.frame;

-- name: ListPlayerMatchupAggregates :many
-- 1v1 only. Per-(own_race, opp_race) sample size, wins, average APM and EAPM
-- for a single player. Mirrors the join shape of ListPlayerMatchups
-- (game_detail_more.sql) but adds APM/EAPM averages for the per-matchup
-- summary card.
SELECT
  self.race AS own_race,
  opp.race AS opp_race,
  COUNT(DISTINCT self.replay_id) AS games,
  CAST(SUM(CASE WHEN self.is_winner = 1 THEN 1 ELSE 0 END) AS INTEGER) AS wins,
  CAST(COALESCE(AVG(CASE WHEN self.apm > 0 THEN self.apm END), 0) AS FLOAT) AS avg_apm,
  CAST(COALESCE(AVG(CASE WHEN self.eapm > 0 THEN self.eapm END), 0) AS FLOAT) AS avg_eapm
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

-- name: ListPlayerMatchupMarkerCounts :many
-- 1v1 only. Per-(own_race, opp_race, marker_event_type) replay counts. The
-- consumer splits build-order markers (event_type LIKE 'bo_%') from the rest
-- and surfaces the top-N per matchup. Excludes meta markers that are not
-- meaningful per-matchup features. Drop subtype game events
-- (dt_drop / reaver_drop / cliff_drop) are also surfaced alongside the
-- generic `made_drops` marker so subtype-level signal lands on the Player
-- summary cards.
SELECT
  self.race AS own_race,
  opp.race AS opp_race,
  re.event_type AS pattern_name,
  COUNT(DISTINCT re.replay_id) AS replay_count
FROM players self
JOIN players opp ON opp.replay_id = self.replay_id AND opp.id != self.id
JOIN replay_events re ON re.source_player_id = self.id AND re.replay_id = self.replay_id
WHERE lower(trim(self.name)) = ?
  AND self.is_observer = 0
  AND lower(trim(coalesce(self.type, ''))) = 'human'
  AND opp.is_observer = 0
  AND lower(trim(coalesce(opp.type, ''))) = 'human'
  AND (
    (re.event_kind = 'marker' AND re.event_type NOT IN (
      'used_hotkey_groups',
      'viewport_multitasking',
      'mid_game_starts',
      'late_game_starts',
      'never_used_hotkeys'
    ))
    OR (re.event_kind = 'game_event' AND re.event_type IN (
      'dt_drop', 'reaver_drop', 'cliff_drop'
    ))
  )
  AND 2 = (
    SELECT COUNT(*) FROM players p
    WHERE p.replay_id = self.replay_id
      AND p.is_observer = 0
      AND lower(trim(coalesce(p.type, ''))) = 'human'
  )
GROUP BY self.race, opp.race, re.event_type;

-- name: ListPlayerByFormatAggregates :many
-- Per-(own_race, team_format, map_kind) APM/EAPM/games/wins for a single
-- player. The own_race split is what makes the Summary tab work for
-- Random players: without it a Random player's 2v2 multi-team card
-- aggregates BOs/markers across three races and the most-played race's
-- patterns dominate the top-N. The Go layer collapses team_format into
-- buckets (2v2, 3v3, multi-team); a CASE expression here triggers a sqlc
-- v1.30 parser bug that mangles the generated SQL string.
SELECT
  p.race AS own_race,
  r.team_format AS team_format,
  r.map_kind AS map_kind,
  COUNT(DISTINCT p.replay_id) AS games,
  CAST(SUM(CASE WHEN p.is_winner = 1 THEN 1 ELSE 0 END) AS INTEGER) AS wins,
  CAST(COALESCE(AVG(CASE WHEN p.apm > 0 THEN p.apm END), 0) AS FLOAT) AS avg_apm,
  CAST(COALESCE(AVG(CASE WHEN p.eapm > 0 THEN p.eapm END), 0) AS FLOAT) AS avg_eapm
FROM players p
JOIN replays r ON r.id = p.replay_id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND r.map_kind IN ('Regular', 'Money')
  AND r.team_format LIKE '%v%'
  AND r.team_format != '1v1'
GROUP BY p.race, r.team_format, r.map_kind;

-- name: ListPlayerByFormatMarkerCounts :many
-- Per-(own_race, team_format, map_kind, marker) replay counts for the by-
-- format summary cards. Same exclusion list as the per-matchup query so
-- meta markers (hotkeys, viewport multitasking, phase boundaries) don't
-- pollute the top-N pills. Drop subtype game events
-- (dt_drop / reaver_drop / cliff_drop) are also surfaced alongside the
-- generic `made_drops` marker for subtype-level signal.
SELECT
  p.race AS own_race,
  r.team_format AS team_format,
  r.map_kind AS map_kind,
  re.event_type AS pattern_name,
  COUNT(DISTINCT re.replay_id) AS replay_count
FROM players p
JOIN replays r ON r.id = p.replay_id
JOIN replay_events re ON re.source_player_id = p.id AND re.replay_id = r.id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND r.map_kind IN ('Regular', 'Money')
  AND r.team_format LIKE '%v%'
  AND r.team_format != '1v1'
  AND (
    (re.event_kind = 'marker' AND re.event_type NOT IN (
      'used_hotkey_groups',
      'viewport_multitasking',
      'mid_game_starts',
      'late_game_starts',
      'never_used_hotkeys'
    ))
    OR (re.event_kind = 'game_event' AND re.event_type IN (
      'dt_drop', 'reaver_drop', 'cliff_drop'
    ))
  )
GROUP BY p.race, r.team_format, r.map_kind, re.event_type;

-- name: CountPlayerMultiTeamMeleeGames :one
-- Replays in which the player participated that are eligible for the
-- never-allied detection: team_format has at least 3 teams (LIKE '%v%v%')
-- and the game type allows dynamic re-allying (Melee/Free For All), and the
-- map is not a UseMapSettings scenario (which we exclude from analytics).
SELECT COUNT(DISTINCT r.id) AS games
FROM replays r
JOIN players p ON p.replay_id = r.id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND r.team_format LIKE '%v%v%'
  AND r.map_kind != 'UseMapSettings'
  AND r.game_type IN ('Melee', 'Free For All');

-- name: CountPlayerAllianceCommandsInMultiTeamMelee :one
-- Counts Alliance commands the player issued across all multi-team melee
-- games they participated in. Sourced from commands_low_value (where
-- Alliance commands live by the storage classifier in
-- internal/storage/sqlite.go), but Alliance is intrinsically low-cardinality
-- per replay so a single aggregate query is cheap and matches the existing
-- ListReplayAllianceCommands access pattern in game_detail.sql.
SELECT COUNT(*) AS alliance_commands
FROM commands_low_value c
JOIN players p ON p.id = c.player_id
JOIN replays r ON r.id = c.replay_id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND c.action_type = 'Alliance'
  AND r.team_format LIKE '%v%v%'
  AND r.map_kind != 'UseMapSettings'
  AND r.game_type IN ('Melee', 'Free For All');

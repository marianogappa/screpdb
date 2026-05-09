package db

import (
	"context"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type PlayerMatchupAggregateRow struct {
	OwnRace string
	OppRace string
	Games   int64
	Wins    int64
	AvgAPM  float64
	AvgEAPM float64
}

type PlayerMatchupMarkerCountRow struct {
	OwnRace     string
	OppRace     string
	PatternName string
	ReplayCount int64
}

type PlayerByFormatAggregateRow struct {
	OwnRace    string
	TeamFormat string
	MapKind    string
	Games      int64
	Wins       int64
	AvgAPM     float64
	AvgEAPM    float64
}

type PlayerByFormatMarkerCountRow struct {
	OwnRace     string
	TeamFormat  string
	MapKind     string
	PatternName string
	ReplayCount int64
}

func (s *Store) ListPlayerMatchupAggregates(ctx context.Context, playerKey string) ([]PlayerMatchupAggregateRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerMatchupAggregates(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerMatchupAggregateRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlayerMatchupAggregateRow{
			OwnRace: row.OwnRace,
			OppRace: row.OppRace,
			Games:   row.Games,
			Wins:    row.Wins,
			AvgAPM:  row.AvgApm,
			AvgEAPM: row.AvgEapm,
		})
	}
	return out, nil
}

func (s *Store) ListPlayerMatchupMarkerCounts(ctx context.Context, playerKey string) ([]PlayerMatchupMarkerCountRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListPlayerMatchupMarkerCounts(ctx, playerKey)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerMatchupMarkerCountRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlayerMatchupMarkerCountRow{
			OwnRace:     row.OwnRace,
			OppRace:     row.OppRace,
			PatternName: row.PatternName,
			ReplayCount: row.ReplayCount,
		})
	}
	return out, nil
}

// ListPlayerByFormatAggregates is bypassing sqlc on purpose: sqlc 1.30
// has a string-truncation bug that mangles the generated GROUP BY
// (e.g. "r.map_kind" rendered as "r.map_ki") whenever this query's exact
// shape is generated. The raw query mirrors the .sql file in the queries
// directory; keep them in sync if you edit either.
const playerByFormatAggregatesSQL = `
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
GROUP BY p.race, r.team_format, r.map_kind
`

func (s *Store) ListPlayerByFormatAggregates(ctx context.Context, playerKey string) ([]PlayerByFormatAggregateRow, error) {
	rows, err := s.ReplayQueryContext(ctx, playerByFormatAggregatesSQL, playerKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlayerByFormatAggregateRow{}
	for rows.Next() {
		var row PlayerByFormatAggregateRow
		if err := rows.Scan(&row.OwnRace, &row.TeamFormat, &row.MapKind, &row.Games, &row.Wins, &row.AvgAPM, &row.AvgEAPM); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

const playerByFormatMarkerCountsSQL = `
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
  AND re.event_kind = 'marker'
  AND re.event_type NOT IN (
    'used_hotkey_groups',
    'viewport_multitasking',
    'mid_game_starts',
    'late_game_starts',
    'never_used_hotkeys'
  )
GROUP BY p.race, r.team_format, r.map_kind, re.event_type
`

func (s *Store) ListPlayerByFormatMarkerCounts(ctx context.Context, playerKey string) ([]PlayerByFormatMarkerCountRow, error) {
	rows, err := s.ReplayQueryContext(ctx, playerByFormatMarkerCountsSQL, playerKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlayerByFormatMarkerCountRow{}
	for rows.Next() {
		var row PlayerByFormatMarkerCountRow
		if err := rows.Scan(&row.OwnRace, &row.TeamFormat, &row.MapKind, &row.PatternName, &row.ReplayCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// Both of these are bypassing sqlc on purpose: sqlc 1.30 has a string-
// truncation bug that mangles these specific queries (the "Free For All"
// literal at the end of an IN list seems to interact poorly with the
// generator's tokenizer). Keep the raw strings here in sync with the .sql
// file in the queries directory.
const playerMultiTeamMeleeGamesSQL = `
SELECT COUNT(DISTINCT r.id) AS games
FROM replays r
JOIN players p ON p.replay_id = r.id
WHERE lower(trim(p.name)) = ?
  AND p.is_observer = 0
  AND lower(trim(coalesce(p.type, ''))) = 'human'
  AND r.team_format LIKE '%v%v%'
  AND r.map_kind != 'UseMapSettings'
  AND r.game_type IN ('Melee', 'Free For All')
`

func (s *Store) CountPlayerMultiTeamMeleeGames(ctx context.Context, playerKey string) (int64, error) {
	row := s.ReplayQueryRowContext(ctx, playerMultiTeamMeleeGamesSQL, playerKey)
	var games int64
	if err := row.Scan(&games); err != nil {
		return 0, err
	}
	return games, nil
}

const playerAllianceCommandsInMultiTeamMeleeSQL = `
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
  AND r.game_type IN ('Melee', 'Free For All')
`

func (s *Store) CountPlayerAllianceCommandsInMultiTeamMelee(ctx context.Context, playerKey string) (int64, error) {
	row := s.ReplayQueryRowContext(ctx, playerAllianceCommandsInMultiTeamMeleeSQL, playerKey)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

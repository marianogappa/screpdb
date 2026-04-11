package db

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type UnitCadenceReplayMetricRow struct {
	ReplayID        int64
	PlayerKey       string
	PlayerName      string
	FileName        string
	DurationSeconds int64
	WindowSeconds   int64
	UnitsProduced   int64
	GapCount        int64
	RatePerMinute   float64
	CVGap           float64
	Burstiness      float64
	Idle20Ratio     float64
	CadenceScore    float64
}

func (s *Store) ListUnitCadenceReplayMetrics(
	ctx context.Context,
	excludedUnits []string,
	onlyPlayerKey string,
	startSeconds int64,
	endFraction float64,
	idleGapSeconds int64,
	minUnitsPerReplay int64,
	minGapsPerReplay int64,
) ([]UnitCadenceReplayMetricRow, error) {
	inClause := strings.TrimRight(strings.Repeat("?,", len(excludedUnits)), ",")
	filterSQL := ""
	args := []any{}
	for _, name := range excludedUnits {
		args = append(args, name)
	}
	if onlyPlayerKey != "" {
		filterSQL = "AND lower(trim(p.name)) = ?"
		args = append(args, onlyPlayerKey)
	}
	rows, err := s.ReplayQueryContext(ctx, `
		WITH base AS (
			SELECT
				c.replay_id,
				lower(trim(p.name)) AS player_key,
				MIN(p.name) AS player_name,
				r.file_name,
				r.duration_seconds,
				c.seconds_from_game_start AS t,
				c.id AS cmd_id
			FROM commands c
			JOIN players p
				ON p.id = c.player_id
			JOIN replays r
				ON r.id = c.replay_id
			WHERE
				p.is_observer = 0
				AND lower(trim(coalesce(p.type, ''))) = 'human'
				AND c.action_type IN ('Train', 'Unit Morph')
				AND c.unit_type IS NOT NULL
				AND trim(c.unit_type) <> ''
				AND c.unit_type NOT IN (`+inClause+`)
				AND c.seconds_from_game_start >= `+strconv.FormatInt(startSeconds, 10)+`
				AND c.seconds_from_game_start <= CAST(`+strconv.FormatFloat(endFraction, 'f', 4, 64)+` * r.duration_seconds AS INTEGER)
				AND CAST(`+strconv.FormatFloat(endFraction, 'f', 4, 64)+` * r.duration_seconds AS INTEGER) > `+strconv.FormatInt(startSeconds, 10)+`
				`+filterSQL+`
			GROUP BY
				c.replay_id,
				player_key,
				r.file_name,
				r.duration_seconds,
				c.seconds_from_game_start,
				c.id
		),
		ordered AS (
			SELECT
				replay_id,
				player_key,
				player_name,
				file_name,
				duration_seconds,
				t,
				cmd_id,
				LAG(t) OVER (PARTITION BY replay_id, player_key ORDER BY t, cmd_id) AS prev_t
			FROM base
		),
		gaps AS (
			SELECT
				replay_id,
				player_key,
				player_name,
				file_name,
				duration_seconds,
				t,
				(t - prev_t) AS gap_s
			FROM ordered
		),
		replay_metrics AS (
			SELECT
				replay_id,
				player_key,
				player_name,
				file_name,
				duration_seconds,
				CAST(`+strconv.FormatFloat(endFraction, 'f', 4, 64)+` * duration_seconds AS INTEGER) - `+strconv.FormatInt(startSeconds, 10)+` AS window_s,
				COUNT(*) AS n_units,
				COUNT(gap_s) AS n_gaps,
				AVG(gap_s * 1.0) AS mean_gap_s,
				sqrt(AVG(gap_s * gap_s * 1.0) - AVG(gap_s * 1.0) * AVG(gap_s * 1.0)) AS std_gap_s,
				SUM(CASE WHEN gap_s >= `+strconv.FormatInt(idleGapSeconds, 10)+` THEN 1 ELSE 0 END) * 1.0 / NULLIF(COUNT(gap_s), 0) AS idle20_ratio
			FROM gaps
			GROUP BY replay_id, player_key, player_name, file_name, duration_seconds
			HAVING
				COUNT(*) >= `+strconv.FormatInt(minUnitsPerReplay, 10)+`
				AND COUNT(gap_s) >= `+strconv.FormatInt(minGapsPerReplay, 10)+`
				AND window_s > 0
		),
		scored AS (
			SELECT
				replay_id,
				player_key,
				player_name,
				file_name,
				duration_seconds,
				window_s,
				n_units,
				n_gaps,
				(n_units * 60.0) / window_s AS rate_per_min,
				(std_gap_s / NULLIF(mean_gap_s, 0)) AS cv_gap,
				(((std_gap_s / NULLIF(mean_gap_s, 0)) - 1.0) / ((std_gap_s / NULLIF(mean_gap_s, 0)) + 1.0)) AS burstiness,
				idle20_ratio,
				((n_units * 60.0) / window_s) / (1.0 + COALESCE((std_gap_s / NULLIF(mean_gap_s, 0)), 9999.0)) AS cadence_score
			FROM replay_metrics
		)
		SELECT
			replay_id,
			player_key,
			player_name,
			file_name,
			duration_seconds,
			window_s,
			n_units,
			n_gaps,
			rate_per_min,
			cv_gap,
			burstiness,
			idle20_ratio,
			cadence_score
		FROM scored
		ORDER BY player_key ASC, replay_id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []UnitCadenceReplayMetricRow{}
	for rows.Next() {
		var row UnitCadenceReplayMetricRow
		var cvGap sql.NullFloat64
		var burstiness sql.NullFloat64
		var idle20 sql.NullFloat64
		var cadence sql.NullFloat64
		if err := rows.Scan(
			&row.ReplayID, &row.PlayerKey, &row.PlayerName, &row.FileName, &row.DurationSeconds,
			&row.WindowSeconds, &row.UnitsProduced, &row.GapCount, &row.RatePerMinute,
			&cvGap, &burstiness, &idle20, &cadence,
		); err != nil {
			return nil, err
		}
		if cvGap.Valid {
			row.CVGap = cvGap.Float64
		}
		if burstiness.Valid {
			row.Burstiness = burstiness.Float64
		}
		if idle20.Valid {
			row.Idle20Ratio = idle20.Float64
		}
		if cadence.Valid {
			row.CadenceScore = cadence.Float64
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

type UnitSliceCommandRow struct {
	PlayerID int64
	Second   int64
	UnitType string
}

func (s *Store) ListUnitSliceCommandRows(ctx context.Context, replayID int64) ([]UnitSliceCommandRow, error) {
	rows, err := sqlcgen.New(s.replayScoped()).ListUnitSliceCommandRows(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]UnitSliceCommandRow, 0, len(rows))
	for _, row := range rows {
		unitType := ""
		if row.UnitType != nil {
			unitType = *row.UnitType
		}
		out = append(out, UnitSliceCommandRow{
			PlayerID: row.PlayerID,
			Second:   row.SecondsFromGameStart,
			UnitType: unitType,
		})
	}
	return out, nil
}

type FirstUnitCommandRow struct {
	PlayerID   int64
	Second     int64
	ActionType string
	UnitType   sql.NullString
	UnitTypes  sql.NullString
}

func (s *Store) ListFirstUnitCommandRows(ctx context.Context, replayID int64) ([]FirstUnitCommandRow, error) {
	rows, err := sqlcgen.New(s.replayScoped()).ListFirstUnitCommandRows(ctx, replayID)
	if err != nil {
		return nil, err
	}
	out := make([]FirstUnitCommandRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, FirstUnitCommandRow{
			PlayerID:   row.PlayerID,
			Second:     row.SecondsFromGameStart,
			ActionType: row.ActionType,
			UnitType:   nullableStringPtrToNullString(row.UnitType),
			UnitTypes:  nullableStringPtrToNullString(row.UnitTypes),
		})
	}
	return out, nil
}

type GameUnitCadenceRow struct {
	PlayerID      int64
	WindowSeconds int64
	UnitsProduced int64
	GapCount      int64
	RatePerMinute sql.NullFloat64
	CVGap         sql.NullFloat64
	Burstiness    sql.NullFloat64
	Idle20Ratio   sql.NullFloat64
	CadenceScore  sql.NullFloat64
}

func (s *Store) ListGameUnitCadenceRows(
	ctx context.Context,
	replayID int64,
	durationSeconds int64,
	excludedUnits []string,
	startSeconds int64,
	endFraction float64,
	idleGapSeconds int64,
) ([]GameUnitCadenceRow, error) {
	placeholders := strings.TrimRight(strings.Repeat("?,", len(excludedUnits)), ",")
	args := []any{replayID}
	for _, name := range excludedUnits {
		args = append(args, name)
	}
	rows, err := s.ReplayQueryContext(ctx, `
		WITH base AS (
			SELECT
				c.player_id,
				c.seconds_from_game_start AS t,
				c.id AS cmd_id
			FROM commands c
			JOIN players p
				ON p.id = c.player_id
			JOIN replays r
				ON r.id = c.replay_id
			WHERE
				c.replay_id = ?
				AND p.is_observer = 0
				AND lower(trim(coalesce(p.type, ''))) = 'human'
				AND c.action_type IN ('Train', 'Unit Morph')
				AND c.unit_type IS NOT NULL
				AND trim(c.unit_type) <> ''
				AND c.unit_type NOT IN (`+placeholders+`)
				AND c.seconds_from_game_start >= `+strconv.FormatInt(startSeconds, 10)+`
				AND c.seconds_from_game_start <= CAST(`+strconv.FormatFloat(endFraction, 'f', 4, 64)+` * r.duration_seconds AS INTEGER)
				AND CAST(`+strconv.FormatFloat(endFraction, 'f', 4, 64)+` * r.duration_seconds AS INTEGER) > `+strconv.FormatInt(startSeconds, 10)+`
		),
		ordered AS (
			SELECT
				player_id,
				t,
				cmd_id,
				LAG(t) OVER (PARTITION BY player_id ORDER BY t, cmd_id) AS prev_t
			FROM base
		),
		gaps AS (
			SELECT
				player_id,
				t,
				(t - prev_t) AS gap_s
			FROM ordered
		),
		replay_metrics AS (
			SELECT
				player_id,
				CAST(`+strconv.FormatFloat(endFraction, 'f', 4, 64)+` * ? AS INTEGER) - `+strconv.FormatInt(startSeconds, 10)+` AS window_s,
				COUNT(*) AS n_units,
				COUNT(gap_s) AS n_gaps,
				AVG(gap_s * 1.0) AS mean_gap_s,
				sqrt(AVG(gap_s * gap_s * 1.0) - AVG(gap_s * 1.0) * AVG(gap_s * 1.0)) AS std_gap_s,
				SUM(CASE WHEN gap_s >= `+strconv.FormatInt(idleGapSeconds, 10)+` THEN 1 ELSE 0 END) * 1.0 / NULLIF(COUNT(gap_s), 0) AS idle20_ratio
			FROM gaps
			GROUP BY player_id
			HAVING window_s > 0
		),
		scored AS (
			SELECT
				player_id,
				window_s,
				n_units,
				n_gaps,
				(n_units * 60.0) / window_s AS rate_per_min,
				(std_gap_s / NULLIF(mean_gap_s, 0)) AS cv_gap,
				(((std_gap_s / NULLIF(mean_gap_s, 0)) - 1.0) / ((std_gap_s / NULLIF(mean_gap_s, 0)) + 1.0)) AS burstiness,
				idle20_ratio,
				((n_units * 60.0) / window_s) / (1.0 + COALESCE((std_gap_s / NULLIF(mean_gap_s, 0)), 9999.0)) AS cadence_score
			FROM replay_metrics
		)
		SELECT
			player_id,
			window_s,
			n_units,
			n_gaps,
			rate_per_min,
			cv_gap,
			burstiness,
			idle20_ratio,
			cadence_score
		FROM scored
	`, append(args, durationSeconds)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GameUnitCadenceRow{}
	for rows.Next() {
		var row GameUnitCadenceRow
		if err := rows.Scan(
			&row.PlayerID, &row.WindowSeconds, &row.UnitsProduced, &row.GapCount,
			&row.RatePerMinute, &row.CVGap, &row.Burstiness, &row.Idle20Ratio, &row.CadenceScore,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

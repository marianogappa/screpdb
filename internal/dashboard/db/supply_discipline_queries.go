package db

import "context"

// SupplyProviderEventRow is one supply-providing command (depot/pylon/overlord/
// base) for a human player, with the context needed to compute supply-discipline
// (race + matchup + game duration). The weighted-gap metric is computed in Go
// (see supply_discipline.go) rather than SQL, because it needs a synthetic t=0
// race-seed event and a supply-cap window that don't express cleanly in SQL.
type SupplyProviderEventRow struct {
	ReplayID        int64
	PlayerID        int64
	PlayerKey       string
	PlayerName      string
	Race            string
	Matchup         string
	DurationSeconds int64
	ActionType      string
	UnitType        string
	Second          int64
}

const supplyProviderWhere = `
	p.is_observer = 0
	AND lower(trim(coalesce(p.type, ''))) = 'human'
	AND (
		(c.action_type = 'Build' AND c.unit_type IN ('Supply Depot','Pylon','Command Center','Nexus','Hatchery'))
		OR (c.action_type = 'Unit Morph' AND c.unit_type = 'Overlord')
	)`

// ListSupplyProviderEventsForReplay returns the supply-provider events for a
// single replay (per-game supply-discipline view).
func (s *Store) ListSupplyProviderEventsForReplay(ctx context.Context, replayID int64) ([]SupplyProviderEventRow, error) {
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT c.replay_id, c.player_id, lower(trim(p.name)) AS player_key, p.name,
		       p.race, r.matchup, r.duration_seconds, c.action_type, c.unit_type, c.seconds_from_game_start
		FROM commands c
		JOIN players p ON p.id = c.player_id
		JOIN replays r ON r.id = c.replay_id
		WHERE c.replay_id = ? AND `+supplyProviderWhere+`
		ORDER BY c.player_id, c.seconds_from_game_start, c.id`, replayID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSupplyProviderRows(rows)
}

// ListSupplyProviderEvents returns supply-provider events across the whole
// (filtered) corpus, ordered so events for one (replay, player) are contiguous.
func (s *Store) ListSupplyProviderEvents(ctx context.Context, onlyPlayerKey string) ([]SupplyProviderEventRow, error) {
	filter := ""
	args := []any{}
	if onlyPlayerKey != "" {
		filter = "AND lower(trim(p.name)) = ?"
		args = append(args, onlyPlayerKey)
	}
	rows, err := s.ReplayQueryContext(ctx, `
		SELECT c.replay_id, c.player_id, lower(trim(p.name)) AS player_key, p.name,
		       p.race, r.matchup, r.duration_seconds, c.action_type, c.unit_type, c.seconds_from_game_start
		FROM commands c
		JOIN players p ON p.id = c.player_id
		JOIN replays r ON r.id = c.replay_id
		WHERE `+supplyProviderWhere+`
		`+filter+`
		ORDER BY c.replay_id, c.player_id, c.seconds_from_game_start, c.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSupplyProviderRows(rows)
}

func scanSupplyProviderRows(rows *Rows) ([]SupplyProviderEventRow, error) {
	out := []SupplyProviderEventRow{}
	for rows.Next() {
		var r SupplyProviderEventRow
		if err := rows.Scan(&r.ReplayID, &r.PlayerID, &r.PlayerKey, &r.PlayerName, &r.Race,
			&r.Matchup, &r.DurationSeconds, &r.ActionType, &r.UnitType, &r.Second); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

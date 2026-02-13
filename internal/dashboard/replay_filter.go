package dashboard

import (
	"database/sql"
	"fmt"
)

func (d *Dashboard) validateReplayFilterSQL(replaysFilterSQL *string) (string, error) {
	if replaysFilterSQL == nil {
		return "", nil
	}
	normalized := normalizeSQL(*replaysFilterSQL)
	if normalized == "" {
		return "", nil
	}
	if !isSelectQuery(normalized) {
		return "", fmt.Errorf("replays_filter_sql must be a SELECT query")
	}
	qualified := qualifyReplayFilterSQL(normalized)
	if hasUnqualifiedReplays(qualified) {
		return "", fmt.Errorf("replays_filter_sql must reference main.replays when used in a view")
	}
	err := d.withFilteredConnection(nil, func(db *sql.DB) error {
		row := db.QueryRowContext(d.ctx, fmt.Sprintf("SELECT 1 FROM (%s) LIMIT 1", qualified))
		var tmp int
		if err := row.Scan(&tmp); err != nil && err != sql.ErrNoRows {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return qualified, nil
}

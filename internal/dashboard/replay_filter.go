package dashboard

import (
	"database/sql"
	"fmt"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
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
		return dashboarddb.ValidateSelectOnDB(d.ctx, db, qualified)
	})
	if err != nil {
		return "", err
	}
	return qualified, nil
}

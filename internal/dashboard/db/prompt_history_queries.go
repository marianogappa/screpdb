package db

import (
	"context"
	"database/sql"
	"time"
)

func UpsertPromptHistory(ctx context.Context, db *sql.DB, widgetID int64, promptHistory string, now time.Time) error {
	query := `
		INSERT INTO dashboard_widget_prompt_history (widget_id, prompt_history, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT (widget_id) DO UPDATE SET
			prompt_history = EXCLUDED.prompt_history,
			updated_at = EXCLUDED.updated_at
	`
	_, err := ExecContextOnDB(ctx, db, query, widgetID, promptHistory, now)
	return err
}

func GetPromptHistory(ctx context.Context, db *sql.DB, widgetID int64) (string, error) {
	var promptHistory string
	err := QueryRowContextOnDB(ctx, db, `SELECT prompt_history FROM dashboard_widget_prompt_history WHERE widget_id = ?`, widgetID).Scan(&promptHistory)
	return promptHistory, err
}

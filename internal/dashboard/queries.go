package dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type dashboardRow struct {
	URL              string
	Name             string
	Description      *string
	ReplaysFilterSQL *string
	CreatedAt        *time.Time
}

type dashboardWidgetRow struct {
	ID          int64
	DashboardID *string
	WidgetOrder *int64
	Name        string
	Description *string
	Config      string
	Query       string
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}

// getDashboardByURL retrieves a dashboard by URL
func (d *Dashboard) getDashboardByURL(ctx context.Context, url string) (*dashboardRow, error) {
	var dash dashboardRow
	var description sql.NullString
	var createdAt any

	query := `SELECT url, name, description, replays_filter_sql, created_at FROM dashboards WHERE url = ?`
	var replaysFilterSQL sql.NullString
	err := d.db.QueryRowContext(ctx, query, url).Scan(&dash.URL, &dash.Name, &description, &replaysFilterSQL, &createdAt)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}

	dash.Description = nullableString(description)
	dash.ReplaysFilterSQL = nullableString(replaysFilterSQL)
	var parseErr error
	dash.CreatedAt, parseErr = nullableTime(createdAt)
	if parseErr != nil {
		return nil, parseErr
	}
	return &dash, nil
}

// listDashboards retrieves all dashboards
func (d *Dashboard) listDashboards(ctx context.Context) ([]dashboardRow, error) {
	query := `SELECT url, name, description, replays_filter_sql, created_at FROM dashboards ORDER BY name ASC`
	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dashboards []dashboardRow
	for rows.Next() {
		var dash dashboardRow
		var description sql.NullString
		var replaysFilterSQL sql.NullString
		var createdAt any
		if err := rows.Scan(&dash.URL, &dash.Name, &description, &replaysFilterSQL, &createdAt); err != nil {
			return nil, err
		}
		dash.Description = nullableString(description)
		dash.ReplaysFilterSQL = nullableString(replaysFilterSQL)
		dash.CreatedAt, err = nullableTime(createdAt)
		if err != nil {
			return nil, err
		}
		dashboards = append(dashboards, dash)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if dashboards == nil {
		dashboards = []dashboardRow{}
	}
	return dashboards, nil
}

// createDashboard creates a new dashboard
func (d *Dashboard) createDashboard(ctx context.Context, url, name string, description *string, replaysFilterSQL *string) (*dashboardRow, error) {
	query := `INSERT INTO dashboards (url, name, description, replays_filter_sql) VALUES (?, ?, ?, ?)`
	if _, err := d.db.ExecContext(ctx, query, url, name, description, replaysFilterSQL); err != nil {
		return nil, err
	}
	return d.getDashboardByURL(ctx, url)
}

// updateDashboard updates an existing dashboard
func (d *Dashboard) updateDashboard(ctx context.Context, url, name string, description *string, replaysFilterSQL *string) error {
	updates := []string{"name = ?"}
	args := []any{name}

	if description != nil {
		updates = append(updates, "description = ?")
		args = append(args, *description)
	} else {
		updates = append(updates, "description = NULL")
	}

	if replaysFilterSQL != nil {
		if strings.TrimSpace(*replaysFilterSQL) == "" {
			updates = append(updates, "replays_filter_sql = NULL")
		} else {
			updates = append(updates, "replays_filter_sql = ?")
			args = append(args, *replaysFilterSQL)
		}
	}

	args = append(args, url)
	query := fmt.Sprintf(`UPDATE dashboards SET %s WHERE url = ?`, strings.Join(updates, ", "))
	_, err := d.db.ExecContext(ctx, query, args...)
	return err
}

// deleteDashboard deletes a dashboard and its widgets
func (d *Dashboard) deleteDashboard(ctx context.Context, url string) error {
	// Delete widgets first
	if _, err := d.db.ExecContext(ctx, `DELETE FROM dashboard_widgets WHERE dashboard_id = ?`, url); err != nil {
		return err
	}

	// Delete dashboard
	_, err := d.db.ExecContext(ctx, `DELETE FROM dashboards WHERE url = ?`, url)
	return err
}

// getDashboardWidgets retrieves all widgets for a dashboard
func (d *Dashboard) getDashboardWidgets(ctx context.Context, dashboardURL string) ([]dashboardWidgetRow, error) {
	query := `
		SELECT id, dashboard_id, widget_order, name, description, config, query, created_at, updated_at
		FROM dashboard_widgets
		WHERE dashboard_id = ?
		ORDER BY widget_order ASC
	`

	rows, err := d.db.QueryContext(ctx, query, dashboardURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var widgets []dashboardWidgetRow
	for rows.Next() {
		var widget dashboardWidgetRow
		var widgetOrder sql.NullInt64
		var dashboardID sql.NullString
		var description sql.NullString
		var createdAt, updatedAt any

		if err := rows.Scan(
			&widget.ID,
			&dashboardID,
			&widgetOrder,
			&widget.Name,
			&description,
			&widget.Config,
			&widget.Query,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}

		widget.DashboardID = nullableString(dashboardID)
		widget.WidgetOrder = nullableInt64(widgetOrder)
		widget.Description = nullableString(description)

		var err error
		widget.CreatedAt, err = nullableTime(createdAt)
		if err != nil {
			return nil, err
		}
		widget.UpdatedAt, err = nullableTime(updatedAt)
		if err != nil {
			return nil, err
		}

		widgets = append(widgets, widget)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	if widgets == nil {
		widgets = []dashboardWidgetRow{}
	}
	return widgets, nil
}

// getDashboardWidgetByID retrieves a widget by ID
func (d *Dashboard) getDashboardWidgetByID(ctx context.Context, widgetID int64) (*dashboardWidgetRow, error) {
	query := `
		SELECT id, dashboard_id, widget_order, name, description, config, query, created_at, updated_at
		FROM dashboard_widgets
		WHERE id = ?
	`

	var widget dashboardWidgetRow
	var widgetOrder sql.NullInt64
	var dashboardID sql.NullString
	var description sql.NullString
	var createdAt, updatedAt any

	err := d.db.QueryRowContext(ctx, query, widgetID).Scan(
		&widget.ID,
		&dashboardID,
		&widgetOrder,
		&widget.Name,
		&description,
		&widget.Config,
		&widget.Query,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}

	widget.DashboardID = nullableString(dashboardID)
	widget.WidgetOrder = nullableInt64(widgetOrder)
	widget.Description = nullableString(description)

	var parseErr error
	widget.CreatedAt, parseErr = nullableTime(createdAt)
	if parseErr != nil {
		return nil, parseErr
	}
	widget.UpdatedAt, parseErr = nullableTime(updatedAt)
	if parseErr != nil {
		return nil, parseErr
	}

	return &widget, nil
}

// createDashboardWidget creates a new widget
func (d *Dashboard) createDashboardWidget(ctx context.Context, dashboardURL string, widgetOrder int64, name string, description *string, config []byte, query string) (*dashboardWidgetRow, error) {
	sqlQuery := `
		INSERT INTO dashboard_widgets (dashboard_id, widget_order, name, description, config, query)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	res, err := d.db.ExecContext(ctx, sqlQuery, dashboardURL, widgetOrder, name, description, string(config), query)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return d.getDashboardWidgetByID(ctx, id)
}

// updateDashboardWidget updates an existing widget
func (d *Dashboard) updateDashboardWidget(ctx context.Context, widgetID int64, name string, description *string, config []byte, query string, widgetOrder *int64) error {
	updates := []string{"name = ?", "config = ?", "query = ?"}
	args := []any{name, string(config), query}

	if description != nil {
		updates = append(updates, "description = ?")
		args = append(args, *description)
	} else {
		updates = append(updates, "description = NULL")
	}

	if widgetOrder != nil {
		updates = append(updates, "widget_order = ?")
		args = append(args, *widgetOrder)
	}

	args = append(args, widgetID)
	sqlQuery := fmt.Sprintf(`
		UPDATE dashboard_widgets
		SET %s
		WHERE id = ?
	`, strings.Join(updates, ", "))

	_, err := d.db.ExecContext(ctx, sqlQuery, args...)
	return err
}

// deleteDashboardWidget deletes a widget
func (d *Dashboard) deleteDashboardWidget(ctx context.Context, widgetID int64) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM dashboard_widgets WHERE id = ?`, widgetID)
	return err
}

// getNextWidgetOrder gets the next widget order for a dashboard
func (d *Dashboard) getNextWidgetOrder(ctx context.Context, dashboardURL string) (int64, error) {
	query := "SELECT COALESCE(MAX(widget_order), 0) + 1 AS next_widget_order FROM dashboard_widgets WHERE dashboard_id = ?"
	var order int64 = 1
	err := d.db.QueryRowContext(ctx, query, dashboardURL).Scan(&order)
	if err != nil && err != sql.ErrNoRows {
		return 1, err
	}
	return order, nil
}

func nullableString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func nullableInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func nullableTime(v any) (*time.Time, error) {
	switch val := v.(type) {
	case nil:
		return nil, nil
	case time.Time:
		return &val, nil
	case []byte:
		return parseTimeString(string(val))
	case string:
		return parseTimeString(val)
	default:
		return nil, nil
	}
}

func parseTimeString(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("failed to parse timestamp: %s", value)
}

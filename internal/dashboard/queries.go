package dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/jackc/pgx/v5"
	jetmodel "github.com/marianogappa/screpdb/internal/jet/screpdb/public/model"
	"github.com/marianogappa/screpdb/internal/jet/screpdb/public/table"
)

// jsonbValue returns the JSON string for use in raw SQL queries
// We'll use raw SQL for JSONB operations since go-jet's type system doesn't handle JSONB casting well
func jsonbValue(data []byte) string {
	return string(data)
}

// getDashboardByURL retrieves a dashboard by URL
func (d *Dashboard) getDashboardByURL(ctx context.Context, url string) (*jetmodel.Dashboards, error) {
	var dash jetmodel.Dashboards
	stmt := table.Dashboards.SELECT(table.Dashboards.AllColumns).
		WHERE(table.Dashboards.URL.EQ(postgres.String(url)))
	err := stmt.QueryContext(ctx, d.db, &dash)
	if err == sql.ErrNoRows || err == pgx.ErrNoRows || err == qrm.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return &dash, nil
}

// listDashboards retrieves all dashboards
func (d *Dashboard) listDashboards(ctx context.Context) ([]jetmodel.Dashboards, error) {
	var dashboards []jetmodel.Dashboards
	stmt := table.Dashboards.SELECT(table.Dashboards.AllColumns).
		ORDER_BY(table.Dashboards.Name.ASC())
	err := stmt.QueryContext(ctx, d.db, &dashboards)
	if err != nil && err != sql.ErrNoRows && err != pgx.ErrNoRows && err != qrm.ErrNoRows {
		return nil, err
	}
	if dashboards == nil {
		dashboards = []jetmodel.Dashboards{}
	}
	return dashboards, nil
}

// createDashboard creates a new dashboard
func (d *Dashboard) createDashboard(ctx context.Context, url, name string, description *string) (*jetmodel.Dashboards, error) {
	var dash jetmodel.Dashboards
	insertStmt := table.Dashboards.INSERT(table.Dashboards.URL, table.Dashboards.Name, table.Dashboards.Description).
		VALUES(url, name, description).
		RETURNING(table.Dashboards.AllColumns)
	err := insertStmt.QueryContext(ctx, d.db, &dash)
	if err != nil {
		return nil, err
	}
	return &dash, nil
}

// updateDashboard updates an existing dashboard
func (d *Dashboard) updateDashboard(ctx context.Context, url, name string, description *string) error {
	updateStmt := table.Dashboards.UPDATE().
		SET(table.Dashboards.Name.SET(postgres.String(name)))
	if description != nil {
		updateStmt = updateStmt.SET(table.Dashboards.Description.SET(postgres.String(*description)))
	}
	updateStmt = updateStmt.WHERE(table.Dashboards.URL.EQ(postgres.String(url)))
	_, err := updateStmt.ExecContext(ctx, d.db)
	return err
}

// deleteDashboard deletes a dashboard and its widgets
func (d *Dashboard) deleteDashboard(ctx context.Context, url string) error {
	// Delete widgets first
	stmt := table.DashboardWidgets.DELETE().
		WHERE(table.DashboardWidgets.DashboardID.EQ(postgres.String(url)))
	_, err := stmt.ExecContext(ctx, d.db)
	if err != nil && err != sql.ErrNoRows && err != pgx.ErrNoRows {
		return err
	}

	// Delete dashboard
	stmt = table.Dashboards.DELETE().
		WHERE(table.Dashboards.URL.EQ(postgres.String(url)))
	_, err = stmt.ExecContext(ctx, d.db)
	return err
}

// getDashboardWidgets retrieves all widgets for a dashboard
func (d *Dashboard) getDashboardWidgets(ctx context.Context, dashboardURL string) ([]jetmodel.DashboardWidgets, error) {
	var widgets []jetmodel.DashboardWidgets
	stmt := table.DashboardWidgets.SELECT(table.DashboardWidgets.AllColumns).
		WHERE(table.DashboardWidgets.DashboardID.EQ(postgres.String(dashboardURL))).
		ORDER_BY(table.DashboardWidgets.WidgetOrder.ASC())
	err := stmt.QueryContext(ctx, d.db, &widgets)
	if err != nil && err != sql.ErrNoRows && err != pgx.ErrNoRows && err != qrm.ErrNoRows {
		return nil, err
	}
	if widgets == nil {
		widgets = []jetmodel.DashboardWidgets{}
	}
	return widgets, nil
}

// getDashboardWidgetByID retrieves a widget by ID
func (d *Dashboard) getDashboardWidgetByID(ctx context.Context, widgetID int64) (*jetmodel.DashboardWidgets, error) {
	var widget jetmodel.DashboardWidgets
	stmt := table.DashboardWidgets.SELECT(table.DashboardWidgets.AllColumns).
		WHERE(table.DashboardWidgets.ID.EQ(postgres.Int64(widgetID)))
	err := stmt.QueryContext(ctx, d.db, &widget)
	if err == sql.ErrNoRows || err == pgx.ErrNoRows || err == qrm.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return &widget, nil
}

// createDashboardWidget creates a new widget
func (d *Dashboard) createDashboardWidget(ctx context.Context, dashboardURL string, widgetOrder int64, name string, description *string, config []byte, query string) (*jetmodel.DashboardWidgets, error) {
	// Use raw SQL for JSONB insertion since go-jet doesn't handle JSONB casting well
	sqlQuery := `
		INSERT INTO dashboard_widgets (dashboard_id, widget_order, name, description, config, query)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)
		RETURNING id, dashboard_id, widget_order, name, description, config, query, created_at, updated_at
	`

	var widget jetmodel.DashboardWidgets
	err := d.db.QueryRowContext(ctx, sqlQuery, dashboardURL, widgetOrder, name, description, jsonbValue(config), query).
		Scan(&widget.ID, &widget.DashboardID, &widget.WidgetOrder, &widget.Name, &widget.Description, &widget.Config, &widget.Query, &widget.CreatedAt, &widget.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &widget, nil
}

// updateDashboardWidget updates an existing widget
func (d *Dashboard) updateDashboardWidget(ctx context.Context, widgetID int64, name string, description *string, config []byte, query string, widgetOrder *int64) error {
	// Use raw SQL for JSONB update since go-jet doesn't handle JSONB casting well
	// Build the UPDATE query dynamically based on which fields are provided
	updates := []string{"name = $2", "config = $3::jsonb", "query = $4"}
	args := []any{widgetID, name, jsonbValue(config), query}
	argPos := 5

	if description != nil {
		updates = append(updates, fmt.Sprintf("description = $%d", argPos))
		args = append(args, *description)
		argPos++
	} else {
		updates = append(updates, "description = NULL")
	}

	if widgetOrder != nil {
		updates = append(updates, fmt.Sprintf("widget_order = $%d", argPos))
		args = append(args, *widgetOrder)
		argPos++
	}

	sqlQuery := fmt.Sprintf(`
		UPDATE dashboard_widgets
		SET %s
		WHERE id = $1
	`, strings.Join(updates, ", "))

	_, err := d.db.ExecContext(ctx, sqlQuery, args...)
	return err
}

// deleteDashboardWidget deletes a widget
func (d *Dashboard) deleteDashboardWidget(ctx context.Context, widgetID int64) error {
	stmt := table.DashboardWidgets.DELETE().
		WHERE(table.DashboardWidgets.ID.EQ(postgres.Int64(widgetID)))
	_, err := stmt.ExecContext(ctx, d.db)
	return err
}

// getNextWidgetOrder gets the next widget order for a dashboard
func (d *Dashboard) getNextWidgetOrder(ctx context.Context, dashboardURL string) (int64, error) {
	query := "SELECT COALESCE(MAX(widget_order), 0) + 1 AS next_widget_order FROM dashboard_widgets WHERE dashboard_id = $1"
	var order int64 = 1
	err := d.db.QueryRowContext(ctx, query, dashboardURL).Scan(&order)
	if err != nil && err != sql.ErrNoRows {
		return 1, err
	}
	return order, nil
}

package dashboard

import (
	"context"
	"database/sql"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
)

type dashboardRow = dashboarddb.DashboardRow
type dashboardWidgetRow = dashboarddb.DashboardWidgetRow

// getDashboardByURL retrieves a dashboard by URL
func (d *Dashboard) getDashboardByURL(ctx context.Context, url string) (*dashboardRow, error) {
	dash, err := d.dbStore.GetDashboardByURL(ctx, url)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return dash, nil
}

// listDashboards retrieves all dashboards
func (d *Dashboard) listDashboards(ctx context.Context) ([]dashboardRow, error) {
	return d.dbStore.ListDashboards(ctx)
}

// createDashboard creates a new dashboard
func (d *Dashboard) createDashboard(ctx context.Context, url, name string, description *string, replaysFilterSQL *string) (*dashboardRow, error) {
	if err := d.dbStore.CreateDashboard(ctx, url, name, description, replaysFilterSQL); err != nil {
		return nil, err
	}
	return d.getDashboardByURL(ctx, url)
}

// updateDashboard updates an existing dashboard
func (d *Dashboard) updateDashboard(ctx context.Context, url, name string, description *string, replaysFilterSQL *string) error {
	return d.dbStore.UpdateDashboard(ctx, url, name, description, replaysFilterSQL)
}

// deleteDashboard deletes a dashboard and its widgets
func (d *Dashboard) deleteDashboard(ctx context.Context, url string) error {
	return d.dbStore.DeleteDashboard(ctx, url)
}

// getDashboardWidgets retrieves all widgets for a dashboard
func (d *Dashboard) getDashboardWidgets(ctx context.Context, dashboardURL string) ([]dashboardWidgetRow, error) {
	return d.dbStore.GetDashboardWidgets(ctx, dashboardURL)
}

// getDashboardWidgetByID retrieves a widget by ID
func (d *Dashboard) getDashboardWidgetByID(ctx context.Context, widgetID int64) (*dashboardWidgetRow, error) {
	widget, err := d.dbStore.GetDashboardWidgetByID(ctx, widgetID)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return widget, nil
}

// createDashboardWidget creates a new widget
func (d *Dashboard) createDashboardWidget(ctx context.Context, dashboardURL string, widgetOrder int64, name string, description *string, config []byte, query string) (*dashboardWidgetRow, error) {
	id, err := d.dbStore.CreateDashboardWidget(ctx, dashboardURL, widgetOrder, name, description, config, query)
	if err != nil {
		return nil, err
	}
	return d.getDashboardWidgetByID(ctx, id)
}

// updateDashboardWidget updates an existing widget
func (d *Dashboard) updateDashboardWidget(ctx context.Context, widgetID int64, name string, description *string, config []byte, query string, widgetOrder *int64) error {
	return d.dbStore.UpdateDashboardWidget(ctx, widgetID, name, description, config, query, widgetOrder)
}

// deleteDashboardWidget deletes a widget
func (d *Dashboard) deleteDashboardWidget(ctx context.Context, widgetID int64) error {
	return d.dbStore.DeleteDashboardWidget(ctx, widgetID)
}

// getNextWidgetOrder gets the next widget order for a dashboard
func (d *Dashboard) getNextWidgetOrder(ctx context.Context, dashboardURL string) (int64, error) {
	return d.dbStore.GetNextWidgetOrder(ctx, dashboardURL)
}

package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type DashboardRow struct {
	URL              string
	Name             string
	Description      *string
	ReplaysFilterSQL *string
	CreatedAt        *time.Time
}

type DashboardWidgetRow struct {
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

func (s *Store) GetDashboardByURL(ctx context.Context, url string) (*DashboardRow, error) {
	sqlcRow, err := sqlcgen.New(s.defaultDB).GetDashboardByURL(ctx, url)
	if err != nil {
		return nil, err
	}
	dash := DashboardRow{
		URL:              sqlcRow.Url,
		Name:             sqlcRow.Name,
		Description:      sqlcRow.Description,
		ReplaysFilterSQL: sqlcRow.ReplaysFilterSql,
	}
	createdAt, parseErr := parseTimeString(lo.FromPtr(sqlcRow.CreatedAt))
	if parseErr != nil {
		return nil, parseErr
	}
	dash.CreatedAt = createdAt
	return &dash, nil
}

func (s *Store) ListDashboards(ctx context.Context) ([]DashboardRow, error) {
	sqlcRows, err := sqlcgen.New(s.defaultDB).ListDashboards(ctx)
	if err != nil {
		return nil, err
	}
	dashboards := make([]DashboardRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		createdAt, err := parseTimeString(lo.FromPtr(row.CreatedAt))
		if err != nil {
			return nil, err
		}
		dashboards = append(dashboards, DashboardRow{
			URL:              row.Url,
			Name:             row.Name,
			Description:      row.Description,
			ReplaysFilterSQL: row.ReplaysFilterSql,
			CreatedAt:        createdAt,
		})
	}
	return dashboards, nil
}

func (s *Store) CreateDashboard(ctx context.Context, url, name string, description *string, replaysFilterSQL *string) error {
	return sqlcgen.New(s.defaultDB).CreateDashboard(ctx, sqlcgen.CreateDashboardParams{
		Url:              url,
		Name:             name,
		Description:      description,
		ReplaysFilterSql: replaysFilterSQL,
	})
}

func (s *Store) UpdateDashboard(ctx context.Context, url, name string, description *string, replaysFilterSQL *string) error {
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
	_, err := s.DefaultExecContext(ctx, query, args...)
	return err
}

func (s *Store) DeleteDashboard(ctx context.Context, url string) error {
	if err := sqlcgen.New(s.defaultDB).DeleteDashboardWidgetsByDashboardID(ctx, lo.ToPtr(url)); err != nil {
		return err
	}
	return sqlcgen.New(s.defaultDB).DeleteDashboardByURL(ctx, url)
}

func scanDashboardWidget(sqlcWidget sqlcgen.DashboardWidget) (*DashboardWidgetRow, error) {
	widget := DashboardWidgetRow{
		ID:          sqlcWidget.ID,
		DashboardID: sqlcWidget.DashboardID,
		WidgetOrder: sqlcWidget.WidgetOrder,
		Name:        sqlcWidget.Name,
		Description: sqlcWidget.Description,
		Config:      sqlcWidget.Config,
		Query:       sqlcWidget.Query,
	}
	createdAt, err := parseTimeString(lo.FromPtr(sqlcWidget.CreatedAt))
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseTimeString(lo.FromPtr(sqlcWidget.UpdatedAt))
	if err != nil {
		return nil, err
	}
	widget.CreatedAt = createdAt
	widget.UpdatedAt = updatedAt
	return &widget, nil
}

func (s *Store) GetDashboardWidgets(ctx context.Context, dashboardURL string) ([]DashboardWidgetRow, error) {
	sqlcWidgets, err := sqlcgen.New(s.defaultDB).GetDashboardWidgets(ctx, lo.ToPtr(dashboardURL))
	if err != nil {
		return nil, err
	}
	widgets := make([]DashboardWidgetRow, 0, len(sqlcWidgets))
	for _, sqlcWidget := range sqlcWidgets {
		widget, err := scanDashboardWidget(sqlcWidget)
		if err != nil {
			return nil, err
		}
		widgets = append(widgets, *widget)
	}
	return widgets, nil
}

func (s *Store) GetDashboardWidgetByID(ctx context.Context, widgetID int64) (*DashboardWidgetRow, error) {
	sqlcWidget, err := sqlcgen.New(s.defaultDB).GetDashboardWidgetByID(ctx, widgetID)
	if err != nil {
		return nil, err
	}
	return scanDashboardWidget(sqlcWidget)
}

func (s *Store) CreateDashboardWidget(ctx context.Context, dashboardURL string, widgetOrder int64, name string, description *string, config []byte, query string) (int64, error) {
	res, err := sqlcgen.New(s.defaultDB).CreateDashboardWidget(ctx, sqlcgen.CreateDashboardWidgetParams{
		DashboardID: lo.ToPtr(dashboardURL),
		WidgetOrder: lo.ToPtr(widgetOrder),
		Name:        name,
		Description: description,
		Config:      string(config),
		Query:       query,
	})
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateDashboardWidget(ctx context.Context, widgetID int64, name string, description *string, config []byte, query string, widgetOrder *int64) error {
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
	_, err := s.DefaultExecContext(ctx, sqlQuery, args...)
	return err
}

func (s *Store) DeleteDashboardWidget(ctx context.Context, widgetID int64) error {
	return sqlcgen.New(s.defaultDB).DeleteDashboardWidget(ctx, widgetID)
}

func (s *Store) GetNextWidgetOrder(ctx context.Context, dashboardURL string) (int64, error) {
	order, err := sqlcgen.New(s.defaultDB).GetNextWidgetOrder(ctx, lo.ToPtr(dashboardURL))
	if err != nil && err != sql.ErrNoRows {
		return 1, err
	}
	return order, nil
}

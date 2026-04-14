-- name: GetDashboardByURL :one
SELECT url, name, description, replays_filter_sql, created_at
FROM dashboards
WHERE url = ?;

-- name: ListDashboards :many
SELECT url, name, description, replays_filter_sql, created_at
FROM dashboards
ORDER BY name ASC;

-- name: CreateDashboard :exec
INSERT INTO dashboards (url, name, description, replays_filter_sql)
VALUES (?, ?, ?, ?);

-- name: DeleteDashboardWidgetsByDashboardID :exec
DELETE FROM dashboard_widgets
WHERE dashboard_id = ?;

-- name: DeleteDashboardByURL :exec
DELETE FROM dashboards
WHERE url = ?;

-- name: GetDashboardWidgets :many
SELECT id, dashboard_id, widget_order, name, description, config, query, created_at, updated_at
FROM dashboard_widgets
WHERE dashboard_id = ?
ORDER BY widget_order ASC;

-- name: GetDashboardWidgetByID :one
SELECT id, dashboard_id, widget_order, name, description, config, query, created_at, updated_at
FROM dashboard_widgets
WHERE id = ?;

-- name: CreateDashboardWidget :execresult
INSERT INTO dashboard_widgets (dashboard_id, widget_order, name, description, config, query)
VALUES (?, ?, ?, ?, ?, ?);

-- name: DeleteDashboardWidget :exec
DELETE FROM dashboard_widgets
WHERE id = ?;

-- name: GetNextWidgetOrder :one
SELECT COALESCE(MAX(widget_order), 0) + 1 AS next_widget_order
FROM dashboard_widgets
WHERE dashboard_id = ?;

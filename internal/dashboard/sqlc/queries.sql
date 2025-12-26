-- name: CreateDashboard :one
INSERT INTO dashboards (
  url, name, description
) VALUES (
  $1, $2, $3
) RETURNING *;

-- name: CreateDashboardWidget :one
INSERT INTO dashboard_widgets (
  dashboard_id, widget_order, name, description, content, query
) VALUES (
  $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetDashboard :one
SELECT * FROM dashboards
WHERE url = $1;

-- name: GetDashboardWidget :one
SELECT * FROM dashboard_widgets
WHERE id = $1;

-- name: GetDashboardWidgetNextWidgetOrder :one
SELECT COALESCE(MAX(widget_order), 0)+1 next_widget_order FROM dashboard_widgets
WHERE dashboard_id = $1;

-- name: ListDashboards :many
SELECT * FROM dashboards
ORDER BY name;

-- name: ListDashboardWidgets :many
SELECT * FROM dashboard_widgets
WHERE dashboard_id = $1
ORDER BY widget_order;

-- name: UpdateDashboard :exec
UPDATE dashboards
  set name = $2,
  description = $3
WHERE url = $1;

-- name: UpdateDashboardWidget :exec
UPDATE dashboard_widgets
  set name = $2,
  description = $3,
  content = $4,
  query = $5,
  widget_order = $6
WHERE id = $1;

-- name: DeleteDashboard :exec
DELETE FROM dashboards
WHERE url = $1;

-- name: DeleteDashboardWidget :exec
DELETE FROM dashboard_widgets
WHERE id = $1;

-- name: DeleteDashboardWidgetsOfDashboard :exec
DELETE FROM dashboard_widgets
WHERE dashboard_id = $1;

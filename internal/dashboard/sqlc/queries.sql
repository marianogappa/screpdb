-- name: CreateDashboard :one
INSERT INTO dashboards (
  name, description
) VALUES (
  $1, $2
) RETURNING *;

-- name: CreateDashboardWidget :one
INSERT INTO dashboard_widgets (
  dashboard_id, widget_order, name, description, content, query
) VALUES (
  $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: CreateDashboardWidgetPromptHistory :one
INSERT INTO dashboard_widgets_prompt_history (
  dashboard_id, dashboard_widget_id, prompt
) VALUES (
  $1, $2, $3
) RETURNING *;

-- name: GetDashboard :one
SELECT * FROM dashboards
WHERE id = $1;

-- name: GetDashboardWidget :one
SELECT * FROM dashboard_widgets
WHERE id = $1;

-- name: GetDashboardWidgetNextWidgetOrder :one
SELECT MAX(widget_order)+1 next_widget_order FROM dashboard_widgets
WHERE dashboard_id = $1;

-- name: ListDashboards :many
SELECT * FROM dashboards
ORDER BY name;

-- name: ListDashboardWidgets :many
SELECT * FROM dashboard_widgets
WHERE dashboard_id = $1
ORDER BY widget_order;

-- name: ListDashboardWidgetPromptHistory :many
SELECT * FROM dashboard_widgets_prompt_history
WHERE dashboard_id = $1 AND dashboard_widget_id = $2
ORDER BY id;

-- name: UpdateDashboard :exec
UPDATE dashboards
  set name = $2,
  description = $3
WHERE id = $1;

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
WHERE id = $1;

-- name: DeleteDashboardWidget :exec
DELETE FROM dashboards
WHERE id = $1;

-- name: DeleteDashboardWidgetPromptHistory :exec
DELETE FROM dashboard_widgets_prompt_history
WHERE dashboard_widget_id = $1;

-- name: DeleteDashboardWidgetPromptHistoriesOfDashboard :exec
DELETE FROM dashboard_widgets_prompt_history
WHERE dashboard_id = $1;

-- name: DeleteDashboardWidgetsOfDashboard :exec
DELETE FROM dashboard_widgets
WHERE dashboard_id = $1;

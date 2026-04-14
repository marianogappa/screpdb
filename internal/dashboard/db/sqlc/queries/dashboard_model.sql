-- name: GetDashboardWithVariablesByURL :one
SELECT url, name, description, replays_filter_sql, variables, created_at
FROM dashboards
WHERE url = ?;

-- name: UpdateDashboardVariables :exec
UPDATE dashboards
SET variables = ?
WHERE url = ?;

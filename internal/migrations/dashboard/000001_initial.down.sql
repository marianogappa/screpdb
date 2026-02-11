BEGIN;

-- Drop indexes first
DROP INDEX IF EXISTS idx_dashboard_widgets_dashboard_id;
DROP INDEX IF EXISTS idx_dashboard_widgets_dashboard_id_widget_order;
DROP INDEX IF EXISTS idx_dashboard_widget_prompt_history_widget_id;

-- Drop tables in reverse order of dependencies
DROP TABLE IF EXISTS dashboard_widgets;
DROP TABLE IF EXISTS dashboards;
DROP TABLE IF EXISTS dashboard_widget_prompt_history;

COMMIT;

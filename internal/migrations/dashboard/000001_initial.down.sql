BEGIN;

-- Drop indexes first
DROP INDEX IF EXISTS idx_dashboard_widgets_dashboard_id;
DROP INDEX IF EXISTS idx_dashboard_widgets_dashboard_id_widget_order;

-- Drop tables in reverse order of dependencies
DROP TABLE IF EXISTS dashboard_widgets CASCADE;
DROP TABLE IF EXISTS dashboards CASCADE;
DROP TABLE IF EXISTS dashboard_widget_prompt_history CASCADE;

COMMIT;

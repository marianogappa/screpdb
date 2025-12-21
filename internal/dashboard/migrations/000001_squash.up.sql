BEGIN;

CREATE TABLE dashboards (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT current_timestamp
);

CREATE TABLE dashboard_widgets (
    id BIGSERIAL PRIMARY KEY,
    dashboard_id BIGINT,
    widget_order BIGINT,
    name TEXT NOT NULL,
    description TEXT,
    content TEXT NOT NULL,
    query TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT current_timestamp,
    updated_at TIMESTAMP DEFAULT current_timestamp
);

CREATE UNIQUE INDEX idx_dashboard_widgets_dashboard_id_widget_order ON dashboard_widgets (dashboard_id, widget_order);
CREATE UNIQUE INDEX idx_dashboard_widgets_dashboard_id_name ON dashboard_widgets (dashboard_id, name);

CREATE TABLE dashboard_widgets_prompt_history (
    id BIGSERIAL PRIMARY KEY,
    dashboard_id BIGINT,
    dashboard_widget_id BIGINT,
    prompt TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT current_timestamp
);

CREATE INDEX idx_dashboard_widgets_dashboard_widget_id_id ON dashboard_widgets_prompt_history (dashboard_widget_id, id);

COMMIT;

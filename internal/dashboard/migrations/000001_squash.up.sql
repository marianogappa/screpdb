BEGIN;

CREATE TABLE dashboards (
    url TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT current_timestamp,
    CONSTRAINT url_safe_check CHECK (url ~ '^[a-zA-Z0-9_-]+$')
);

CREATE TABLE dashboard_widgets (
    id BIGSERIAL PRIMARY KEY,
    dashboard_id TEXT,
    widget_order BIGINT,
    name TEXT NOT NULL,
    description TEXT,
    content TEXT NOT NULL,
    query TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT current_timestamp,
    updated_at TIMESTAMP DEFAULT current_timestamp,
    FOREIGN KEY (dashboard_id) REFERENCES dashboards(url) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_dashboard_widgets_dashboard_id_widget_order ON dashboard_widgets (dashboard_id, widget_order);
CREATE UNIQUE INDEX idx_dashboard_widgets_dashboard_id_name ON dashboard_widgets (dashboard_id, name);

INSERT INTO dashboards (url, name, description) VALUES ('default', 'Default Dashboard', 'The default dashboard');

COMMIT;

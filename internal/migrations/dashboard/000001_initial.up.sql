BEGIN;

-- Dashboard tables
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
	config JSONB NOT NULL,
	query TEXT NOT NULL,
	created_at TIMESTAMP DEFAULT current_timestamp,
	updated_at TIMESTAMP DEFAULT current_timestamp,
	FOREIGN KEY (dashboard_id) REFERENCES dashboards(url) ON DELETE CASCADE
);

CREATE TABLE dashboard_widget_prompt_history (
    id BIGSERIAL PRIMARY KEY,
    widget_id bigint NOT NULL,
    prompt_history jsonb NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE UNIQUE INDEX idx_dashboard_widgets_dashboard_id_widget_order ON dashboard_widgets (dashboard_id, widget_order);
CREATE INDEX idx_dashboard_widgets_dashboard_id ON dashboard_widgets (dashboard_id);
CREATE UNIQUE INDEX idx_dashboard_widget_prompt_history_widget_id ON dashboard_widget_prompt_history(widget_id int8_ops);

-- Initial data
INSERT INTO dashboards (url, name, description) VALUES ('default', 'Default Dashboard', 'The default dashboard');

COMMIT;

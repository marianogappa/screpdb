BEGIN;

-- Dashboard tables
CREATE TABLE IF NOT EXISTS dashboards (
	url TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT,
	created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT url_safe_check CHECK (url <> '' AND url NOT GLOB '*[^A-Za-z0-9_-]*')
);

CREATE TABLE IF NOT EXISTS dashboard_widgets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	dashboard_id TEXT,
	widget_order BIGINT,
	name TEXT NOT NULL,
	description TEXT,
	config TEXT NOT NULL,
	query TEXT NOT NULL,
	created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (dashboard_id) REFERENCES dashboards(url) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS dashboard_widget_prompt_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    widget_id BIGINT NOT NULL,
    prompt_history TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_widgets_dashboard_id_widget_order ON dashboard_widgets (dashboard_id, widget_order);
CREATE INDEX IF NOT EXISTS idx_dashboard_widgets_dashboard_id ON dashboard_widgets (dashboard_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_widget_prompt_history_widget_id ON dashboard_widget_prompt_history(widget_id);

-- Initial data
INSERT OR IGNORE INTO dashboards (url, name, description) VALUES ('default', 'Default Dashboard', 'The default dashboard');

COMMIT;

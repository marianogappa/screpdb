BEGIN;

-- Single-row settings table holding the global replay filter and ingestion preferences.
-- The CHECK constraint enforces that only the well-known 'global' key may exist.
-- (game_type / included_maps / excluded_maps / included_players / excluded_players are
-- legacy columns retained for backward-compatible reads; the active filter lives in the
-- *_mode + maps/map_kinds/players/game_types JSON columns.)
CREATE TABLE IF NOT EXISTS settings (
	config_key TEXT PRIMARY KEY,
	game_type TEXT NOT NULL DEFAULT 'all',
	exclude_short_games BOOLEAN NOT NULL DEFAULT 1,
	exclude_computers BOOLEAN NOT NULL DEFAULT 1,
	included_maps TEXT NOT NULL DEFAULT '[]',
	excluded_maps TEXT NOT NULL DEFAULT '[]',
	included_players TEXT NOT NULL DEFAULT '[]',
	excluded_players TEXT NOT NULL DEFAULT '[]',
	ingest_input_dir TEXT NOT NULL DEFAULT '',
	game_types_mode TEXT NOT NULL DEFAULT 'only_these',
	game_types TEXT NOT NULL DEFAULT '[]',
	map_filter_mode TEXT NOT NULL DEFAULT 'only_these',
	maps TEXT NOT NULL DEFAULT '[]',
	map_kind_filter_mode TEXT NOT NULL DEFAULT 'only_these',
	map_kinds TEXT NOT NULL DEFAULT '["regular","money"]',
	player_filter_mode TEXT NOT NULL DEFAULT 'only_these',
	players TEXT NOT NULL DEFAULT '[]',
	compiled_replays_filter_sql TEXT,
	updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT settings_config_key_check CHECK (config_key = 'global')
);

INSERT OR IGNORE INTO settings (
	config_key,
	game_type,
	exclude_short_games,
	exclude_computers,
	included_maps,
	excluded_maps,
	included_players,
	excluded_players
) VALUES (
	'global',
	'all',
	1,
	1,
	'[]',
	'[]',
	'[]',
	'[]'
);

COMMIT;

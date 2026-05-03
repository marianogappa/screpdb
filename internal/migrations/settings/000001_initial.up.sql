BEGIN;

-- The "settings" migration set owns user-curated state that must survive
-- both --clean (drops replay tables) and --clean-dashboard (drops dashboard
-- tables). Currently: aliases + the global replay-filter / ingestion
-- preferences row.
--
-- These tables also exist in older migration sets (player_aliases lived in
-- replay; settings lived in dashboard). The CREATE TABLE IF NOT EXISTS
-- statements here are no-ops on existing DBs and create-from-scratch on
-- fresh ones. The drop-by-set logic in internal/migrations/migrate.go
-- explicitly preserves these table names when dropping the replay /
-- dashboard sets, so the data persists across erase operations.

CREATE TABLE IF NOT EXISTS player_aliases (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	canonical_alias TEXT NOT NULL,
	battle_tag_normalized TEXT NOT NULL,
	battle_tag_raw TEXT NOT NULL,
	aurora_id INTEGER,
	source TEXT NOT NULL CHECK (source IN ('imported', 'manual', 'you')),
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

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

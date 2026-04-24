BEGIN;

-- Drop the markers-migration artefacts. On a fresh DB where replay_events / indexes
-- haven't been created yet, the IF EXISTS guards keep this a no-op. Re-detection
-- repopulates the old tables if someone re-runs up migrations.
DROP INDEX IF EXISTS idx_replay_events_marker_unique;
DROP INDEX IF EXISTS idx_replay_events_kind;

DROP TABLE IF EXISTS replay_events;
DROP TABLE IF EXISTS marker_algorithm_state;

-- Rebuild the pre-migration replay_events shape empty so subsequent up-migrations
-- (if any were to run again) find a known starting point.
CREATE TABLE IF NOT EXISTS replay_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	replay_id INTEGER NOT NULL,
	seconds_from_game_start INTEGER NOT NULL,
	event_type TEXT NOT NULL CHECK (event_type IN (
		'player_start',
		'leave_game',
		'expansion',
		'attack',
		'scout',
		'drop',
		'reaver_drop',
		'dt_drop',
		'recall',
		'nuke',
		'cannon_rush',
		'bunker_rush',
		'zergling_rush',
		'proxy_gate',
		'proxy_rax',
		'proxy_factory',
		'location_inactive',
		'takeover',
		'became_terran',
		'became_zerg'
	)),
	location_base_type TEXT CHECK (location_base_type IN ('starting', 'natural', 'expansion')),
	location_base_oclock INTEGER CHECK (location_base_oclock IS NULL OR (location_base_oclock >= 0 AND location_base_oclock <= 12)),
	location_natural_of_oclock INTEGER CHECK (location_natural_of_oclock IS NULL OR (location_natural_of_oclock >= 0 AND location_natural_of_oclock <= 12)),
	location_mineral_only BOOLEAN,
	source_player_id INTEGER,
	target_player_id INTEGER,
	attack_unit_types TEXT,
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE,
	FOREIGN KEY (source_player_id) REFERENCES players(id) ON DELETE SET NULL,
	FOREIGN KEY (target_player_id) REFERENCES players(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_replay_events_replay_second ON replay_events(replay_id, seconds_from_game_start);
CREATE INDEX IF NOT EXISTS idx_replay_events_event_type_second ON replay_events(event_type, seconds_from_game_start);
CREATE INDEX IF NOT EXISTS idx_replay_events_event_location ON replay_events(event_type, location_base_type, location_base_oclock);
CREATE INDEX IF NOT EXISTS idx_replay_events_source_type ON replay_events(source_player_id, event_type);
CREATE INDEX IF NOT EXISTS idx_replay_events_target_type ON replay_events(target_player_id, event_type);

-- Restore the legacy marker tables so earlier up-migrations don't conflict if the
-- migration chain is re-run.
CREATE TABLE IF NOT EXISTS detected_patterns_replay (
	replay_id INTEGER NOT NULL,
	algorithm_version INTEGER NOT NULL,
	pattern_name TEXT NOT NULL,
	value_bool BOOLEAN,
	value_int INTEGER,
	value_string TEXT,
	value_timestamp BIGINT,
	PRIMARY KEY (replay_id, pattern_name),
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS detected_patterns_replay_player (
	replay_id INTEGER NOT NULL,
	player_id INTEGER NOT NULL,
	pattern_name TEXT NOT NULL,
	value_bool BOOLEAN,
	value_int INTEGER,
	value_string TEXT,
	value_timestamp BIGINT,
	PRIMARY KEY (replay_id, player_id, pattern_name),
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE,
	FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);

COMMIT;

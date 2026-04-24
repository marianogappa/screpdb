BEGIN;

CREATE TABLE IF NOT EXISTS replay_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	replay_id INTEGER NOT NULL,
	seconds_from_game_start INTEGER NOT NULL,
	event_type TEXT NOT NULL CHECK (event_type IN (
		'player_start',
		'leave',
		'expansion',
		'attack',
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
		'loss',
		'takeover',
		'became_terran',
		'became_zerg'
	)),
	description TEXT NOT NULL,
	location_base_type TEXT CHECK (location_base_type IN ('starting', 'natural', 'expansion')),
	-- Allow 0 for "center base" (scmapanalyzer's middle-of-map rich expansion).
	location_base_oclock INTEGER CHECK (location_base_oclock IS NULL OR (location_base_oclock >= 0 AND location_base_oclock <= 12)),
	location_natural_of_oclock INTEGER CHECK (location_natural_of_oclock IS NULL OR (location_natural_of_oclock >= 0 AND location_natural_of_oclock <= 12)),
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

DROP INDEX IF EXISTS idx_detected_patterns_replay_team_replay_id;
DROP TABLE IF EXISTS detected_patterns_replay_team;

COMMIT;

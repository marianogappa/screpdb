BEGIN;

-- Main replay tables
CREATE TABLE IF NOT EXISTS replays (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	file_path TEXT UNIQUE NOT NULL,
	file_checksum TEXT UNIQUE NOT NULL,
	file_name TEXT NOT NULL,
	created_at TEXT NOT NULL,
	replay_date TEXT NOT NULL,
	title TEXT,
	host TEXT,
	map_name TEXT NOT NULL,
	map_width INTEGER NOT NULL,
	map_height INTEGER NOT NULL,
	duration_seconds INTEGER NOT NULL,
	frame_count INTEGER NOT NULL,
	engine_version TEXT NOT NULL,
	engine TEXT NOT NULL,
	game_speed TEXT NOT NULL,
	game_type TEXT NOT NULL,
	home_team_size TEXT NOT NULL,
	avail_slots_count INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS players (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	replay_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	race TEXT NOT NULL,
	type TEXT NOT NULL,
	color TEXT NOT NULL,
	team INTEGER NOT NULL,
	is_observer BOOLEAN NOT NULL,
	apm INTEGER NOT NULL,
	eapm INTEGER NOT NULL, -- effective apm is apm excluding actions deemed ineffective
	is_winner BOOLEAN NOT NULL,
	start_location_x INTEGER,
	start_location_y INTEGER,
	start_location_oclock INTEGER,
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS commands (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	replay_id INTEGER NOT NULL,
	player_id INTEGER NOT NULL,
	frame INTEGER NOT NULL,
	seconds_from_game_start INTEGER NOT NULL,
	run_at TEXT NOT NULL,
	action_type TEXT NOT NULL,
	x INTEGER,
	y INTEGER,
	
	-- Common fields (used by multiple command types)
	is_queued BOOLEAN,
	order_name TEXT,
	
	-- Unit information (normalized fields)
	unit_type TEXT, -- Single unit type
	unit_types TEXT, -- JSON array of unit types for multiple units
	
	-- Tech command fields
	tech_name TEXT,
	
	-- Upgrade command fields
	upgrade_name TEXT,
	
	-- Hotkey command fields
	hotkey_type TEXT,
	hotkey_group INTEGER,
	
	-- Game Speed command fields
	game_speed TEXT,
	
	-- Vision command fields
	vision_player_ids TEXT, -- JSON array of player IDs

	-- Alliance command fields
	alliance_player_ids TEXT, -- JSON array of player IDs
	is_allied_victory BOOLEAN,
	
	-- General command fields (for unhandled commands)
	general_data TEXT, -- Hex string of raw data
	
	-- Chat and leave game fields
	chat_message TEXT,
	leave_reason TEXT,
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE,
	FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);


CREATE TABLE IF NOT EXISTS detected_patterns_replay (
	replay_id INTEGER NOT NULL,
	algorithm_version INTEGER NOT NULL,
	file_path TEXT NOT NULL,
	file_checksum TEXT NOT NULL,
	pattern_name TEXT NOT NULL,
	value_bool BOOLEAN,
	value_int INTEGER,
	value_string TEXT,
	value_timestamp BIGINT,
	PRIMARY KEY (replay_id, pattern_name),
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS detected_patterns_replay_team (
	replay_id INTEGER NOT NULL,
	team INTEGER NOT NULL,
	pattern_name TEXT NOT NULL,
	value_bool BOOLEAN,
	value_int INTEGER,
	value_string TEXT,
	value_timestamp BIGINT,
	PRIMARY KEY (replay_id, team, pattern_name),
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

-- Indexes
CREATE INDEX IF NOT EXISTS idx_replays_file_path ON replays(file_path);
CREATE INDEX IF NOT EXISTS idx_replays_file_checksum ON replays(file_checksum);
CREATE INDEX IF NOT EXISTS idx_replays_replay_date ON replays(replay_date);
CREATE INDEX IF NOT EXISTS idx_players_replay_id ON players(replay_id);
CREATE INDEX IF NOT EXISTS idx_commands_replay_id ON commands(replay_id);
CREATE INDEX IF NOT EXISTS idx_commands_player_id ON commands(player_id);
CREATE INDEX IF NOT EXISTS idx_commands_frame ON commands(frame);
CREATE INDEX IF NOT EXISTS idx_detected_patterns_replay_replay_id ON detected_patterns_replay(replay_id);
CREATE INDEX IF NOT EXISTS idx_detected_patterns_replay_team_replay_id ON detected_patterns_replay_team(replay_id);
CREATE INDEX IF NOT EXISTS idx_detected_patterns_replay_player_replay_id ON detected_patterns_replay_player(replay_id);
CREATE INDEX IF NOT EXISTS idx_detected_patterns_replay_player_player_id ON detected_patterns_replay_player(player_id);

COMMIT;

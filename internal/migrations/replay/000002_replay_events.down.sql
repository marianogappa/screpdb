BEGIN;

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

CREATE INDEX IF NOT EXISTS idx_detected_patterns_replay_team_replay_id ON detected_patterns_replay_team(replay_id);

DROP INDEX IF EXISTS idx_replay_events_replay_second;
DROP INDEX IF EXISTS idx_replay_events_event_type_second;
DROP INDEX IF EXISTS idx_replay_events_event_location;
DROP INDEX IF EXISTS idx_replay_events_source_type;
DROP INDEX IF EXISTS idx_replay_events_target_type;
DROP TABLE IF EXISTS replay_events;

COMMIT;

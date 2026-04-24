BEGIN;

-- Rebuild replay_events to add event_kind + payload columns.
-- SQLite requires full table swap for CHECK / column changes (same pattern as 000003, 000004).
-- event_kind distinguishes game-event rows from marker rows so they can share the table.
-- payload stores optional JSON for markers that need data beyond presence (hotkey groups, viewport stats).
-- The event_type CHECK is intentionally dropped: the Go-side allowlist plus the marker registry
-- becomes the source of truth, so adding/renaming a marker doesn't force a schema migration.
CREATE TABLE replay_events_v4 (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	replay_id INTEGER NOT NULL,
	seconds_from_game_start INTEGER NOT NULL,
	event_kind TEXT NOT NULL CHECK (event_kind IN ('game_event', 'marker')),
	event_type TEXT NOT NULL,
	location_base_type TEXT CHECK (location_base_type IN ('starting', 'natural', 'expansion')),
	location_base_oclock INTEGER CHECK (location_base_oclock IS NULL OR (location_base_oclock >= 0 AND location_base_oclock <= 12)),
	location_natural_of_oclock INTEGER CHECK (location_natural_of_oclock IS NULL OR (location_natural_of_oclock >= 0 AND location_natural_of_oclock <= 12)),
	location_mineral_only BOOLEAN,
	source_player_id INTEGER,
	target_player_id INTEGER,
	attack_unit_types TEXT,
	payload TEXT,
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE,
	FOREIGN KEY (source_player_id) REFERENCES players(id) ON DELETE CASCADE,
	FOREIGN KEY (target_player_id) REFERENCES players(id) ON DELETE SET NULL
);

INSERT INTO replay_events_v4 (
	id,
	replay_id,
	seconds_from_game_start,
	event_kind,
	event_type,
	location_base_type,
	location_base_oclock,
	location_natural_of_oclock,
	location_mineral_only,
	source_player_id,
	target_player_id,
	attack_unit_types,
	payload
)
SELECT
	id,
	replay_id,
	seconds_from_game_start,
	'game_event',
	event_type,
	location_base_type,
	location_base_oclock,
	location_natural_of_oclock,
	location_mineral_only,
	source_player_id,
	target_player_id,
	attack_unit_types,
	NULL
FROM replay_events;

DROP TABLE replay_events;
ALTER TABLE replay_events_v4 RENAME TO replay_events;

CREATE INDEX IF NOT EXISTS idx_replay_events_replay_second ON replay_events(replay_id, seconds_from_game_start);
CREATE INDEX IF NOT EXISTS idx_replay_events_event_type_second ON replay_events(event_type, seconds_from_game_start);
CREATE INDEX IF NOT EXISTS idx_replay_events_event_location ON replay_events(event_type, location_base_type, location_base_oclock);
CREATE INDEX IF NOT EXISTS idx_replay_events_source_type ON replay_events(source_player_id, event_type);
CREATE INDEX IF NOT EXISTS idx_replay_events_target_type ON replay_events(target_player_id, event_type);
CREATE INDEX IF NOT EXISTS idx_replay_events_kind ON replay_events(event_kind);

-- Partial unique index enforces one marker row per (replay, player_or_NULL, event_type).
-- COALESCE(source_player_id, 0) is safe: players.id AUTOINCREMENT starts at 1 so 0 cannot collide.
CREATE UNIQUE INDEX IF NOT EXISTS idx_replay_events_marker_unique
	ON replay_events(replay_id, COALESCE(source_player_id, 0), event_type)
	WHERE event_kind = 'marker';

-- Tracks marker detection algorithm version per replay, replaces the algorithm_version column
-- on the old detected_patterns_replay table.
CREATE TABLE IF NOT EXISTS marker_algorithm_state (
	replay_id INTEGER PRIMARY KEY,
	algorithm_version INTEGER NOT NULL,
	detected_at TEXT NOT NULL,
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE
);

-- Drop legacy marker tables. Algorithm-version bump in Go re-populates marker rows on next ingest.
DROP TABLE IF EXISTS detected_patterns_replay;
DROP TABLE IF EXISTS detected_patterns_replay_player;

COMMIT;

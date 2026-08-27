-- Per-(replay, player) scfingerprint feature vectors extracted at ingest.
-- vector is a little-endian float64 array (see internal/fpvec); vectors are
-- only comparable within a feature_version, so consumers must filter on it.
CREATE TABLE IF NOT EXISTS player_fingerprint_vectors (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	replay_id INTEGER NOT NULL,
	player_id INTEGER NOT NULL,
	feature_version INTEGER NOT NULL,
	model_tag TEXT NOT NULL,
	race TEXT NOT NULL,
	frames INTEGER NOT NULL,
	cmd_count INTEGER NOT NULL,
	vector BLOB NOT NULL,
	FOREIGN KEY (replay_id) REFERENCES replays(id) ON DELETE CASCADE,
	FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE,
	UNIQUE (player_id)
);

CREATE INDEX IF NOT EXISTS idx_player_fingerprint_vectors_replay_id
	ON player_fingerprint_vectors(replay_id);

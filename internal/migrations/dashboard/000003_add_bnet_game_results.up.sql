-- Rolling cache of the game_results entries seen in cached aurora profiles.
-- Battle.net only ever reports an account's last ~20 games, so every profile
-- fetch the app already makes appends what it saw here; over weeks that
-- accumulates enough history to describe when someone plays (weekday versus
-- weekend, time of day). Keyed on (aurora_id, game_id): one row per game per
-- account, whichever toon it was played on. No extra bridge requests are made
-- to fill it, and nothing here allows downloading a replay.
CREATE TABLE IF NOT EXISTS bnet_game_results (
	aurora_id INTEGER NOT NULL,
	game_id TEXT NOT NULL,
	create_time INTEGER NOT NULL,
	toon TEXT NOT NULL DEFAULT '',
	gateway INTEGER NOT NULL DEFAULT 0,
	race TEXT NOT NULL DEFAULT '',
	result TEXT NOT NULL DEFAULT '',
	apm INTEGER NOT NULL DEFAULT 0,
	duration_seconds INTEGER NOT NULL DEFAULT 0,
	map_name TEXT NOT NULL DEFAULT '',
	match_guid TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (aurora_id, game_id)
);
CREATE INDEX IF NOT EXISTS idx_bnet_game_results_aurora_time ON bnet_game_results (aurora_id, create_time DESC);

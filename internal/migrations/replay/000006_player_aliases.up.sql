BEGIN;

CREATE TABLE IF NOT EXISTS player_aliases (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	canonical_alias TEXT NOT NULL,
	battle_tag_normalized TEXT NOT NULL,
	battle_tag_raw TEXT NOT NULL,
	aurora_id INTEGER,
	source TEXT NOT NULL CHECK (source IN ('imported', 'manual', 'you')),
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_player_aliases_unique_source_tag_alias
	ON player_aliases(source, battle_tag_normalized, canonical_alias);

CREATE INDEX IF NOT EXISTS idx_player_aliases_tag
	ON player_aliases(battle_tag_normalized);

CREATE INDEX IF NOT EXISTS idx_player_aliases_tag_source_updated
	ON player_aliases(battle_tag_normalized, source, updated_at DESC);

COMMIT;

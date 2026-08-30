-- Cached SC:R bridge aurora-profile responses (issue #329), keyed on
-- (toon, gateway) — never on a player identity, per issue #344. payload is the
-- full UTF-8-normalized JSON; found=0 caches the bridge's "unknown toon"
-- response (HTTP 200 with aurora_id 0) so misses don't re-spend the daily
-- bridge budget. Freshness (24h TTL, per Blizzard's Cache-Control
-- max-age=86400) is enforced in code off fetched_at.
CREATE TABLE IF NOT EXISTS bnet_profiles (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	toon TEXT NOT NULL,
	gateway INTEGER NOT NULL,
	found BOOLEAN NOT NULL,
	aurora_id INTEGER NOT NULL DEFAULT 0,
	battle_tag TEXT NOT NULL DEFAULT '',
	country_code TEXT NOT NULL DEFAULT '',
	payload TEXT NOT NULL,
	fetched_at TEXT NOT NULL,
	UNIQUE (toon, gateway)
);

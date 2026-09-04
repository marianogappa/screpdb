-- The dashboard no longer uses this database: it reads the replay folder into
-- memory and keeps its settings and Battle.net caches in JSON files. These
-- tables were its own, and nothing reads them any more.
--
-- The settings and bnet_profiles tables are deliberately NOT dropped here: on
-- a database written by an older version they still hold the user's replay
-- folder and their rate-limited profile cache, which the dashboard imports
-- once on its first run. Dropping them from under an ingest run would lose
-- that. They are simply never created again.
DROP TABLE IF EXISTS player_aliases;

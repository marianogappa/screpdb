BEGIN;

-- Manual player aliasing has been removed. Identity is now resolved
-- automatically: fingerprinting links a person's accounts, and the "you"
-- players are derived from CSettings.json and held in memory, since they are a
-- pure function of that file and a stored copy could only go stale.
--
-- 000001 created this table and stays as it is, being history; this is the
-- forward migration that removes it. DROP TABLE IF EXISTS is a no-op on a DB
-- that never had it.

DROP TABLE IF EXISTS player_aliases;

COMMIT;

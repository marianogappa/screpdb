BEGIN;

-- SQLite has no DROP COLUMN; replay_events is rebuilt by earlier down migrations.
SELECT 1;

COMMIT;

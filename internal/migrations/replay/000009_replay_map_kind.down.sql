BEGIN;

-- Cannot ALTER TABLE on a fresh DB (DropMigrationSet runs all downs unconditionally).
-- The replays table is dropped by 000001 down; this migration's column goes with it.
SELECT 1;

COMMIT;

BEGIN;

ALTER TABLE replays ADD COLUMN team_stacking BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE replays ADD COLUMN team_info_incomplete BOOLEAN NOT NULL DEFAULT 0;

-- slot_id is needed to translate alliance commands (which reference slot IDs)
-- to player rows when reconstructing the alliance topology at query time.
ALTER TABLE players ADD COLUMN slot_id INTEGER NOT NULL DEFAULT 0;

COMMIT;

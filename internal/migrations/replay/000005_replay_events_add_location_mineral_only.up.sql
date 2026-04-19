BEGIN;

ALTER TABLE replay_events
ADD COLUMN location_mineral_only BOOLEAN;

COMMIT;

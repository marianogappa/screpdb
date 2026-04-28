BEGIN;

ALTER TABLE replays
ADD COLUMN map_kind TEXT NOT NULL DEFAULT 'Regular'
    CHECK (map_kind IN ('Regular', 'Money', 'UseMapSettings'));

COMMIT;

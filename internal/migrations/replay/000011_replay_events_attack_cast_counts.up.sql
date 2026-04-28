BEGIN;

-- attack_cast_counts is a JSON object keyed by canonical cast subject
-- (e.g. {"PsionicStorm":3,"Plague":1,"Recall":1}) that tallies aggressive
-- casts seen inside the attack pressure window. Populated only for
-- event_type='attack' rows; NULL elsewhere.
ALTER TABLE replay_events
ADD COLUMN attack_cast_counts TEXT;

COMMIT;

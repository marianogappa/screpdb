BEGIN;

DELETE FROM player_aliases
WHERE lower(trim(canonical_alias)) = lower(trim(battle_tag_normalized));

COMMIT;

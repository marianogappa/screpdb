BEGIN;

-- Drop indexes first
DROP INDEX IF EXISTS idx_detected_patterns_replay_player_player_id;
DROP INDEX IF EXISTS idx_detected_patterns_replay_player_replay_id;
DROP INDEX IF EXISTS idx_detected_patterns_replay_team_replay_id;
DROP INDEX IF EXISTS idx_detected_patterns_replay_replay_id;
DROP INDEX IF EXISTS idx_commands_frame;
DROP INDEX IF EXISTS idx_commands_player_id;
DROP INDEX IF EXISTS idx_commands_replay_id;
DROP INDEX IF EXISTS idx_players_replay_id;
DROP INDEX IF EXISTS idx_replays_replay_date;
DROP INDEX IF EXISTS idx_replays_file_checksum;
DROP INDEX IF EXISTS idx_replays_file_path;

-- Drop tables in reverse order of dependencies
DROP TABLE IF EXISTS detected_patterns_replay_player CASCADE;
DROP TABLE IF EXISTS detected_patterns_replay_team CASCADE;
DROP TABLE IF EXISTS detected_patterns_replay CASCADE;
DROP TABLE IF EXISTS commands CASCADE;
DROP TABLE IF EXISTS players CASCADE;
DROP TABLE IF EXISTS replays CASCADE;

COMMIT;

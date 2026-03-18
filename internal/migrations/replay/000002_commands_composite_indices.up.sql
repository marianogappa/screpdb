CREATE INDEX IF NOT EXISTS idx_commands_player_id_action_type ON commands(player_id, action_type);
CREATE INDEX IF NOT EXISTS idx_commands_replay_id_player_id_action_type ON commands(replay_id, player_id, action_type);

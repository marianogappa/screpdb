ALTER TABLE replays ADD COLUMN game_source TEXT NOT NULL DEFAULT 'Unknown'
  CHECK (game_source IN ('AssumedBattleNet','ShieldBattery','PreSCR','SinglePlayer','Unknown'));
ALTER TABLE replays ADD COLUMN lobby_kind TEXT NOT NULL DEFAULT 'Unknown'
  CHECK (lobby_kind IN ('Matchmaking','Custom','Unknown'));

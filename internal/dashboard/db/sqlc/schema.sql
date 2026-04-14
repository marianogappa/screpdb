CREATE TABLE settings (
  config_key TEXT PRIMARY KEY,
  game_type TEXT NOT NULL DEFAULT 'all',
  exclude_short_games BOOLEAN NOT NULL DEFAULT 1,
  exclude_computers BOOLEAN NOT NULL DEFAULT 1,
  included_maps TEXT NOT NULL DEFAULT '[]',
  excluded_maps TEXT NOT NULL DEFAULT '[]',
  included_players TEXT NOT NULL DEFAULT '[]',
  excluded_players TEXT NOT NULL DEFAULT '[]',
  ingest_input_dir TEXT NOT NULL DEFAULT '',
  game_types_mode TEXT NOT NULL DEFAULT 'only_these',
  game_types TEXT NOT NULL DEFAULT '[]',
  map_filter_mode TEXT NOT NULL DEFAULT 'only_these',
  maps TEXT NOT NULL DEFAULT '[]',
  player_filter_mode TEXT NOT NULL DEFAULT 'only_these',
  players TEXT NOT NULL DEFAULT '[]',
  compiled_replays_filter_sql TEXT,
  updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE replays (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  file_path TEXT UNIQUE NOT NULL,
  file_name TEXT NOT NULL,
  replay_date TEXT NOT NULL,
  map_name TEXT NOT NULL,
  duration_seconds INTEGER NOT NULL,
  game_type TEXT NOT NULL
);

CREATE TABLE players (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  replay_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  race TEXT NOT NULL,
  type TEXT NOT NULL,
  team INTEGER NOT NULL,
  is_observer BOOLEAN NOT NULL,
  apm INTEGER NOT NULL,
  eapm INTEGER NOT NULL,
  is_winner BOOLEAN NOT NULL
);

CREATE TABLE dashboards (
  url TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  replays_filter_sql TEXT,
  variables TEXT,
  created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE dashboard_widgets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  dashboard_id TEXT,
  widget_order BIGINT,
  name TEXT NOT NULL,
  description TEXT,
  config TEXT NOT NULL,
  query TEXT NOT NULL,
  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE detected_patterns_replay_player (
  replay_id INTEGER NOT NULL,
  player_id INTEGER NOT NULL,
  pattern_name TEXT NOT NULL,
  value_bool BOOLEAN,
  value_int INTEGER,
  value_string TEXT,
  value_timestamp BIGINT
);

CREATE TABLE detected_patterns_replay (
  replay_id INTEGER NOT NULL,
  pattern_name TEXT NOT NULL,
  value_bool BOOLEAN,
  value_int INTEGER,
  value_string TEXT,
  value_timestamp BIGINT
);

CREATE TABLE detected_patterns_replay_team (
  replay_id INTEGER NOT NULL,
  team INTEGER NOT NULL,
  pattern_name TEXT NOT NULL,
  value_bool BOOLEAN,
  value_int INTEGER,
  value_string TEXT,
  value_timestamp BIGINT
);

CREATE TABLE commands (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  replay_id INTEGER NOT NULL,
  player_id INTEGER NOT NULL,
  seconds_from_game_start INTEGER NOT NULL,
  action_type TEXT NOT NULL,
  is_queued BOOLEAN,
  unit_type TEXT,
  unit_types TEXT,
  tech_name TEXT,
  upgrade_name TEXT,
  hotkey_type TEXT,
  chat_message TEXT
);

CREATE TABLE commands_low_value (
  player_id INTEGER NOT NULL,
  action_type TEXT NOT NULL,
  hotkey_type TEXT
);

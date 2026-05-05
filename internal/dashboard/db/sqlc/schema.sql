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
  map_kind_filter_mode TEXT NOT NULL DEFAULT 'only_these',
  map_kinds TEXT NOT NULL DEFAULT '["regular","money"]',
  player_filter_mode TEXT NOT NULL DEFAULT 'only_these',
  players TEXT NOT NULL DEFAULT '[]',
  compiled_replays_filter_sql TEXT,
  updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE replays (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  file_path TEXT UNIQUE NOT NULL,
  file_checksum TEXT NOT NULL,
  file_name TEXT NOT NULL,
  replay_date TEXT NOT NULL,
  map_name TEXT NOT NULL,
  map_kind TEXT NOT NULL DEFAULT 'Regular',
  duration_seconds INTEGER NOT NULL,
  game_type TEXT NOT NULL,
  matchup TEXT NOT NULL DEFAULT '',
  team_stacking BOOLEAN NOT NULL DEFAULT 0,
  team_info_incomplete BOOLEAN NOT NULL DEFAULT 0,
  analyzer_algorithm_version INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE players (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  replay_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  color TEXT NOT NULL,
  race TEXT NOT NULL,
  type TEXT NOT NULL,
  team INTEGER NOT NULL,
  is_observer BOOLEAN NOT NULL,
  apm INTEGER NOT NULL,
  eapm INTEGER NOT NULL,
  is_winner BOOLEAN NOT NULL,
  start_location_oclock INTEGER,
  slot_id INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE player_aliases (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  canonical_alias TEXT NOT NULL,
  battle_tag_normalized TEXT NOT NULL,
  battle_tag_raw TEXT NOT NULL,
  aurora_id INTEGER,
  source TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE replay_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  replay_id INTEGER NOT NULL,
  seconds_from_game_start INTEGER NOT NULL,
  event_kind TEXT NOT NULL,
  event_type TEXT NOT NULL,
  location_base_type TEXT,
  location_base_oclock INTEGER,
  location_natural_of_oclock INTEGER,
  location_mineral_only BOOLEAN,
  source_player_id INTEGER,
  target_player_id INTEGER,
  attack_unit_types TEXT,
  attack_cast_counts TEXT,
  payload TEXT
);

CREATE TABLE commands (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  replay_id INTEGER NOT NULL,
  player_id INTEGER NOT NULL,
  frame INTEGER NOT NULL DEFAULT 0,
  seconds_from_game_start INTEGER NOT NULL,
  action_type TEXT NOT NULL,
  is_queued BOOLEAN,
  unit_type TEXT,
  unit_types TEXT,
  tech_name TEXT,
  upgrade_name TEXT,
  hotkey_type TEXT,
  chat_message TEXT,
  order_name TEXT
);

CREATE TABLE commands_low_value (
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
  chat_message TEXT,
  alliance_player_ids TEXT
);

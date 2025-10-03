package mcp

// GetDatabaseSchema returns a comprehensive description of the database schema
func GetDatabaseSchema() string {
	return `# StarCraft Replay Database Schema

This database contains structured data extracted from StarCraft: Brood War replay files (.rep). The data is organized into several related tables that capture all aspects of gameplay.

## Tables Overview

### 1. replays
Main table containing replay metadata and file information.

**Columns:**
- id (INTEGER, PRIMARY KEY): Unique identifier for each replay
- file_path (TEXT, UNIQUE): Full path to the replay file
- file_checksum (TEXT, UNIQUE): SHA256 checksum of the file for deduplication
- file_name (TEXT): Name of the replay file
- file_size (INTEGER): Size of the file in bytes
- created_at (DATETIME): When this record was created in the database
- replay_date (DATETIME): When the original game was played
- game_speed (INTEGER): Game speed setting (1-6)
- game_type (TEXT): Type of game (e.g., "Melee", "Use Map Settings")
- map_name (TEXT): Name of the map played
- map_width (INTEGER): Map width in pixels
- map_height (INTEGER): Map height in pixels
- duration (INTEGER): Game duration in seconds
- frame_count (INTEGER): Total number of game frames
- version (TEXT): StarCraft version (e.g., "1.16.1")
- build (INTEGER): Build number
- is_multiplayer (BOOLEAN): Whether this was a multiplayer game

### 2. players
Contains information about each player in the replay.

**Columns:**
- id (INTEGER, PRIMARY KEY): Unique identifier for each player record
- replay_id (INTEGER, FOREIGN KEY): References replays.id
- slot (INTEGER): Player slot number (0-7)
- name (TEXT): Player's name
- race (TEXT): Player's race ("Terran", "Protoss", "Zerg")
- type (TEXT): Player type ("Human", "Computer")
- color (INTEGER): Player's color (0-7)
- team (INTEGER): Team number
- is_winner (BOOLEAN): Whether this player won the game
- apm (INTEGER): Actions per minute
- spm (INTEGER): Supply per minute

### 3. actions
Contains all game actions/events that occurred during the replay.

**Columns:**
- id (INTEGER, PRIMARY KEY): Unique identifier for each action
- replay_id (INTEGER, FOREIGN KEY): References replays.id
- player_id (INTEGER, FOREIGN KEY): References players.id
- frame (INTEGER): Game frame when action occurred
- run_at (DATETIME): Calculated time when action occurred
- type (TEXT): Type of action (e.g., "Unit", "Building", "Upgrade")
- action (TEXT): Specific action (e.g., "Create", "Destroy", "Move", "Attack")
- unit_type (TEXT): Type of unit involved (if applicable)
- x (INTEGER): X coordinate where action occurred
- y (INTEGER): Y coordinate where action occurred

### 4. units
Contains information about all units that existed during the game.

**Columns:**
- id (INTEGER, PRIMARY KEY): Unique identifier for each unit record
- replay_id (INTEGER, FOREIGN KEY): References replays.id
- player_id (INTEGER, FOREIGN KEY): References players.id
- unit_id (INTEGER): Unique unit ID within the replay
- type (TEXT): Unit type (e.g., "Marine", "Zealot", "Zergling")
- name (TEXT): Unit name
- created_at (DATETIME): When the unit was created
- destroyed (DATETIME): When the unit was destroyed (NULL if still alive)
- x (INTEGER): Current X coordinate
- y (INTEGER): Current Y coordinate
- hp (INTEGER): Current hit points
- max_hp (INTEGER): Maximum hit points
- shield (INTEGER): Current shield points
- max_shield (INTEGER): Maximum shield points
- energy (INTEGER): Current energy
- max_energy (INTEGER): Maximum energy

### 5. buildings
Contains information about all buildings that existed during the game.

**Columns:**
- id (INTEGER, PRIMARY KEY): Unique identifier for each building record
- replay_id (INTEGER, FOREIGN KEY): References replays.id
- player_id (INTEGER, FOREIGN KEY): References players.id
- type (TEXT): Building type (e.g., "Command Center", "Nexus", "Hatchery")
- created_at (DATETIME): When the building was created
- destroyed (DATETIME): When the building was destroyed (NULL if still standing)
- x (INTEGER): Current X coordinate
- y (INTEGER): Current Y coordinate
- hp (INTEGER): Current hit points
- max_hp (INTEGER): Maximum hit points
- shield (INTEGER): Current shield points
- max_shield (INTEGER): Maximum shield points
- energy (INTEGER): Current energy
- max_energy (INTEGER): Maximum energy

## Common Query Examples

### Get all replays with player count
SELECT r.*, COUNT(p.id) as player_count 
FROM replays r 
LEFT JOIN players p ON r.id = p.replay_id 
GROUP BY r.id 
ORDER BY r.replay_date DESC;

### Get player statistics
SELECT 
    p.name,
    p.race,
    COUNT(DISTINCT r.id) as games_played,
    SUM(CASE WHEN p.is_winner THEN 1 ELSE 0 END) as wins,
    AVG(p.apm) as avg_apm,
    AVG(p.spm) as avg_spm
FROM players p
JOIN replays r ON p.replay_id = r.id
GROUP BY p.name, p.race
ORDER BY games_played DESC;

### Get unit creation statistics
SELECT 
    u.type,
    COUNT(*) as total_created,
    COUNT(CASE WHEN u.destroyed IS NOT NULL THEN 1 END) as destroyed,
    AVG(CASE WHEN u.destroyed IS NOT NULL 
        THEN (julianday(u.destroyed) - julianday(u.created_at)) * 24 * 60 * 60 
        END) as avg_lifetime_seconds
FROM units u
GROUP BY u.type
ORDER BY total_created DESC;

### Get action frequency by type
SELECT 
    a.type,
    a.action,
    COUNT(*) as frequency,
    COUNT(DISTINCT a.replay_id) as replays_with_action
FROM actions a
GROUP BY a.type, a.action
ORDER BY frequency DESC;

### Get recent games by race matchup
SELECT 
    r.replay_date,
    r.map_name,
    GROUP_CONCAT(p.name || ' (' || p.race || ')') as players,
    r.duration,
    r.is_multiplayer
FROM replays r
JOIN players p ON r.id = p.replay_id
WHERE r.replay_date >= datetime('now', '-30 days')
GROUP BY r.id
ORDER BY r.replay_date DESC
LIMIT 20;

## Indexes

The database includes several indexes for optimal query performance:
- idx_replays_file_path: On replays.file_path
- idx_replays_file_checksum: On replays.file_checksum  
- idx_replays_replay_date: On replays.replay_date
- idx_players_replay_id: On players.replay_id
- idx_actions_replay_id: On actions.replay_id
- idx_actions_player_id: On actions.player_id
- idx_actions_frame: On actions.frame
- idx_units_replay_id: On units.replay_id
- idx_units_player_id: On units.player_id
- idx_buildings_replay_id: On buildings.replay_id
- idx_buildings_player_id: On buildings.player_id

## Notes

- All timestamps are stored in UTC
- Frame numbers correspond to StarCraft's internal frame counter (~24 FPS)
- Coordinates are in StarCraft's internal coordinate system
- The database is designed to be idempotent - running the ingest command multiple times will not create duplicates
- File deduplication is based on both file path and SHA256 checksum`
}

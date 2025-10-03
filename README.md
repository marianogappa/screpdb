# ScrepDB - StarCraft Replay Database

A comprehensive CLI tool for ingesting StarCraft: Brood War replay files into a database and providing MCP (Model Context Protocol) server functionality for querying replay data.

## Features

- **Replay Ingestion**: Parse and store StarCraft: Brood War replay files (.rep) into SQLite or PostgreSQL databases
- **MCP Server**: Provide SQL query capabilities through a Model Context Protocol server
- **Concurrent Processing**: Efficiently process multiple replay files with configurable concurrency
- **File Watching**: Monitor directories for new replay files and automatically ingest them
- **Deduplication**: Prevent duplicate processing using file checksums
- **Flexible Storage**: Support for both SQLite and PostgreSQL backends

## Installation

### Prerequisites

- Go 1.24.5 or later
- StarCraft: Brood War replay files (.rep)

### Build from Source

```bash
git clone https://github.com/marianogappa/screpdb.git
cd screpdb
go build -o screpdb
```

## Usage

### Ingesting Replay Files

The `ingest` command processes StarCraft replay files and stores them in a database:

```bash
# Basic usage - process all .rep files in default directory
./screpdb ingest

# Specify input directory and output database
./screpdb ingest -i /path/to/replays -o my_replays.db

# Use PostgreSQL instead of SQLite
./screpdb ingest -p "host=localhost port=5432 user=myuser dbname=screpdb sslmode=disable"

# Watch for new files and process them automatically
./screpdb ingest -w

# Limit processing to recent files
./screpdb ingest -m 6  # Last 6 months
./screpdb ingest -d 2024-01-01  # Up to specific date

# Control concurrency
./screpdb ingest -c 8  # Use 8 concurrent workers
```

#### Ingest Command Options

- `-i, --input-dir`: Input directory containing replay files (default: system replay directory)
- `-o, --sqlite-output-file`: Output SQLite database file (default: screp.db)
- `-p, --postgres-connection-string`: PostgreSQL connection string
- `-w, --watch`: Watch for new files and ingest them as they appear
- `-n, --stop-after-n-reps`: Stop after processing N replay files (0 = no limit)
- `-d, --up-to-yyyy-mm-dd`: Only process files up to this date (YYYY-MM-DD format)
- `-m, --up-to-n-months`: Only process files from the last N months (0 = no limit)
- `-c, --max-concurrency`: Maximum number of concurrent goroutines (default: 4)

### MCP Server

The `mcp` command starts a Model Context Protocol server that provides SQL query capabilities for the SQLite database:

```bash
# Start MCP server with default SQLite database
./screpdb mcp

# Specify custom database file
./screpdb mcp -i /path/to/custom.db
```

#### MCP Command Options

- `-i, --sqlite-input-file`: Input SQLite database file (default: screp.db)

## MCP Server Implementation

The MCP server is built using the `github.com/mark3labs/mcp-go` library and provides the following capabilities:

### Available Tools

#### query_database

Execute SQL queries against the StarCraft replay SQLite database. The database contains tables: replays (metadata), players (player info), actions (game events), units (unit data), buildings (building data). Use this tool to analyze replay statistics, player performance, unit usage, and game patterns.

**Parameters:**
- `sql` (string, required): SQL query to execute against the StarCraft replay SQLite database

**Example Usage:**
```json
{
  "name": "query_database",
  "arguments": {
    "sql": "SELECT COUNT(*) FROM replays WHERE is_multiplayer = 1"
  }
}
```

#### get_schema

Get detailed information about the StarCraft replay SQLite database schema including table structures, relationships, and example queries.

**Parameters:**
- None

**Example Usage:**
```json
{
  "name": "get_schema",
  "arguments": {}
}
```

### SQLite Database Schema

The SQLite database contains comprehensive information about StarCraft replays organized into the following tables:

#### replays
Main table containing replay metadata and file information.

**Columns:**
- `id` (INTEGER, PRIMARY KEY): Unique identifier for each replay
- `file_path` (TEXT, UNIQUE): Full path to the replay file
- `file_checksum` (TEXT, UNIQUE): SHA256 checksum of the file for deduplication
- `file_name` (TEXT): Name of the replay file
- `file_size` (INTEGER): Size of the file in bytes
- `created_at` (DATETIME): When this record was created in the database
- `replay_date` (DATETIME): When the original game was played
- `game_speed` (INTEGER): Game speed setting (1-6)
- `game_type` (TEXT): Type of game (e.g., "Melee", "Use Map Settings")
- `map_name` (TEXT): Name of the map played
- `map_width` (INTEGER): Map width in pixels
- `map_height` (INTEGER): Map height in pixels
- `duration` (INTEGER): Game duration in seconds
- `frame_count` (INTEGER): Total number of game frames
- `version` (TEXT): StarCraft version (e.g., "1.16.1")
- `build` (INTEGER): Build number
- `is_multiplayer` (BOOLEAN): Whether this was a multiplayer game

#### players
Contains information about each player in the replay.

**Columns:**
- `id` (INTEGER, PRIMARY KEY): Unique identifier for each player record
- `replay_id` (INTEGER, FOREIGN KEY): References replays.id
- `slot` (INTEGER): Player slot number (0-7)
- `name` (TEXT): Player's name
- `race` (TEXT): Player's race ("Terran", "Protoss", "Zerg")
- `type` (TEXT): Player type ("Human", "Computer")
- `color` (INTEGER): Player's color (0-7)
- `team` (INTEGER): Team number
- `is_winner` (BOOLEAN): Whether this player won the game
- `apm` (INTEGER): Actions per minute
- `spm` (INTEGER): Supply per minute

#### actions
Contains all game actions/events that occurred during the replay.

**Columns:**
- `id` (INTEGER, PRIMARY KEY): Unique identifier for each action
- `replay_id` (INTEGER, FOREIGN KEY): References replays.id
- `player_id` (INTEGER, FOREIGN KEY): References players.id
- `frame` (INTEGER): Game frame when action occurred
- `time` (DATETIME): Calculated time when action occurred
- `type` (TEXT): Type of action (e.g., "Unit", "Building", "Upgrade")
- `action` (TEXT): Specific action (e.g., "Create", "Destroy", "Move", "Attack")
- `unit_type` (TEXT): Type of unit involved (if applicable)
- `x` (INTEGER): X coordinate where action occurred
- `y` (INTEGER): Y coordinate where action occurred

#### units
Contains information about all units that existed during the game.

**Columns:**
- `id` (INTEGER, PRIMARY KEY): Unique identifier for each unit record
- `replay_id` (INTEGER, FOREIGN KEY): References replays.id
- `player_id` (INTEGER, FOREIGN KEY): References players.id
- `unit_id` (INTEGER): Unique unit ID within the replay
- `type` (TEXT): Unit type (e.g., "Marine", "Zealot", "Zergling")
- `name` (TEXT): Unit name
- `created` (DATETIME): When the unit was created
- `destroyed` (DATETIME): When the unit was destroyed (NULL if still alive)
- `x` (INTEGER): Current X coordinate
- `y` (INTEGER): Current Y coordinate
- `hp` (INTEGER): Current hit points
- `max_hp` (INTEGER): Maximum hit points
- `shield` (INTEGER): Current shield points
- `max_shield` (INTEGER): Maximum shield points
- `energy` (INTEGER): Current energy
- `max_energy` (INTEGER): Maximum energy

#### buildings
Contains information about all buildings that existed during the game.

**Columns:**
- `id` (INTEGER, PRIMARY KEY): Unique identifier for each building record
- `replay_id` (INTEGER, FOREIGN KEY): References replays.id
- `player_id` (INTEGER, FOREIGN KEY): References players.id
- `building_id` (INTEGER): Unique building ID within the replay
- `type` (TEXT): Building type (e.g., "Command Center", "Nexus", "Hatchery")
- `name` (TEXT): Building name
- `created` (DATETIME): When the building was created
- `destroyed` (DATETIME): When the building was destroyed (NULL if still standing)
- `x` (INTEGER): Current X coordinate
- `y` (INTEGER): Current Y coordinate
- `hp` (INTEGER): Current hit points
- `max_hp` (INTEGER): Maximum hit points
- `shield` (INTEGER): Current shield points
- `max_shield` (INTEGER): Maximum shield points
- `energy` (INTEGER): Current energy
- `max_energy` (INTEGER): Maximum energy

### Common Query Examples

#### Get all replays with player count
```sql
SELECT r.*, COUNT(p.id) as player_count 
FROM replays r 
LEFT JOIN players p ON r.id = p.replay_id 
GROUP BY r.id 
ORDER BY r.replay_date DESC;
```

#### Get player statistics
```sql
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
```

#### Get unit creation statistics
```sql
SELECT 
    u.type,
    COUNT(*) as total_created,
    COUNT(CASE WHEN u.destroyed IS NOT NULL THEN 1 END) as destroyed,
    AVG(CASE WHEN u.destroyed IS NOT NULL 
        THEN (julianday(u.destroyed) - julianday(u.created)) * 24 * 60 * 60 
        END) as avg_lifetime_seconds
FROM units u
GROUP BY u.type
ORDER BY total_created DESC;
```

#### Get action frequency by type
```sql
SELECT 
    a.type,
    a.action,
    COUNT(*) as frequency,
    COUNT(DISTINCT a.replay_id) as replays_with_action
FROM actions a
GROUP BY a.type, a.action
ORDER BY frequency DESC;
```

#### Get recent games by race matchup
```sql
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
```

## Architecture

### MCP Implementation

The MCP server is implemented using the `github.com/mark3labs/mcp-go` library, which provides:

- **Standardized Protocol**: Full compliance with the Model Context Protocol specification
- **Tool Registration**: Easy registration of SQL query tools with proper schema validation
- **Error Handling**: Comprehensive error handling and reporting
- **Transport Layer**: Built-in stdio transport for seamless integration with MCP clients

### Key Components

1. **Storage Interface**: Abstracted storage layer supporting both SQLite and PostgreSQL
2. **Parser**: StarCraft replay file parser using the `github.com/icza/screp` library
3. **File Operations**: File discovery, watching, and deduplication utilities
4. **MCP Server**: Model Context Protocol server for querying capabilities

## Development

### Project Structure

```
screpdb/
├── cmd/                    # CLI commands
│   ├── ingest.go         # Replay ingestion command
│   └── mcp.go            # MCP server command
├── internal/
│   ├── fileops/          # File operations and watching
│   ├── mcp/              # MCP server implementation
│   ├── models/           # Data models
│   ├── parser/           # Replay parsing logic
│   ├── screp/            # StarCraft replay utilities
│   └── storage/          # Database storage implementations
├── main.go               # Application entry point
└── go.mod               # Go module definition
```

### Building

```bash
# Build the application
go build -o screpdb

# Run tests
go test ./...

# Format code
go fmt ./...
```

## License

This project is licensed under the MIT License. See the LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgments

- Built using the `github.com/icza/screp` library for StarCraft replay parsing
- MCP server implementation powered by `github.com/mark3labs/mcp-go`
- Database schema designed for comprehensive replay analysis

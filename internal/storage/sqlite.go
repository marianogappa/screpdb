package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements Storage interface using SQLite
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &SQLiteStorage{db: db}, nil
}

// Helper function to serialize slot IDs to JSON
func (s *SQLiteStorage) serializeSlotIDs(slotIDs *[]int) (string, error) {
	if slotIDs == nil {
		return "", nil
	}
	data, err := json.Marshal(*slotIDs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Initialize creates the database schema
// If clean is true, drops all existing tables before creating new ones
func (s *SQLiteStorage) Initialize(ctx context.Context, clean bool) error {
	if clean {
		// Drop all tables in the correct order to handle foreign key constraints
		dropTables := `
			DROP TABLE IF EXISTS chat_messages;
			DROP TABLE IF EXISTS leave_games;
			DROP TABLE IF EXISTS placed_units;
			DROP TABLE IF EXISTS start_locations;
			DROP TABLE IF EXISTS resources;
			DROP TABLE IF EXISTS buildings;
			DROP TABLE IF EXISTS units;
			DROP TABLE IF EXISTS commands;
			DROP TABLE IF EXISTS players;
			DROP TABLE IF EXISTS replays;
		`
		if _, err := s.db.ExecContext(ctx, dropTables); err != nil {
			return fmt.Errorf("failed to drop tables: %w", err)
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS replays (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_path TEXT UNIQUE NOT NULL,
		file_checksum TEXT UNIQUE NOT NULL,
		file_name TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		replay_date DATETIME NOT NULL,
		title TEXT,
		host TEXT,
		map_name TEXT NOT NULL,
		map_width INTEGER NOT NULL,
		map_height INTEGER NOT NULL,
		duration INTEGER NOT NULL,
		frame_count INTEGER NOT NULL,
		version TEXT NOT NULL,
		engine TEXT NOT NULL,
		speed TEXT NOT NULL,
		game_type TEXT NOT NULL,
		sub_type TEXT NOT NULL,
		avail_slots_count INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS players (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		slot_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		race TEXT NOT NULL,
		type TEXT NOT NULL,
		color TEXT NOT NULL,
		team INTEGER NOT NULL,
		observer BOOLEAN NOT NULL,
		apm INTEGER NOT NULL,
		spm INTEGER NOT NULL,
		is_winner BOOLEAN NOT NULL,
		start_location_x INTEGER,
		start_location_y INTEGER,
		FOREIGN KEY (replay_id) REFERENCES replays(id),
		UNIQUE(replay_id, slot_id)
	);

	CREATE TABLE IF NOT EXISTS commands (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		frame INTEGER NOT NULL,
		time DATETIME NOT NULL,
		action_type TEXT NOT NULL,
		action_id INTEGER NOT NULL,
		unit_id INTEGER,
		target_id INTEGER NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		effective BOOLEAN NOT NULL,
		
		-- Common fields (used by multiple command types)
		queued BOOLEAN,
		order_id INTEGER,
		order_name TEXT,
		
		-- Unit information (normalized fields)
		unit_type TEXT, -- Single unit type
		unit_player_id INTEGER, -- Single unit player ID
		unit_types TEXT, -- JSON array of unit types for multiple units
		unit_ids TEXT, -- JSON array of unit IDs for multiple units
		
		-- Select command fields (legacy)
		select_unit_tags TEXT, -- JSON array of unit tags
		select_unit_types TEXT, -- JSON map of unit tag -> unit type
		
		-- Build command fields
		build_unit_name TEXT,
		
		-- Train command fields
		train_unit_name TEXT,
		
		-- Building Morph command fields
		building_morph_unit_name TEXT,
		
		-- Tech command fields
		tech_name TEXT,
		
		-- Upgrade command fields
		upgrade_name TEXT,
		
		-- Hotkey command fields
		hotkey_type TEXT,
		hotkey_group INTEGER,
		
		-- Game Speed command fields
		game_speed TEXT,
		
		-- Chat command fields
		chat_sender_slot_id INTEGER,
		chat_message TEXT,
		
		-- Vision command fields
		vision_slot_ids TEXT, -- JSON array of slot IDs
		
		-- Alliance command fields
		alliance_slot_ids TEXT, -- JSON array of slot IDs
		allied_victory BOOLEAN,
		
		-- Leave Game command fields
		leave_reason TEXT,
		
		-- Minimap Ping command fields
		minimap_ping_x INTEGER,
		minimap_ping_y INTEGER,
		
		-- General command fields (for unhandled commands)
		general_data TEXT, -- Hex string of raw data
		
		FOREIGN KEY (replay_id) REFERENCES replays(id),
		FOREIGN KEY (player_id) REFERENCES players(id)
	);

	CREATE TABLE IF NOT EXISTS units (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		unit_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		created DATETIME NOT NULL,
		created_frame INTEGER NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		FOREIGN KEY (replay_id) REFERENCES replays(id),
		FOREIGN KEY (player_id) REFERENCES players(id)
	);

	CREATE TABLE IF NOT EXISTS buildings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		building_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		created DATETIME NOT NULL,
		created_frame INTEGER NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		FOREIGN KEY (replay_id) REFERENCES replays(id),
		FOREIGN KEY (player_id) REFERENCES players(id)
	);

	CREATE TABLE IF NOT EXISTS resources (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		amount INTEGER NOT NULL,
		FOREIGN KEY (replay_id) REFERENCES replays(id)
	);

	CREATE TABLE IF NOT EXISTS start_locations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		FOREIGN KEY (replay_id) REFERENCES replays(id)
	);

	CREATE TABLE IF NOT EXISTS placed_units (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		FOREIGN KEY (replay_id) REFERENCES replays(id),
		FOREIGN KEY (player_id) REFERENCES players(id)
	);

	CREATE TABLE IF NOT EXISTS chat_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		sender_slot_id INTEGER NOT NULL,
		message TEXT NOT NULL,
		frame INTEGER NOT NULL,
		time DATETIME NOT NULL,
		FOREIGN KEY (replay_id) REFERENCES replays(id),
		FOREIGN KEY (player_id) REFERENCES players(id)
	);

	CREATE TABLE IF NOT EXISTS leave_games (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		reason TEXT,
		frame INTEGER NOT NULL,
		time DATETIME NOT NULL,
		FOREIGN KEY (replay_id) REFERENCES replays(id),
		FOREIGN KEY (player_id) REFERENCES players(id)
	);

	CREATE INDEX IF NOT EXISTS idx_replays_file_path ON replays(file_path);
	CREATE INDEX IF NOT EXISTS idx_replays_file_checksum ON replays(file_checksum);
	CREATE INDEX IF NOT EXISTS idx_replays_replay_date ON replays(replay_date);
	CREATE INDEX IF NOT EXISTS idx_players_replay_id ON players(replay_id);
	CREATE INDEX IF NOT EXISTS idx_commands_replay_id ON commands(replay_id);
	CREATE INDEX IF NOT EXISTS idx_commands_player_id ON commands(player_id);
	CREATE INDEX IF NOT EXISTS idx_commands_frame ON commands(frame);
	CREATE INDEX IF NOT EXISTS idx_units_replay_id ON units(replay_id);
	CREATE INDEX IF NOT EXISTS idx_buildings_replay_id ON buildings(replay_id);
	CREATE INDEX IF NOT EXISTS idx_resources_replay_id ON resources(replay_id);
	CREATE INDEX IF NOT EXISTS idx_start_locations_replay_id ON start_locations(replay_id);
	CREATE INDEX IF NOT EXISTS idx_placed_units_replay_id ON placed_units(replay_id);
	CREATE INDEX IF NOT EXISTS idx_placed_units_player_id ON placed_units(player_id);
	CREATE INDEX IF NOT EXISTS idx_chat_messages_replay_id ON chat_messages(replay_id);
	CREATE INDEX IF NOT EXISTS idx_chat_messages_player_id ON chat_messages(player_id);
	CREATE INDEX IF NOT EXISTS idx_chat_messages_frame ON chat_messages(frame);
	CREATE INDEX IF NOT EXISTS idx_leave_games_replay_id ON leave_games(replay_id);
	CREATE INDEX IF NOT EXISTS idx_leave_games_player_id ON leave_games(player_id);
	CREATE INDEX IF NOT EXISTS idx_leave_games_frame ON leave_games(frame);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// StoreReplay stores a complete replay data structure
func (s *SQLiteStorage) StoreReplay(ctx context.Context, data *models.ReplayData) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert replay
	replayQuery := `
		INSERT INTO replays (
			file_path, file_checksum, file_name, file_size, created_at, replay_date,
			title, host, map_name, map_width, map_height, duration,
			frame_count, version, engine, speed, game_type, sub_type, avail_slots_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := tx.ExecContext(ctx, replayQuery,
		data.Replay.FilePath, data.Replay.FileChecksum, data.Replay.FileName,
		data.Replay.FileSize, data.Replay.CreatedAt, data.Replay.ReplayDate,
		data.Replay.Title, data.Replay.Host, data.Replay.MapName,
		data.Replay.MapWidth, data.Replay.MapHeight, data.Replay.Duration,
		data.Replay.FrameCount, data.Replay.Version, data.Replay.Engine,
		data.Replay.Speed, data.Replay.GameType, data.Replay.SubType,
		data.Replay.AvailSlotsCount,
	)
	if err != nil {
		return fmt.Errorf("failed to insert replay: %w", err)
	}

	replayID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get replay ID: %w", err)
	}

	// Insert players
	playerQuery := `
		INSERT INTO players (
			replay_id, slot_id, player_id, name, race, type, color, team, observer, apm, spm, is_winner, start_location_x, start_location_y
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	playerIDs := make(map[byte]int64) // player_id -> database_id
	for _, player := range data.Players {
		player.ReplayID = replayID
		result, err := tx.ExecContext(ctx, playerQuery,
			player.ReplayID, player.SlotID, player.PlayerID, player.Name, player.Race, player.Type,
			player.Color, player.Team, player.Observer, player.APM, player.SPM, player.IsWinner,
			player.StartLocationX, player.StartLocationY,
		)
		if err != nil {
			return fmt.Errorf("failed to insert player: %w", err)
		}

		playerID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get player ID: %w", err)
		}
		playerIDs[player.PlayerID] = playerID
	}

	// Insert commands
	commandQuery := `
		INSERT INTO commands (
			replay_id, player_id, frame, time, action_type, action_id, unit_id, target_id, x, y, effective,
			queued, order_id, order_name, unit_type, unit_player_id, unit_types, unit_ids, select_unit_tags, select_unit_types, build_unit_name,
			train_unit_name, building_morph_unit_name, tech_name, upgrade_name, hotkey_type, hotkey_group, game_speed,
			chat_sender_slot_id, chat_message, vision_slot_ids, alliance_slot_ids, allied_victory,
			leave_reason, minimap_ping_x, minimap_ping_y, general_data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	for _, command := range data.Commands {
		command.ReplayID = replayID
		playerID, exists := playerIDs[byte(command.PlayerID)]
		if !exists {
			continue // Skip commands for players not found
		}
		command.PlayerID = playerID

		// Serialize slot IDs to JSON
		visionSlotIDsJSON, err := s.serializeSlotIDs(command.VisionSlotIDs)
		if err != nil {
			return fmt.Errorf("failed to serialize vision slot IDs: %w", err)
		}
		allianceSlotIDsJSON, err := s.serializeSlotIDs(command.AllianceSlotIDs)
		if err != nil {
			return fmt.Errorf("failed to serialize alliance slot IDs: %w", err)
		}

		// Serialize unit information to JSON
		unitTypesJSON, err := s.serializeString(command.UnitTypes)
		if err != nil {
			return fmt.Errorf("failed to serialize unit types: %w", err)
		}
		unitIDsJSON, err := s.serializeString(command.UnitIDs)
		if err != nil {
			return fmt.Errorf("failed to serialize unit IDs: %w", err)
		}
		selectUnitTagsJSON, err := s.serializeString(command.SelectUnitTags)
		if err != nil {
			return fmt.Errorf("failed to serialize select unit tags: %w", err)
		}
		selectUnitTypesJSON, err := s.serializeString(command.SelectUnitTypes)
		if err != nil {
			return fmt.Errorf("failed to serialize select unit types: %w", err)
		}

		_, err = tx.ExecContext(ctx, commandQuery,
			command.ReplayID, command.PlayerID, command.Frame, command.Time,
			command.ActionType, command.ActionID, command.UnitID, command.TargetID,
			command.X, command.Y, command.Effective,
			command.Queued, command.OrderID, command.OrderName,
			command.UnitType, command.UnitPlayerID, unitTypesJSON, unitIDsJSON, selectUnitTagsJSON, selectUnitTypesJSON, command.BuildUnitName,
			command.TrainUnitName, command.BuildingMorphUnitName, command.TechName, command.UpgradeName,
			command.HotkeyType, command.HotkeyGroup, command.GameSpeed,
			command.ChatSenderSlotID, command.ChatMessage, visionSlotIDsJSON,
			allianceSlotIDsJSON, command.AlliedVictory, command.LeaveReason,
			command.MinimapPingX, command.MinimapPingY, command.GeneralData,
		)
		if err != nil {
			return fmt.Errorf("failed to insert command: %w", err)
		}
	}

	// Insert units
	unitQuery := `
		INSERT INTO units (
			replay_id, player_id, unit_id, type, name, created, created_frame, x, y
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	for _, unit := range data.Units {
		unit.ReplayID = replayID
		// Find the player ID from the unit's PlayerID
		playerID, exists := playerIDs[byte(unit.PlayerID)]
		if !exists {
			continue // Skip units for players not found
		}
		unit.PlayerID = playerID

		_, err := tx.ExecContext(ctx, unitQuery,
			unit.ReplayID, unit.PlayerID, unit.UnitID, unit.Type, unit.Name,
			unit.Created, unit.CreatedFrame, unit.X, unit.Y,
		)
		if err != nil {
			return fmt.Errorf("failed to insert unit: %w", err)
		}
	}

	// Insert buildings
	buildingQuery := `
		INSERT INTO buildings (
			replay_id, player_id, building_id, type, name, created, created_frame, x, y
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	for _, building := range data.Buildings {
		building.ReplayID = replayID
		// Find the player ID from the building's PlayerID
		playerID, exists := playerIDs[byte(building.PlayerID)]
		if !exists {
			continue // Skip buildings for players not found
		}
		building.PlayerID = playerID

		_, err := tx.ExecContext(ctx, buildingQuery,
			building.ReplayID, building.PlayerID, building.BuildingID, building.Type,
			building.Name, building.Created, building.CreatedFrame, building.X, building.Y,
		)
		if err != nil {
			return fmt.Errorf("failed to insert building: %w", err)
		}
	}

	// Insert resources
	resourceQuery := `
		INSERT INTO resources (
			replay_id, type, x, y, amount
		) VALUES (?, ?, ?, ?, ?)
	`

	for _, resource := range data.Resources {
		resource.ReplayID = replayID

		_, err := tx.ExecContext(ctx, resourceQuery,
			resource.ReplayID, resource.Type, resource.X, resource.Y, resource.Amount,
		)
		if err != nil {
			return fmt.Errorf("failed to insert resource: %w", err)
		}
	}

	// Insert start locations
	startLocationQuery := `
		INSERT INTO start_locations (
			replay_id, x, y
		) VALUES (?, ?, ?)
	`

	for _, startLoc := range data.StartLocations {
		startLoc.ReplayID = replayID

		_, err := tx.ExecContext(ctx, startLocationQuery,
			startLoc.ReplayID, startLoc.X, startLoc.Y,
		)
		if err != nil {
			return fmt.Errorf("failed to insert start location: %w", err)
		}
	}

	// Insert placed units
	placedUnitQuery := `
		INSERT INTO placed_units (
			replay_id, player_id, type, name, x, y
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	for _, placedUnit := range data.PlacedUnits {
		placedUnit.ReplayID = replayID
		playerID, exists := playerIDs[byte(placedUnit.PlayerID)]
		if !exists {
			continue
		}
		placedUnit.PlayerID = playerID

		_, err := tx.ExecContext(ctx, placedUnitQuery,
			placedUnit.ReplayID, placedUnit.PlayerID, placedUnit.Type, placedUnit.Name,
			placedUnit.X, placedUnit.Y,
		)
		if err != nil {
			return fmt.Errorf("failed to insert placed unit: %w", err)
		}
	}

	// Insert chat messages
	chatMessageQuery := `
		INSERT INTO chat_messages (
			replay_id, player_id, sender_slot_id, message, frame, time
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	for _, chatMsg := range data.ChatMessages {
		chatMsg.ReplayID = replayID
		playerID, exists := playerIDs[byte(chatMsg.PlayerID)]
		if !exists {
			continue
		}
		chatMsg.PlayerID = playerID

		_, err := tx.ExecContext(ctx, chatMessageQuery,
			chatMsg.ReplayID, chatMsg.PlayerID, chatMsg.SenderSlotID, chatMsg.Message,
			chatMsg.Frame, chatMsg.Time,
		)
		if err != nil {
			return fmt.Errorf("failed to insert chat message: %w", err)
		}
	}

	// Insert leave games
	leaveGameQuery := `
		INSERT INTO leave_games (
			replay_id, player_id, reason, frame, time
		) VALUES (?, ?, ?, ?, ?)
	`

	for _, leaveGame := range data.LeaveGames {
		leaveGame.ReplayID = replayID
		playerID, exists := playerIDs[byte(leaveGame.PlayerID)]
		if !exists {
			continue
		}
		leaveGame.PlayerID = playerID

		_, err := tx.ExecContext(ctx, leaveGameQuery,
			leaveGame.ReplayID, leaveGame.PlayerID, leaveGame.Reason,
			leaveGame.Frame, leaveGame.Time,
		)
		if err != nil {
			return fmt.Errorf("failed to insert leave game: %w", err)
		}
	}

	return tx.Commit()
}

// ReplayExists checks if a replay already exists by file path or checksum
func (s *SQLiteStorage) ReplayExists(ctx context.Context, filePath, checksum string) (bool, error) {
	query := `SELECT 1 FROM replays WHERE file_path = ? OR file_checksum = ? LIMIT 1`
	var exists int
	err := s.db.QueryRowContext(ctx, query, filePath, checksum).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Query executes a SQL query and returns results
func (s *SQLiteStorage) Query(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

// serializeString serializes a string pointer to JSON, returning NULL for nil pointers
func (s *SQLiteStorage) serializeString(str *string) (interface{}, error) {
	if str == nil {
		return nil, nil
	}
	return *str, nil
}

// StorageName returns the storage backend name
func (s *SQLiteStorage) StorageName() string {
	return StorageSQLite
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

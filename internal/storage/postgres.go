package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/marianogappa/screpdb/internal/models"
)

// PostgresStorage implements Storage interface using PostgreSQL
type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage creates a new PostgreSQL storage instance
func NewPostgresStorage(connectionString string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresStorage{db: db}, nil
}

// Initialize creates the database schema
// If clean is true, drops all existing tables before creating new ones
func (s *PostgresStorage) Initialize(ctx context.Context, clean bool) error {
	if clean {
		// Drop all tables in the correct order to handle foreign key constraints
		dropTables := `
			DROP TABLE IF EXISTS chat_messages CASCADE;
			DROP TABLE IF EXISTS leave_games CASCADE;
			DROP TABLE IF EXISTS placed_units CASCADE;
			DROP TABLE IF EXISTS start_locations CASCADE;
			DROP TABLE IF EXISTS resources CASCADE;
			DROP TABLE IF EXISTS buildings CASCADE;
			DROP TABLE IF EXISTS units CASCADE;
			DROP TABLE IF EXISTS commands CASCADE;
			DROP TABLE IF EXISTS players CASCADE;
			DROP TABLE IF EXISTS replays CASCADE;
		`
		if _, err := s.db.ExecContext(ctx, dropTables); err != nil {
			return fmt.Errorf("failed to drop tables: %w", err)
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS replays (
		id SERIAL PRIMARY KEY,
		file_path TEXT UNIQUE NOT NULL,
		file_checksum TEXT UNIQUE NOT NULL,
		file_name TEXT NOT NULL,
		file_size BIGINT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL,
		replay_date TIMESTAMP WITH TIME ZONE NOT NULL,
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
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL REFERENCES replays(id) ON DELETE CASCADE,
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
		UNIQUE(replay_id, slot_id)
	);

	CREATE TABLE IF NOT EXISTS commands (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL REFERENCES replays(id) ON DELETE CASCADE,
		player_id INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
		frame INTEGER NOT NULL,
		time TIMESTAMP WITH TIME ZONE NOT NULL,
		action_type TEXT NOT NULL,
		action_id INTEGER NOT NULL,
		unit_id INTEGER NOT NULL,
		target_id INTEGER NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		data TEXT,
		effective BOOLEAN NOT NULL,
		
		-- Common fields (used by multiple command types)
		queued BOOLEAN,
		unit_tag INTEGER,
		order_id INTEGER,
		order_name TEXT,
		
		-- Select command fields
		select_unit_tags TEXT, -- JSON array of unit tags
		
		-- Build command fields
		build_unit_name TEXT,
		
		-- Right Click command fields
		right_click_unit_tag INTEGER,
		right_click_unit_name TEXT,
		
		-- Targeted Order command fields
		targeted_order_unit_tag INTEGER,
		targeted_order_unit_name TEXT,
		
		-- Train command fields
		train_unit_name TEXT,
		
		-- Cancel Train command fields
		cancel_train_unit_tag INTEGER,
		
		-- Unload command fields
		unload_unit_tag INTEGER,
		
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
		general_data TEXT -- Hex string of raw data
	);

	CREATE TABLE IF NOT EXISTS units (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL REFERENCES replays(id) ON DELETE CASCADE,
		player_id INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
		unit_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		created TIMESTAMP WITH TIME ZONE NOT NULL,
		destroyed TIMESTAMP WITH TIME ZONE,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		hp INTEGER NOT NULL,
		max_hp INTEGER NOT NULL,
		shield INTEGER NOT NULL,
		max_shield INTEGER NOT NULL,
		energy INTEGER NOT NULL,
		max_energy INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS buildings (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL REFERENCES replays(id) ON DELETE CASCADE,
		player_id INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
		building_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		created TIMESTAMP WITH TIME ZONE NOT NULL,
		destroyed TIMESTAMP WITH TIME ZONE,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		hp INTEGER NOT NULL,
		max_hp INTEGER NOT NULL,
		shield INTEGER NOT NULL,
		max_shield INTEGER NOT NULL,
		energy INTEGER NOT NULL,
		max_energy INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS resources (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL REFERENCES replays(id) ON DELETE CASCADE,
		type TEXT NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		amount INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS start_locations (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL REFERENCES replays(id) ON DELETE CASCADE,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS placed_units (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL REFERENCES replays(id) ON DELETE CASCADE,
		player_id INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		hp INTEGER NOT NULL,
		max_hp INTEGER NOT NULL,
		shield INTEGER NOT NULL,
		max_shield INTEGER NOT NULL,
		energy INTEGER NOT NULL,
		max_energy INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS chat_messages (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL REFERENCES replays(id) ON DELETE CASCADE,
		player_id INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
		sender_slot_id INTEGER NOT NULL,
		message TEXT NOT NULL,
		frame INTEGER NOT NULL,
		time TIMESTAMP WITH TIME ZONE NOT NULL
	);

	CREATE TABLE IF NOT EXISTS leave_games (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL REFERENCES replays(id) ON DELETE CASCADE,
		player_id INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
		reason TEXT,
		frame INTEGER NOT NULL,
		time TIMESTAMP WITH TIME ZONE NOT NULL
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
func (s *PostgresStorage) StoreReplay(ctx context.Context, data *models.ReplayData) error {
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
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id
	`

	var replayID int64
	err = tx.QueryRowContext(ctx, replayQuery,
		data.Replay.FilePath, data.Replay.FileChecksum, data.Replay.FileName,
		data.Replay.FileSize, data.Replay.CreatedAt, data.Replay.ReplayDate,
		data.Replay.Title, data.Replay.Host, data.Replay.MapName,
		data.Replay.MapWidth, data.Replay.MapHeight, data.Replay.Duration,
		data.Replay.FrameCount, data.Replay.Version, data.Replay.Engine,
		data.Replay.Speed, data.Replay.GameType, data.Replay.SubType,
		data.Replay.AvailSlotsCount,
	).Scan(&replayID)
	if err != nil {
		return fmt.Errorf("failed to insert replay: %w", err)
	}

	// Insert players
	playerQuery := `
		INSERT INTO players (
			replay_id, slot_id, player_id, name, race, type, color, team, observer, apm, spm, is_winner, start_location_x, start_location_y
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id
	`

	playerIDs := make(map[byte]int64) // player_id -> database_id
	for _, player := range data.Players {
		player.ReplayID = replayID
		var playerID int64
		err := tx.QueryRowContext(ctx, playerQuery,
			player.ReplayID, player.SlotID, player.PlayerID, player.Name, player.Race, player.Type,
			player.Color, player.Team, player.Observer, player.APM, player.SPM, player.IsWinner,
			player.StartLocationX, player.StartLocationY,
		).Scan(&playerID)
		if err != nil {
			return fmt.Errorf("failed to insert player: %w", err)
		}
		playerIDs[player.PlayerID] = playerID
	}

	// Insert commands
	commandQuery := `
		INSERT INTO commands (
			replay_id, player_id, frame, time, action_type, action_id, unit_id, target_id, x, y, data, effective,
			queued, unit_tag, order_id, order_name, select_unit_tags, build_unit_name,
			right_click_unit_tag, right_click_unit_name, targeted_order_unit_tag, targeted_order_unit_name,
			train_unit_name, cancel_train_unit_tag, unload_unit_tag, building_morph_unit_name,
			tech_name, upgrade_name, hotkey_type, hotkey_group, game_speed,
			chat_sender_slot_id, chat_message, vision_slot_ids, alliance_slot_ids, allied_victory,
			leave_reason, minimap_ping_x, minimap_ping_y, general_data
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40)
	`

	for _, command := range data.Commands {
		command.ReplayID = replayID
		playerID, exists := playerIDs[byte(command.PlayerID)]
		if !exists {
			continue // Skip commands for players not found
		}
		command.PlayerID = playerID

		_, err := tx.ExecContext(ctx, commandQuery,
			command.ReplayID, command.PlayerID, command.Frame, command.Time,
			command.ActionType, command.ActionID, command.UnitID, command.TargetID,
			command.X, command.Y, command.Data, command.Effective,
			command.Queued, command.UnitTag, command.OrderID, command.OrderName,
			command.SelectUnitTags, command.BuildUnitName,
			command.RightClickUnitTag, command.RightClickUnitName,
			command.TargetedOrderUnitTag, command.TargetedOrderUnitName,
			command.TrainUnitName, command.CancelTrainUnitTag, command.UnloadUnitTag,
			command.BuildingMorphUnitName, command.TechName, command.UpgradeName,
			command.HotkeyType, command.HotkeyGroup, command.GameSpeed,
			command.ChatSenderSlotID, command.ChatMessage, command.VisionSlotIDs,
			command.AllianceSlotIDs, command.AlliedVictory, command.LeaveReason,
			command.MinimapPingX, command.MinimapPingY, command.GeneralData,
		)
		if err != nil {
			return fmt.Errorf("failed to insert command: %w", err)
		}
	}

	// Insert units
	unitQuery := `
		INSERT INTO units (
			replay_id, player_id, unit_id, type, name, created, destroyed,
			x, y, hp, max_hp, shield, max_shield, energy, max_energy
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
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
			unit.Created, unit.Destroyed, unit.X, unit.Y, unit.HP, unit.MaxHP,
			unit.Shield, unit.MaxShield, unit.Energy, unit.MaxEnergy,
		)
		if err != nil {
			return fmt.Errorf("failed to insert unit: %w", err)
		}
	}

	// Insert buildings
	buildingQuery := `
		INSERT INTO buildings (
			replay_id, player_id, building_id, type, name, created, destroyed,
			x, y, hp, max_hp, shield, max_shield, energy, max_energy
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
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
			building.Name, building.Created, building.Destroyed, building.X, building.Y,
			building.HP, building.MaxHP, building.Shield, building.MaxShield,
			building.Energy, building.MaxEnergy,
		)
		if err != nil {
			return fmt.Errorf("failed to insert building: %w", err)
		}
	}

	// Insert resources
	resourceQuery := `
		INSERT INTO resources (
			replay_id, type, x, y, amount
		) VALUES ($1, $2, $3, $4, $5)
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
		) VALUES ($1, $2, $3)
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
			replay_id, player_id, type, name, x, y, hp, max_hp, shield, max_shield, energy, max_energy
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
			placedUnit.X, placedUnit.Y, placedUnit.HP, placedUnit.MaxHP,
			placedUnit.Shield, placedUnit.MaxShield, placedUnit.Energy, placedUnit.MaxEnergy,
		)
		if err != nil {
			return fmt.Errorf("failed to insert placed unit: %w", err)
		}
	}

	// Insert chat messages
	chatMessageQuery := `
		INSERT INTO chat_messages (
			replay_id, player_id, sender_slot_id, message, frame, time
		) VALUES ($1, $2, $3, $4, $5, $6)
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
		) VALUES ($1, $2, $3, $4, $5)
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
func (s *PostgresStorage) ReplayExists(ctx context.Context, filePath, checksum string) (bool, error) {
	query := `SELECT 1 FROM replays WHERE file_path = $1 OR file_checksum = $2 LIMIT 1`
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
func (s *PostgresStorage) Query(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
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

// StorageName returns the storage backend name
func (s *PostgresStorage) StorageName() string {
	return StoragePostgreSQL
}

// Close closes the database connection
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

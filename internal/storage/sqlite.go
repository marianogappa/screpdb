package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements Storage interface using SQLite
type SQLiteStorage struct {
	db *sql.DB
}

// SQLiteBatchInserter defines the interface for entity-specific batch insertion for SQLite
type SQLiteBatchInserter interface {
	TableName() string
	ColumnNames() []string
	BuildArgs(entity any, args []any, offset int) error
	EntityCount() int
}

// GenericSQLiteBatchInserter provides a generic implementation for batch insertion in SQLite
type GenericSQLiteBatchInserter struct {
	inserter SQLiteBatchInserter
	db       *sql.DB
}

// NewGenericSQLiteBatchInserter creates a new generic SQLite batch inserter
func NewGenericSQLiteBatchInserter(inserter SQLiteBatchInserter, db *sql.DB) *GenericSQLiteBatchInserter {
	return &GenericSQLiteBatchInserter{
		inserter: inserter,
		db:       db,
	}
}

// InsertBatch performs a batch insert for any entity type in SQLite
func (g *GenericSQLiteBatchInserter) InsertBatch(ctx context.Context, entities []any) error {
	if len(entities) == 0 {
		return nil
	}

	// Process in batches using SQLite batch size
	for i := 0; i < len(entities); i += sqliteBatchSize {
		end := i + sqliteBatchSize
		if end > len(entities) {
			end = len(entities)
		}

		batch := entities[i:end]
		if err := g.insertBatchChunk(ctx, batch); err != nil {
			return fmt.Errorf("failed to insert %s batch: %w", g.inserter.TableName(), err)
		}
	}

	return nil
}

// insertBatchChunk inserts a chunk of entities
func (g *GenericSQLiteBatchInserter) insertBatchChunk(ctx context.Context, entities []any) error {
	if len(entities) == 0 {
		return nil
	}

	// Build the batch insert query
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES ",
		g.inserter.TableName(),
		strings.Join(g.inserter.ColumnNames(), ", "))

	// Build placeholders and args
	placeholders := make([]string, len(entities))
	columnCount := g.inserter.EntityCount()
	args := make([]any, len(entities)*columnCount)

	for i, entity := range entities {
		// Build placeholder for this entity (SQLite uses ? placeholders)
		placeholderArgs := make([]string, columnCount)
		for j := 0; j < columnCount; j++ {
			placeholderArgs[j] = "?"
		}
		placeholders[i] = fmt.Sprintf("(%s)", strings.Join(placeholderArgs, ", "))

		// Build args for this entity
		if err := g.inserter.BuildArgs(entity, args, i*columnCount); err != nil {
			return fmt.Errorf("failed to build args for entity %d: %w", i, err)
		}
	}

	query += strings.Join(placeholders, ", ")

	_, err := g.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert %s batch: %w", g.inserter.TableName(), err)
	}

	return nil
}

// Batching configuration for SQLite
const (
	sqliteBatchSize     = 500 // Smaller batch size for SQLite
	sqliteFlushInterval = 3 * time.Second
)

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &SQLiteStorage{db: db}, nil
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
		general_data TEXT -- Hex string of raw data
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
		y INTEGER NOT NULL
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
		y INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS resources (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		amount INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS start_locations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS placed_units (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS chat_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		sender_slot_id INTEGER NOT NULL,
		message TEXT NOT NULL,
		frame INTEGER NOT NULL,
		time DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS leave_games (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		reason TEXT,
		frame INTEGER NOT NULL,
		time DATETIME NOT NULL
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

// StartIngestion starts the ingestion process with batching (sequential for SQLite)
func (s *SQLiteStorage) StartIngestion(ctx context.Context) (ReplayDataChannel, <-chan error) {
	dataChan := make(ReplayDataChannel, 50) // Smaller buffer for SQLite
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)

		// Process replays sequentially to handle dependencies
		for {
			select {
			case data, ok := <-dataChan:
				if !ok {
					// Channel closed, we're done
					errChan <- nil
					return
				}

				// Process this replay completely before moving to the next
				if err := s.storeReplayWithBatching(ctx, data); err != nil {
					fmt.Printf("Error storing replay: %v\n", err)
					errChan <- err
					return
				}

			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}
		}
	}()

	return dataChan, errChan
}

// storeReplayWithBatching stores a replay data structure using the new batching approach (sequential for SQLite)
func (s *SQLiteStorage) storeReplayWithBatching(ctx context.Context, data *models.ReplayData) error {
	// Step 1: Insert replay sequentially
	replayID, err := s.insertReplaySequential(ctx, data.Replay)
	if err != nil {
		return fmt.Errorf("failed to insert replay: %w", err)
	}

	// Step 2: Insert players in batch
	playerIDs, err := s.insertPlayersBatch(ctx, replayID, data.Players)
	if err != nil {
		return fmt.Errorf("failed to insert players: %w", err)
	}

	// Step 3: Insert all other entities sequentially with batching using generic inserters
	// Update all entities with the correct IDs
	s.updateEntityIDs(data, replayID, playerIDs)

	// Convert slices to []any for generic processing
	commandsAny := make([]any, len(data.Commands))
	for i, cmd := range data.Commands {
		commandsAny[i] = cmd
	}
	unitsAny := make([]any, len(data.Units))
	for i, unit := range data.Units {
		unitsAny[i] = unit
	}
	buildingsAny := make([]any, len(data.Buildings))
	for i, building := range data.Buildings {
		buildingsAny[i] = building
	}
	resourcesAny := make([]any, len(data.Resources))
	for i, resource := range data.Resources {
		resourcesAny[i] = resource
	}
	startLocationsAny := make([]any, len(data.StartLocations))
	for i, startLoc := range data.StartLocations {
		startLocationsAny[i] = startLoc
	}
	placedUnitsAny := make([]any, len(data.PlacedUnits))
	for i, placedUnit := range data.PlacedUnits {
		placedUnitsAny[i] = placedUnit
	}
	chatMessagesAny := make([]any, len(data.ChatMessages))
	for i, chatMsg := range data.ChatMessages {
		chatMessagesAny[i] = chatMsg
	}
	leaveGamesAny := make([]any, len(data.LeaveGames))
	for i, leaveGame := range data.LeaveGames {
		leaveGamesAny[i] = leaveGame
	}

	// Create generic batch inserters
	commandsInserter := NewGenericSQLiteBatchInserter(NewSQLiteCommandsInserter(), s.db)
	unitsInserter := NewGenericSQLiteBatchInserter(NewSQLiteUnitsInserter(), s.db)
	buildingsInserter := NewGenericSQLiteBatchInserter(NewSQLiteBuildingsInserter(), s.db)
	resourcesInserter := NewGenericSQLiteBatchInserter(NewSQLiteResourcesInserter(), s.db)
	startLocationsInserter := NewGenericSQLiteBatchInserter(NewSQLiteStartLocationsInserter(), s.db)
	placedUnitsInserter := NewGenericSQLiteBatchInserter(NewSQLitePlacedUnitsInserter(), s.db)
	chatMessagesInserter := NewGenericSQLiteBatchInserter(NewSQLiteChatMessagesInserter(), s.db)
	leaveGamesInserter := NewGenericSQLiteBatchInserter(NewSQLiteLeaveGamesInserter(), s.db)

	// Insert all other entities in batches sequentially
	if err := commandsInserter.InsertBatch(ctx, commandsAny); err != nil {
		return fmt.Errorf("failed to insert commands: %w", err)
	}
	if err := unitsInserter.InsertBatch(ctx, unitsAny); err != nil {
		return fmt.Errorf("failed to insert units: %w", err)
	}
	if err := buildingsInserter.InsertBatch(ctx, buildingsAny); err != nil {
		return fmt.Errorf("failed to insert buildings: %w", err)
	}
	if err := resourcesInserter.InsertBatch(ctx, resourcesAny); err != nil {
		return fmt.Errorf("failed to insert resources: %w", err)
	}
	if err := startLocationsInserter.InsertBatch(ctx, startLocationsAny); err != nil {
		return fmt.Errorf("failed to insert start locations: %w", err)
	}
	if err := placedUnitsInserter.InsertBatch(ctx, placedUnitsAny); err != nil {
		return fmt.Errorf("failed to insert placed units: %w", err)
	}
	if err := chatMessagesInserter.InsertBatch(ctx, chatMessagesAny); err != nil {
		return fmt.Errorf("failed to insert chat messages: %w", err)
	}
	if err := leaveGamesInserter.InsertBatch(ctx, leaveGamesAny); err != nil {
		return fmt.Errorf("failed to insert leave games: %w", err)
	}

	log.Printf("Successfully stored replay: %s", data.Replay.FileName)
	return nil
}

// insertReplaySequential inserts a single replay and returns its ID
func (s *SQLiteStorage) insertReplaySequential(ctx context.Context, replay *models.Replay) (int64, error) {
	query := `
		INSERT INTO replays (
			file_path, file_checksum, file_name, file_size, created_at, replay_date,
			title, host, map_name, map_width, map_height, duration,
			frame_count, version, engine, speed, game_type, sub_type, avail_slots_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.ExecContext(ctx, query,
		replay.FilePath, replay.FileChecksum, replay.FileName,
		replay.FileSize, replay.CreatedAt, replay.ReplayDate,
		replay.Title, replay.Host, replay.MapName,
		replay.MapWidth, replay.MapHeight, replay.Duration,
		replay.FrameCount, replay.Version, replay.Engine,
		replay.Speed, replay.GameType, replay.SubType,
		replay.AvailSlotsCount,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert replay: %w", err)
	}

	replayID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get replay ID: %w", err)
	}

	return replayID, nil
}

// insertPlayersBatch inserts all players for a replay in a single batch and returns player ID mapping
func (s *SQLiteStorage) insertPlayersBatch(ctx context.Context, replayID int64, players []*models.Player) (map[byte]int64, error) {
	if len(players) == 0 {
		return make(map[byte]int64), nil
	}

	// Build the batch insert query
	query := `
		INSERT INTO players (
			replay_id, slot_id, player_id, name, race, type, color, team, observer, apm, spm, is_winner, start_location_x, start_location_y
		) VALUES `

	// Build placeholders and args
	placeholders := make([]string, len(players))
	args := make([]any, len(players)*14)

	for i, player := range players {
		placeholders[i] = "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

		args[i*14] = replayID
		args[i*14+1] = player.SlotID
		args[i*14+2] = player.PlayerID
		args[i*14+3] = player.Name
		args[i*14+4] = player.Race
		args[i*14+5] = player.Type
		args[i*14+6] = player.Color
		args[i*14+7] = player.Team
		args[i*14+8] = player.Observer
		args[i*14+9] = player.APM
		args[i*14+10] = player.SPM
		args[i*14+11] = player.IsWinner
		args[i*14+12] = player.StartLocationX
		args[i*14+13] = player.StartLocationY
	}

	query += strings.Join(placeholders, ", ")

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert players batch: %w", err)
	}

	// Get the first inserted ID and calculate the rest
	firstID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get first player ID: %w", err)
	}

	playerIDs := make(map[byte]int64)
	for i, player := range players {
		playerIDs[player.PlayerID] = firstID + int64(i)
	}

	return playerIDs, nil
}

// updateEntityIDs updates all entities with the correct replay ID and player IDs
func (s *SQLiteStorage) updateEntityIDs(data *models.ReplayData, replayID int64, playerIDs map[byte]int64) {
	// Update commands
	for _, command := range data.Commands {
		command.ReplayID = replayID
		if playerID, exists := playerIDs[byte(command.PlayerID)]; exists {
			command.PlayerID = playerID
		}
	}

	// Update units
	for _, unit := range data.Units {
		unit.ReplayID = replayID
		if playerID, exists := playerIDs[byte(unit.PlayerID)]; exists {
			unit.PlayerID = playerID
		}
	}

	// Update buildings
	for _, building := range data.Buildings {
		building.ReplayID = replayID
		if playerID, exists := playerIDs[byte(building.PlayerID)]; exists {
			building.PlayerID = playerID
		}
	}

	// Update resources
	for _, resource := range data.Resources {
		resource.ReplayID = replayID
	}

	// Update start locations
	for _, startLoc := range data.StartLocations {
		startLoc.ReplayID = replayID
	}

	// Update placed units
	for _, placedUnit := range data.PlacedUnits {
		placedUnit.ReplayID = replayID
		if playerID, exists := playerIDs[byte(placedUnit.PlayerID)]; exists {
			placedUnit.PlayerID = playerID
		}
	}

	// Update chat messages
	for _, chatMsg := range data.ChatMessages {
		chatMsg.ReplayID = replayID
		if playerID, exists := playerIDs[byte(chatMsg.PlayerID)]; exists {
			chatMsg.PlayerID = playerID
		}
	}

	// Update leave games
	for _, leaveGame := range data.LeaveGames {
		leaveGame.ReplayID = replayID
		if playerID, exists := playerIDs[byte(leaveGame.PlayerID)]; exists {
			leaveGame.PlayerID = playerID
		}
	}
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

// BatchReplayExists checks if multiple replays already exist by file paths and checksums
func (s *SQLiteStorage) BatchReplayExists(ctx context.Context, filePaths, checksums []string) (map[string]bool, error) {
	if len(filePaths) != len(checksums) {
		return nil, fmt.Errorf("filePaths and checksums must have the same length")
	}

	if len(filePaths) == 0 {
		return make(map[string]bool), nil
	}

	// Build the query with placeholders
	placeholders := make([]string, len(filePaths))
	args := make([]any, len(filePaths)*2)

	for i := 0; i < len(filePaths); i++ {
		placeholders[i] = "(?, ?)"
		args[i*2] = filePaths[i]
		args[i*2+1] = checksums[i]
	}

	// Join all placeholders with commas
	placeholderStr := ""
	for i, placeholder := range placeholders {
		if i > 0 {
			placeholderStr += ", "
		}
		placeholderStr += placeholder
	}

	query := fmt.Sprintf(`
		SELECT file_path FROM replays 
		WHERE (file_path, file_checksum) IN (%s)
	`, placeholderStr)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing replays: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]bool)
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			return nil, fmt.Errorf("failed to scan file path: %w", err)
		}
		existing[filePath] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return existing, nil
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

// StorageName returns the storage backend name
func (s *SQLiteStorage) StorageName() string {
	return StorageSQLite
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

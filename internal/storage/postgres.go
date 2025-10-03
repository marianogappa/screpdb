package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/marianogappa/screpdb/internal/models"
)

// PostgresStorage implements Storage interface using PostgreSQL
type PostgresStorage struct {
	db *sql.DB
}

// BatchInserter defines the interface for entity-specific batch insertion
type BatchInserter interface {
	TableName() string
	ColumnNames() []string
	BuildArgs(entity any, args []any, offset int) error
	EntityCount() int
}

// GenericBatchInserter provides a generic implementation for batch insertion
type GenericBatchInserter struct {
	inserter BatchInserter
	db       *sql.DB
}

// NewGenericBatchInserter creates a new generic batch inserter
func NewGenericBatchInserter(inserter BatchInserter, db *sql.DB) *GenericBatchInserter {
	return &GenericBatchInserter{
		inserter: inserter,
		db:       db,
	}
}

// InsertBatch performs a batch insert for any entity type
func (g *GenericBatchInserter) InsertBatch(ctx context.Context, entities []any) error {
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
		// Build placeholder for this entity
		placeholderArgs := make([]string, columnCount)
		for j := 0; j < columnCount; j++ {
			placeholderArgs[j] = fmt.Sprintf("$%d", i*columnCount+j+1)
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

// BatchInsertWorker handles batched inserts for any entity type
func (g *GenericBatchInserter) BatchInsertWorker(ctx context.Context, entities []any, wg *sync.WaitGroup, errChan chan<- error) {
	defer wg.Done()

	if len(entities) == 0 {
		return
	}

	// Process in batches
	for i := 0; i < len(entities); i += batchSize {
		end := i + batchSize
		if end > len(entities) {
			end = len(entities)
		}

		batch := entities[i:end]
		if err := g.InsertBatch(ctx, batch); err != nil {
			errChan <- fmt.Errorf("failed to insert %s batch: %w", g.inserter.TableName(), err)
			return
		}
	}
}

// Batching configuration
const (
	batchSize     = 1000            // Number of records per batch
	flushInterval = 5 * time.Second // Time-based flush interval
)

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
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		frame INTEGER NOT NULL,
		time TIMESTAMP WITH TIME ZONE NOT NULL,
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
		vision_slot_ids JSONB, -- JSON array of slot IDs
		
		-- Alliance command fields
		alliance_slot_ids JSONB, -- JSON array of slot IDs
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
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		unit_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		created TIMESTAMP WITH TIME ZONE NOT NULL,
		created_frame INTEGER NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS buildings (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		building_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		created TIMESTAMP WITH TIME ZONE NOT NULL,
		created_frame INTEGER NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS resources (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		amount INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS start_locations (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS placed_units (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS chat_messages (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		sender_slot_id INTEGER NOT NULL,
		message TEXT NOT NULL,
		frame INTEGER NOT NULL,
		time TIMESTAMP WITH TIME ZONE NOT NULL
	);

	CREATE TABLE IF NOT EXISTS leave_games (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
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

// StartIngestion starts the ingestion process with batching
func (s *PostgresStorage) StartIngestion(ctx context.Context) (ReplayDataChannel, <-chan error) {
	dataChan := make(ReplayDataChannel, 100) // Buffered channel
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

// storeReplayWithBatching stores a replay data structure using the new batching approach
func (s *PostgresStorage) storeReplayWithBatching(ctx context.Context, data *models.ReplayData) error {
	// Step 1: Insert replay sequentially (with RETURNING)
	replayID, err := s.insertReplaySequential(ctx, data.Replay)
	if err != nil {
		return fmt.Errorf("failed to insert replay: %w", err)
	}

	// Step 2: Insert players in batch (with RETURNING)
	playerIDs, err := s.insertPlayersBatch(ctx, replayID, data.Players)
	if err != nil {
		return fmt.Errorf("failed to insert players: %w", err)
	}

	// Step 3: Insert all other entities concurrently using worker goroutines
	errChan := make(chan error, 8) // One for each table
	var wg sync.WaitGroup

	// Update all entities with the correct IDs
	s.updateEntityIDs(data, replayID, playerIDs)

	// Start worker goroutines for each table using generic batch inserters
	wg.Add(8)

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
	commandsInserter := NewGenericBatchInserter(NewCommandsInserter(), s.db)
	unitsInserter := NewGenericBatchInserter(NewUnitsInserter(), s.db)
	buildingsInserter := NewGenericBatchInserter(NewBuildingsInserter(), s.db)
	resourcesInserter := NewGenericBatchInserter(NewResourcesInserter(), s.db)
	startLocationsInserter := NewGenericBatchInserter(NewStartLocationsInserter(), s.db)
	placedUnitsInserter := NewGenericBatchInserter(NewPlacedUnitsInserter(), s.db)
	chatMessagesInserter := NewGenericBatchInserter(NewChatMessagesInserter(), s.db)
	leaveGamesInserter := NewGenericBatchInserter(NewLeaveGamesInserter(), s.db)

	go commandsInserter.BatchInsertWorker(ctx, commandsAny, &wg, errChan)
	go unitsInserter.BatchInsertWorker(ctx, unitsAny, &wg, errChan)
	go buildingsInserter.BatchInsertWorker(ctx, buildingsAny, &wg, errChan)
	go resourcesInserter.BatchInsertWorker(ctx, resourcesAny, &wg, errChan)
	go startLocationsInserter.BatchInsertWorker(ctx, startLocationsAny, &wg, errChan)
	go placedUnitsInserter.BatchInsertWorker(ctx, placedUnitsAny, &wg, errChan)
	go chatMessagesInserter.BatchInsertWorker(ctx, chatMessagesAny, &wg, errChan)
	go leaveGamesInserter.BatchInsertWorker(ctx, leaveGamesAny, &wg, errChan)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return fmt.Errorf("worker error: %w", err)
		}
	}

	return nil
}

// insertReplaySequential inserts a single replay and returns its ID
func (s *PostgresStorage) insertReplaySequential(ctx context.Context, replay *models.Replay) (int64, error) {
	query := `
		INSERT INTO replays (
			file_path, file_checksum, file_name, file_size, created_at, replay_date,
			title, host, map_name, map_width, map_height, duration,
			frame_count, version, engine, speed, game_type, sub_type, avail_slots_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id
	`

	var replayID int64
	err := s.db.QueryRowContext(ctx, query,
		replay.FilePath, replay.FileChecksum, replay.FileName,
		replay.FileSize, replay.CreatedAt, replay.ReplayDate,
		replay.Title, replay.Host, replay.MapName,
		replay.MapWidth, replay.MapHeight, replay.Duration,
		replay.FrameCount, replay.Version, replay.Engine,
		replay.Speed, replay.GameType, replay.SubType,
		replay.AvailSlotsCount,
	).Scan(&replayID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert replay: %w", err)
	}

	return replayID, nil
}

// insertPlayersBatch inserts all players for a replay in a single batch and returns player ID mapping
func (s *PostgresStorage) insertPlayersBatch(ctx context.Context, replayID int64, players []*models.Player) (map[byte]int64, error) {
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
		placeholders[i] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*14+1, i*14+2, i*14+3, i*14+4, i*14+5, i*14+6, i*14+7, i*14+8, i*14+9, i*14+10, i*14+11, i*14+12, i*14+13, i*14+14)

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

	query += strings.Join(placeholders, ", ") + " RETURNING id, player_id"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert players batch: %w", err)
	}
	defer rows.Close()

	playerIDs := make(map[byte]int64)
	for rows.Next() {
		var id int64
		var playerID byte
		if err := rows.Scan(&id, &playerID); err != nil {
			return nil, fmt.Errorf("failed to scan player result: %w", err)
		}
		playerIDs[playerID] = id
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating player rows: %w", err)
	}

	return playerIDs, nil
}

// updateEntityIDs updates all entities with the correct replay ID and player IDs
func (s *PostgresStorage) updateEntityIDs(data *models.ReplayData, replayID int64, playerIDs map[byte]int64) {
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

// BatchReplayExists checks if multiple replays already exist by file paths and checksums
func (s *PostgresStorage) BatchReplayExists(ctx context.Context, filePaths, checksums []string) (map[string]bool, error) {
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
		placeholders[i] = fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2)
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

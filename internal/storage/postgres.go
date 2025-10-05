package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	"github.com/marianogappa/screpdb/internal/models"
)

// PostgresStorage implements Storage interface using PostgreSQL
type PostgresStorage struct {
	db *sql.DB
}

// Batching configuration
const (
	batchSize = 1000 // Number of records per batch
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
			DROP TABLE IF EXISTS commands CASCADE;
			DROP TABLE IF EXISTS players CASCADE;
			DROP TABLE IF EXISTS replays CASCADE;

			-- Legacy: these tables are no longer used
			DROP TABLE IF EXISTS chat_messages CASCADE;
			DROP TABLE IF EXISTS leave_games CASCADE;
			DROP TABLE IF EXISTS placed_units CASCADE;
			DROP TABLE IF EXISTS available_start_locations CASCADE;
			DROP TABLE IF EXISTS start_locations CASCADE;
			DROP TABLE IF EXISTS resources CASCADE;
			DROP TABLE IF EXISTS buildings CASCADE;
			DROP TABLE IF EXISTS units CASCADE;
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
		created_at TIMESTAMP WITH TIME ZONE NOT NULL,
		replay_date TIMESTAMP WITH TIME ZONE NOT NULL,
		title TEXT,
		host TEXT,
		map_name TEXT NOT NULL,
		map_width INTEGER NOT NULL,
		map_height INTEGER NOT NULL,
		duration_seconds INTEGER NOT NULL,
		frame_count INTEGER NOT NULL,
		engine_version TEXT NOT NULL,
		engine TEXT NOT NULL,
		game_speed TEXT NOT NULL,
		game_type TEXT NOT NULL,
		home_team_size TEXT NOT NULL,
		avail_slots_count INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS players (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		race TEXT NOT NULL,
		type TEXT NOT NULL,
		color TEXT NOT NULL,
		team INTEGER NOT NULL,
		is_observer BOOLEAN NOT NULL,
		apm INTEGER NOT NULL,
		eapm INTEGER NOT NULL, -- effective apm is apm excluding actions deemed ineffective
		is_winner BOOLEAN NOT NULL,
		start_location_x INTEGER,
		start_location_y INTEGER,
		start_location_oclock INTEGER
	);

	CREATE TABLE IF NOT EXISTS commands (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		frame INTEGER NOT NULL,
		run_at TIMESTAMP WITH TIME ZONE NOT NULL,
		action_type TEXT NOT NULL,
		x INTEGER NOT NULL,
		y INTEGER NOT NULL,
		is_effective BOOLEAN NOT NULL,
		
		-- Common fields (used by multiple command types)
		is_queued BOOLEAN,
		order_name TEXT,
		
		-- Unit information (normalized fields)
		unit_type TEXT, -- Single unit type
		unit_types TEXT, -- JSON array of unit types for multiple units
		
		-- Tech command fields
		tech_name TEXT,
		
		-- Upgrade command fields
		upgrade_name TEXT,
		
		-- Hotkey command fields
		hotkey_type TEXT,
		hotkey_group INTEGER,
		
		-- Game Speed command fields
		game_speed TEXT,
		
		-- Vision command fields
		vision_player_ids JSONB, -- JSON array of player IDs
		
		-- Alliance command fields
		alliance_player_ids JSONB, -- JSON array of player IDs
		is_allied_victory BOOLEAN,
		
		-- General command fields (for unhandled commands)
		general_data TEXT, -- Hex string of raw data
		
		-- Chat and leave game fields
		chat_message TEXT,
		leave_reason TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_replays_file_path ON replays(file_path);
	CREATE INDEX IF NOT EXISTS idx_replays_file_checksum ON replays(file_checksum);
	CREATE INDEX IF NOT EXISTS idx_replays_replay_date ON replays(replay_date);
	CREATE INDEX IF NOT EXISTS idx_players_replay_id ON players(replay_id);
	CREATE INDEX IF NOT EXISTS idx_commands_replay_id ON commands(replay_id);
	CREATE INDEX IF NOT EXISTS idx_commands_player_id ON commands(player_id);
	CREATE INDEX IF NOT EXISTS idx_commands_frame ON commands(frame);
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

// storeReplayWithBatching stores a replay data structure using sequential processing
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

	// Step 3: Update commands with correct IDs and insert them
	s.updateEntityIDs(data, replayID, playerIDs)

	// Step 4: Insert commands in batch
	if len(data.Commands) > 0 {
		if err := s.insertCommandsBatch(ctx, data.Commands); err != nil {
			return fmt.Errorf("failed to insert commands: %w", err)
		}
	}

	return nil
}

// insertReplaySequential inserts a single replay and returns its ID
func (s *PostgresStorage) insertReplaySequential(ctx context.Context, replay *models.Replay) (int64, error) {
	query := `
		INSERT INTO replays (
			file_path, file_checksum, file_name, created_at, replay_date,
			title, host, map_name, map_width, map_height, duration_seconds,
			frame_count, engine_version, engine, game_speed, game_type, home_team_size, avail_slots_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id
	`

	var replayID int64
	err := s.db.QueryRowContext(ctx, query,
		replay.FilePath, replay.FileChecksum, replay.FileName,
		replay.CreatedAt, replay.ReplayDate,
		replay.Title, replay.Host, replay.MapName,
		replay.MapWidth, replay.MapHeight, replay.DurationSeconds,
		replay.FrameCount, replay.EngineVersion, replay.Engine,
		replay.GameSpeed, replay.GameType, replay.HomeTeamSize,
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
			replay_id, name, race, type, color, team, is_observer, apm, eapm, is_winner, start_location_x, start_location_y, start_location_oclock
		) VALUES `

	// Build placeholders and args
	placeholders := make([]string, len(players))
	args := make([]any, len(players)*13)

	for i, player := range players {
		placeholders[i] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*13+1, i*13+2, i*13+3, i*13+4, i*13+5, i*13+6, i*13+7, i*13+8, i*13+9, i*13+10, i*13+11, i*13+12, i*13+13)

		args[i*13] = replayID
		args[i*13+1] = player.Name
		args[i*13+2] = player.Race
		args[i*13+3] = player.Type
		args[i*13+4] = player.Color
		args[i*13+5] = player.Team
		args[i*13+6] = player.IsObserver
		args[i*13+7] = player.APM
		args[i*13+8] = player.EAPM
		args[i*13+9] = player.IsWinner
		args[i*13+10] = player.StartLocationX
		args[i*13+11] = player.StartLocationY
		args[i*13+12] = player.StartLocationOclock
	}

	query += strings.Join(placeholders, ", ") + " RETURNING id"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert players batch: %w", err)
	}
	defer rows.Close()

	playerIDs := make(map[byte]int64)
	i := 0
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan player result: %w", err)
		}
		playerIDs[players[i].PlayerID] = id
		i++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating player rows: %w", err)
	}

	return playerIDs, nil
}

// insertCommandsBatch inserts all commands for a replay in batches
func (s *PostgresStorage) insertCommandsBatch(ctx context.Context, commands []*models.Command) error {
	if len(commands) == 0 {
		return nil
	}

	// Process in batches
	for i := 0; i < len(commands); i += batchSize {
		end := min(i+batchSize, len(commands))
		batch := commands[i:end]
		if err := s.insertCommandsBatchChunk(ctx, batch); err != nil {
			return fmt.Errorf("failed to insert commands batch: %w", err)
		}
	}

	return nil
}

// insertCommandsBatchChunk inserts a chunk of commands
func (s *PostgresStorage) insertCommandsBatchChunk(ctx context.Context, commands []*models.Command) error {
	if len(commands) == 0 {
		return nil
	}

	// Build the batch insert query
	query := `
		INSERT INTO commands (
			replay_id, player_id, frame, run_at, action_type, x, y, is_effective,
			is_queued, order_name, unit_type, unit_types,
			tech_name, upgrade_name, hotkey_type, hotkey_group, game_speed,
			vision_player_ids, alliance_player_ids, is_allied_victory,
			general_data, chat_message, leave_reason
		) VALUES `

	// Build placeholders and args
	placeholders := make([]string, len(commands))
	args := make([]any, len(commands)*23) // 23 columns

	for i, command := range commands {
		placeholders[i] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*23+1, i*23+2, i*23+3, i*23+4, i*23+5, i*23+6, i*23+7, i*23+8, i*23+9, i*23+10, i*23+11, i*23+12, i*23+13, i*23+14, i*23+15, i*23+16, i*23+17, i*23+18, i*23+19, i*23+20, i*23+21, i*23+22, i*23+23)

		// Serialize player IDs to JSON
		visionPlayerIDsJSON := s.serializePlayerIDs(command.VisionPlayerIDs)
		alliancePlayerIDsJSON := s.serializePlayerIDs(command.AlliancePlayerIDs)

		// Serialize unit information to JSON
		unitTypesJSON := s.serializeString(command.UnitTypes)

		args[i*23] = command.ReplayID
		args[i*23+1] = command.PlayerID
		args[i*23+2] = command.Frame
		args[i*23+3] = command.RunAt
		args[i*23+4] = command.ActionType
		args[i*23+5] = command.X
		args[i*23+6] = command.Y
		args[i*23+7] = command.IsEffective
		args[i*23+8] = command.IsQueued
		args[i*23+9] = command.OrderName
		args[i*23+10] = s.getUnitTypeOrNull(command.UnitType)
		args[i*23+11] = unitTypesJSON
		args[i*23+12] = command.TechName
		args[i*23+13] = command.UpgradeName
		args[i*23+14] = command.HotkeyType
		args[i*23+15] = command.HotkeyGroup
		args[i*23+16] = command.GameSpeed
		args[i*23+17] = visionPlayerIDsJSON
		args[i*23+18] = alliancePlayerIDsJSON
		args[i*23+19] = command.IsAlliedVictory
		args[i*23+20] = command.GeneralData
		args[i*23+21] = command.ChatMessage
		args[i*23+22] = command.LeaveReason
	}

	query += strings.Join(placeholders, ", ")

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert commands batch: %w", err)
	}

	return nil
}

// Helper functions for serialization
func (s *PostgresStorage) serializePlayerIDs(playerIDs *[]int64) interface{} {
	if playerIDs == nil {
		return nil
	}
	data, err := json.Marshal(*playerIDs)
	if err != nil {
		return nil
	}
	return string(data)
}

func (s *PostgresStorage) serializeString(str *string) interface{} {
	if str == nil {
		return nil
	}
	return *str
}

func (s *PostgresStorage) getUnitTypeOrNull(unitType *string) interface{} {
	if unitType == nil || *unitType == "None" {
		return nil
	}
	return *unitType
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

// GetDatabaseSchema returns the database schema information
func (s *PostgresStorage) GetDatabaseSchema(ctx context.Context) (string, error) {
	query := `
		SELECT table_schema, table_name, column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_name IN ('commands', 'players', 'replays')
		ORDER BY table_schema, table_name, ordinal_position;
	`

	results, err := s.Query(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to query schema: %w", err)
	}

	var schema strings.Builder
	schema.WriteString("# Database Schema\n\n")

	// Group results by table
	tableSchemas := make(map[string][]map[string]any)
	for _, row := range results {
		tableName := fmt.Sprintf("%v", row["table_name"])
		tableSchemas[tableName] = append(tableSchemas[tableName], row)
	}

	// Write schema for each table
	for _, tableName := range []string{"replays", "players", "commands"} {
		if columns, exists := tableSchemas[tableName]; exists {
			schema.WriteString(fmt.Sprintf("## %s\n\n", tableName))
			schema.WriteString("| Column | Type | Nullable |\n")
			schema.WriteString("|--------|------|----------|\n")

			for _, col := range columns {
				columnName := fmt.Sprintf("%v", col["column_name"])
				dataType := fmt.Sprintf("%v", col["data_type"])
				isNullable := fmt.Sprintf("%v", col["is_nullable"])

				schema.WriteString(fmt.Sprintf("| %s | %s | %s |\n", columnName, dataType, isNullable))
			}
			schema.WriteString("\n")
		}
	}

	return schema.String(), nil
}

// Close closes the database connection
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

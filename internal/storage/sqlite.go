package storage

import (
	"context"
	"database/sql"
	"encoding/json"
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
		created_at DATETIME NOT NULL,
		replay_date DATETIME NOT NULL,
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
		id INTEGER PRIMARY KEY AUTOINCREMENT,
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
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		replay_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		frame INTEGER NOT NULL,
		run_at DATETIME NOT NULL,
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
		vision_player_ids TEXT, -- JSON array of player IDs
		
		-- Alliance command fields
		alliance_player_ids TEXT, -- JSON array of player IDs
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

// storeReplayWithBatching stores a replay data structure using sequential processing
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

	// Step 3: Update commands with correct IDs and insert them
	s.updateEntityIDs(data, replayID, playerIDs)

	// Step 4: Insert commands in batch
	if len(data.Commands) > 0 {
		if err := s.insertCommandsBatch(ctx, data.Commands); err != nil {
			return fmt.Errorf("failed to insert commands: %w", err)
		}
	}

	log.Printf("Successfully stored replay: %s", data.Replay.FileName)
	return nil
}

// insertReplaySequential inserts a single replay and returns its ID
func (s *SQLiteStorage) insertReplaySequential(ctx context.Context, replay *models.Replay) (int64, error) {
	query := `
		INSERT INTO replays (
			file_path, file_checksum, file_name, created_at, replay_date,
			title, host, map_name, map_width, map_height, duration_seconds,
			frame_count, engine_version, engine, game_speed, game_type, home_team_size, avail_slots_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.ExecContext(ctx, query,
		replay.FilePath, replay.FileChecksum, replay.FileName,
		replay.CreatedAt, replay.ReplayDate,
		replay.Title, replay.Host, replay.MapName,
		replay.MapWidth, replay.MapHeight, replay.DurationSeconds,
		replay.FrameCount, replay.EngineVersion, replay.Engine,
		replay.GameSpeed, replay.GameType, replay.HomeTeamSize,
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
			replay_id, name, race, type, color, team, is_observer, apm, eapm, is_winner, start_location_x, start_location_y, start_location_oclock
		) VALUES `

	// Build placeholders and args
	placeholders := make([]string, len(players))
	args := make([]any, len(players)*13)

	for i, player := range players {
		placeholders[i] = "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

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

// insertCommandsBatch inserts all commands for a replay in batches
func (s *SQLiteStorage) insertCommandsBatch(ctx context.Context, commands []*models.Command) error {
	if len(commands) == 0 {
		return nil
	}

	// Process in batches using SQLite batch size
	for i := 0; i < len(commands); i += sqliteBatchSize {
		end := min(i+sqliteBatchSize, len(commands))
		batch := commands[i:end]
		if err := s.insertCommandsBatchChunk(ctx, batch); err != nil {
			return fmt.Errorf("failed to insert commands batch: %w", err)
		}
	}

	return nil
}

// insertCommandsBatchChunk inserts a chunk of commands
func (s *SQLiteStorage) insertCommandsBatchChunk(ctx context.Context, commands []*models.Command) error {
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
		placeholders[i] = "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

		// Serialize player IDs to JSON
		visionPlayerIDsJSON := s.serializePlayerIDsForSQLite(command.VisionPlayerIDs)
		alliancePlayerIDsJSON := s.serializePlayerIDsForSQLite(command.AlliancePlayerIDs)

		// Serialize unit information to JSON
		unitTypesJSON := s.serializeStringForSQLite(command.UnitTypes)

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
		args[i*23+10] = s.getUnitTypeOrNullForSQLite(command.UnitType)
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

// Helper functions for SQLite serialization
func (s *SQLiteStorage) serializePlayerIDsForSQLite(playerIDs *[]int64) string {
	if playerIDs == nil {
		return ""
	}
	data, err := json.Marshal(*playerIDs)
	if err != nil {
		return ""
	}
	return string(data)
}

func (s *SQLiteStorage) serializeStringForSQLite(str *string) interface{} {
	if str == nil {
		return nil
	}
	return *str
}

func (s *SQLiteStorage) getUnitTypeOrNullForSQLite(unitType *string) interface{} {
	if unitType == nil || *unitType == "None" {
		return nil
	}
	return *unitType
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

// GetDatabaseSchema returns the database schema information
func (s *SQLiteStorage) GetDatabaseSchema(ctx context.Context) (string, error) {
	query := `
		SELECT sql
		FROM sqlite_master
		WHERE type = 'table' AND name IN ('commands', 'players', 'replays')
		ORDER BY name;
	`

	results, err := s.Query(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to query schema: %w", err)
	}

	var schema strings.Builder
	schema.WriteString("# Database Schema\n\n")

	for _, row := range results {
		if sql, exists := row["sql"]; exists && sql != nil {
			schema.WriteString(fmt.Sprintf("```sql\n%v\n```\n\n", sql))
		}
	}

	return schema.String(), nil
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

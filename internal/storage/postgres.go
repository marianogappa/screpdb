package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/migrations"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// PostgresStorage implements Storage interface using PostgreSQL
type PostgresStorage struct {
	db               *sql.DB
	connectionString string
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

	return &PostgresStorage{db: db, connectionString: connectionString}, nil
}

// Initialize creates the database schema using migrations
// If clean is true, drops all existing tables before creating new ones
func (s *PostgresStorage) Initialize(ctx context.Context, clean bool) error {
	if clean {
		if err := migrations.CleanAndRunMigrations(s.connectionString); err != nil {
			return fmt.Errorf("failed to clean and run migrations: %w", err)
		}
		return nil
	}

	if err := migrations.RunMigrations(s.connectionString); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
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

	// Step 5: Process pattern detection results if orchestrator is present
	if data.PatternOrchestrator != nil {
		if err := s.processPatternResults(ctx, data.PatternOrchestrator, replayID, playerIDs); err != nil {
			return fmt.Errorf("failed to process pattern results: %w", err)
		}
	}

	return nil
}

// processPatternResults processes pattern detection results
func (s *PostgresStorage) processPatternResults(ctx context.Context, orchestrator any, replayID int64, playerIDMap map[byte]int64) error {
	// Type assert to *patterns.Orchestrator
	patternOrch, ok := orchestrator.(*patterns.Orchestrator)
	if !ok {
		return nil // Not a pattern orchestrator, skip
	}

	// Convert replay player IDs to database IDs
	patternOrch.ConvertResultsToDatabaseIDs(playerIDMap)

	// Get results
	results := patternOrch.GetResults()

	// Update all results to use the correct database replay ID
	for _, result := range results {
		result.ReplayID = replayID
	}

	// Insert results in batch
	if len(results) > 0 {
		if err := s.BatchInsertPatternResults(ctx, results); err != nil {
			return fmt.Errorf("failed to insert pattern results: %w", err)
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
			replay_id, player_id, frame, seconds_from_game_start, run_at, action_type, x, y,
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
		args[i*23+3] = command.SecondsFromGameStart
		args[i*23+4] = command.RunAt
		args[i*23+5] = command.ActionType
		args[i*23+6] = command.X
		args[i*23+7] = command.Y
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

// FilterOutExistingReplays filters out replays that already exist in the database
func (s *PostgresStorage) FilterOutExistingReplays(ctx context.Context, files []fileops.FileInfo) ([]fileops.FileInfo, error) {
	if len(files) == 0 {
		return []fileops.FileInfo{}, nil
	}

	// Extract file paths and checksums
	filePaths := make([]string, len(files))
	checksums := make([]string, len(files))
	for i, file := range files {
		filePaths[i] = file.Path
		checksums[i] = file.Checksum
	}

	// Build placeholders for file_paths
	filePathPlaceholders := make([]string, len(filePaths))
	for i := range filePaths {
		filePathPlaceholders[i] = fmt.Sprintf("$%d", i+1)
	}

	// Build placeholders for checksums
	checksumPlaceholders := make([]string, len(checksums))
	for i := range checksums {
		checksumPlaceholders[i] = fmt.Sprintf("$%d", len(filePaths)+i+1)
	}

	// Combine all args: file_paths first, then checksums
	args := make([]any, 0, len(filePaths)+len(checksums))
	for _, fp := range filePaths {
		args = append(args, fp)
	}
	for _, cs := range checksums {
		args = append(args, cs)
	}

	query := fmt.Sprintf(`
		SELECT file_path, file_checksum FROM replays 
		WHERE file_path IN (%s) OR file_checksum IN (%s)
	`, strings.Join(filePathPlaceholders, ", "), strings.Join(checksumPlaceholders, ", "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing replays: %w", err)
	}
	defer rows.Close()

	existingPaths := make(map[string]bool)
	existingChecksums := make(map[string]bool)
	for rows.Next() {
		var filePath, fileChecksum string
		if err := rows.Scan(&filePath, &fileChecksum); err != nil {
			return nil, fmt.Errorf("failed to scan file path and checksum: %w", err)
		}
		existingPaths[filePath] = true
		existingChecksums[fileChecksum] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Filter out existing files (by path or checksum)
	var filtered []fileops.FileInfo
	for _, file := range files {
		if !existingPaths[file.Path] && !existingChecksums[file.Checksum] {
			filtered = append(filtered, file)
		}
	}

	return filtered, nil
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

// FilterOutExistingPatternDetections filters out replays that already have pattern detection run
// with the current or higher algorithm version
func (s *PostgresStorage) FilterOutExistingPatternDetections(ctx context.Context, files []fileops.FileInfo, algorithmVersion int) ([]fileops.FileInfo, error) {
	if len(files) == 0 {
		return []fileops.FileInfo{}, nil
	}

	// Extract file paths and checksums
	filePaths := make([]string, len(files))
	checksums := make([]string, len(files))
	for i, file := range files {
		filePaths[i] = file.Path
		checksums[i] = file.Checksum
	}

	// Build placeholders for file_paths
	filePathPlaceholders := make([]string, len(filePaths))
	for i := range filePaths {
		filePathPlaceholders[i] = fmt.Sprintf("$%d", i+1)
	}

	// Build placeholders for checksums
	checksumPlaceholders := make([]string, len(checksums))
	for i := range checksums {
		checksumPlaceholders[i] = fmt.Sprintf("$%d", len(filePaths)+i+1)
	}

	// Combine all args: file_paths first, then checksums, then algorithm_version
	args := make([]any, 0, len(filePaths)+len(checksums)+1)
	for _, fp := range filePaths {
		args = append(args, fp)
	}
	for _, cs := range checksums {
		args = append(args, cs)
	}
	args = append(args, algorithmVersion)

	query := fmt.Sprintf(`
		SELECT DISTINCT file_path, file_checksum 
		FROM detected_patterns_replay 
		WHERE (file_path IN (%s) OR file_checksum IN (%s))
		AND algorithm_version >= $%d
	`, strings.Join(filePathPlaceholders, ", "), strings.Join(checksumPlaceholders, ", "), len(args))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing pattern detections: %w", err)
	}
	defer rows.Close()

	existingPaths := make(map[string]bool)
	existingChecksums := make(map[string]bool)
	for rows.Next() {
		var filePath, fileChecksum string
		if err := rows.Scan(&filePath, &fileChecksum); err != nil {
			return nil, fmt.Errorf("failed to scan file path and checksum: %w", err)
		}
		existingPaths[filePath] = true
		existingChecksums[fileChecksum] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Filter out existing files (by path or checksum)
	var filtered []fileops.FileInfo
	for _, file := range files {
		if !existingPaths[file.Path] && !existingChecksums[file.Checksum] {
			filtered = append(filtered, file)
		}
	}

	return filtered, nil
}

// DeletePatternDetectionsForReplay deletes all pattern detection results for a replay
func (s *PostgresStorage) DeletePatternDetectionsForReplay(ctx context.Context, replayID int64) error {
	// Delete from all three tables
	queries := []string{
		"DELETE FROM detected_patterns_replay WHERE replay_id = $1",
		"DELETE FROM detected_patterns_replay_team WHERE replay_id = $1",
		"DELETE FROM detected_patterns_replay_player WHERE replay_id = $1",
	}

	for _, query := range queries {
		if _, err := s.db.ExecContext(ctx, query, replayID); err != nil {
			return fmt.Errorf("failed to delete pattern detections: %w", err)
		}
	}

	return nil
}

// BatchInsertPatternResults inserts pattern detection results in batch
func (s *PostgresStorage) BatchInsertPatternResults(ctx context.Context, results []*core.PatternResult) error {
	if len(results) == 0 {
		return nil
	}

	// Separate results by level
	var replayResults []*core.PatternResult
	var teamResults []*core.PatternResult
	var playerResults []*core.PatternResult

	for _, result := range results {
		switch result.Level {
		case core.LevelReplay:
			replayResults = append(replayResults, result)
		case core.LevelTeam:
			teamResults = append(teamResults, result)
		case core.LevelPlayer:
			playerResults = append(playerResults, result)
		}
	}

	// Insert replay-level results
	if len(replayResults) > 0 {
		if err := s.insertReplayPatternResults(ctx, replayResults); err != nil {
			return fmt.Errorf("failed to insert replay pattern results: %w", err)
		}
	}

	// Insert team-level results
	if len(teamResults) > 0 {
		if err := s.insertTeamPatternResults(ctx, teamResults); err != nil {
			return fmt.Errorf("failed to insert team pattern results: %w", err)
		}
	}

	// Insert player-level results
	if len(playerResults) > 0 {
		if err := s.insertPlayerPatternResults(ctx, playerResults); err != nil {
			return fmt.Errorf("failed to insert player pattern results: %w", err)
		}
	}

	return nil
}

// insertReplayPatternResults inserts replay-level pattern results
func (s *PostgresStorage) insertReplayPatternResults(ctx context.Context, results []*core.PatternResult) error {
	// First, get file_path and file_checksum for each replay_id
	replayIDs := make([]int64, 0, len(results))
	replayIDSet := make(map[int64]bool)
	for _, result := range results {
		if !replayIDSet[result.ReplayID] {
			replayIDs = append(replayIDs, result.ReplayID)
			replayIDSet[result.ReplayID] = true
		}
	}

	// Query for file info
	placeholders := make([]string, len(replayIDs))
	args := make([]any, len(replayIDs))
	for i, id := range replayIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, file_path, file_checksum FROM replays WHERE id IN (%s)
	`, strings.Join(placeholders, ", "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to query replay file info: %w", err)
	}
	defer rows.Close()

	replayFileInfo := make(map[int64]struct {
		filePath     string
		fileChecksum string
	})
	for rows.Next() {
		var id int64
		var filePath, fileChecksum string
		if err := rows.Scan(&id, &filePath, &fileChecksum); err != nil {
			return fmt.Errorf("failed to scan replay file info: %w", err)
		}
		replayFileInfo[id] = struct {
			filePath     string
			fileChecksum string
		}{filePath, fileChecksum}
	}

	// Now insert results
	const batchSize = 100
	for i := 0; i < len(results); i += batchSize {
		end := min(i+batchSize, len(results))
		batch := results[i:end]

		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]any, 0, len(batch)*7)
		argPos := 1

		for _, result := range batch {
			fileInfo, exists := replayFileInfo[result.ReplayID]
			if !exists {
				continue // Skip if we don't have file info
			}

			valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				argPos, argPos+1, argPos+2, argPos+3, argPos+4, argPos+5, argPos+6, argPos+7, argPos+8))
			valueArgs = append(valueArgs, result.ReplayID, core.AlgorithmVersion, fileInfo.filePath, fileInfo.fileChecksum, result.PatternName)
			argPos += 5

			// Add value fields (only one should be set)
			if result.ValueBool != nil {
				valueArgs = append(valueArgs, *result.ValueBool, nil, nil, nil)
			} else if result.ValueInt != nil {
				valueArgs = append(valueArgs, nil, *result.ValueInt, nil, nil)
			} else if result.ValueString != nil {
				valueArgs = append(valueArgs, nil, nil, *result.ValueString, nil)
			} else if result.ValueTime != nil {
				valueArgs = append(valueArgs, nil, nil, nil, *result.ValueTime)
			} else {
				valueArgs = append(valueArgs, nil, nil, nil, nil)
			}
			argPos += 4
		}

		if len(valueStrings) == 0 {
			continue
		}

		query := fmt.Sprintf(`
			INSERT INTO detected_patterns_replay 
			(replay_id, algorithm_version, file_path, file_checksum, pattern_name, value_bool, value_int, value_string, value_timestamp)
			VALUES %s
			ON CONFLICT (replay_id, pattern_name) DO UPDATE SET
				algorithm_version = EXCLUDED.algorithm_version,
				value_bool = EXCLUDED.value_bool,
				value_int = EXCLUDED.value_int,
				value_string = EXCLUDED.value_string,
				value_timestamp = EXCLUDED.value_timestamp
		`, strings.Join(valueStrings, ", "))

		if _, err := s.db.ExecContext(ctx, query, valueArgs...); err != nil {
			return fmt.Errorf("failed to insert replay pattern results: %w", err)
		}
	}

	return nil
}

// insertTeamPatternResults inserts team-level pattern results
func (s *PostgresStorage) insertTeamPatternResults(ctx context.Context, results []*core.PatternResult) error {
	const batchSize = 100
	for i := 0; i < len(results); i += batchSize {
		end := min(i+batchSize, len(results))
		batch := results[i:end]

		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]any, 0, len(batch)*6)
		argPos := 1

		for _, result := range batch {
			if result.Team == nil {
				continue // Skip if team is nil
			}

			valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				argPos, argPos+1, argPos+2, argPos+3, argPos+4, argPos+5, argPos+6))
			valueArgs = append(valueArgs, result.ReplayID, *result.Team, result.PatternName)
			argPos += 3

			// Add value fields (only one should be set)
			if result.ValueBool != nil {
				valueArgs = append(valueArgs, *result.ValueBool, nil, nil, nil)
			} else if result.ValueInt != nil {
				valueArgs = append(valueArgs, nil, *result.ValueInt, nil, nil)
			} else if result.ValueString != nil {
				valueArgs = append(valueArgs, nil, nil, *result.ValueString, nil)
			} else if result.ValueTime != nil {
				valueArgs = append(valueArgs, nil, nil, nil, *result.ValueTime)
			} else {
				valueArgs = append(valueArgs, nil, nil, nil, nil)
			}
			argPos += 4
		}

		if len(valueStrings) == 0 {
			continue
		}

		query := fmt.Sprintf(`
			INSERT INTO detected_patterns_replay_team 
			(replay_id, team, pattern_name, value_bool, value_int, value_string, value_timestamp)
			VALUES %s
			ON CONFLICT (replay_id, team, pattern_name) DO UPDATE SET
				value_bool = EXCLUDED.value_bool,
				value_int = EXCLUDED.value_int,
				value_string = EXCLUDED.value_string,
				value_timestamp = EXCLUDED.value_timestamp
		`, strings.Join(valueStrings, ", "))

		if _, err := s.db.ExecContext(ctx, query, valueArgs...); err != nil {
			return fmt.Errorf("failed to insert team pattern results: %w", err)
		}
	}

	return nil
}

// insertPlayerPatternResults inserts player-level pattern results
func (s *PostgresStorage) insertPlayerPatternResults(ctx context.Context, results []*core.PatternResult) error {
	const batchSize = 100
	for i := 0; i < len(results); i += batchSize {
		end := min(i+batchSize, len(results))
		batch := results[i:end]

		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]any, 0, len(batch)*6)
		argPos := 1

		for _, result := range batch {
			if result.PlayerID == nil {
				continue // Skip if player ID is nil
			}

			valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				argPos, argPos+1, argPos+2, argPos+3, argPos+4, argPos+5, argPos+6))
			valueArgs = append(valueArgs, result.ReplayID, *result.PlayerID, result.PatternName)
			argPos += 3

			// Add value fields (only one should be set)
			if result.ValueBool != nil {
				valueArgs = append(valueArgs, *result.ValueBool, nil, nil, nil)
			} else if result.ValueInt != nil {
				valueArgs = append(valueArgs, nil, *result.ValueInt, nil, nil)
			} else if result.ValueString != nil {
				valueArgs = append(valueArgs, nil, nil, *result.ValueString, nil)
			} else if result.ValueTime != nil {
				valueArgs = append(valueArgs, nil, nil, nil, *result.ValueTime)
			} else {
				valueArgs = append(valueArgs, nil, nil, nil, nil)
			}
			argPos += 4
		}

		if len(valueStrings) == 0 {
			continue
		}

		query := fmt.Sprintf(`
			INSERT INTO detected_patterns_replay_player 
			(replay_id, player_id, pattern_name, value_bool, value_int, value_string, value_timestamp)
			VALUES %s
			ON CONFLICT (replay_id, player_id, pattern_name) DO UPDATE SET
				value_bool = EXCLUDED.value_bool,
				value_int = EXCLUDED.value_int,
				value_string = EXCLUDED.value_string,
				value_timestamp = EXCLUDED.value_timestamp
		`, strings.Join(valueStrings, ", "))

		if _, err := s.db.ExecContext(ctx, query, valueArgs...); err != nil {
			return fmt.Errorf("failed to insert player pattern results: %w", err)
		}
	}

	return nil
}

// Close closes the database connection
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

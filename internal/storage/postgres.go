package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-jet/jet/v2/qrm"
	_ "github.com/lib/pq"
	"github.com/marianogappa/screpdb/internal/fileops"
	jetmodel "github.com/marianogappa/screpdb/internal/jet/screpdb/public/model"
	"github.com/marianogappa/screpdb/internal/jet/screpdb/public/table"
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
// If clean is true, drops all non-dashboard tables before creating new ones
// If cleanDashboard is true, drops all dashboard tables
func (s *PostgresStorage) Initialize(ctx context.Context, clean bool, cleanDashboard bool) error {
	// Handle dashboard cleanup first if requested
	if cleanDashboard {
		if err := migrations.DropDashboardTables(s.connectionString); err != nil {
			return fmt.Errorf("failed to drop dashboard tables: %w", err)
		}
	}

	// Handle non-dashboard cleanup if requested
	if clean {
		if err := migrations.CleanNonDashboardAndRunMigrations(s.connectionString); err != nil {
			return fmt.Errorf("failed to clean and run migrations: %w", err)
		}
		return nil
	}

	// Run migrations normally (they will create tables if they don't exist)
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
	// Use a transaction to ensure all inserts are atomic and foreign key constraints work
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Insert replay sequentially (with RETURNING)
	replayID, err := s.insertReplaySequentialTx(ctx, tx, data.Replay)
	if err != nil {
		return fmt.Errorf("failed to insert replay: %w", err)
	}
	if replayID == 0 {
		return fmt.Errorf("replay insert returned invalid ID: 0")
	}

	// Step 2: Insert players in batch (with RETURNING)
	playerIDs, err := s.insertPlayersBatchTx(ctx, tx, replayID, data.Players)
	if err != nil {
		return fmt.Errorf("failed to insert players: %w", err)
	}

	// Step 3: Update commands with correct IDs and insert them
	s.updateEntityIDs(data, replayID, playerIDs)

	// Step 4: Insert commands in batch
	if len(data.Commands) > 0 {
		if err := s.insertCommandsBatchTx(ctx, tx, data.Commands); err != nil {
			return fmt.Errorf("failed to insert commands: %w", err)
		}
	}

	// Step 5: Process pattern detection results if orchestrator is present
	if data.PatternOrchestrator != nil {
		if err := s.processPatternResultsTx(ctx, tx, data.PatternOrchestrator, replayID, playerIDs); err != nil {
			return fmt.Errorf("failed to process pattern results: %w", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// processPatternResults processes pattern detection results (uses default connection)
func (s *PostgresStorage) processPatternResults(ctx context.Context, orchestrator any, replayID int64, playerIDMap map[byte]int64) error {
	return s.processPatternResultsTx(ctx, s.db, orchestrator, replayID, playerIDMap)
}

// processPatternResultsTx processes pattern detection results (uses provided connection/transaction)
func (s *PostgresStorage) processPatternResultsTx(ctx context.Context, db qrm.Executable, orchestrator any, replayID int64, playerIDMap map[byte]int64) error {
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
		if err := s.BatchInsertPatternResultsTx(ctx, db, results); err != nil {
			return fmt.Errorf("failed to insert pattern results: %w", err)
		}
	}

	return nil
}

// insertReplaySequential inserts a single replay and returns its ID (uses default connection)
func (s *PostgresStorage) insertReplaySequential(ctx context.Context, replay *models.Replay) (int64, error) {
	return s.insertReplaySequentialTx(ctx, s.db, replay)
}

// insertReplaySequentialTx inserts a single replay and returns its ID (uses provided connection/transaction)
func (s *PostgresStorage) insertReplaySequentialTx(ctx context.Context, db qrm.Queryable, replay *models.Replay) (int64, error) {
	stmt := table.Replays.INSERT(
		table.Replays.FilePath,
		table.Replays.FileChecksum,
		table.Replays.FileName,
		table.Replays.CreatedAt,
		table.Replays.ReplayDate,
		table.Replays.Title,
		table.Replays.Host,
		table.Replays.MapName,
		table.Replays.MapWidth,
		table.Replays.MapHeight,
		table.Replays.DurationSeconds,
		table.Replays.FrameCount,
		table.Replays.EngineVersion,
		table.Replays.Engine,
		table.Replays.GameSpeed,
		table.Replays.GameType,
		table.Replays.HomeTeamSize,
		table.Replays.AvailSlotsCount,
	).VALUES(
		replay.FilePath,
		replay.FileChecksum,
		replay.FileName,
		replay.CreatedAt,
		replay.ReplayDate,
		replay.Title,
		replay.Host,
		replay.MapName,
		int32(replay.MapWidth),
		int32(replay.MapHeight),
		int32(replay.DurationSeconds),
		replay.FrameCount,
		replay.EngineVersion,
		replay.Engine,
		replay.GameSpeed,
		replay.GameType,
		fmt.Sprintf("%d", replay.HomeTeamSize),
		int32(replay.AvailSlotsCount),
	).RETURNING(table.Replays.ID)

	var result jetmodel.Replays
	err := stmt.QueryContext(ctx, db, &result)
	if err != nil {
		return 0, fmt.Errorf("failed to insert replay: %w", err)
	}

	if result.ID == 0 {
		return 0, fmt.Errorf("replay insert returned invalid ID: 0")
	}

	return int64(result.ID), nil
}

// insertPlayersBatch inserts all players for a replay in a single batch and returns player ID mapping (uses default connection)
func (s *PostgresStorage) insertPlayersBatch(ctx context.Context, replayID int64, players []*models.Player) (map[byte]int64, error) {
	return s.insertPlayersBatchTx(ctx, s.db, replayID, players)
}

// insertPlayersBatchTx inserts all players for a replay in a single batch and returns player ID mapping (uses provided connection/transaction)
func (s *PostgresStorage) insertPlayersBatchTx(ctx context.Context, db qrm.Queryable, replayID int64, players []*models.Player) (map[byte]int64, error) {
	if len(players) == 0 {
		return make(map[byte]int64), nil
	}

	// Build the batch insert statement using jet
	stmt := table.Players.INSERT(
		table.Players.ReplayID,
		table.Players.Name,
		table.Players.Race,
		table.Players.Type,
		table.Players.Color,
		table.Players.Team,
		table.Players.IsObserver,
		table.Players.Apm,
		table.Players.Eapm,
		table.Players.IsWinner,
		table.Players.StartLocationX,
		table.Players.StartLocationY,
		table.Players.StartLocationOclock,
	)

	// Add all players
	for _, player := range players {
		var startX, startY, startOclock *int32
		if player.StartLocationX != nil {
			x := int32(*player.StartLocationX)
			startX = &x
		}
		if player.StartLocationY != nil {
			y := int32(*player.StartLocationY)
			startY = &y
		}
		if player.StartLocationOclock != nil {
			o := int32(*player.StartLocationOclock)
			startOclock = &o
		}

		stmt = stmt.VALUES(
			int32(replayID),
			player.Name,
			player.Race,
			player.Type,
			player.Color,
			int32(player.Team),
			player.IsObserver,
			int32(player.APM),
			int32(player.EAPM),
			player.IsWinner,
			startX,
			startY,
			startOclock,
		)
	}

	stmt = stmt.RETURNING(table.Players.ID)

	var results []jetmodel.Players
	err := stmt.QueryContext(ctx, db, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to insert players batch: %w", err)
	}

	// Map player IDs back to player PlayerID (byte)
	// The results should be in the same order as the inserted players
	if len(results) != len(players) {
		return nil, fmt.Errorf("mismatch: inserted %d players but got %d IDs back", len(players), len(results))
	}

	playerIDMap := make(map[byte]int64)
	for i, result := range results {
		if result.ID == 0 {
			return nil, fmt.Errorf("player insert returned invalid ID: 0 for player %d", i)
		}
		playerIDMap[players[i].PlayerID] = int64(result.ID)
	}

	return playerIDMap, nil
}

// insertCommandsBatch inserts all commands for a replay in batches (uses default connection)
func (s *PostgresStorage) insertCommandsBatch(ctx context.Context, commands []*models.Command) error {
	return s.insertCommandsBatchTx(ctx, s.db, commands)
}

// insertCommandsBatchTx inserts all commands for a replay in batches (uses provided connection/transaction)
func (s *PostgresStorage) insertCommandsBatchTx(ctx context.Context, db qrm.Executable, commands []*models.Command) error {
	if len(commands) == 0 {
		return nil
	}

	// Process in batches
	for i := 0; i < len(commands); i += batchSize {
		end := min(i+batchSize, len(commands))
		batch := commands[i:end]
		if err := s.insertCommandsBatchChunkTx(ctx, db, batch); err != nil {
			return fmt.Errorf("failed to insert commands batch: %w", err)
		}
	}

	return nil
}

// insertCommandsBatchChunkTx inserts a chunk of commands (uses provided connection/transaction)
func (s *PostgresStorage) insertCommandsBatchChunkTx(ctx context.Context, db qrm.Executable, commands []*models.Command) error {
	if len(commands) == 0 {
		return nil
	}

	// Build the batch insert statement using jet
	stmt := table.Commands.INSERT(
		table.Commands.ReplayID,
		table.Commands.PlayerID,
		table.Commands.Frame,
		table.Commands.SecondsFromGameStart,
		table.Commands.RunAt,
		table.Commands.ActionType,
		table.Commands.X,
		table.Commands.Y,
		table.Commands.IsQueued,
		table.Commands.OrderName,
		table.Commands.UnitType,
		table.Commands.UnitTypes,
		table.Commands.TechName,
		table.Commands.UpgradeName,
		table.Commands.HotkeyType,
		table.Commands.HotkeyGroup,
		table.Commands.GameSpeed,
		table.Commands.VisionPlayerIds,
		table.Commands.AlliancePlayerIds,
		table.Commands.IsAlliedVictory,
		table.Commands.GeneralData,
		table.Commands.ChatMessage,
		table.Commands.LeaveReason,
	)

	// Add all commands as VALUES
	for _, command := range commands {
		// Serialize player IDs to JSON string
		var visionPlayerIdsJSON, alliancePlayerIdsJSON *string
		if command.VisionPlayerIDs != nil {
			data, err := json.Marshal(*command.VisionPlayerIDs)
			if err == nil {
				s := string(data)
				visionPlayerIdsJSON = &s
			}
		}
		if command.AlliancePlayerIDs != nil {
			data, err := json.Marshal(*command.AlliancePlayerIDs)
			if err == nil {
				s := string(data)
				alliancePlayerIdsJSON = &s
			}
		}

		// Convert unit type (convert "None" to null)
		var unitType *string
		if command.UnitType != nil && *command.UnitType != "None" {
			unitType = command.UnitType
		}

		// Convert nullable int fields
		var x, y *int32
		if command.X != nil {
			val := int32(*command.X)
			x = &val
		}
		if command.Y != nil {
			val := int32(*command.Y)
			y = &val
		}

		// Convert nullable byte to int32
		var hotkeyGroup *int32
		if command.HotkeyGroup != nil {
			val := int32(*command.HotkeyGroup)
			hotkeyGroup = &val
		}

		stmt = stmt.VALUES(
			int32(command.ReplayID),
			int32(command.PlayerID),
			command.Frame,
			int32(command.SecondsFromGameStart),
			command.RunAt,
			command.ActionType,
			x,
			y,
			command.IsQueued,
			command.OrderName,
			unitType,
			command.UnitTypes,
			command.TechName,
			command.UpgradeName,
			command.HotkeyType,
			hotkeyGroup,
			command.GameSpeed,
			visionPlayerIdsJSON,
			alliancePlayerIdsJSON,
			command.IsAlliedVictory,
			command.GeneralData,
			command.ChatMessage,
			command.LeaveReason,
		)
	}

	_, err := stmt.ExecContext(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to insert commands batch: %w", err)
	}

	return nil
}

// updateEntityIDs updates all entities with the correct replay ID and player IDs
func (s *PostgresStorage) updateEntityIDs(data *models.ReplayData, replayID int64, playerIDs map[byte]int64) {
	// Update commands
	for _, command := range data.Commands {
		command.ReplayID = replayID
		// command.PlayerID at this point is the replay's player ID (byte), not the database ID
		// We need to look it up in the map
		originalPlayerID := byte(command.PlayerID)
		if playerID, exists := playerIDs[originalPlayerID]; exists {
			command.PlayerID = playerID
		} else {
			// This shouldn't happen, but log it if it does
			fmt.Printf("Warning: player ID %d not found in playerIDs map (map has %d entries)\n", originalPlayerID, len(playerIDs))
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

// BatchInsertPatternResults inserts pattern detection results in batch (uses default connection)
func (s *PostgresStorage) BatchInsertPatternResults(ctx context.Context, results []*core.PatternResult) error {
	return s.BatchInsertPatternResultsTx(ctx, s.db, results)
}

// BatchInsertPatternResultsTx inserts pattern detection results in batch (uses provided connection/transaction)
func (s *PostgresStorage) BatchInsertPatternResultsTx(ctx context.Context, db qrm.Executable, results []*core.PatternResult) error {
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
		// Convert qrm.Executable to qrm.Queryable for insertReplayPatternResultsTx
		var queryDB qrm.Queryable
		if tx, ok := db.(*sql.Tx); ok {
			queryDB = tx
		} else if sqlDB, ok := db.(*sql.DB); ok {
			queryDB = sqlDB
		} else {
			return fmt.Errorf("unsupported database type")
		}
		if err := s.insertReplayPatternResultsTx(ctx, queryDB, replayResults); err != nil {
			return fmt.Errorf("failed to insert replay pattern results: %w", err)
		}
	}

	// Insert team-level results
	if len(teamResults) > 0 {
		if err := s.insertTeamPatternResultsTx(ctx, db, teamResults); err != nil {
			return fmt.Errorf("failed to insert team pattern results: %w", err)
		}
	}

	// Insert player-level results
	if len(playerResults) > 0 {
		if err := s.insertPlayerPatternResultsTx(ctx, db, playerResults); err != nil {
			return fmt.Errorf("failed to insert player pattern results: %w", err)
		}
	}

	return nil
}

// insertReplayPatternResults inserts replay-level pattern results (uses default connection)
func (s *PostgresStorage) insertReplayPatternResults(ctx context.Context, results []*core.PatternResult) error {
	return s.insertReplayPatternResultsTx(ctx, s.db, results)
}

// insertReplayPatternResultsTx inserts replay-level pattern results (uses provided connection/transaction)
func (s *PostgresStorage) insertReplayPatternResultsTx(ctx context.Context, db qrm.Queryable, results []*core.PatternResult) error {
	// db needs to support both Query and Exec, so we'll use sql.DB or sql.Tx
	var queryDB *sql.DB
	var queryTx *sql.Tx
	if tx, ok := db.(*sql.Tx); ok {
		queryTx = tx
	} else if sqlDB, ok := db.(*sql.DB); ok {
		queryDB = sqlDB
	} else {
		return fmt.Errorf("unsupported database type")
	}
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

	var rows *sql.Rows
	var err error
	if queryTx != nil {
		rows, err = queryTx.QueryContext(ctx, query, args...)
	} else {
		rows, err = queryDB.QueryContext(ctx, query, args...)
	}
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

		var execErr error
		if queryTx != nil {
			_, execErr = queryTx.ExecContext(ctx, query, valueArgs...)
		} else {
			_, execErr = queryDB.ExecContext(ctx, query, valueArgs...)
		}
		if execErr != nil {
			return fmt.Errorf("failed to insert replay pattern results: %w", execErr)
		}
	}

	return nil
}

// insertTeamPatternResults inserts team-level pattern results (uses default connection)
func (s *PostgresStorage) insertTeamPatternResults(ctx context.Context, results []*core.PatternResult) error {
	return s.insertTeamPatternResultsTx(ctx, s.db, results)
}

// insertTeamPatternResultsTx inserts team-level pattern results (uses provided connection/transaction)
func (s *PostgresStorage) insertTeamPatternResultsTx(ctx context.Context, db qrm.Executable, results []*core.PatternResult) error {
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

		if _, err := db.ExecContext(ctx, query, valueArgs...); err != nil {
			return fmt.Errorf("failed to insert team pattern results: %w", err)
		}
	}

	return nil
}

// insertPlayerPatternResults inserts player-level pattern results (uses default connection)
func (s *PostgresStorage) insertPlayerPatternResults(ctx context.Context, results []*core.PatternResult) error {
	return s.insertPlayerPatternResultsTx(ctx, s.db, results)
}

// insertPlayerPatternResultsTx inserts player-level pattern results (uses provided connection/transaction)
func (s *PostgresStorage) insertPlayerPatternResultsTx(ctx context.Context, db qrm.Executable, results []*core.PatternResult) error {
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

		if _, err := db.ExecContext(ctx, query, valueArgs...); err != nil {
			return fmt.Errorf("failed to insert player pattern results: %w", err)
		}
	}

	return nil
}

// Close closes the database connection
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

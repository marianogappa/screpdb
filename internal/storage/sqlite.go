package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/rep/repcore"
	_ "modernc.org/sqlite"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/migrations"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// SQLiteStorage implements Storage interface using SQLite
type SQLiteStorage struct {
	db               *sql.DB
	dbPath           string
	storeRightClicks bool
	skipHotkeys      bool
}

type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

const unknownEnumValue = "UNKNOWN"

var (
	allowedCommandActionTypes = enumSetFromNames(typeNamesFromRepcmd())
	allowedCommandOrders      = enumSetFromNames(orderNamesFromRepcmd())
	allowedCommandUnits       = enumSetFromNames(unitNamesFromRepcmd())
	allowedCommandTechs       = enumSetFromNames(techNamesFromRepcmd())
	allowedCommandUpgrades    = enumSetFromNames(upgradeNamesFromRepcmd())
	allowedCommandHotkeys     = enumSetFromNames(hotkeyNamesFromRepcmd())
	allowedLeaveReasons       = enumSetFromNames(leaveReasonNamesFromRepcmd())
	allowedCommandSpeeds      = enumSetFromNames([]string{
		"Slowest", "Slower", "Slow", "Normal", "Fast", "Faster", "Fastest", unknownEnumValue,
	})
	allowedPlayerRaces      = enumSetFromNames(playerRaceNamesFromRepcore())
	allowedPlayerTypes      = enumSetFromNames(playerTypeNamesFromRepcore())
	allowedPlayerColors     = enumSetFromNames(playerColorNamesFromRepcore())
	allowedReplayEventTypes = enumSetFromNames([]string{
		"player_start",
		"leave_game",
		"expansion",
		"attack",
		"scout",
		"drop",
		"reaver_drop",
		"dt_drop",
		"recall",
		"nuke",
		"cannon_rush",
		"bunker_rush",
		"zergling_rush",
		"proxy_gate",
		"proxy_rax",
		"proxy_factory",
		"location_inactive",
		"takeover",
		"became_terran",
		"became_zerg",
	})
	allowedReplayEventLocationTypes = enumSetFromNames([]string{
		"starting",
		"natural",
		"expansion",
	})
	lowValueActionTypes = enumSetFromNames([]string{
		"Right Click",
		"Hotkey",
		"Minimap Ping",
		"Alliance",
		"Vision",
		"Cheat",
		"Game Speed",
	})
)

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	dsn := sqliteDSN(dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// SQLite write-path tuning for ingestion-heavy workloads.
	if _, err := db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		return nil, fmt.Errorf("failed to set journal_mode=WAL: %w", err)
	}
	if _, err := db.Exec(`PRAGMA synchronous = NORMAL;`); err != nil {
		return nil, fmt.Errorf("failed to set synchronous=NORMAL: %w", err)
	}
	if _, err := db.Exec(`PRAGMA temp_store = MEMORY;`); err != nil {
		return nil, fmt.Errorf("failed to set temp_store=MEMORY: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		return nil, fmt.Errorf("failed to set busy_timeout=5000: %w", err)
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &SQLiteStorage{db: db, dbPath: dbPath}, nil
}

// SetCommandStorageOptions controls low-value command persistence behavior.
func (s *SQLiteStorage) SetCommandStorageOptions(storeRightClicks bool, skipHotkeys bool) {
	s.storeRightClicks = storeRightClicks
	s.skipHotkeys = skipHotkeys
}

// Initialize creates the database schema using migrations
// If clean is true, drops all non-dashboard tables before creating new ones
// If cleanDashboard is true, drops all dashboard tables
func (s *SQLiteStorage) Initialize(ctx context.Context, clean bool, cleanDashboard bool) error {
	_ = ctx

	// Drop dashboard migrations if requested
	if cleanDashboard {
		if err := migrations.DropMigrationSet(s.dbPath, migrations.MigrationSetDashboard); err != nil {
			return fmt.Errorf("failed to drop dashboard migrations: %w", err)
		}
	}

	// Drop replay migrations if requested
	if clean {
		if err := migrations.DropMigrationSet(s.dbPath, migrations.MigrationSetReplay); err != nil {
			return fmt.Errorf("failed to drop replay migrations: %w", err)
		}
	}

	// Always run both migration sets to ensure everything is up to date
	if err := migrations.RunMigrationSet(s.dbPath, migrations.MigrationSetReplay); err != nil {
		return fmt.Errorf("failed to run replay migrations: %w", err)
	}
	if err := migrations.RunMigrationSet(s.dbPath, migrations.MigrationSetDashboard); err != nil {
		return fmt.Errorf("failed to run dashboard migrations: %w", err)
	}
	return nil
}

// StartIngestion starts the ingestion process with batching
func (s *SQLiteStorage) StartIngestion(ctx context.Context, hooks IngestionHooks) (ReplayDataChannel, <-chan error) {
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
					if isDuplicateReplayError(err) {
						if hooks.OnDuplicateReplay != nil {
							hooks.OnDuplicateReplay(err)
						} else {
							fmt.Printf("Skipping duplicate replay: %v\n", err)
						}
						continue
					}
					if hooks.OnStoreError != nil {
						hooks.OnStoreError(err)
					} else {
						fmt.Printf("Error storing replay: %v\n", err)
					}
					errChan <- err
					return
				}
				if hooks.OnReplayStored != nil {
					hooks.OnReplayStored()
				}

			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}
		}
	}()

	return dataChan, errChan
}

func isDuplicateReplayError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: replays.file_checksum")
}

// storeReplayWithBatching stores a replay data structure using sequential processing
func (s *SQLiteStorage) storeReplayWithBatching(ctx context.Context, data *models.ReplayData) error {
	// Use a transaction to ensure all inserts are atomic and foreign key constraints work
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Insert replay sequentially
	replayID, err := s.insertReplaySequentialTx(ctx, tx, data.Replay)
	if err != nil {
		return fmt.Errorf("failed to insert replay: %w", err)
	}
	if replayID == 0 {
		return fmt.Errorf("replay insert returned invalid ID: 0")
	}

	// Step 2: Insert players and map IDs
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

// processPatternResultsTx processes pattern detection results (uses provided connection/transaction)
func (s *SQLiteStorage) processPatternResultsTx(ctx context.Context, db dbtx, orchestrator any, replayID int64, playerIDMap map[byte]int64) error {
	// Type assert to *patterns.Orchestrator
	patternOrch, ok := orchestrator.(*patterns.Orchestrator)
	if !ok {
		return nil // Not a pattern orchestrator, skip
	}

	// Convert replay player IDs to database IDs
	// Get results
	results := patternOrch.GetResults()
	replayEvents := patternOrch.ReplayEvents()

	if len(replayEvents) > 0 {
		if err := s.insertReplayEventsTx(ctx, db, replayID, replayEvents, playerIDMap); err != nil {
			return fmt.Errorf("failed to insert replay events: %w", err)
		}
	}

	// Convert replay player IDs to database IDs
	patternOrch.ConvertResultsToDatabaseIDs(playerIDMap)

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

// insertReplaySequentialTx inserts a single replay and returns its ID (uses provided connection/transaction)
func (s *SQLiteStorage) insertReplaySequentialTx(ctx context.Context, db dbtx, replay *models.Replay) (int64, error) {
	query := `
		INSERT INTO replays (
			file_path, file_checksum, file_name, created_at, replay_date, title, host, map_name, map_width, map_height,
			duration_seconds, frame_count, engine_version, engine, game_speed, game_type, home_team_size, avail_slots_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	res, err := db.ExecContext(ctx, query,
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
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert replay: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get replay ID: %w", err)
	}
	if id == 0 {
		return 0, fmt.Errorf("replay insert returned invalid ID: 0")
	}

	return id, nil
}

// insertPlayersBatchTx inserts all players for a replay and returns player ID mapping (uses provided connection/transaction)
func (s *SQLiteStorage) insertPlayersBatchTx(ctx context.Context, db dbtx, replayID int64, players []*models.Player) (map[byte]int64, error) {
	if len(players) == 0 {
		return make(map[byte]int64), nil
	}

	columns := []string{
		"replay_id",
		"name",
		"race",
		"type",
		"color",
		"team",
		"is_observer",
		"apm",
		"eapm",
		"is_winner",
		"start_location_x",
		"start_location_y",
		"start_location_oclock",
	}

	valueStrings := make([]string, 0, len(players))
	valueArgs := make([]any, 0, len(players)*len(columns))

	for _, player := range players {
		race := normalizeEnumValue(player.Race, allowedPlayerRaces)
		playerType := normalizeEnumValue(player.Type, allowedPlayerTypes)
		color := normalizeEnumValue(player.Color, allowedPlayerColors)

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

		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		valueArgs = append(valueArgs,
			int32(replayID),
			player.Name,
			race,
			playerType,
			color,
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

	query := fmt.Sprintf(
		"INSERT INTO players (%s) VALUES %s RETURNING id",
		strings.Join(columns, ", "),
		strings.Join(valueStrings, ", "),
	)

	rows, err := db.QueryContext(ctx, query, valueArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert players batch: %w", err)
	}
	defer rows.Close()

	playerIDMap := make(map[byte]int64)
	i := 0
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan player ID: %w", err)
		}
		if i >= len(players) {
			return nil, fmt.Errorf("received more player IDs than inserted")
		}
		playerIDMap[players[i].PlayerID] = id
		i++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate player IDs: %w", err)
	}
	if i != len(players) {
		return nil, fmt.Errorf("mismatch: inserted %d players but got %d IDs back", len(players), i)
	}

	return playerIDMap, nil
}

// insertCommandsBatchTx inserts all commands for a replay in batches (uses provided connection/transaction)
func (s *SQLiteStorage) insertCommandsBatchTx(ctx context.Context, db dbtx, commands []*models.Command) error {
	if len(commands) == 0 {
		return nil
	}

	insertCommandSQL := `
		INSERT INTO %s (
			replay_id, player_id, frame, seconds_from_game_start, action_type,
			x, y, is_queued, order_name, unit_type, unit_types, tech_name, upgrade_name,
			hotkey_type, hotkey_group, game_speed, vision_player_ids, alliance_player_ids,
			is_allied_victory, general_data, chat_message, leave_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	highValueStmt, err := db.PrepareContext(ctx, fmt.Sprintf(insertCommandSQL, "commands"))
	if err != nil {
		return fmt.Errorf("failed to prepare command insert statement: %w", err)
	}
	defer highValueStmt.Close()

	lowValueStmt, err := db.PrepareContext(ctx, fmt.Sprintf(insertCommandSQL, "commands_low_value"))
	if err != nil {
		return fmt.Errorf("failed to prepare low-value command insert statement: %w", err)
	}
	defer lowValueStmt.Close()

	for _, command := range commands {
		actionType := normalizeEnumValue(command.ActionType, allowedCommandActionTypes)
		if actionType == "Right Click" && !s.storeRightClicks {
			continue
		}
		if actionType == "Hotkey" && s.skipHotkeys {
			continue
		}

		targetStmt := highValueStmt
		if _, ok := lowValueActionTypes[actionType]; ok {
			targetStmt = lowValueStmt
		}

		orderName := normalizeNullableEnum(command.OrderName, allowedCommandOrders)
		techName := normalizeNullableEnum(command.TechName, allowedCommandTechs)
		upgradeName := normalizeNullableEnum(command.UpgradeName, allowedCommandUpgrades)
		hotkeyType := normalizeNullableEnum(command.HotkeyType, allowedCommandHotkeys)
		commandSpeed := normalizeNullableEnum(command.GameSpeed, allowedCommandSpeeds)
		leaveReason := normalizeNullableEnum(command.LeaveReason, allowedLeaveReasons)

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
		normalizedUnitType := normalizeNullableEnum(command.UnitType, allowedCommandUnits)
		if normalizedUnitType != nil && *normalizedUnitType != "None" {
			unitType = normalizedUnitType
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

		if _, err := targetStmt.ExecContext(ctx,
			int32(command.ReplayID),
			int32(command.PlayerID),
			command.Frame,
			int32(command.SecondsFromGameStart),
			actionType,
			x,
			y,
			command.IsQueued,
			orderName,
			unitType,
			command.UnitTypes,
			techName,
			upgradeName,
			hotkeyType,
			hotkeyGroup,
			commandSpeed,
			visionPlayerIdsJSON,
			alliancePlayerIdsJSON,
			command.IsAlliedVictory,
			command.GeneralData,
			command.ChatMessage,
			leaveReason,
		); err != nil {
			return fmt.Errorf("failed to execute command insert: %w", err)
		}
	}

	return nil
}

// updateEntityIDs updates all entities with the correct replay ID and player IDs
func (s *SQLiteStorage) updateEntityIDs(data *models.ReplayData, replayID int64, playerIDs map[byte]int64) {
	data.Replay.ID = replayID
	for i := range data.Players {
		data.Players[i].ID = playerIDs[data.Players[i].PlayerID]
	}
	for i := range data.Commands {
		data.Commands[i].ReplayID = replayID
		data.Commands[i].PlayerID = data.Commands[i].Player.ID

		// TODO Special case: VisionPlayerIDs hold slot ids
		// TODO Special case: AlliancePlayerIDs
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

// FilterOutExistingReplays filters out replays that already exist in the database
func (s *SQLiteStorage) FilterOutExistingReplays(ctx context.Context, files []fileops.FileInfo) ([]fileops.FileInfo, error) {
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
		filePathPlaceholders[i] = "?"
	}

	// Build placeholders for checksums
	checksumPlaceholders := make([]string, len(checksums))
	for i := range checksums {
		checksumPlaceholders[i] = "?"
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
	tables := []string{"replays", "players", "commands", "commands_low_value"}

	var schema strings.Builder
	schema.WriteString("# Database Schema\n\n")

	for _, tableName := range tables {
		query := fmt.Sprintf("PRAGMA table_info(%s);", tableName)
		rows, err := s.db.QueryContext(ctx, query)
		if err != nil {
			return "", fmt.Errorf("failed to query schema for %s: %w", tableName, err)
		}

		var columns []struct {
			name     string
			dataType string
			nullable string
		}

		for rows.Next() {
			var cid int
			var name, dataType string
			var notNull int
			var dflt any
			var pk int
			if err := rows.Scan(&cid, &name, &dataType, &notNull, &dflt, &pk); err != nil {
				rows.Close()
				return "", fmt.Errorf("failed to scan schema info for %s: %w", tableName, err)
			}
			nullable := "YES"
			if notNull == 1 || pk == 1 {
				nullable = "NO"
			}
			columns = append(columns, struct {
				name     string
				dataType string
				nullable string
			}{name: name, dataType: dataType, nullable: nullable})
		}
		rows.Close()

		schema.WriteString(fmt.Sprintf("## %s\n\n", tableName))
		schema.WriteString("| Column | Type | Nullable |\n")
		schema.WriteString("|--------|------|----------|\n")
		for _, col := range columns {
			schema.WriteString(fmt.Sprintf("| %s | %s | %s |\n", col.name, col.dataType, col.nullable))
		}
		schema.WriteString("\n")
	}

	return schema.String(), nil
}

// FilterOutExistingPatternDetections filters out replays that already have pattern detection run
// with the current or higher algorithm version
func (s *SQLiteStorage) FilterOutExistingPatternDetections(ctx context.Context, files []fileops.FileInfo, algorithmVersion int) ([]fileops.FileInfo, error) {
	if len(files) == 0 {
		return []fileops.FileInfo{}, nil
	}
	_ = algorithmVersion

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
		filePathPlaceholders[i] = "?"
	}

	// Build placeholders for checksums
	checksumPlaceholders := make([]string, len(checksums))
	for i := range checksums {
		checksumPlaceholders[i] = "?"
	}

	// Combine all args: file_paths first, then checksums.
	args := make([]any, 0, len(filePaths)+len(checksums))
	for _, fp := range filePaths {
		args = append(args, fp)
	}
	for _, cs := range checksums {
		args = append(args, cs)
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT r.file_path, r.file_checksum
		FROM replay_events re
		JOIN replays r ON r.id = re.replay_id
		WHERE (r.file_path IN (%s) OR r.file_checksum IN (%s))
	`, strings.Join(filePathPlaceholders, ", "), strings.Join(checksumPlaceholders, ", "))

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
func (s *SQLiteStorage) DeletePatternDetectionsForReplay(ctx context.Context, replayID int64) error {
	// Delete from all pattern/event tables.
	queries := []string{
		"DELETE FROM replay_events WHERE replay_id = ?",
		"DELETE FROM detected_patterns_replay WHERE replay_id = ?",
		"DELETE FROM detected_patterns_replay_player WHERE replay_id = ?",
	}

	for _, query := range queries {
		if _, err := s.db.ExecContext(ctx, query, replayID); err != nil {
			return fmt.Errorf("failed to delete pattern detections: %w", err)
		}
	}

	return nil
}

// BatchInsertPatternResults inserts pattern detection results in batch (uses default connection)
func (s *SQLiteStorage) BatchInsertPatternResults(ctx context.Context, results []*core.PatternResult) error {
	return s.BatchInsertPatternResultsTx(ctx, s.db, results)
}

// BatchInsertPatternResultsTx inserts pattern detection results in batch (uses provided connection/transaction)
func (s *SQLiteStorage) BatchInsertPatternResultsTx(ctx context.Context, db dbtx, results []*core.PatternResult) error {
	if len(results) == 0 {
		return nil
	}

	// Separate results by level
	var replayResults []*core.PatternResult
	var playerResults []*core.PatternResult

	for _, result := range results {
		switch result.Level {
		case core.LevelReplay:
			replayResults = append(replayResults, result)
		case core.LevelPlayer:
			playerResults = append(playerResults, result)
		}
	}

	// Insert replay-level results
	if len(replayResults) > 0 {
		if err := s.insertReplayPatternResultsTx(ctx, db, replayResults); err != nil {
			return fmt.Errorf("failed to insert replay pattern results: %w", err)
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

// insertReplayPatternResultsTx inserts replay-level pattern results (uses provided connection/transaction)
func (s *SQLiteStorage) insertReplayPatternResultsTx(ctx context.Context, db dbtx, results []*core.PatternResult) error {
	const batchSize = 100
	for i := 0; i < len(results); i += batchSize {
		end := min(i+batchSize, len(results))
		batch := results[i:end]

		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]any, 0, len(batch)*7)

		for _, result := range batch {
			valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?)")
			valueArgs = append(valueArgs, result.ReplayID, core.AlgorithmVersion, result.PatternName)

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
		}

		if len(valueStrings) == 0 {
			continue
		}

		query := fmt.Sprintf(`
			INSERT INTO detected_patterns_replay
			(replay_id, algorithm_version, pattern_name, value_bool, value_int, value_string, value_timestamp)
			VALUES %s
			ON CONFLICT (replay_id, pattern_name) DO UPDATE SET
				algorithm_version = excluded.algorithm_version,
				value_bool = excluded.value_bool,
				value_int = excluded.value_int,
				value_string = excluded.value_string,
				value_timestamp = excluded.value_timestamp
		`, strings.Join(valueStrings, ", "))

		if _, err := db.ExecContext(ctx, query, valueArgs...); err != nil {
			return fmt.Errorf("failed to insert replay pattern results: %w", err)
		}
	}

	return nil
}

func (s *SQLiteStorage) insertReplayEventsTx(ctx context.Context, db dbtx, replayID int64, events []worldstate.ReplayEvent, playerIDMap map[byte]int64) error {
	const batchSize = 200
	for i := 0; i < len(events); i += batchSize {
		end := min(i+batchSize, len(events))
		batch := events[i:end]

		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]any, 0, len(batch)*10)

		for _, event := range batch {
			eventType := normalizeEnumValue(event.EventType, allowedReplayEventTypes)
			var locationType *string
			if event.LocationBaseType != nil {
				trimmed := strings.TrimSpace(*event.LocationBaseType)
				if trimmed != "" {
					if _, ok := allowedReplayEventLocationTypes[trimmed]; ok {
						value := trimmed
						locationType = &value
					}
				}
			}

			var sourcePlayerID *int64
			if event.SourceReplayPlayerID != nil {
				if dbPlayerID, ok := playerIDMap[*event.SourceReplayPlayerID]; ok {
					sourcePlayerID = &dbPlayerID
				}
			}

			var targetPlayerID *int64
			if event.TargetReplayPlayerID != nil {
				if dbPlayerID, ok := playerIDMap[*event.TargetReplayPlayerID]; ok {
					targetPlayerID = &dbPlayerID
				}
			}

			var attackUnitTypes *string
			if len(event.AttackUnitTypes) > 0 {
				filtered := make([]string, 0, len(event.AttackUnitTypes))
				for _, unitType := range event.AttackUnitTypes {
					normalized := normalizeEnumValue(unitType, allowedCommandUnits)
					if normalized == unknownEnumValue {
						continue
					}
					filtered = append(filtered, normalized)
				}
				if len(filtered) > 0 {
					payload, err := json.Marshal(filtered)
					if err != nil {
						return fmt.Errorf("failed to marshal replay event unit types: %w", err)
					}
					text := string(payload)
					attackUnitTypes = &text
				}
			}

			var locationMineralOnly *bool
			if event.LocationMineralOnly != nil && *event.LocationMineralOnly {
				locationMineralOnly = event.LocationMineralOnly
			}

			valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			valueArgs = append(valueArgs,
				replayID,
				event.Second,
				eventType,
				locationType,
				event.LocationBaseOclock,
				event.LocationNaturalOfClock,
				locationMineralOnly,
				sourcePlayerID,
				targetPlayerID,
				attackUnitTypes,
			)
		}

		if len(valueStrings) == 0 {
			continue
		}

		query := fmt.Sprintf(`
			INSERT INTO replay_events (
				replay_id,
				seconds_from_game_start,
				event_type,
				location_base_type,
				location_base_oclock,
				location_natural_of_oclock,
				location_mineral_only,
				source_player_id,
				target_player_id,
				attack_unit_types
			)
			VALUES %s
		`, strings.Join(valueStrings, ", "))

		if _, err := db.ExecContext(ctx, query, valueArgs...); err != nil {
			return fmt.Errorf("failed to insert replay events: %w", err)
		}
	}
	return nil
}

// insertPlayerPatternResultsTx inserts player-level pattern results (uses provided connection/transaction)
func (s *SQLiteStorage) insertPlayerPatternResultsTx(ctx context.Context, db dbtx, results []*core.PatternResult) error {
	const batchSize = 100
	for i := 0; i < len(results); i += batchSize {
		end := min(i+batchSize, len(results))
		batch := results[i:end]

		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]any, 0, len(batch)*7)

		for _, result := range batch {
			if result.PlayerID == nil {
				continue // Skip if player ID is nil
			}

			valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?)")
			valueArgs = append(valueArgs, result.ReplayID, *result.PlayerID, result.PatternName)

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
		}

		if len(valueStrings) == 0 {
			continue
		}

		query := fmt.Sprintf(`
			INSERT INTO detected_patterns_replay_player
			(replay_id, player_id, pattern_name, value_bool, value_int, value_string, value_timestamp)
			VALUES %s
			ON CONFLICT (replay_id, player_id, pattern_name) DO UPDATE SET
				value_bool = excluded.value_bool,
				value_int = excluded.value_int,
				value_string = excluded.value_string,
				value_timestamp = excluded.value_timestamp
		`, strings.Join(valueStrings, ", "))

		if _, err := db.ExecContext(ctx, query, valueArgs...); err != nil {
			return fmt.Errorf("failed to insert player pattern results: %w", err)
		}
	}

	return nil
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

func sqliteDSN(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "file:screp.db?_pragma=foreign_keys(1)"
	}
	if path == ":memory:" || strings.HasPrefix(path, "file:") {
		if strings.Contains(path, "_pragma=foreign_keys(1)") {
			return path
		}
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		return path + sep + "_pragma=foreign_keys(1)"
	}
	return fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path)
}

func enumSetFromNames(values []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := strings.TrimSpace(value)
		if key == "" {
			continue
		}
		allowed[key] = struct{}{}
	}
	allowed[unknownEnumValue] = struct{}{}
	return allowed
}

func normalizeNullableEnum(value *string, allowed map[string]struct{}) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	normalized := normalizeEnumValue(trimmed, allowed)
	return &normalized
}

func normalizeEnumValue(value string, allowed map[string]struct{}) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return unknownEnumValue
	}
	if _, ok := allowed[trimmed]; ok {
		return trimmed
	}
	return unknownEnumValue
}

func typeNamesFromRepcmd() []string {
	values := make([]string, 0, len(repcmd.Types)+1)
	for _, value := range repcmd.Types {
		if value == nil {
			continue
		}
		values = append(values, value.Name)
	}
	return append(values, unknownEnumValue)
}

func orderNamesFromRepcmd() []string {
	values := make([]string, 0, len(repcmd.Orders)+1)
	for _, value := range repcmd.Orders {
		if value == nil {
			continue
		}
		values = append(values, value.Name)
	}
	return append(values, unknownEnumValue)
}

func unitNamesFromRepcmd() []string {
	values := make([]string, 0, len(repcmd.Units)+1)
	for _, value := range repcmd.Units {
		if value == nil {
			continue
		}
		values = append(values, value.Name)
	}
	return append(values, unknownEnumValue)
}

func techNamesFromRepcmd() []string {
	values := make([]string, 0, len(repcmd.Techs)+1)
	for _, value := range repcmd.Techs {
		if value == nil {
			continue
		}
		values = append(values, value.Name)
	}
	return append(values, unknownEnumValue)
}

func upgradeNamesFromRepcmd() []string {
	values := make([]string, 0, len(repcmd.Upgrades)+1)
	for _, value := range repcmd.Upgrades {
		if value == nil {
			continue
		}
		values = append(values, value.Name)
	}
	return append(values, unknownEnumValue)
}

func hotkeyNamesFromRepcmd() []string {
	values := make([]string, 0, len(repcmd.HotkeyTypes)+1)
	for _, value := range repcmd.HotkeyTypes {
		if value == nil {
			continue
		}
		values = append(values, value.Name)
	}
	return append(values, unknownEnumValue)
}

func leaveReasonNamesFromRepcmd() []string {
	values := make([]string, 0, len(repcmd.LeaveReasons)+1)
	for _, value := range repcmd.LeaveReasons {
		if value == nil {
			continue
		}
		values = append(values, value.Name)
	}
	return append(values, unknownEnumValue)
}

func playerRaceNamesFromRepcore() []string {
	values := make([]string, 0, len(repcore.Races)+1)
	for _, value := range repcore.Races {
		if value == nil {
			continue
		}
		values = append(values, value.Name)
	}
	return append(values, unknownEnumValue)
}

func playerTypeNamesFromRepcore() []string {
	values := make([]string, 0, len(repcore.PlayerTypes)+1)
	for _, value := range repcore.PlayerTypes {
		if value == nil {
			continue
		}
		values = append(values, value.Name)
	}
	return append(values, unknownEnumValue)
}

func playerColorNamesFromRepcore() []string {
	values := make([]string, 0, len(repcore.Colors)+1)
	for _, value := range repcore.Colors {
		if value == nil {
			continue
		}
		values = append(values, value.Name)
	}
	return append(values, unknownEnumValue)
}

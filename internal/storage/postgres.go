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
func (s *PostgresStorage) Initialize(ctx context.Context) error {
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
		effective BOOLEAN NOT NULL
	);

	CREATE TABLE IF NOT EXISTS units (
		id SERIAL PRIMARY KEY,
		replay_id INTEGER NOT NULL REFERENCES replays(id) ON DELETE CASCADE,
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
			replay_id, slot_id, player_id, name, race, type, color, team, observer, apm, spm, is_winner
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`

	playerIDs := make(map[byte]int64) // player_id -> database_id
	for _, player := range data.Players {
		player.ReplayID = replayID
		var playerID int64
		err := tx.QueryRowContext(ctx, playerQuery,
			player.ReplayID, player.SlotID, player.PlayerID, player.Name, player.Race, player.Type,
			player.Color, player.Team, player.Observer, player.APM, player.SPM, player.IsWinner,
		).Scan(&playerID)
		if err != nil {
			return fmt.Errorf("failed to insert player: %w", err)
		}
		playerIDs[player.PlayerID] = playerID
	}

	// Insert commands
	commandQuery := `
		INSERT INTO commands (
			replay_id, player_id, frame, time, action_type, action_id, unit_id, target_id, x, y, data, effective
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
		)
		if err != nil {
			return fmt.Errorf("failed to insert command: %w", err)
		}
	}

	// Insert units
	unitQuery := `
		INSERT INTO units (
			replay_id, unit_id, type, name, created, destroyed,
			x, y, hp, max_hp, shield, max_shield, energy, max_energy
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	for _, unit := range data.Units {
		unit.ReplayID = replayID

		_, err := tx.ExecContext(ctx, unitQuery,
			unit.ReplayID, unit.UnitID, unit.Type, unit.Name,
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
			replay_id, building_id, type, name, created, destroyed,
			x, y, hp, max_hp, shield, max_shield, energy, max_energy
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	for _, building := range data.Buildings {
		building.ReplayID = replayID

		_, err := tx.ExecContext(ctx, buildingQuery,
			building.ReplayID, building.BuildingID, building.Type,
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

// Close closes the database connection
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

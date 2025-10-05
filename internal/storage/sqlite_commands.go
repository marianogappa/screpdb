package storage

import (
	"encoding/json"
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// SQLiteCommandsInserter implements SQLiteBatchInserter for commands
type SQLiteCommandsInserter struct{}

// NewSQLiteCommandsInserter creates a new SQLite commands inserter
func NewSQLiteCommandsInserter() *SQLiteCommandsInserter {
	return &SQLiteCommandsInserter{}
}

// TableName returns the table name for commands
func (c *SQLiteCommandsInserter) TableName() string {
	return "commands"
}

var sqliteCommandsColumnNames = []string{
	"replay_id", "player_id", "frame", "run_at", "action_type", "x", "y", "is_effective",
	"is_queued", "order_name", "unit_type", "unit_player_id", "unit_types", "build_unit_name",
	"train_unit_name", "building_morph_unit_name", "tech_name", "upgrade_name", "hotkey_type", "hotkey_group", "game_speed",
	"vision_player_ids", "alliance_player_ids", "is_allied_victory",
	"general_data",
}

// ColumnNames returns the column names for commands
func (c *SQLiteCommandsInserter) ColumnNames() []string {
	return sqliteCommandsColumnNames
}

// EntityCount returns the number of columns for commands
func (c *SQLiteCommandsInserter) EntityCount() int {
	return len(sqliteCommandsColumnNames)
}

// BuildArgs builds the arguments for a command entity
func (c *SQLiteCommandsInserter) BuildArgs(entity any, args []any, offset int) error {
	command, ok := entity.(*models.Command)
	if !ok {
		return fmt.Errorf("expected *models.Command, got %T", entity)
	}

	// Serialize player IDs to JSON
	visionPlayerIDsJSON, err := serializePlayerIDsForSQLite(command.VisionPlayerIDs)
	if err != nil {
		return fmt.Errorf("failed to serialize vision player IDs: %w", err)
	}
	alliancePlayerIDsJSON, err := serializePlayerIDsForSQLite(command.AlliancePlayerIDs)
	if err != nil {
		return fmt.Errorf("failed to serialize alliance player IDs: %w", err)
	}

	// Serialize unit information to JSON
	unitTypesJSON, err := serializeStringForSQLite(command.UnitTypes)
	if err != nil {
		return fmt.Errorf("failed to serialize unit types: %w", err)
	}
	args[offset] = command.ReplayID
	args[offset+1] = command.PlayerID
	args[offset+2] = command.Frame
	args[offset+3] = command.RunAt
	args[offset+4] = command.ActionType
	args[offset+5] = command.X
	args[offset+6] = command.Y
	args[offset+7] = command.IsEffective
	args[offset+8] = command.IsQueued
	args[offset+9] = command.OrderName
	args[offset+10] = getUnitTypeOrNullForSQLite(command.UnitType)
	args[offset+11] = command.UnitPlayerID
	args[offset+12] = unitTypesJSON
	args[offset+13] = command.BuildUnitName
	args[offset+14] = command.TrainUnitName
	args[offset+15] = command.BuildingMorphUnitName
	args[offset+16] = command.TechName
	args[offset+17] = command.UpgradeName
	args[offset+18] = command.HotkeyType
	args[offset+19] = command.HotkeyGroup
	args[offset+20] = command.GameSpeed
	args[offset+21] = visionPlayerIDsJSON
	args[offset+22] = alliancePlayerIDsJSON
	args[offset+23] = command.IsAlliedVictory
	args[offset+24] = command.GeneralData

	return nil
}

// Helper functions for SQLite serialization
func serializePlayerIDsForSQLite(playerIDs *[]int64) (string, error) {
	if playerIDs == nil {
		return "", nil
	}
	data, err := json.Marshal(*playerIDs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func serializeStringForSQLite(str *string) (interface{}, error) {
	if str == nil {
		return nil, nil
	}
	return *str, nil
}

func getUnitTypeOrNullForSQLite(unitType *string) interface{} {
	if unitType == nil || *unitType == "None" {
		return nil
	}
	return *unitType
}

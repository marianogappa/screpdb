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
	"replay_id", "player_id", "frame", "run_at", "action_type", "unit_id", "x", "y", "is_effective",
	"is_queued", "order_id", "order_name", "unit_type", "unit_player_id", "unit_types", "unit_ids", "build_unit_name",
	"train_unit_name", "building_morph_unit_name", "tech_name", "upgrade_name", "hotkey_type", "hotkey_group", "game_speed",
	"vision_slot_ids", "alliance_slot_ids", "is_allied_victory",
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

	// Serialize slot IDs to JSON
	visionSlotIDsJSON, err := serializeSlotIDsForSQLite(command.VisionSlotIDs)
	if err != nil {
		return fmt.Errorf("failed to serialize vision slot IDs: %w", err)
	}
	allianceSlotIDsJSON, err := serializeSlotIDsForSQLite(command.AllianceSlotIDs)
	if err != nil {
		return fmt.Errorf("failed to serialize alliance slot IDs: %w", err)
	}

	// Serialize unit information to JSON
	unitTypesJSON, err := serializeStringForSQLite(command.UnitTypes)
	if err != nil {
		return fmt.Errorf("failed to serialize unit types: %w", err)
	}
	unitIDsJSON, err := serializeStringForSQLite(command.UnitIDs)
	if err != nil {
		return fmt.Errorf("failed to serialize unit IDs: %w", err)
	}
	args[offset] = command.ReplayID
	args[offset+1] = command.PlayerID
	args[offset+2] = command.Frame
	args[offset+3] = command.RunAt
	args[offset+4] = command.ActionType
	args[offset+5] = command.UnitID
	args[offset+6] = command.X
	args[offset+7] = command.Y
	args[offset+8] = command.IsEffective
	args[offset+9] = command.IsQueued
	args[offset+10] = command.OrderID
	args[offset+11] = command.OrderName
	args[offset+12] = command.UnitType
	args[offset+13] = command.UnitPlayerID
	args[offset+14] = unitTypesJSON
	args[offset+15] = unitIDsJSON
	args[offset+16] = command.BuildUnitName
	args[offset+17] = command.TrainUnitName
	args[offset+18] = command.BuildingMorphUnitName
	args[offset+19] = command.TechName
	args[offset+20] = command.UpgradeName
	args[offset+21] = command.HotkeyType
	args[offset+22] = command.HotkeyGroup
	args[offset+23] = command.GameSpeed
	args[offset+24] = visionSlotIDsJSON
	args[offset+25] = allianceSlotIDsJSON
	args[offset+26] = command.IsAlliedVictory
	args[offset+27] = command.GeneralData

	return nil
}

// Helper functions for SQLite serialization
func serializeSlotIDsForSQLite(slotIDs *[]int) (string, error) {
	if slotIDs == nil {
		return "", nil
	}
	data, err := json.Marshal(*slotIDs)
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

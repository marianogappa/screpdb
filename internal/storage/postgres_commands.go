package storage

import (
	"encoding/json"
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// CommandsInserter implements BatchInserter for commands
type CommandsInserter struct{}

// NewCommandsInserter creates a new commands inserter
func NewCommandsInserter() *CommandsInserter {
	return &CommandsInserter{}
}

// TableName returns the table name for commands
func (c *CommandsInserter) TableName() string {
	return "commands"
}

var commandsColumnNames = []string{
	"replay_id", "player_id", "frame", "run_at", "action_type", "x", "y", "is_effective",
	"is_queued", "order_name", "unit_type", "unit_player_id", "unit_types", "build_unit_name",
	"train_unit_name", "building_morph_unit_name", "tech_name", "upgrade_name", "hotkey_type", "hotkey_group", "game_speed",
	"vision_player_ids", "alliance_player_ids", "is_allied_victory",
	"general_data",
}

// ColumnNames returns the column names for commands
func (c *CommandsInserter) ColumnNames() []string {
	return commandsColumnNames
}

// EntityCount returns the number of columns for commands
func (c *CommandsInserter) EntityCount() int {
	return len(commandsColumnNames)
}

// BuildArgs builds the arguments for a command entity
func (c *CommandsInserter) BuildArgs(entity any, args []any, offset int) error {
	command, ok := entity.(*models.Command)
	if !ok {
		return fmt.Errorf("expected *models.Command, got %T", entity)
	}

	// Serialize player IDs to JSON
	visionPlayerIDsBytes, err := serializePlayerIDs(command.VisionPlayerIDs)
	if err != nil {
		return fmt.Errorf("failed to serialize vision player IDs: %w", err)
	}
	alliancePlayerIDsBytes, err := serializePlayerIDs(command.AlliancePlayerIDs)
	if err != nil {
		return fmt.Errorf("failed to serialize alliance player IDs: %w", err)
	}

	// Convert []byte to string for PostgreSQL
	var visionPlayerIDsJSON, alliancePlayerIDsJSON interface{}
	if visionPlayerIDsBytes != nil {
		visionPlayerIDsJSON = string(visionPlayerIDsBytes)
	}
	if alliancePlayerIDsBytes != nil {
		alliancePlayerIDsJSON = string(alliancePlayerIDsBytes)
	}

	// Serialize unit information to JSON
	unitTypesJSON, err := serializeString(command.UnitTypes)
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
	args[offset+10] = getUnitTypeOrNull(command.UnitType)
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

// Helper functions for serialization
func serializePlayerIDs(playerIDs *[]int64) ([]byte, error) {
	if playerIDs == nil {
		return nil, nil
	}
	return json.Marshal(*playerIDs)
}

func serializeString(str *string) (interface{}, error) {
	if str == nil {
		return nil, nil
	}
	return *str, nil
}

func getUnitTypeOrNull(unitType *string) interface{} {
	if unitType == nil || *unitType == "None" {
		return nil
	}
	return *unitType
}

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

// ColumnNames returns the column names for commands
func (c *CommandsInserter) ColumnNames() []string {
	return []string{
		"replay_id", "player_id", "frame", "time", "action_type", "action_id", "unit_id", "target_id", "x", "y", "effective",
		"queued", "order_id", "order_name", "unit_type", "unit_player_id", "unit_types", "unit_ids", "select_unit_tags", "select_unit_types", "build_unit_name",
		"train_unit_name", "building_morph_unit_name", "tech_name", "upgrade_name", "hotkey_type", "hotkey_group", "game_speed",
		"chat_sender_slot_id", "chat_message", "vision_slot_ids", "alliance_slot_ids", "allied_victory",
		"leave_reason", "minimap_ping_x", "minimap_ping_y", "general_data",
	}
}

// EntityCount returns the number of columns for commands
func (c *CommandsInserter) EntityCount() int {
	return 37
}

// BuildArgs builds the arguments for a command entity
func (c *CommandsInserter) BuildArgs(entity any, args []any, offset int) error {
	command, ok := entity.(*models.Command)
	if !ok {
		return fmt.Errorf("expected *models.Command, got %T", entity)
	}

	// Serialize slot IDs to JSON
	visionSlotIDsBytes, err := serializeSlotIDs(command.VisionSlotIDs)
	if err != nil {
		return fmt.Errorf("failed to serialize vision slot IDs: %w", err)
	}
	allianceSlotIDsBytes, err := serializeSlotIDs(command.AllianceSlotIDs)
	if err != nil {
		return fmt.Errorf("failed to serialize alliance slot IDs: %w", err)
	}

	// Convert []byte to string for PostgreSQL
	var visionSlotIDsJSON, allianceSlotIDsJSON interface{}
	if visionSlotIDsBytes != nil {
		visionSlotIDsJSON = string(visionSlotIDsBytes)
	}
	if allianceSlotIDsBytes != nil {
		allianceSlotIDsJSON = string(allianceSlotIDsBytes)
	}

	// Serialize unit information to JSON
	unitTypesJSON, err := serializeString(command.UnitTypes)
	if err != nil {
		return fmt.Errorf("failed to serialize unit types: %w", err)
	}
	unitIDsJSON, err := serializeString(command.UnitIDs)
	if err != nil {
		return fmt.Errorf("failed to serialize unit IDs: %w", err)
	}
	selectUnitTagsJSON, err := serializeString(command.SelectUnitTags)
	if err != nil {
		return fmt.Errorf("failed to serialize select unit tags: %w", err)
	}
	selectUnitTypesJSON, err := serializeString(command.SelectUnitTypes)
	if err != nil {
		return fmt.Errorf("failed to serialize select unit types: %w", err)
	}

	args[offset] = command.ReplayID
	args[offset+1] = command.PlayerID
	args[offset+2] = command.Frame
	args[offset+3] = command.Time
	args[offset+4] = command.ActionType
	args[offset+5] = command.ActionID
	args[offset+6] = command.UnitID
	args[offset+7] = command.TargetID
	args[offset+8] = command.X
	args[offset+9] = command.Y
	args[offset+10] = command.Effective
	args[offset+11] = command.Queued
	args[offset+12] = command.OrderID
	args[offset+13] = command.OrderName
	args[offset+14] = command.UnitType
	args[offset+15] = command.UnitPlayerID
	args[offset+16] = unitTypesJSON
	args[offset+17] = unitIDsJSON
	args[offset+18] = selectUnitTagsJSON
	args[offset+19] = selectUnitTypesJSON
	args[offset+20] = command.BuildUnitName
	args[offset+21] = command.TrainUnitName
	args[offset+22] = command.BuildingMorphUnitName
	args[offset+23] = command.TechName
	args[offset+24] = command.UpgradeName
	args[offset+25] = command.HotkeyType
	args[offset+26] = command.HotkeyGroup
	args[offset+27] = command.GameSpeed
	args[offset+28] = command.ChatSenderSlotID
	args[offset+29] = command.ChatMessage
	args[offset+30] = visionSlotIDsJSON
	args[offset+31] = allianceSlotIDsJSON
	args[offset+32] = command.AlliedVictory
	args[offset+33] = command.LeaveReason
	args[offset+34] = command.MinimapPingX
	args[offset+35] = command.MinimapPingY
	args[offset+36] = command.GeneralData

	return nil
}

// Helper functions for serialization
func serializeSlotIDs(slotIDs *[]int) ([]byte, error) {
	if slotIDs == nil {
		return nil, nil
	}
	return json.Marshal(*slotIDs)
}

func serializeString(str *string) (interface{}, error) {
	if str == nil {
		return nil, nil
	}
	return *str, nil
}

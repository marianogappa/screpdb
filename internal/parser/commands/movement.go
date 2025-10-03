package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// RightClickCommandHandler handles RightClick commands (including 121 version)
type RightClickCommandHandler struct {
	BaseCommandHandler
}

func NewRightClickCommandHandler(actionType string, actionID byte) *RightClickCommandHandler {
	return &RightClickCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: actionType,
			actionID:   actionID,
		},
	}
}

func (h *RightClickCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	rightClickCmd := cmd.(*repcmd.RightClickCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(rightClickCmd.Pos.X)
	command.Y = int(rightClickCmd.Pos.Y)

	command.IsQueued = boolPtr(rightClickCmd.Queued)

	return command
}

// HandleWithUnit handles RightClick commands with resolved unit information
func (h *RightClickCommandHandler) HandleWithUnit(cmd repcmd.Cmd, base *repcmd.Base, unit *models.UnitInfo) *models.Command {
	rightClickCmd := cmd.(*repcmd.RightClickCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(rightClickCmd.Pos.X)
	command.Y = int(rightClickCmd.Pos.Y)

	// Set the normalized unit fields
	if unit != nil {
		command.UnitType = &unit.UnitType
		command.UnitPlayerID = &unit.PlayerID
		command.UnitID = bytePtr(byte(unit.UnitID))
	}

	command.IsQueued = boolPtr(rightClickCmd.Queued)

	return command
}

// TargetedOrderCommandHandler handles TargetedOrder commands (including 121 version)
type TargetedOrderCommandHandler struct {
	BaseCommandHandler
}

func NewTargetedOrderCommandHandler(actionType string, actionID byte) *TargetedOrderCommandHandler {
	return &TargetedOrderCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: actionType,
			actionID:   actionID,
		},
	}
}

func (h *TargetedOrderCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	targetedOrderCmd := cmd.(*repcmd.TargetedOrderCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(targetedOrderCmd.Pos.X)
	command.Y = int(targetedOrderCmd.Pos.Y)

	if targetedOrderCmd.Order != nil {
		command.OrderID = bytePtr(targetedOrderCmd.Order.ID)
		command.OrderName = stringPtr(targetedOrderCmd.Order.Name)
	}

	command.IsQueued = boolPtr(targetedOrderCmd.Queued)

	return command
}

// HandleWithUnit handles TargetedOrder commands with resolved unit information
func (h *TargetedOrderCommandHandler) HandleWithUnit(cmd repcmd.Cmd, base *repcmd.Base, unit *models.UnitInfo) *models.Command {
	targetedOrderCmd := cmd.(*repcmd.TargetedOrderCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(targetedOrderCmd.Pos.X)
	command.Y = int(targetedOrderCmd.Pos.Y)

	// Set the normalized unit fields
	if unit != nil {
		command.UnitType = &unit.UnitType
		command.UnitPlayerID = &unit.PlayerID
		command.UnitID = bytePtr(byte(unit.UnitID))
	}

	if targetedOrderCmd.Order != nil {
		command.OrderID = bytePtr(targetedOrderCmd.Order.ID)
		command.OrderName = stringPtr(targetedOrderCmd.Order.Name)
	}

	command.IsQueued = boolPtr(targetedOrderCmd.Queued)

	return command
}

// MinimapPingCommandHandler handles MinimapPing commands
type MinimapPingCommandHandler struct {
	BaseCommandHandler
}

func NewMinimapPingCommandHandler() *MinimapPingCommandHandler {
	return &MinimapPingCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "MinimapPing",
			actionID:   repcmd.TypeIDMinimapPing,
		},
	}
}

func (h *MinimapPingCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	minimapPingCmd := cmd.(*repcmd.MinimapPingCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(minimapPingCmd.Pos.X)
	command.Y = int(minimapPingCmd.Pos.Y)

	return command
}

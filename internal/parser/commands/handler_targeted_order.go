package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

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

func (h *TargetedOrderCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	targetedOrderCmd := cmd.(*repcmd.TargetedOrderCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = pInt(int(targetedOrderCmd.Pos.X))
	command.Y = pInt(int(targetedOrderCmd.Pos.Y))

	if targetedOrderCmd.Order != nil {
		command.OrderID = bytePtr(targetedOrderCmd.Order.ID)
		command.OrderName = stringPtr(targetedOrderCmd.Order.Name)
	}

	command.IsQueued = boolPtr(targetedOrderCmd.Queued)

	return command
}

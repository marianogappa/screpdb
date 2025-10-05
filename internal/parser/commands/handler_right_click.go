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

func (h *RightClickCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	rightClickCmd := cmd.(*repcmd.RightClickCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(rightClickCmd.Pos.X)
	command.Y = int(rightClickCmd.Pos.Y)

	command.IsQueued = boolPtr(rightClickCmd.Queued)

	return command
}

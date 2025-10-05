package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// LiftOffCommandHandler handles LiftOff commands
type LiftOffCommandHandler struct {
	BaseCommandHandler
}

func NewLiftOffCommandHandler() *LiftOffCommandHandler {
	return &LiftOffCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "LiftOff",
			actionID:   repcmd.TypeIDLiftOff,
		},
	}
}

func (h *LiftOffCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	liftOffCmd := cmd.(*repcmd.LiftOffCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(liftOffCmd.Pos.X)
	command.Y = int(liftOffCmd.Pos.Y)

	return command
}

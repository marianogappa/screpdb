package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// UnitMorphCommandHandler handles UnitMorph commands
type UnitMorphCommandHandler struct {
	BaseCommandHandler
}

func NewUnitMorphCommandHandler() *UnitMorphCommandHandler {
	return &UnitMorphCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "UnitMorph",
			actionID:   repcmd.TypeIDUnitMorph,
		},
	}
}

func (h *UnitMorphCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	trainCmd := cmd.(*repcmd.TrainCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if trainCmd.Unit != nil {
		command.UnitID = bytePtr(byte(trainCmd.Unit.ID))
		command.UnitType = stringPtr(trainCmd.Unit.Name)
	}

	return command
}

package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// TrainFighterCommandHandler handles TrainFighter commands (renamed to BuildInterceptorOrScarab)
type TrainFighterCommandHandler struct {
	BaseCommandHandler
}

func NewTrainFighterCommandHandler() *TrainFighterCommandHandler {
	return &TrainFighterCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "BuildInterceptorOrScarab",
			actionID:   repcmd.TypeIDTrainFighter,
		},
	}
}

func (h *TrainFighterCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	// Try to extract data from TrainCmd if it's that type
	if trainCmd, ok := cmd.(*repcmd.TrainCmd); ok {
		if trainCmd.Unit != nil {
			command.UnitID = bytePtr(byte(trainCmd.Unit.ID))
			command.UnitType = stringPtr(trainCmd.Unit.Name)
		}
	} else {
		// For other command types, we'll store basic information without unit details
		command.UnitID = nil
		command.UnitType = nil
	}

	return command
}

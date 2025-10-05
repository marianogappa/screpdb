package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// TrainCommandHandler handles Train commands
type TrainCommandHandler struct {
	BaseCommandHandler
}

func NewTrainCommandHandler() *TrainCommandHandler {
	return &TrainCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Train",
			actionID:   repcmd.TypeIDTrain,
		},
	}
}

func (h *TrainCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	trainCmd := cmd.(*repcmd.TrainCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if trainCmd.Unit != nil {
		command.UnitID = bytePtr(byte(trainCmd.Unit.ID))
		command.TrainUnitName = stringPtr(trainCmd.Unit.Name)
	}

	return command
}

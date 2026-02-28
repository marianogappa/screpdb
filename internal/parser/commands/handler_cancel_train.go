package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// CancelTrainCommandHandler handles CancelTrain commands
type CancelTrainCommandHandler struct {
	BaseCommandHandler
}

func NewCancelTrainCommandHandler() *CancelTrainCommandHandler {
	return &CancelTrainCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "CancelTrain",
			actionID:   repcmd.TypeIDCancelTrain,
		},
	}
}

func (h *CancelTrainCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller
	return command
}

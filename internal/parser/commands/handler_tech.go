package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// TechCommandHandler handles Tech commands
type TechCommandHandler struct {
	BaseCommandHandler
}

func NewTechCommandHandler() *TechCommandHandler {
	return &TechCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Tech",
			actionID:   repcmd.TypeIDTech,
		},
	}
}

func (h *TechCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	techCmd := cmd.(*repcmd.TechCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if techCmd.Tech != nil {
		command.TechName = stringPtr(techCmd.Tech.Name)
	}

	return command
}

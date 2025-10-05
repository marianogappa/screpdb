package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// VisionCommandHandler handles Vision commands
type VisionCommandHandler struct {
	BaseCommandHandler
}

func NewVisionCommandHandler() *VisionCommandHandler {
	return &VisionCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Vision",
			actionID:   repcmd.TypeIDVision,
		},
	}
}

func (h *VisionCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	visionCmd := cmd.(*repcmd.VisionCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.VisionPlayerIDs = slotIDsToPlayerIDs(visionCmd.SlotIDs, slotToPlayerMap)

	return command
}

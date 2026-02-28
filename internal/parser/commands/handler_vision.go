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

func (h *VisionCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	visionCmd := cmd.(*repcmd.VisionCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.VisionPlayerIDs = bytsToInts(visionCmd.SlotIDs) // TODO these need to be mapped after insertion

	return command
}

func bytsToInts(bs []byte) *[]int64 {
	ints := make([]int64, len(bs))
	for i, b := range bs {
		ints[i] = int64(b)
	}
	return &ints
}

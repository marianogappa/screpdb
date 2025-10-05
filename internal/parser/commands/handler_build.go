package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// BuildCommandHandler handles Build commands
type BuildCommandHandler struct {
	BaseCommandHandler
}

func NewBuildCommandHandler() *BuildCommandHandler {
	return &BuildCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Build",
			actionID:   repcmd.TypeIDBuild,
		},
	}
}

func (h *BuildCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	buildCmd := cmd.(*repcmd.BuildCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(buildCmd.Pos.X)
	command.Y = int(buildCmd.Pos.Y)

	if buildCmd.Unit != nil {
		command.UnitID = bytePtr(byte(buildCmd.Unit.ID))
		command.UnitType = stringPtr(buildCmd.Unit.Name)
	}

	if buildCmd.Order != nil {
		command.OrderID = bytePtr(buildCmd.Order.ID)
		command.OrderName = stringPtr(buildCmd.Order.Name)
	}

	return command
}

package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// BuildingMorphCommandHandler handles BuildingMorph commands
type BuildingMorphCommandHandler struct {
	BaseCommandHandler
}

func NewBuildingMorphCommandHandler() *BuildingMorphCommandHandler {
	return &BuildingMorphCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "BuildingMorph",
			actionID:   repcmd.TypeIDBuildingMorph,
		},
	}
}

func (h *BuildingMorphCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	buildingMorphCmd := cmd.(*repcmd.BuildingMorphCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if buildingMorphCmd.Unit != nil {
		command.UnitID = bytePtr(byte(buildingMorphCmd.Unit.ID))
		command.BuildingMorphUnitName = stringPtr(buildingMorphCmd.Unit.Name)
	}

	return command
}

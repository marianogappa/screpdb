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

func (h *TrainCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	trainCmd := cmd.(*repcmd.TrainCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if trainCmd.Unit != nil {
		command.UnitID = byte(trainCmd.Unit.ID)
		command.TrainUnitName = stringPtr(trainCmd.Unit.Name)
	}

	return command
}

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

func (h *TrainFighterCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	// Try to extract data from TrainCmd if it's that type
	if trainCmd, ok := cmd.(*repcmd.TrainCmd); ok {
		if trainCmd.Unit != nil {
			command.UnitID = byte(trainCmd.Unit.ID)
			command.TrainUnitName = stringPtr(trainCmd.Unit.Name)
		}
	} else {
		// For other command types, we'll store basic information without unit details
		command.UnitID = 0
		command.TrainUnitName = nil
	}

	return command
}

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

func (h *UnitMorphCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	trainCmd := cmd.(*repcmd.TrainCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if trainCmd.Unit != nil {
		command.UnitID = byte(trainCmd.Unit.ID)
		command.TrainUnitName = stringPtr(trainCmd.Unit.Name)
	}

	return command
}

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
	cancelTrainCmd := cmd.(*repcmd.CancelTrainCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if cancelTrainCmd.UnitTag != 0 {
		command.CancelTrainUnitTag = uint16Ptr(uint16(cancelTrainCmd.UnitTag))
	}

	return command
}

// UnloadCommandHandler handles Unload commands (including 121 version)
type UnloadCommandHandler struct {
	BaseCommandHandler
}

func NewUnloadCommandHandler(actionType string, actionID byte) *UnloadCommandHandler {
	return &UnloadCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: actionType,
			actionID:   actionID,
		},
	}
}

func (h *UnloadCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	unloadCmd := cmd.(*repcmd.UnloadCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if unloadCmd.UnitTag != 0 {
		command.UnloadUnitTag = uint16Ptr(uint16(unloadCmd.UnitTag))
	}

	return command
}

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

func (h *BuildingMorphCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	buildingMorphCmd := cmd.(*repcmd.BuildingMorphCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if buildingMorphCmd.Unit != nil {
		command.UnitID = byte(buildingMorphCmd.Unit.ID)
		command.BuildingMorphUnitName = stringPtr(buildingMorphCmd.Unit.Name)
	}

	return command
}

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

func (h *LiftOffCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	liftOffCmd := cmd.(*repcmd.LiftOffCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(liftOffCmd.Pos.X)
	command.Y = int(liftOffCmd.Pos.Y)

	return command
}

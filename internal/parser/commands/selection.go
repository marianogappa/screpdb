package commands

import (
	"encoding/json"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// SelectCommandHandler handles Select, SelectAdd, SelectRemove commands (including 121 versions)
type SelectCommandHandler struct {
	BaseCommandHandler
}

func NewSelectCommandHandler(actionType string, actionID byte) *SelectCommandHandler {
	return &SelectCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: actionType,
			actionID:   actionID,
		},
	}
}

func (h *SelectCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	return command
}

// HandleWithUnits handles Select commands with resolved unit information
func (h *SelectCommandHandler) HandleWithUnits(cmd repcmd.Cmd, base *repcmd.Base, units []*models.UnitInfo) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	// Populate normalized unit fields
	if len(units) > 0 {
		// For multiple units, store types and IDs as JSON arrays
		unitTypes := make([]string, len(units))
		unitIDs := make([]uint16, len(units))
		for i, unit := range units {
			unitTypes[i] = unit.UnitType
			unitIDs[i] = unit.UnitID
		}

		unitTypesJSON, _ := json.Marshal(unitTypes)
		unitIDsJSON, _ := json.Marshal(unitIDs)

		command.UnitTypes = stringPtr(string(unitTypesJSON))
		command.UnitIDs = stringPtr(string(unitIDsJSON))
		command.UnitPlayerID = &units[0].PlayerID // Use first unit's player ID
	}

	return command
}

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

func (h *BuildCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	buildCmd := cmd.(*repcmd.BuildCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(buildCmd.Pos.X)
	command.Y = int(buildCmd.Pos.Y)

	if buildCmd.Unit != nil {
		command.UnitID = bytePtr(byte(buildCmd.Unit.ID))
		command.BuildUnitName = stringPtr(buildCmd.Unit.Name)
	}

	if buildCmd.Order != nil {
		command.OrderID = bytePtr(buildCmd.Order.ID)
		command.OrderName = stringPtr(buildCmd.Order.Name)
	}

	return command
}

// HandleWithUnit handles Build commands with resolved unit information
func (h *BuildCommandHandler) HandleWithUnit(cmd repcmd.Cmd, base *repcmd.Base, unit *models.UnitInfo) *models.Command {
	buildCmd := cmd.(*repcmd.BuildCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(buildCmd.Pos.X)
	command.Y = int(buildCmd.Pos.Y)

	// Set the normalized unit fields
	if unit != nil {
		command.UnitType = &unit.UnitType
		command.UnitPlayerID = &unit.PlayerID
		command.UnitID = bytePtr(byte(unit.UnitID))
	}

	if buildCmd.Unit != nil {
		command.BuildUnitName = stringPtr(buildCmd.Unit.Name)
	}

	if buildCmd.Order != nil {
		command.OrderID = bytePtr(buildCmd.Order.ID)
		command.OrderName = stringPtr(buildCmd.Order.Name)
	}

	return command
}

// LandCommandHandler handles Land commands (virtual type)
type LandCommandHandler struct {
	BaseCommandHandler
}

func NewLandCommandHandler() *LandCommandHandler {
	return &LandCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Land",
			actionID:   repcmd.VirtualTypeIDLand,
		},
	}
}

func (h *LandCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	landCmd := cmd.(*repcmd.LandCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(landCmd.Pos.X)
	command.Y = int(landCmd.Pos.Y)

	if landCmd.Unit != nil {
		command.UnitID = bytePtr(byte(landCmd.Unit.ID))
		command.BuildUnitName = stringPtr(landCmd.Unit.Name)
	}

	if landCmd.Order != nil {
		command.OrderID = bytePtr(landCmd.Order.ID)
		command.OrderName = stringPtr(landCmd.Order.Name)
	}

	return command
}

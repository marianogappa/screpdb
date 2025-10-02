package commands

import (
	"time"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// CommandRegistry manages command handlers and provides a unified interface for command processing
type CommandRegistry struct {
	handlers map[byte]CommandHandler
}

// NewCommandRegistry creates a new command registry with all handlers registered
func NewCommandRegistry() *CommandRegistry {
	registry := &CommandRegistry{
		handlers: make(map[byte]CommandHandler),
	}

	registry.registerHandlers()
	return registry
}

// registerHandlers registers all command handlers
func (r *CommandRegistry) registerHandlers() {
	// Selection commands (including 121 versions)
	r.register(repcmd.TypeIDSelect, NewSelectCommandHandler("Select", repcmd.TypeIDSelect))
	r.register(repcmd.TypeIDSelectAdd, NewSelectCommandHandler("SelectAdd", repcmd.TypeIDSelectAdd))
	r.register(repcmd.TypeIDSelectRemove, NewSelectCommandHandler("SelectRemove", repcmd.TypeIDSelectRemove))
	r.register(repcmd.TypeIDSelect121, NewSelectCommandHandler("Select", repcmd.TypeIDSelect121))
	r.register(repcmd.TypeIDSelectAdd121, NewSelectCommandHandler("SelectAdd", repcmd.TypeIDSelectAdd121))
	r.register(repcmd.TypeIDSelectRemove121, NewSelectCommandHandler("SelectRemove", repcmd.TypeIDSelectRemove121))

	// Build commands
	r.register(repcmd.TypeIDBuild, NewBuildCommandHandler())
	r.register(repcmd.VirtualTypeIDLand, NewLandCommandHandler())

	// Movement and targeting commands
	r.register(repcmd.TypeIDRightClick, NewRightClickCommandHandler("RightClick", repcmd.TypeIDRightClick))
	r.register(repcmd.TypeIDRightClick121, NewRightClickCommandHandler("RightClick", repcmd.TypeIDRightClick121))
	r.register(repcmd.TypeIDTargetedOrder, NewTargetedOrderCommandHandler("TargetedOrder", repcmd.TypeIDTargetedOrder))
	r.register(repcmd.TypeIDTargetedOrder121, NewTargetedOrderCommandHandler("TargetedOrder", repcmd.TypeIDTargetedOrder121))
	r.register(repcmd.TypeIDMinimapPing, NewMinimapPingCommandHandler())

	// Unit management commands
	r.register(repcmd.TypeIDTrain, NewTrainCommandHandler())
	r.register(repcmd.TypeIDTrainFighter, NewTrainFighterCommandHandler()) // Renamed to BuildInterceptorOrScarab
	r.register(repcmd.TypeIDUnitMorph, NewUnitMorphCommandHandler())
	r.register(repcmd.TypeIDCancelTrain, NewCancelTrainCommandHandler())
	r.register(repcmd.TypeIDUnload, NewUnloadCommandHandler("Unload", repcmd.TypeIDUnload))
	r.register(repcmd.TypeIDUnload121, NewUnloadCommandHandler("Unload", repcmd.TypeIDUnload121))
	r.register(repcmd.TypeIDBuildingMorph, NewBuildingMorphCommandHandler())
	r.register(repcmd.TypeIDLiftOff, NewLiftOffCommandHandler())

	// Research and upgrade commands
	r.register(repcmd.TypeIDTech, NewTechCommandHandler())
	r.register(repcmd.TypeIDUpgrade, NewUpgradeCommandHandler())

	// Game control commands
	r.register(repcmd.TypeIDGameSpeed, NewGameSpeedCommandHandler())
	r.register(repcmd.TypeIDHotkey, NewHotkeyCommandHandler())
	r.register(repcmd.TypeIDChat, NewChatCommandHandler())
	r.register(repcmd.TypeIDVision, NewVisionCommandHandler())
	r.register(repcmd.TypeIDAlliance, NewAllianceCommandHandler())
	r.register(repcmd.TypeIDLeaveGame, NewLeaveGameCommandHandler())

	// Queueable commands
	r.register(repcmd.TypeIDStop, NewQueueableCommandHandler("Stop", repcmd.TypeIDStop))
	r.register(repcmd.TypeIDCarrierStop, NewQueueableCommandHandler("CarrierStop", repcmd.TypeIDCarrierStop))
	r.register(repcmd.TypeIDReaverStop, NewQueueableCommandHandler("ReaverStop", repcmd.TypeIDReaverStop))
	r.register(repcmd.TypeIDReturnCargo, NewQueueableCommandHandler("ReturnCargo", repcmd.TypeIDReturnCargo))
	r.register(repcmd.TypeIDUnloadAll, NewQueueableCommandHandler("UnloadAll", repcmd.TypeIDUnloadAll))
	r.register(repcmd.TypeIDHoldPosition, NewQueueableCommandHandler("HoldPosition", repcmd.TypeIDHoldPosition))
	r.register(repcmd.TypeIDBurrow, NewQueueableCommandHandler("Burrow", repcmd.TypeIDBurrow))
	r.register(repcmd.TypeIDUnburrow, NewQueueableCommandHandler("Unburrow", repcmd.TypeIDUnburrow))
	r.register(repcmd.TypeIDSiege, NewQueueableCommandHandler("Siege", repcmd.TypeIDSiege))
	r.register(repcmd.TypeIDUnsiege, NewQueueableCommandHandler("Unsiege", repcmd.TypeIDUnsiege))
	r.register(repcmd.TypeIDCloack, NewQueueableCommandHandler("Cloack", repcmd.TypeIDCloack))
	r.register(repcmd.TypeIDDecloack, NewQueueableCommandHandler("Decloack", repcmd.TypeIDDecloack))
}

// register registers a command handler for a specific command type
func (r *CommandRegistry) register(commandType byte, handler CommandHandler) {
	r.handlers[commandType] = handler
}

// ProcessCommand processes a command using the appropriate handler
func (r *CommandRegistry) ProcessCommand(cmd repcmd.Cmd, replayID int64, startTime int64) *models.Command {
	base := cmd.BaseCmd()

	// Check if we should ignore this command type
	if r.shouldIgnoreCommand(base.Type.ID) {
		return nil
	}

	handler, exists := r.handlers[base.Type.ID]
	if !exists {
		// Use general handler for unhandled commands
		handler = NewGeneralCommandHandler(base.Type.String(), base.Type.ID)
	}

	command := handler.Handle(cmd, base)
	if command != nil {
		command.ReplayID = replayID
		command.Time = time.Unix(startTime+int64(base.Frame.Duration().Seconds()), 0)
	}

	return command
}

// shouldIgnoreCommand checks if a command type should be ignored
func (r *CommandRegistry) shouldIgnoreCommand(commandType byte) bool {
	ignoredTypes := map[byte]bool{
		repcmd.TypeIDSaveGame:           true,
		repcmd.TypeIDLoadGame:           true,
		repcmd.TypeIDKeepAlive:          true,
		repcmd.TypeIDRestartGame:        true,
		repcmd.TypeIDSync:               true,
		repcmd.TypeIDVoiceEnable:        true,
		repcmd.TypeIDVoiceDisable:       true,
		repcmd.TypeIDVoiceSquelch:       true,
		repcmd.TypeIDVoiceUnsquelch:     true,
		repcmd.TypeIDStartGame:          true,
		repcmd.TypeIDDownloadPercentage: true,
		repcmd.TypeIDChangeGameSlot:     true,
		repcmd.TypeIDNewNetPlayer:       true,
		repcmd.TypeIDJoinedGame:         true,
		repcmd.TypeIDChangeRace:         true,
		repcmd.TypeIDTeamGameTeam:       true,
		repcmd.TypeIDUMSTeam:            true,
		repcmd.TypeIDMeleeTeam:          true,
		repcmd.TypeIDSwapPlayers:        true,
		repcmd.TypeIDSavedData:          true,
		repcmd.TypeIDBriefingStart:      true,
		repcmd.TypeIDLatency:            true,
		repcmd.TypeIDReplaySpeed:        true,
		repcmd.TypeIDMakeGamePublic:     true,
	}

	return ignoredTypes[commandType]
}

// GetSupportedCommandTypes returns a list of all supported command types
func (r *CommandRegistry) GetSupportedCommandTypes() []byte {
	var types []byte
	for commandType := range r.handlers {
		if !r.shouldIgnoreCommand(commandType) {
			types = append(types, commandType)
		}
	}
	return types
}

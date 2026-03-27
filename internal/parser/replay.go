package parser

import (
	"fmt"
	"time"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser/commands"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/screp"
	"github.com/marianogappa/screpdb/internal/utils"
)

// ParseReplay parses a StarCraft: Brood War replay file and returns structured data
func ParseReplay(filePath string, fileInfo *models.Replay) (*models.ReplayData, error) {
	// Parse the replay file using the real screp library
	rep, err := screp.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse replay file: %w", err)
	}

	// Create the replay data structure
	data := &models.ReplayData{
		Replay:     fileInfo,
		Players:    []*models.Player{},
		Commands:   []*models.Command{},
		MapContext: &models.ReplayMapContext{},
	}

	// Parse replay metadata
	data.Replay.ReplayDate = rep.Header.StartTime
	data.Replay.Title = rep.Header.Title
	data.Replay.Host = rep.Header.Host
	data.Replay.MapName = rep.Header.Map
	data.Replay.MapWidth = rep.Header.MapWidth
	data.Replay.MapHeight = rep.Header.MapHeight
	data.Replay.DurationSeconds = int(rep.Header.Duration().Seconds())
	data.Replay.FrameCount = int32(rep.Header.Frames)
	data.Replay.EngineVersion = rep.Header.Version
	data.Replay.Engine = rep.Header.Engine.String()
	data.Replay.GameSpeed = rep.Header.Speed.String()
	data.Replay.GameType = rep.Header.Type.String()
	data.Replay.AvailSlotsCount = rep.Header.AvailSlotsCount

	// On Melee & Free for all this is always 1, and on Top vs Bottom it's what the game creator set for the home team.
	data.Replay.HomeTeamSize = rep.Header.SubType

	if rep.MapData != nil {
		for _, m := range rep.MapData.MineralFields {
			data.MapContext.MineralFields = append(data.MapContext.MineralFields, models.MapResourcePosition{
				X: int(m.X),
				Y: int(m.Y),
			})
		}
		for _, g := range rep.MapData.Geysers {
			data.MapContext.Geysers = append(data.MapContext.Geysers, models.MapResourcePosition{
				X: int(g.X),
				Y: int(g.Y),
			})
		}
		for _, sl := range rep.MapData.StartLocations {
			data.MapContext.StartLocations = append(data.MapContext.StartLocations, models.MapStartLocation{
				X:      int(sl.X),
				Y:      int(sl.Y),
				SlotID: sl.SlotID,
			})
		}
	}

	// Parse players
	for i, player := range rep.Header.Players {
		if player == nil {
			continue
		}

		// Extract APM and EAPM from computed data
		apm := 0
		eapm := 0
		isWinner := false

		if rep.Computed != nil && i < len(rep.Computed.PlayerDescs) {
			pd := rep.Computed.PlayerDescs[i]
			apm = int(pd.APM)
			eapm = int(pd.EAPM)

			// Check if this player is on the winning team
			if rep.Computed.WinnerTeam != 0 && player.Team == rep.Computed.WinnerTeam {
				isWinner = true
			}
		}

		// Extract start location if available
		var startX, startY, startOclock *int
		if rep.Computed != nil && i < len(rep.Computed.PlayerDescs) {
			pd := rep.Computed.PlayerDescs[i]
			if pd.StartLocation != nil {
				x := int(pd.StartLocation.X)
				y := int(pd.StartLocation.Y)
				startX = &x
				startY = &y

				// Calculate oclock position
				oclock := utils.CalculateStartLocationOclock(int(data.Replay.MapWidth), int(data.Replay.MapHeight), x, y)
				startOclock = &oclock
			}
		}

		data.Players = append(data.Players, &models.Player{
			SlotID:              player.SlotID,
			PlayerID:            player.ID,
			Name:                player.Name,
			Race:                player.Race.String(),
			Type:                player.Type.String(),
			Color:               player.Color.String(),
			Team:                player.Team,
			IsObserver:          player.Observer,
			APM:                 apm,
			EAPM:                eapm, // Effective APM (APM excluding actions deemed ineffective)
			IsWinner:            isWinner,
			StartLocationX:      startX,
			StartLocationY:      startY,
			StartLocationOclock: startOclock,
			Replay:              data.Replay,
		})
	}

	data.Replay.Players = data.Players

	// Initialize pattern detection orchestrator
	patternOrchestrator := patterns.NewOrchestrator()
	patternOrchestrator.Initialize(data.Replay, data.Players, data.MapContext)

	// Create slot-to-player mapping for alliance and vision commands
	playerIDToPlayer := make(map[byte]*models.Player)
	slotIDToPlayer := make(map[byte]*models.Player)
	for _, player := range data.Players {
		playerIDToPlayer[player.PlayerID] = player
		slotIDToPlayer[byte(player.SlotID)] = player
	}

	// Parse commands using the command handling system
	commandRegistry := commands.NewCommandRegistry()
	startTime := rep.Header.StartTime.Unix()

	if rep.Commands != nil {
		for _, cmd := range rep.Commands.Cmds {
			base := cmd.BaseCmd()
			if int(base.PlayerID) >= len(data.Players) {
				continue
			}

			// Process command using the registry
			command := commandRegistry.ProcessCommand(cmd, startTime)

			if command != nil {
				// Set additional fields (registry already sets ReplayID and RunAt)
				command.Frame = int32(base.Frame)
				command.Replay = data.Replay
				command.Player = playerIDToPlayer[base.PlayerID]

				// Edge case: ChatCmd doesn't populate PlayerID, but populates SenderSlotID
				if command.ActionType == "Chat" {
					chatCommand := cmd.(*repcmd.ChatCmd)
					command.Player = slotIDToPlayer[chatCommand.SenderSlotID]
				}

				data.Commands = append(data.Commands, command)

				// Process command through pattern detection
				patternOrchestrator.ProcessCommand(command)
			}
		}
	}

	// Store pattern orchestrator in data for later use
	data.PatternOrchestrator = patternOrchestrator

	return data, nil
}

// CreateReplayFromFileInfo creates a Replay model from file information
func CreateReplayFromFileInfo(filePath, fileName string, fileSize int64, checksum string) *models.Replay {
	return &models.Replay{
		FilePath:     filePath,
		FileChecksum: checksum,
		FileName:     fileName,
		CreatedAt:    time.Now(),
	}
}

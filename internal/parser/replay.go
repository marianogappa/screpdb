package parser

import (
	"fmt"
	"time"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser/commands"
	"github.com/marianogappa/screpdb/internal/screp"
	"github.com/marianogappa/screpdb/internal/tracking"
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
		Replay:         fileInfo,
		Players:        []*models.Player{},
		Commands:       []*models.Command{},
		Units:          []*models.Unit{},
		Buildings:      []*models.Building{},
		Resources:      []*models.Resource{},
		StartLocations: []*models.StartLocation{},
		PlacedUnits:    []*models.PlacedUnit{},
		ChatMessages:   []*models.ChatMessage{},
		LeaveGames:     []*models.LeaveGame{},
	}

	// Parse replay metadata
	data.Replay.ReplayDate = rep.Header.StartTime
	data.Replay.Title = rep.Header.Title
	data.Replay.Host = rep.Header.Host
	data.Replay.MapName = rep.Header.Map
	data.Replay.MapWidth = rep.Header.MapWidth
	data.Replay.MapHeight = rep.Header.MapHeight
	data.Replay.Duration = int(rep.Header.Duration().Seconds())
	data.Replay.FrameCount = int32(rep.Header.Frames)
	data.Replay.Version = rep.Header.Version
	data.Replay.Engine = rep.Header.Engine.String()
	data.Replay.Speed = rep.Header.Speed.String()
	data.Replay.GameType = rep.Header.Type.String()
	data.Replay.SubType = rep.Header.SubType
	data.Replay.AvailSlotsCount = rep.Header.AvailSlotsCount

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
		var startX, startY *int
		if rep.Computed != nil && i < len(rep.Computed.PlayerDescs) {
			pd := rep.Computed.PlayerDescs[i]
			if pd.StartLocation != nil {
				x := int(pd.StartLocation.X)
				y := int(pd.StartLocation.Y)
				startX = &x
				startY = &y
			}
		}

		data.Players = append(data.Players, &models.Player{
			SlotID:         player.SlotID,
			PlayerID:       player.ID,
			Name:           player.Name,
			Race:           player.Race.String(),
			Type:           player.Type.String(),
			Color:          player.Color.String(),
			Team:           player.Team,
			Observer:       player.Observer,
			APM:            apm,
			SPM:            eapm, // Using EAPM as SPM for now
			IsWinner:       isWinner,
			StartLocationX: startX,
			StartLocationY: startY,
		})
	}

	// Parse commands using the new command handling system with unit tracking
	commandRegistry := commands.NewCommandRegistry()
	unitTracker := tracking.NewUnitTracker()
	startTime := rep.Header.StartTime.Unix()

	if rep.Commands != nil {
		for _, cmd := range rep.Commands.Cmds {
			base := cmd.BaseCmd()
			playerID := int64(base.PlayerID)
			if playerID >= int64(len(data.Players)) {
				continue
			}

			// Process command with unit tracker to get resolved units
			resolvedUnits := unitTracker.ProcessCommand(cmd, playerID, int32(base.Frame), time.Unix(startTime+int64(base.Frame.Duration().Seconds()), 0))

			// Handle different command types with resolved unit information
			var command *models.Command
			baseCmd := cmd.BaseCmd()
			cmdTime := time.Unix(startTime+int64(baseCmd.Frame.Duration().Seconds()), 0)

			switch cmd.(type) {
			case *repcmd.SelectCmd:
				// Skip Select commands that don't have any resolved units
				if len(resolvedUnits) == 0 {
					continue // Skip this command as it's useless without unit info
				}
				// Convert tracking.UnitInfo to models.UnitInfo
				modelUnits := make([]*models.UnitInfo, len(resolvedUnits))
				for i, unit := range resolvedUnits {
					modelUnits[i] = &models.UnitInfo{
						UnitTag:      unit.UnitTag,
						UnitType:     unit.UnitType,
						UnitID:       unit.UnitID,
						PlayerID:     unit.PlayerID,
						CreatedAt:    unit.CreatedAt,
						CreatedFrame: unit.CreatedFrame,
						X:            unit.X,
						Y:            unit.Y,
						IsAlive:      unit.IsAlive,
					}
				}
				selectHandler := commands.NewSelectCommandHandler(baseCmd.Type.String(), baseCmd.Type.ID)
				command = selectHandler.HandleWithUnits(cmd, baseCmd, modelUnits)

			case *repcmd.RightClickCmd:
				rightClickHandler := commands.NewRightClickCommandHandler(baseCmd.Type.String(), baseCmd.Type.ID)
				if len(resolvedUnits) > 0 {
					unit := resolvedUnits[0]
					modelUnit := &models.UnitInfo{
						UnitTag:      unit.UnitTag,
						UnitType:     unit.UnitType,
						UnitID:       unit.UnitID,
						PlayerID:     unit.PlayerID,
						CreatedAt:    unit.CreatedAt,
						CreatedFrame: unit.CreatedFrame,
						X:            unit.X,
						Y:            unit.Y,
						IsAlive:      unit.IsAlive,
					}
					command = rightClickHandler.HandleWithUnit(cmd, baseCmd, modelUnit)
				} else {
					command = rightClickHandler.Handle(cmd, baseCmd)
				}

			case *repcmd.TargetedOrderCmd:
				targetedOrderHandler := commands.NewTargetedOrderCommandHandler(baseCmd.Type.String(), baseCmd.Type.ID)
				if len(resolvedUnits) > 0 {
					unit := resolvedUnits[0]
					modelUnit := &models.UnitInfo{
						UnitTag:      unit.UnitTag,
						UnitType:     unit.UnitType,
						UnitID:       unit.UnitID,
						PlayerID:     unit.PlayerID,
						CreatedAt:    unit.CreatedAt,
						CreatedFrame: unit.CreatedFrame,
						X:            unit.X,
						Y:            unit.Y,
						IsAlive:      unit.IsAlive,
					}
					command = targetedOrderHandler.HandleWithUnit(cmd, baseCmd, modelUnit)
				} else {
					command = targetedOrderHandler.Handle(cmd, baseCmd)
				}

			case *repcmd.CancelTrainCmd:
				cancelTrainHandler := commands.NewCancelTrainCommandHandler()
				if len(resolvedUnits) > 0 {
					unit := resolvedUnits[0]
					modelUnit := &models.UnitInfo{
						UnitTag:      unit.UnitTag,
						UnitType:     unit.UnitType,
						UnitID:       unit.UnitID,
						PlayerID:     unit.PlayerID,
						CreatedAt:    unit.CreatedAt,
						CreatedFrame: unit.CreatedFrame,
						X:            unit.X,
						Y:            unit.Y,
						IsAlive:      unit.IsAlive,
					}
					command = cancelTrainHandler.HandleWithUnit(cmd, baseCmd, modelUnit)
				} else {
					command = cancelTrainHandler.Handle(cmd, baseCmd)
				}

			case *repcmd.UnloadCmd:
				unloadHandler := commands.NewUnloadCommandHandler(baseCmd.Type.String(), baseCmd.Type.ID)
				if len(resolvedUnits) > 0 {
					unit := resolvedUnits[0]
					modelUnit := &models.UnitInfo{
						UnitTag:      unit.UnitTag,
						UnitType:     unit.UnitType,
						UnitID:       unit.UnitID,
						PlayerID:     unit.PlayerID,
						CreatedAt:    unit.CreatedAt,
						CreatedFrame: unit.CreatedFrame,
						X:            unit.X,
						Y:            unit.Y,
						IsAlive:      unit.IsAlive,
					}
					command = unloadHandler.HandleWithUnit(cmd, baseCmd, modelUnit)
				} else {
					command = unloadHandler.Handle(cmd, baseCmd)
				}

			case *repcmd.TrainCmd:
				trainHandler := commands.NewTrainCommandHandler()
				if len(resolvedUnits) > 0 {
					unit := resolvedUnits[0]
					modelUnit := &models.UnitInfo{
						UnitTag:      unit.UnitTag,
						UnitType:     unit.UnitType,
						UnitID:       unit.UnitID,
						PlayerID:     unit.PlayerID,
						CreatedAt:    unit.CreatedAt,
						CreatedFrame: unit.CreatedFrame,
						X:            unit.X,
						Y:            unit.Y,
						IsAlive:      unit.IsAlive,
					}
					command = trainHandler.HandleWithUnit(cmd, baseCmd, modelUnit)
				} else {
					command = trainHandler.Handle(cmd, baseCmd)
				}

			case *repcmd.BuildCmd:
				buildHandler := commands.NewBuildCommandHandler()
				if len(resolvedUnits) > 0 {
					unit := resolvedUnits[0]
					modelUnit := &models.UnitInfo{
						UnitTag:      unit.UnitTag,
						UnitType:     unit.UnitType,
						UnitID:       unit.UnitID,
						PlayerID:     unit.PlayerID,
						CreatedAt:    unit.CreatedAt,
						CreatedFrame: unit.CreatedFrame,
						X:            unit.X,
						Y:            unit.Y,
						IsAlive:      unit.IsAlive,
					}
					command = buildHandler.HandleWithUnit(cmd, baseCmd, modelUnit)
				} else {
					command = buildHandler.Handle(cmd, baseCmd)
				}

			case *repcmd.BuildingMorphCmd:
				buildingMorphHandler := commands.NewBuildingMorphCommandHandler()
				if len(resolvedUnits) > 0 {
					unit := resolvedUnits[0]
					modelUnit := &models.UnitInfo{
						UnitTag:      unit.UnitTag,
						UnitType:     unit.UnitType,
						UnitID:       unit.UnitID,
						PlayerID:     unit.PlayerID,
						CreatedAt:    unit.CreatedAt,
						CreatedFrame: unit.CreatedFrame,
						X:            unit.X,
						Y:            unit.Y,
						IsAlive:      unit.IsAlive,
					}
					command = buildingMorphHandler.HandleWithUnit(cmd, baseCmd, modelUnit)
				} else {
					command = buildingMorphHandler.Handle(cmd, baseCmd)
				}

			default:
				// Use the registry for other command types
				command = commandRegistry.ProcessCommand(cmd, data.Replay.ID, startTime)
			}

			if command != nil {
				// Set common fields
				command.ReplayID = data.Replay.ID
				command.PlayerID = playerID
				command.Frame = int32(baseCmd.Frame)
				command.Time = cmdTime
				command.Effective = baseCmd.IneffKind.Effective()

				// Extract unit/building information for specific command types
				switch c := cmd.(type) {
				case *repcmd.BuildCmd:
					if c.Unit != nil {
						// Create building entry
						data.Buildings = append(data.Buildings, &models.Building{
							ReplayID:     data.Replay.ID,
							BuildingID:   c.Unit.ID,
							Type:         c.Unit.Name,
							Name:         c.Unit.Name,
							Created:      command.Time,
							CreatedFrame: command.Frame,
							X:            command.X,
							Y:            command.Y,
						})
					}
				case *repcmd.BuildingMorphCmd:
					if c.Unit != nil {
						// Create building morph entry (could be treated as building update)
						data.Buildings = append(data.Buildings, &models.Building{
							ReplayID:     data.Replay.ID,
							BuildingID:   c.Unit.ID,
							Type:         c.Unit.Name,
							Name:         c.Unit.Name,
							Created:      command.Time,
							CreatedFrame: command.Frame,
							X:            0, // Position would need to be tracked
							Y:            0,
						})
					}
				case *repcmd.TrainCmd:
					if c.Unit != nil {
						// Create unit entry
						data.Units = append(data.Units, &models.Unit{
							ReplayID:     data.Replay.ID,
							UnitID:       c.Unit.ID,
							Type:         c.Unit.Name,
							Created:      command.Time,
							CreatedFrame: command.Frame,
						})
					}
				}

				data.Commands = append(data.Commands, command)
			}
		}
	}

	// Parse map data
	if rep.MapData != nil {
		// Parse resources (mineral fields and geysers)
		for _, mineral := range rep.MapData.MineralFields {
			data.Resources = append(data.Resources, &models.Resource{
				Type:   "mineral",
				X:      int(mineral.X),
				Y:      int(mineral.Y),
				Amount: int(mineral.Amount),
			})
		}

		for _, geyser := range rep.MapData.Geysers {
			data.Resources = append(data.Resources, &models.Resource{
				Type:   "geyser",
				X:      int(geyser.X),
				Y:      int(geyser.Y),
				Amount: int(geyser.Amount),
			})
		}

		// Parse start locations
		for _, startLoc := range rep.MapData.StartLocations {
			data.StartLocations = append(data.StartLocations, &models.StartLocation{
				X: int(startLoc.X),
				Y: int(startLoc.Y),
			})
		}

		// Parse placed units (units that start on the map)
		if rep.MapData.MapGraphics != nil {
			for _, placedUnit := range rep.MapData.MapGraphics.PlacedUnits {
				// Find the player ID from slot ID
				playerID := int64(0)
				for _, player := range rep.Header.Players {
					if player != nil && player.SlotID == uint16(placedUnit.SlotID) {
						playerID = int64(player.ID)
						break
					}
				}

				data.PlacedUnits = append(data.PlacedUnits, &models.PlacedUnit{
					ReplayID: data.Replay.ID,
					PlayerID: playerID,
					Type:     fmt.Sprintf("UnitID_%d", placedUnit.UnitID), // Use UnitID as type since Name is not available
					Name:     fmt.Sprintf("UnitID_%d", placedUnit.UnitID),
					X:        int(placedUnit.X),
					Y:        int(placedUnit.Y),
				})
			}
		}
	}

	// Extract chat messages and leave game commands from computed data
	if rep.Computed != nil {
		// Extract chat messages
		for _, chatCmd := range rep.Computed.ChatCmds {
			baseCmd := chatCmd.BaseCmd()
			data.ChatMessages = append(data.ChatMessages, &models.ChatMessage{
				ReplayID:     data.Replay.ID,
				PlayerID:     int64(baseCmd.PlayerID),
				SenderSlotID: chatCmd.SenderSlotID,
				Message:      chatCmd.Message,
				Frame:        int32(baseCmd.Frame),
				Time:         time.Unix(startTime+int64(baseCmd.Frame.Duration().Seconds()), 0),
			})
		}

		// Extract leave game commands
		for _, leaveCmd := range rep.Computed.LeaveGameCmds {
			baseCmd := leaveCmd.BaseCmd()
			reason := ""
			if leaveCmd.Reason != nil {
				reason = leaveCmd.Reason.String()
			}
			data.LeaveGames = append(data.LeaveGames, &models.LeaveGame{
				ReplayID: data.Replay.ID,
				PlayerID: int64(baseCmd.PlayerID),
				Reason:   reason,
				Frame:    int32(baseCmd.Frame),
				Time:     time.Unix(startTime+int64(baseCmd.Frame.Duration().Seconds()), 0),
			})
		}
	}

	return data, nil
}

// CreateReplayFromFileInfo creates a Replay model from file information
func CreateReplayFromFileInfo(filePath, fileName string, fileSize int64, checksum string) *models.Replay {
	return &models.Replay{
		FilePath:     filePath,
		FileChecksum: checksum,
		FileName:     fileName,
		FileSize:     fileSize,
		CreatedAt:    time.Now(),
	}
}

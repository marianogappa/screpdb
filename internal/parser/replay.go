package parser

import (
	"fmt"
	"time"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser/commands"
	"github.com/marianogappa/screpdb/internal/screp"
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

	// Parse commands using the new command handling system
	commandRegistry := commands.NewCommandRegistry()
	startTime := rep.Header.StartTime.Unix()

	if rep.Commands != nil {
		for _, cmd := range rep.Commands.Cmds {
			base := cmd.BaseCmd()
			playerID := int64(base.PlayerID)
			if playerID >= int64(len(data.Players)) {
				continue
			}

			// Process command using the registry
			command := commandRegistry.ProcessCommand(cmd, data.Replay.ID, startTime)
			if command != nil {
				// Extract command effectiveness from screp's computed data
				baseCmd := cmd.BaseCmd()
				command.Effective = baseCmd.IneffKind.Effective()
				// Extract unit/building information for specific command types
				switch c := cmd.(type) {
				case *repcmd.BuildCmd:
					if c.Unit != nil {
						// Create building entry
						data.Buildings = append(data.Buildings, &models.Building{
							ReplayID:   data.Replay.ID,
							BuildingID: c.Unit.ID,
							Type:       c.Unit.Name,
							Name:       c.Unit.Name,
							Created:    command.Time,
							X:          command.X,
							Y:          command.Y,
							HP:         0, // Would need to track from game state
							MaxHP:      0,
							Shield:     0,
							MaxShield:  0,
							Energy:     0,
							MaxEnergy:  0,
						})
					}
				case *repcmd.BuildingMorphCmd:
					if c.Unit != nil {
						// Create building morph entry (could be treated as building update)
						data.Buildings = append(data.Buildings, &models.Building{
							ReplayID:   data.Replay.ID,
							BuildingID: c.Unit.ID,
							Type:       c.Unit.Name,
							Name:       c.Unit.Name,
							Created:    command.Time,
							X:          0, // Position would need to be tracked
							Y:          0,
							HP:         0,
							MaxHP:      0,
							Shield:     0,
							MaxShield:  0,
							Energy:     0,
							MaxEnergy:  0,
						})
					}
				case *repcmd.TrainCmd:
					if c.Unit != nil {
						// Create unit entry
						data.Units = append(data.Units, &models.Unit{
							ReplayID:  data.Replay.ID,
							UnitID:    c.Unit.ID,
							Type:      c.Unit.Name,
							Name:      c.Unit.Name,
							Created:   command.Time,
							X:         0, // Would need to track from game state
							Y:         0,
							HP:        0,
							MaxHP:     0,
							Shield:    0,
							MaxShield: 0,
							Energy:    0,
							MaxEnergy: 0,
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
					ReplayID:  data.Replay.ID,
					PlayerID:  playerID,
					Type:      fmt.Sprintf("UnitID_%d", placedUnit.UnitID), // Use UnitID as type since Name is not available
					Name:      fmt.Sprintf("UnitID_%d", placedUnit.UnitID),
					X:         int(placedUnit.X),
					Y:         int(placedUnit.Y),
					HP:        0, // Not available in PlacedUnit
					MaxHP:     0,
					Shield:    0,
					MaxShield: 0,
					Energy:    0,
					MaxEnergy: 0,
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

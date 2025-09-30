package parser

import (
	"fmt"
	"time"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
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
	for _, player := range rep.Header.Players {
		if player == nil {
			continue
		}

		// Calculate APM and SPM (these would need to be computed from commands)
		apm := 0
		spm := 0
		isWinner := false // This would need to be determined from game outcome

		data.Players = append(data.Players, &models.Player{
			SlotID:   player.SlotID,
			PlayerID: player.ID,
			Name:     player.Name,
			Race:     player.Race.String(),
			Type:     player.Type.String(),
			Color:    player.Color.String(),
			Team:     player.Team,
			Observer: player.Observer,
			APM:      apm,
			SPM:      spm,
			IsWinner: isWinner,
		})
	}

	// Parse commands and extract unit/building information
	if rep.Commands != nil {
		for _, cmd := range rep.Commands.Cmds {
			base := cmd.BaseCmd()
			playerID := int64(base.PlayerID)
			if playerID >= int64(len(data.Players)) {
				continue
			}

			command := &models.Command{
				PlayerID:   playerID,
				Frame:      int32(base.Frame),
				Time:       rep.Header.StartTime.Add(base.Frame.Duration()),
				ActionType: base.Type.String(),
				ActionID:   byte(base.Type.ID),
				UnitID:     0,
				TargetID:   0,
				X:          0,
				Y:          0,
				Data:       cmd.Params(true),
				Effective:  base.IneffKind.Effective(),
			}

			// Extract specific command data based on command type
			switch c := cmd.(type) {
			case *repcmd.AllianceCmd:
				// Alliance commands don't have position data
				command.Data = fmt.Sprintf("AlliedVictory:%t,SlotIDs:%v", c.AlliedVictory, c.SlotIDs)
			case *repcmd.BuildCmd:
				command.X = int(c.Pos.X)
				command.Y = int(c.Pos.Y)
				if c.Unit != nil {
					command.UnitID = byte(c.Unit.ID)
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
					command.UnitID = byte(c.Unit.ID)
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
			case *repcmd.CancelTrainCmd:
				command.UnitID = byte(c.UnitTag)
			case *repcmd.ChatCmd:
				command.Data = fmt.Sprintf("SenderSlotID:%d,Message:%s", c.SenderSlotID, c.Message)
			case *repcmd.GameSpeedCmd:
				// Game speed commands don't have position data
			case *repcmd.GeneralCmd:
				command.Data = fmt.Sprintf("RawData:%v", c.Data)
			case *repcmd.HotkeyCmd:
				if c.HotkeyType != nil {
					command.Data = fmt.Sprintf("HotkeyType:%s,Group:%d", c.HotkeyType.Name, c.Group)
				}
			case *repcmd.LandCmd:
				command.X = int(c.Pos.X)
				command.Y = int(c.Pos.Y)
				if c.Unit != nil {
					command.UnitID = byte(c.Unit.ID)
				}
			case *repcmd.LatencyCmd:
				// Latency commands don't have position data
			case *repcmd.LeaveGameCmd:
				// Leave game commands don't have position data
			case *repcmd.LiftOffCmd:
				command.X = int(c.Pos.X)
				command.Y = int(c.Pos.Y)
			case *repcmd.MinimapPingCmd:
				command.X = int(c.Pos.X)
				command.Y = int(c.Pos.Y)
			case *repcmd.ParseErrCmd:
				// Parse error commands don't have position data
			case *repcmd.QueueableCmd:
				// QueueableCmd is a base type, not a specific command
			case *repcmd.RightClickCmd:
				command.X = int(c.Pos.X)
				command.Y = int(c.Pos.Y)
			case *repcmd.SelectCmd:
				command.Data = fmt.Sprintf("UnitTags:%v", c.UnitTags)
			case *repcmd.TargetedOrderCmd:
				command.X = int(c.Pos.X)
				command.Y = int(c.Pos.Y)
				if c.Unit != nil {
					command.TargetID = byte(c.Unit.ID)
				}
			case *repcmd.TechCmd:
				if c.Tech != nil {
					command.Data = fmt.Sprintf("Tech:%s", c.Tech.Name)
				}
			case *repcmd.TrainCmd:
				if c.Unit != nil {
					command.UnitID = byte(c.Unit.ID)
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
			case *repcmd.UnloadCmd:
				command.UnitID = byte(c.UnitTag)
			case *repcmd.UpgradeCmd:
				if c.Upgrade != nil {
					command.Data = fmt.Sprintf("Upgrade:%s", c.Upgrade.Name)
				}
			case *repcmd.VisionCmd:
				command.Data = fmt.Sprintf("SlotIDs:%v", c.SlotIDs)
			default:
				// Handle any unknown command types
				command.Data = fmt.Sprintf("UnknownCommand:%T", cmd)
			}

			data.Commands = append(data.Commands, command)
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

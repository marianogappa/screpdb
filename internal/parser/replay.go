package parser

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/earlyfilter"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser/commands"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/screp"
	"github.com/marianogappa/screpdb/internal/utils"
)

// Options controls optional behaviour of ParseReplayWithOptions. The
// zero-value runs the early-game spam filter without writing any debug
// trace.
type Options struct {
	// EarlyFilterDebugDir, when non-empty, makes the early-game spam
	// filter dump a per-replay JSON trace into this directory. See
	// internal/earlyfilter for the trace format.
	EarlyFilterDebugDir string
}

// ParseReplay parses a StarCraft: Brood War replay file and returns
// structured data. Equivalent to ParseReplayWithOptions with default Options.
func ParseReplay(filePath string, fileInfo *models.Replay) (*models.ReplayData, error) {
	return ParseReplayWithOptions(filePath, fileInfo, Options{})
}

// ParseReplayWithOptions is the configurable entry point. The early-game
// spam filter always runs; opts.EarlyFilterDebugDir controls only the
// optional JSON debug trace.
func ParseReplayWithOptions(filePath string, fileInfo *models.Replay, opts Options) (*models.ReplayData, error) {
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

	switch {
	case data.Replay.GameType == "Use map settings":
		data.Replay.MapKind = "UseMapSettings"
	case rep.MapData != nil && len(rep.MapData.MineralFields) > 0 && rep.MapData.MineralFields[0].Amount > 10000:
		data.Replay.MapKind = "Money"
	default:
		data.Replay.MapKind = "Regular"
	}
	if layout, err := buildMapContextLayoutFromReplay(filePath); err == nil && layout != nil {
		data.MapContext.Layout = layout
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
	data.Replay.TeamFormat, data.Replay.Matchup = computeTeamFormatAndMatchup(data.Players)

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
				// Set additional fields (registry already sets ReplayID)
				command.Frame = int32(base.Frame)
				command.Replay = data.Replay
				command.Player = playerIDToPlayer[base.PlayerID]

				// Edge case: ChatCmd doesn't populate PlayerID, but populates SenderSlotID
				if command.ActionType == "Chat" {
					chatCommand := cmd.(*repcmd.ChatCmd)
					command.Player = slotIDToPlayer[chatCommand.SenderSlotID]
				}

				data.Commands = append(data.Commands, command)
			}
		}
	}

	// Run the early-game spam filter before pattern detection so the
	// orchestrator only sees commands the filter believes were real.
	filterResult := earlyfilter.Apply(data.Replay, data.Players, data.MapContext, data.Commands, earlyfilter.Options{
		DebugDir: opts.EarlyFilterDebugDir,
	})
	data.Commands = filterResult.Commands

	// Feed the filtered command stream through pattern detection.
	for _, command := range data.Commands {
		patternOrchestrator.ProcessCommand(command)
	}

	// Store pattern orchestrator in data for later use
	data.PatternOrchestrator = patternOrchestrator

	return data, nil
}

// computeTeamFormatAndMatchup derives team_format (e.g. "1v1", "2v2", "2v2v2") and
// matchup (e.g. "PvT", "PTvZZ") from player race+team. Observers are excluded.
// Within each team, race initials are sorted lex; teams are then sorted lex.
// Team sizes in team_format are sorted descending.
func computeTeamFormatAndMatchup(players []*models.Player) (string, string) {
	teams := map[byte][]string{}
	for _, p := range players {
		if p.IsObserver {
			continue
		}
		teams[p.Team] = append(teams[p.Team], p.Race)
	}
	if len(teams) == 0 {
		return "", ""
	}

	sizes := make([]int, 0, len(teams))
	teamRaces := make([]string, 0, len(teams))
	for _, races := range teams {
		sizes = append(sizes, len(races))
		initials := make([]byte, 0, len(races))
		for _, r := range races {
			initials = append(initials, raceInitial(r))
		}
		sort.Slice(initials, func(i, j int) bool { return initials[i] < initials[j] })
		teamRaces = append(teamRaces, string(initials))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sizes)))
	sort.Strings(teamRaces)

	parts := make([]string, len(sizes))
	for i, s := range sizes {
		parts[i] = strconv.Itoa(s)
	}
	return strings.Join(parts, "v"), strings.Join(teamRaces, "v")
}

// raceInitial returns the first letter of the race name (P/T/Z/R/U). Falls back
// to '?' for empty input.
func raceInitial(race string) byte {
	if race == "" {
		return '?'
	}
	return race[0]
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

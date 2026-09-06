package parser

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	scraprep "github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/rep/repcore"
	"github.com/icza/screp/repparser/repdecoder"
	"github.com/marianogappa/screpdb/internal/builddedup"
	"github.com/marianogappa/screpdb/internal/cmddedup"
	"github.com/marianogappa/screpdb/internal/earlyfilter"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser/commands"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/screp"
	"github.com/marianogappa/screpdb/internal/unittags"
	"github.com/marianogappa/screpdb/internal/utils"
)

// The zero value runs the early-game spam filter without writing a trace.
type Options struct {
	// When non-empty, the early-game spam filter dumps a per-replay JSON trace
	// here. See internal/earlyfilter for the format.
	EarlyFilterDebugDir string
}

// ParseReplay is ParseReplayWithOptions with default Options.
func ParseReplay(filePath string, fileInfo *models.Replay) (*models.ReplayData, error) {
	return ParseReplayWithOptions(filePath, fileInfo, Options{})
}

// The early-game spam filter always runs; opts.EarlyFilterDebugDir controls
// only the optional JSON debug trace.
func ParseReplayWithOptions(filePath string, fileInfo *models.Replay, opts Options) (*models.ReplayData, error) {
	rep, err := screp.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse replay file: %w", err)
	}

	data := &models.ReplayData{
		Replay:     fileInfo,
		Players:    []*models.Player{},
		Commands:   []*models.Command{},
		MapContext: &models.ReplayMapContext{},
	}

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

	// Always 1 on Melee and FFA; on Top vs Bottom it is what the creator set.
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
	case isMoneyMap(rep):
		data.Replay.MapKind = "Money"
	default:
		data.Replay.MapKind = "Regular"
	}
	if layout, err := buildMapContextLayoutFromReplay(filePath, data.Replay.MapName, int(rep.Header.MapWidth), int(rep.Header.MapHeight)); err == nil && layout != nil {
		data.MapContext.Layout = layout
	}

	data.Replay.GameSource = deriveGameSource(rep)
	data.Replay.LobbyKind = deriveLobbyKind(data.Replay.GameSource, data.Replay.Title)

	for i, player := range rep.Header.Players {
		if player == nil {
			continue
		}

		apm := 0
		eapm := 0
		isWinner := false

		if rep.Computed != nil && i < len(rep.Computed.PlayerDescs) {
			pd := rep.Computed.PlayerDescs[i]
			apm = int(pd.APM)
			eapm = int(pd.EAPM)

			if rep.Computed.WinnerTeam != 0 && player.Team == rep.Computed.WinnerTeam {
				isWinner = true
			}
		}

		var startX, startY, startOclock *int
		if rep.Computed != nil && i < len(rep.Computed.PlayerDescs) {
			pd := rep.Computed.PlayerDescs[i]
			if pd.StartLocation != nil {
				x := int(pd.StartLocation.X)
				y := int(pd.StartLocation.Y)
				startX = &x
				startY = &y

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

	patternOrchestrator := patterns.NewOrchestrator()
	patternOrchestrator.Initialize(data.Replay, data.Players, data.MapContext)

	playerIDToPlayer := make(map[byte]*models.Player)
	slotIDToPlayer := make(map[byte]*models.Player)
	for _, player := range data.Players {
		playerIDToPlayer[player.PlayerID] = player
		slotIDToPlayer[byte(player.SlotID)] = player
	}

	commandRegistry := commands.NewCommandRegistry()
	startTime := rep.Header.StartTime.Unix()

	// Infer pixel coordinates for production / research / cancel commands the raw
	// stream leaves spatially blank, by binding each to the producing building
	// recovered from selection state. Keyed by index into rep.Commands.Cmds (#175).
	commandCoords := unittags.Coordinates(rep, data.Players)

	// One morph command can create several units (all selected larvae morph at
	// once). The early filter caps this intended count by what was available.
	morphSelectionSizes := unittags.MorphSelectionSizes(rep)

	if rep.Commands != nil {
		for i, cmd := range rep.Commands.Cmds {
			base := cmd.BaseCmd()

			command := commandRegistry.ProcessCommand(cmd, startTime)
			if command == nil {
				continue
			}

			command.Frame = int32(base.Frame)
			command.Replay = data.Replay
			command.Player = playerIDToPlayer[base.PlayerID]

			// ChatCmd populates SenderSlotID rather than PlayerID. The assertion is
			// guarded so a future screp change can't panic the parse.
			if command.ActionType == "Chat" {
				if chatCommand, ok := cmd.(*repcmd.ChatCmd); ok {
					command.Player = slotIDToPlayer[chatCommand.SenderSlotID]
				}
			}

			// PlayerIDs are not contiguous (observer/computer slots leave gaps), so a
			// command's PlayerID can be absent here. Downstream already ignores nil-Player
			// commands, and the persistence pass dereferences it — emitting them would
			// crash ingestion (#234).
			if command.Player == nil {
				continue
			}

			if cc, ok := commandCoords[i]; ok && command.X == nil && command.Y == nil {
				x, y := cc.X, cc.Y
				command.X, command.Y = &x, &y
			}

			if n, ok := morphSelectionSizes[i]; ok {
				command.SelectedUnits = n
			}

			data.Commands = append(data.Commands, command)
		}
	}

	var repSaverPID *byte
	if rep.Computed != nil && rep.Computed.RepSaverPlayerID != nil {
		v := *rep.Computed.RepSaverPlayerID
		repSaverPID = &v
	}

	// Runs on the full unfiltered stream because earlyfilter / dedup don't touch
	// Alliance commands, which keeps the analyzer independent of those passes.
	var allianceResult *AllianceResult
	if data.Replay.GameType == "Melee" && countActiveMeleePlayers(data.Players) > 2 {
		activity := ComputeActivity(data.Players, data.Commands, data.Replay.DurationSeconds)
		ar := AnalyzeAlliances(data.Players, data.Commands, data.Replay.DurationSeconds, activity)
		allianceResult = &ar
		data.Replay.TeamStacking = ar.TeamStackingFlag
		data.AllianceSnapshots = allianceSnapshotsToModels(ar.Snapshots)

		// Team DISPLAY prefers our longest-held topology whenever real mutual
		// alliances were observed: screp's computeMeleeTeams only inspects the first
		// ~90s and, finding nothing, makes everyone a singleton, so trusting it misses
		// alliances formed later. No single static set captures alliance dynamism —
		// longest-held is the most representative, and the Alliances tab has the
		// full timeline.
		if ar.AnyMutualResolved {
			for _, p := range data.Players {
				if p.IsObserver || p.Type == "Computer" {
					continue
				}
				if t, ok := ar.ResolvedTeams[p.PlayerID]; ok {
					p.Team = t
				}
			}
			data.Replay.TeamFormat, data.Replay.Matchup = computeTeamFormatAndMatchup(data.Players)
		}

		if !allActivePlayersHaveTeam(data.Players) {
			data.Replay.TeamInfoIncomplete = true
		}

		// Winner attribution is deliberately decoupled from the display teams: it
		// groups by the END-OF-GAME coalition, so a team that allied after screp's 90s
		// window still gets credited, and a winning coalition may span two display
		// teams. Runs even with incomplete team assignment — a clear surviving
		// coalition is still a clear winner.
		DeriveWinnersFromFinalTopology(data.Players, data.Commands, ar, repSaverPID)
	}

	// A saver disconnect masquerades as "everyone else left", and both winner paths
	// would credit the saver's team a phantom win — the game never resolved, so
	// nobody wins (issue #358). Also threaded into worldstate so the timeline
	// condenses the phantom leave cluster into one connection-lost event.
	if md := DetectMassDisconnectEnd(data.Players, data.Commands, repSaverPID, data.Replay.DurationSeconds); md != nil {
		for _, p := range data.Players {
			if p != nil {
				p.IsWinner = false
			}
		}
		patternOrchestrator.SetMassDisconnectEnd(md.SaverPID, md.ClusterSecond)
	}

	// Reconstruct selection state from the raw stream's Select/Hotkey tags, which
	// command extraction above discards, and plan the tag-based build dedup. The
	// plan is applied inside the early filter.
	unitTagEvidence := unittags.Analyze(rep)
	buildDedupPlan := builddedup.Compute(unitTagEvidence, data.Players)

	// Encoding needs the raw stream's Select/Hotkey tags, which command extraction
	// discards, so it runs here rather than in storage.
	data.HotkeyStreams = map[byte][]byte{}
	for pid, events := range hotkeystream.Extract(rep) {
		data.HotkeyStreams[pid] = hotkeystream.Encode(events)
	}

	// A Train/Morph proves the producing building — and so its base — is still
	// alive, refreshing ownership where movement/build commands alone would let the
	// base time out.
	patternOrchestrator.SetProductionSignals(unitTagEvidence)

	// Gated on Zerg + Spire + muta production (#194).
	patternOrchestrator.SetMutaHarass(unittags.DetectMutaHarass(rep, data.Players))

	// Filter before pattern detection so the orchestrator only sees commands the
	// filter believes were real.
	filterResult := earlyfilter.Apply(data.Replay, data.Players, data.MapContext, data.Commands, earlyfilter.Options{
		DebugDir:   opts.EarlyFilterDebugDir,
		ShouldDrop: buildDedupPlan.ShouldDrop,
	})
	data.Commands = filterResult.Commands

	// A larva morph cancelled before the first Overlord is provably a Drone, so it
	// refunds a supply and must not inflate the opener count. Runs on the filtered
	// stream so the count sees only morphs that actually stuck.
	data.Commands = commands.DropCancelledMorphs(data.Commands)

	// Operates over the whole game, not just the early window: Forge rebuilt
	// mid-Ground-Weapons-1, double-clicked Lurker Aspect, and similar.
	data.Commands = cmddedup.Dedup(data.Commands)

	// Rewrite Right Click → Load / LoadBunker when the target is a transport, so
	// the drop detector can pair Loads against later Unloads. Must precede pattern
	// detection so the orchestrator sees the rewritten action_type.
	commands.ClassifyLoads(data.Commands)

	for _, command := range data.Commands {
		patternOrchestrator.ProcessCommand(command)
	}

	// Push alliance-derived events into the orchestrator's channel so they land in
	// replay_events alongside leave_game and attacks; its Finalize sorts the merge.
	if allianceResult != nil {
		extraEvents := BuildAllianceDerivedEvents(data.Players, *allianceResult)
		patternOrchestrator.AppendReplayEvents(extraEvents)
	}

	data.PatternOrchestrator = patternOrchestrator

	data.FingerprintVectors = extractFingerprintVectors(rep)

	return data, nil
}

func allianceSnapshotsToModels(snapshots []AllianceSnapshot) []models.AllianceSnapshot {
	if len(snapshots) == 0 {
		return nil
	}
	out := make([]models.AllianceSnapshot, len(snapshots))
	for i, s := range snapshots {
		out[i] = models.AllianceSnapshot{Sec: s.Sec, Teams: s.Teams, Stacking: s.Stacking}
	}
	return out
}

// Observers are excluded. Within each team race initials sort lexically, then
// teams sort lexically; team sizes in team_format sort descending.
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

// The population that participates in alliance topology.
func countActiveMeleePlayers(players []*models.Player) int {
	n := 0
	for _, p := range players {
		if p == nil || p.IsObserver || p.Type == "Computer" {
			continue
		}
		n++
	}
	return n
}

// Drives the team_info_incomplete flag.
func allActivePlayersHaveTeam(players []*models.Player) bool {
	for _, p := range players {
		if p == nil || p.IsObserver || p.Type == "Computer" {
			continue
		}
		if p.Team == 0 {
			return false
		}
	}
	return true
}

// Falls back to '?' for empty input.
func raceInitial(race string) byte {
	if race == "" {
		return '?'
	}
	return race[0]
}

func deriveGameSource(r *scraprep.Replay) string {
	if r.ShieldBattery != nil {
		return "ShieldBattery"
	}
	humanCount := 0
	for _, p := range r.Header.Players {
		if p != nil && p.Type == repcore.PlayerTypeHuman {
			humanCount++
		}
	}
	if humanCount == 1 {
		return "SinglePlayer"
	}
	if r.RepFormat == repdecoder.RepFormatLegacy {
		return "PreSCR"
	}
	return "AssumedBattleNet"
}

func deriveLobbyKind(gameSource, title string) string {
	switch gameSource {
	case "PreSCR", "SinglePlayer", "ShieldBattery":
		return "Unknown"
	}
	if len(title) == 12 && isMatchmakingTitle(title) {
		return "Matchmaking"
	}
	return "Custom"
}

func isMatchmakingTitle(title string) bool {
	for i := 0; i < len(title); i++ {
		if title[i] < 0x41 || title[i] > 0x7A {
			return false
		}
	}
	return true
}

func CreateReplayFromFileInfo(filePath, fileName string, fileSize int64, checksum string) *models.Replay {
	return &models.Replay{
		FilePath:     filePath,
		FileChecksum: checksum,
		FileName:     fileName,
		CreatedAt:    time.Now(),
	}
}

// What every mineral field carries on a stock melee map. Anything above it is a
// deliberately enriched patch, which is what makes a map a "money" map.
const standardMineralPatch = 1500

// isMoneyMap classifies by the MEDIAN mineral field, not any single one.
//
// Fields arrive in whatever order the map file stores them, and money maps mix
// amounts: "Big Game Hunters - Remastered" lists a 10000 field first and 20000
// for most of the rest, while plain "Big Game Hunters" lists a 20000 first — so
// sampling MineralFields[0] classified two versions of the same map
// differently. The median also survives partial mining: a regular map picks up
// a tail of half-empty patches as the game goes on, and the median stays pinned
// at 1500 where a mean would drift.
func isMoneyMap(rep *scraprep.Replay) bool {
	if rep == nil || rep.MapData == nil || len(rep.MapData.MineralFields) == 0 {
		return false
	}
	amounts := make([]int, 0, len(rep.MapData.MineralFields))
	for _, field := range rep.MapData.MineralFields {
		amounts = append(amounts, int(field.Amount))
	}
	sort.Ints(amounts)
	return amounts[len(amounts)/2] > standardMineralPatch
}

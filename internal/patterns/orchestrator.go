package patterns

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/detectors"
)

// ReplayLevelDetectorFactory creates a replay-level detector
type ReplayLevelDetectorFactory func() core.Detector

// PlayerLevelDetectorFactory creates a player-level detector for a specific player
type PlayerLevelDetectorFactory func(replayPlayerID byte) core.Detector

// TeamLevelDetectorFactory creates a team-level detector for a specific team
type TeamLevelDetectorFactory func(team byte) core.Detector

var (
	// replayLevelDetectors is the list of replay-level detector factories
	replayLevelDetectors = []ReplayLevelDetectorFactory{
		func() core.Detector { return detectors.NewHadCarriersReplayDetector() },
	}

	// playerLevelDetectors is the list of player-level detector factories
	playerLevelDetectors = []PlayerLevelDetectorFactory{
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewDidCarriersPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
	}

	// teamLevelDetectors is the list of team-level detector factories
	teamLevelDetectors = []TeamLevelDetectorFactory{
		func(team byte) core.Detector {
			detector := detectors.NewDidCarriersTeamDetector()
			detector.SetTeam(team)
			return detector
		},
	}
)

// Orchestrator manages all pattern detectors for a replay
type Orchestrator struct {
	detectors []core.Detector
	results   []*core.PatternResult
	replay    *models.Replay
	players   []*models.Player
}

// NewOrchestrator creates a new pattern detection orchestrator
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		detectors: make([]core.Detector, 0),
		results:   make([]*core.PatternResult, 0),
	}
}

// Initialize initializes all detectors with the replay and players
// This creates detector instances for each player and team as needed
func (o *Orchestrator) Initialize(replay *models.Replay, players []*models.Player) {
	o.replay = replay
	o.players = players

	// Create replay-level detectors (one per replay)
	for _, factory := range replayLevelDetectors {
		o.detectors = append(o.detectors, factory())
	}

	// Create player-level detectors (one per player)
	for _, player := range players {
		if player.IsObserver {
			continue
		}
		for _, factory := range playerLevelDetectors {
			o.detectors = append(o.detectors, factory(player.PlayerID))
		}
	}

	// Create team-level detectors (one per team, but only if teams are detected)
	teamMap := make(map[byte]bool)
	for _, player := range players {
		if player.IsObserver {
			continue
		}
		if player.Team != 0 {
			teamMap[player.Team] = true
		}
	}

	for team := range teamMap {
		for _, factory := range teamLevelDetectors {
			o.detectors = append(o.detectors, factory(team))
		}
	}

	// Initialize all detectors
	for _, detector := range o.detectors {
		detector.Initialize(replay, players)
	}
}

// ProcessCommand processes a command through all active detectors
func (o *Orchestrator) ProcessCommand(command *models.Command) {
	for _, detector := range o.detectors {
		if !detector.IsFinished() {
			finished := detector.ProcessCommand(command)
			if finished {
				// Detector finished, check if it has a result to save
				if result := detector.GetResult(); result != nil && detector.ShouldSave() {
					o.results = append(o.results, result)
				}
			}
		}
	}
}

// GetResults returns all pattern results that should be saved
func (o *Orchestrator) GetResults() []*core.PatternResult {
	// Collect any remaining results from detectors that finished after command processing
	for _, detector := range o.detectors {
		if detector.IsFinished() {
			if result := detector.GetResult(); result != nil && detector.ShouldSave() {
				// Check if we already have this result
				found := false
				for _, existing := range o.results {
					if existing.PatternName == result.PatternName &&
						existing.Level == result.Level &&
						existing.ReplayID == result.ReplayID &&
						((existing.Team == nil && result.Team == nil) || (existing.Team != nil && result.Team != nil && *existing.Team == *result.Team)) &&
						((existing.PlayerID == nil && result.PlayerID == nil) || (existing.PlayerID != nil && result.PlayerID != nil && *existing.PlayerID == *result.PlayerID)) {
						found = true
						break
					}
				}
				if !found {
					o.results = append(o.results, result)
				}
			}
		}
	}
	return o.results
}

// ConvertResultsToDatabaseIDs converts pattern results from replay player IDs to database player IDs
func (o *Orchestrator) ConvertResultsToDatabaseIDs(playerIDMap map[byte]int64) {
	// Convert player-level results
	for _, result := range o.results {
		if result.Level == core.LevelPlayer && result.PlayerID == nil && result.ReplayPlayerID != nil {
			if dbPlayerID, exists := playerIDMap[*result.ReplayPlayerID]; exists {
				result.PlayerID = &dbPlayerID
			}
		}
	}
}

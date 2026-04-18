package patterns

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/detectors"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// ReplayLevelDetectorFactory creates a replay-level detector
type ReplayLevelDetectorFactory func() core.Detector

// PlayerLevelDetectorFactory creates a player-level detector for a specific player
type PlayerLevelDetectorFactory func(replayPlayerID byte) core.Detector

var (
	// replayLevelDetectors is the list of replay-level detector factories
	replayLevelDetectors = []ReplayLevelDetectorFactory{}

	// playerLevelDetectors is the list of player-level detector factories
	playerLevelDetectors = []PlayerLevelDetectorFactory{
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewUsedHotkeyGroupsPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewNeverUsedHotkeysPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewQuickFactoryPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewMechPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewBattlecruisersPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewCarriersPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewMadeDropsPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewMadeRecallsPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewThrewNukesPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewBecameTerranPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewBecameZergPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewFastExpaPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewGateThenForgePlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewForgeThenGatePlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewNeverUpgradedPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewNeverResearchedPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewHatchBeforePoolPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewExpaBeforeGatePlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewExpaBeforeBarracksPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
		func(replayPlayerID byte) core.Detector {
			detector := detectors.NewViewportMultitaskingPlayerDetector()
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		},
	}
)

// Orchestrator manages all pattern detectors for a replay
type Orchestrator struct {
	detectors  []core.Detector
	results    []*core.PatternResult
	replay     *models.Replay
	players    []*models.Player
	worldState *worldstate.Engine
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
func (o *Orchestrator) Initialize(replay *models.Replay, players []*models.Player, mapContext *models.ReplayMapContext) {
	o.replay = replay
	o.players = players
	o.worldState = worldstate.NewEngine(replay, players, mapContext)

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

	// Initialize all detectors
	for _, detector := range o.detectors {
		if consumer, ok := detector.(core.WorldStateConsumer); ok {
			consumer.SetWorldState(o.worldState)
		}
		detector.Initialize(replay, players)
	}
}

// ProcessCommand processes a command through all active detectors
func (o *Orchestrator) ProcessCommand(command *models.Command) {
	if o.worldState != nil {
		o.worldState.ProcessCommand(command)
	}

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
	// Give detectors a chance to finish using full replay context.
	for _, detector := range o.detectors {
		detector.Finalize()
	}

	// Collect any remaining results from detectors that finished after command processing
	for _, detector := range o.detectors {
		if detector.IsFinished() {
			if result := detector.GetResult(); result != nil && detector.ShouldSave() {
				// Check if we already have this result
				found := false
				for _, existing := range o.results {
					samePlayer := (existing.PlayerID == nil && result.PlayerID == nil) || (existing.PlayerID != nil && result.PlayerID != nil && *existing.PlayerID == *result.PlayerID)
					// Before DB IDs are mapped, player-level results are identified by ReplayPlayerID.
					sameReplayPlayer := (existing.ReplayPlayerID == nil && result.ReplayPlayerID == nil) ||
						(existing.ReplayPlayerID != nil && result.ReplayPlayerID != nil && *existing.ReplayPlayerID == *result.ReplayPlayerID)

					if existing.PatternName == result.PatternName &&
						existing.Level == result.Level &&
						existing.ReplayID == result.ReplayID &&
						samePlayer &&
						sameReplayPlayer {
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

func (o *Orchestrator) ReplayEvents() []worldstate.ReplayEvent {
	if o.worldState == nil {
		return nil
	}
	return o.worldState.ReplayEvents()
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

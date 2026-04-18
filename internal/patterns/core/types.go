package core

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// AlgorithmVersion is the current version of the pattern detection algorithm
// Increment this when the algorithm changes to trigger re-detection
const AlgorithmVersion = 8

// DetectorLevel indicates at which level a pattern detector operates
type DetectorLevel string

const (
	LevelReplay DetectorLevel = "replay"
	LevelPlayer DetectorLevel = "player"
)

// PatternResult represents the result of a pattern detection
type PatternResult struct {
	PatternName    string
	Level          DetectorLevel
	ReplayID       int64
	PlayerID       *int64 // nil for replay-level patterns (database ID)
	ReplayPlayerID *byte  // Temporary: replay player ID (byte) for player-level results, converted to PlayerID later
	ValueBool      *bool
	ValueInt       *int
	ValueString    *string
	ValueTime      *int64 // Unix timestamp
}

// Detector is the interface that all pattern detectors must implement
type Detector interface {
	// Name returns the unique name of this pattern detector
	Name() string

	// Level returns the level at which this detector operates
	Level() DetectorLevel

	// Initialize is called once at the start of replay parsing
	// It receives the replay and all players
	Initialize(replay *models.Replay, players []*models.Player)

	// ProcessCommand is called for each command during replay parsing
	// Returns true if the detector is finished and no longer needs commands
	ProcessCommand(command *models.Command) bool

	// Finalize is called after all commands were processed.
	// Detectors that require full-replay context can complete here.
	Finalize()

	// IsFinished returns true if the detector has finished and won't change
	IsFinished() bool

	// GetResult returns the pattern result if the detector is finished
	// Returns nil if the pattern was not detected or should not be saved
	GetResult() *PatternResult

	// ShouldSave returns true if the result should be saved to the database
	ShouldSave() bool
}

// WorldStateConsumer can receive orchestrator-owned runtime world state context.
type WorldStateConsumer interface {
	SetWorldState(worldState *worldstate.Engine)
}

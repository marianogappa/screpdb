package core

import (
	"github.com/marianogappa/screpdb/internal/models"
)

// AlgorithmVersion is the current version of the pattern detection algorithm
// Increment this when the algorithm changes to trigger re-detection
const AlgorithmVersion = 1

// DetectorLevel indicates at which level a pattern detector operates
type DetectorLevel string

const (
	LevelReplay DetectorLevel = "replay"
	LevelTeam   DetectorLevel = "team"
	LevelPlayer DetectorLevel = "player"
)

// PatternResult represents the result of a pattern detection
type PatternResult struct {
	PatternName    string
	Level          DetectorLevel
	ReplayID       int64
	Team           *byte  // nil for replay and player level
	PlayerID       *int64 // nil for replay and team level (database ID)
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

	// IsFinished returns true if the detector has finished and won't change
	IsFinished() bool

	// GetResult returns the pattern result if the detector is finished
	// Returns nil if the pattern was not detected or should not be saved
	GetResult() *PatternResult

	// ShouldSave returns true if the result should be saved to the database
	ShouldSave() bool
}


package core

import (
	"encoding/json"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// AlgorithmVersion is the current version of the pattern detection algorithm
// Increment this when the algorithm changes to trigger re-detection
const AlgorithmVersion = 11

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

	// DetectedAtSecond is the replay second at which the marker fired.
	// Stored in replay_events.seconds_from_game_start. Source depends on marker family:
	//   Rule markers       → second of the fact that flipped Decision→Matched
	//   First-event markers → second of the first qualifying narrative event
	//   Absence markers     → replay duration (marker commits at end-of-replay)
	//   Viewport/Hotkeys    → documented per-evaluator
	DetectedAtSecond int

	// Payload is the optional JSON blob persisted to replay_events.payload.
	// Empty for presence-only markers. Populated only by markers that carry extra data
	// beyond presence (currently: used_hotkey_groups, viewport_multitasking).
	Payload json.RawMessage
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

package core

import (
	"encoding/json"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// DetectorVersion identifies the output of the detection pipeline. Its one job
// is to keep the embedded progamer pack honest: scripts/pro-pack stamps the
// pack it builds with this value and internal/propack's test refuses a pack
// stamped older than the code, because the dashboard plots the pack's
// precomputed APM / cadence / viewport figures against the same figures
// computed locally, and two detector versions are not comparable.
//
// Bump it only when a change would alter what scripts/pro-pack computes, and
// regenerate the pack in the same change. Nothing re-detects or re-ingests on
// it: the corpus is read into memory and detected on every launch.
// docs/DETECTOR_VERSIONS.md logs the history.
const DetectorVersion = 69

type DetectorLevel string

const (
	LevelReplay DetectorLevel = "replay"
	LevelPlayer DetectorLevel = "player"
)

type PatternResult struct {
	PatternName    string
	Level          DetectorLevel
	ReplayID       int64
	PlayerID       *int64 // nil for replay-level patterns (database ID)
	ReplayPlayerID *byte  // Temporary: replay player ID (byte) for player-level results, converted to PlayerID later

	// DetectedAtSecond is the replay second at which the marker fired, stored in
	// replay_events.seconds_from_game_start. Source depends on the marker family:
	//   Rule markers        → second of the fact that flipped Decision→Matched
	//   First-event markers → second of the first qualifying narrative event
	//   Absence markers     → replay duration (commits at end-of-replay)
	//   Viewport/Hotkeys    → documented per-evaluator
	DetectedAtSecond int

	// Payload is the optional JSON blob persisted to replay_events.payload, empty
	// for presence-only markers (currently only viewport_multitasking sets it).
	Payload json.RawMessage
}

type Detector interface {
	Name() string
	Level() DetectorLevel

	Initialize(replay *models.Replay, players []*models.Player)

	// ProcessCommand returns true once the detector no longer needs commands.
	ProcessCommand(command *models.Command) bool

	// Finalize runs after all commands, for detectors needing full-replay context.
	Finalize()

	IsFinished() bool

	// GetResult returns nil if the pattern was not detected or should not be saved.
	GetResult() *PatternResult

	ShouldSave() bool
}

// WorldStateConsumer can receive orchestrator-owned runtime world state context.
type WorldStateConsumer interface {
	SetWorldState(worldState *worldstate.Engine)
}

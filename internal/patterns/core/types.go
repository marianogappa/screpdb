package core

import (
	"encoding/json"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// AlgorithmVersion is the current version of the pattern detection algorithm
// Increment this when the algorithm changes to trigger re-detection
//
// 26: build-order overhaul — Zerg 5/6/7/8/10/11 Pool rungs, loosened FFE &
// 1 Rax FE timings, widened Protoss expand/core matchups, Bunker Rush, per-race
// residual "… (Other)" catch-alls, and the "Opener unresolved" N/A marker.
//
// 27: Build dedup now requires the same build tile, not just a 3s window —
// stops merging distinct same-type buildings placed seconds apart at different
// spots (the time-only heuristic mis-merged ~55% of its collapses). The dead
// non-streaming ApplyBuildDedup mirror was also removed.
//
// 28: Selection-tag build dedup (internal/unittags + internal/builddedup),
// applied ahead of earlyfilter: provable worker one-at-a-time drops (Terran SCV
// / Zerg Drone redirected to a different-tile build before the prior could
// finish) and never-produced production buildings within the build-order
// window. Removes redundant Build commands so building counts reflect reality.
//
// 29: Terran build-order revamp (issue #155). The topology openers 1 Rax 1 Fac
// / 1 Rax FE / 2 Rax CC and the style markers Mech / 1-1-1 / SK Terran / Mech
// transition are replaced by composition-based initial BOs classified at 10:00:
// Wraith, Goliath, N-Rax Bio, 1-1-1 (+ into Mech), N-Fac Mech, N-Fac Tankless
// Mech — split by Barracks/Factory count and bio-vs-mech predominance. New DSL
// primitives (Predominant, time-bounded produce/build counts) and a non-1v1
// matchup gate back them. CC First / BBS / Bunker Rush are kept; the Terran
// residual is now "Terran (Other)" (bo_terran_other).
const AlgorithmVersion = 29

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

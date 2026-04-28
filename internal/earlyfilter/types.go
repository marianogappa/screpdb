package earlyfilter

import (
	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// Options configures Apply. Zero-value Options uses sensible defaults
// (5-minute window, no debug output).
type Options struct {
	// MaxSecond bounds the in-game second range the filter operates on.
	// Commands at or after MaxSecond pass through unchanged.
	// Zero means use the default (300).
	MaxSecond int

	// DebugDir, when non-empty, names a directory where Apply writes one
	// JSON trace file per replay (named "<replay-checksum>.json") with the
	// per-player tick state and every keep/drop/readmit decision and reason.
	// Empty disables tracing entirely.
	DebugDir string
}

// Result is the output of Apply.
type Result struct {
	// Commands is the filtered, time-ordered command list. The slice
	// header is new; element pointers are aliases of the input slice.
	Commands []*models.Command

	// Trace carries per-player simulation state and decision log.
	// Nil unless Options.DebugDir was set.
	Trace *Trace

	// Stats are coarse counts useful for a one-line per-replay log summary.
	Stats Stats
}

// Stats summarises the filter's decisions at the player level.
type Stats struct {
	PerPlayer map[int64]PlayerStats
}

// PlayerStats counts the decisions made for one player's command stream.
type PlayerStats struct {
	// Total commands inspected within the window (includes pass-through).
	Total int
	// Kept commands the filter decided were real (or unclassified pass-through).
	Kept int
	// Dropped commands the filter dropped via the forward resource simulation.
	Dropped int
	// Readmitted dropped Build commands that backtracking restored due to a
	// tech-tree consequent (Zealot ⇒ Gateway, etc.).
	Readmitted int
	// WorkerDropsForBacktrack worker trains the backtrack pass forcibly
	// dropped to free minerals for re-admitted prerequisites.
	WorkerDropsForBacktrack int
}

// defaultMaxSecond is applied when Options.MaxSecond is zero. 4 minutes
// covers the early-spam window without straying into Tier-2 territory
// where chaos theory makes per-command bookkeeping unreliable.
const defaultMaxSecond = 240

// kindFiltered reports whether a Kind is subject to filtering. Move /
// Attack / Hotkey commands pass through.
func kindFiltered(k cmdenrich.Kind) bool {
	return k == cmdenrich.KindMakeBuilding || k == cmdenrich.KindMakeUnit
}

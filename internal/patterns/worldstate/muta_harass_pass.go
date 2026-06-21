package worldstate

// Mutalisk hit-and-run harass (issue #194). Detection of the harass itself —
// reconstructing per-player selection / hotkey state and finding the oscillating
// dart-in/pull-back volley rhythm — lives in internal/unittags (it owns the raw
// command stream and selection-tag state). The orchestrator threads the detected
// episodes here as MutaHarassCandidates.
//
// Per-window TIMING is deliberately NOT surfaced: a well-microed muta attack is
// mechanically indistinguishable from hit-n-run in command geometry, so exact
// windows are too error-prone to render on a timeline. Instead the engine
// exposes a conservative per-player CONFIDENCE flag (HasMutaHitnRun) that the
// markers layer turns into presence-only pills. The per-game-player flag stays
// correct even when individual window timings are off: the strongest sustained
// campaign in a real muta game clears the bar by a wide margin, while the noise
// in a non-harass game stays well below it.

const (
	// mutaHitnRunMinVolleys / mutaHitnRunMinDurSec define the high-confidence
	// bar: a single sustained window with at least this many dart-in/pull-back
	// volleys over at least this many seconds. Tuned conservatively against
	// human-labeled replays — clears comfortably for real campaigns (40-110
	// volleys) and rejects microed-attack noise (≤~20).
	mutaHitnRunMinVolleys = 30
	mutaHitnRunMinDurSec  = 20
)

// MutaHarassCandidate is one detected harass window, threaded in from unittags
// via the orchestrator. Path is the ordered coordinate trail [sec, x, y] (px) —
// retained for diagnostics but not surfaced.
type MutaHarassCandidate struct {
	PID       byte
	StartSec  int
	EndSec    int
	Cycles    int
	GroupSize int
	Path      [][3]int
}

// SetMutaHarassCandidates supplies the selection-derived harass windows. Must
// be called before results are read (mirrors SetProductionSignals).
func (e *Engine) SetMutaHarassCandidates(c []MutaHarassCandidate) {
	e.mutaHarass = c
}

// HasMutaHitnRun reports whether the player ran a high-confidence Mutalisk
// hit-and-run campaign (a sustained oscillating-volley window clearing the
// conservative bar). Drives the presence-only markers; carries no timing.
func (e *Engine) HasMutaHitnRun(pid byte) bool {
	for _, c := range e.mutaHarass {
		if c.PID != pid {
			continue
		}
		if c.Cycles >= mutaHitnRunMinVolleys && c.EndSec-c.StartSec >= mutaHitnRunMinDurSec {
			return true
		}
	}
	return false
}

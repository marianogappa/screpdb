// Package profile provides lightweight phase timing for replay ingestion.
//
// A nil *Sink is the disabled case: every method on *Sink and *Run is
// nil-safe and returns near-instantly, so callers can sprinkle Phase()
// calls without an enabled-check on the hot path.
//
// Sourced from the SCREPDB_INGEST_PROFILE env var; not user-facing.
package profile

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type Mode int

const (
	ModeOff Mode = iota
	ModeSummary
	ModeVerbose
)

// ModeFromEnv parses SCREPDB_INGEST_PROFILE values: "", "0", "false" → off;
// "verbose" → verbose; anything else (e.g. "1", "summary", "true") → summary.
func ModeFromEnv(v string) Mode {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "off":
		return ModeOff
	case "verbose":
		return ModeVerbose
	default:
		return ModeSummary
	}
}

// phaseOrder is the canonical column order in the per-replay one-liner. Phases
// not in this list still get accumulated but won't appear in the headline.
var phaseOrder = []string{
	"parse",
	"replay_ins",
	"players",
	"cmds",
	"events",
	"patterns",
	"commit",
}

// Sink collects per-replay phase timings and reports an aggregate at end.
// nil-safe: a nil *Sink swallows all calls.
type Sink struct {
	mode Mode
	out  io.Writer
	mu   sync.Mutex
	runs []map[string]time.Duration
}

// NewSink returns nil if mode is ModeOff (so callers branch only here).
func NewSink(mode Mode) *Sink {
	if mode == ModeOff {
		return nil
	}
	return &Sink{mode: mode, out: os.Stderr}
}

// SetWriter overrides the output destination. Useful for benchmarks
// (io.Discard) or tests that want to capture output. nil-safe.
func (s *Sink) SetWriter(w io.Writer) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.out = w
	s.mu.Unlock()
}

// Run is the timer for a single replay's lifecycle.
type Run struct {
	sink    *Sink
	name    string
	mu      sync.Mutex
	timings map[string]time.Duration
}

// Replay starts a per-replay run. Returns nil when sink is disabled.
func (s *Sink) Replay(name string) *Run {
	if s == nil {
		return nil
	}
	return &Run{sink: s, name: name, timings: make(map[string]time.Duration, 8)}
}

// Phase returns a closer that records the elapsed time when called.
// Idiomatic use: `defer run.Phase("commit")()`. nil-safe.
func (r *Run) Phase(name string) func() {
	if r == nil {
		return func() {}
	}
	start := time.Now()
	return func() {
		r.mu.Lock()
		r.timings[name] += time.Since(start)
		r.mu.Unlock()
	}
}

// Add accumulates a precomputed duration into a phase. Useful when the timing
// boundary doesn't match a single defer scope. nil-safe.
func (r *Run) Add(name string, d time.Duration) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.timings[name] += d
	r.mu.Unlock()
}

// Done logs the per-replay one-liner and folds the run into the sink's
// aggregate. nil-safe.
func (r *Run) Done() {
	if r == nil {
		return
	}
	r.sink.mu.Lock()
	defer r.sink.mu.Unlock()

	var total time.Duration
	for _, d := range r.timings {
		total += d
	}

	r.sink.runs = append(r.sink.runs, r.timings)

	var b strings.Builder
	fmt.Fprintf(&b, "[ingest-profile] %s total=%s", trim(r.name), fmtDur(total))
	for _, p := range phaseOrder {
		if d, ok := r.timings[p]; ok {
			fmt.Fprintf(&b, " %s=%s", p, fmtDur(d))
		}
	}
	if r.sink.mode == ModeVerbose && total > 0 {
		var maxName string
		var maxD time.Duration
		for p, d := range r.timings {
			if d > maxD {
				maxD, maxName = d, p
			}
		}
		pct := float64(maxD) / float64(total) * 100
		fmt.Fprintf(&b, " heaviest=%s(%.0f%%)", maxName, pct)
		if pct > 40 {
			b.WriteString(" *bottleneck*")
		}
	}
	fmt.Fprintln(r.sink.out, b.String())
}

// PhaseTotals returns the sum of each phase's duration across all completed
// runs. Useful for benchmark ReportMetric. nil-safe — returns nil if disabled.
func (s *Sink) PhaseTotals() map[string]time.Duration {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	totals := map[string]time.Duration{}
	for _, run := range s.runs {
		for p, d := range run {
			totals[p] += d
		}
	}
	return totals
}

// Reset clears all collected runs. nil-safe.
func (s *Sink) Reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.runs = nil
	s.mu.Unlock()
}

// Aggregate prints p50 / p95 / mean per phase across all completed runs and
// resets the buffer. Call once after ingest finishes. nil-safe.
func (s *Sink) Aggregate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.runs) == 0 {
		return
	}

	phases := map[string][]time.Duration{}
	for _, run := range s.runs {
		for p, d := range run {
			phases[p] = append(phases[p], d)
		}
	}

	type row struct {
		phase           string
		p50, p95, total time.Duration
	}
	rows := make([]row, 0, len(phases))
	for p, ds := range phases {
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		var total time.Duration
		for _, d := range ds {
			total += d
		}
		rows = append(rows, row{
			phase: p,
			p50:   percentile(ds, 0.50),
			p95:   percentile(ds, 0.95),
			total: total,
		})
	}
	// Order: known phases first (in canonical order), then anything else
	// alphabetically. Stable sort lets us layer the two passes.
	sort.Slice(rows, func(i, j int) bool { return rows[i].phase < rows[j].phase })
	sort.SliceStable(rows, func(i, j int) bool {
		return phaseRank(rows[i].phase) < phaseRank(rows[j].phase)
	})

	fmt.Fprintf(s.out, "[ingest-profile] aggregate over %d replays:\n", len(s.runs))
	for _, r := range rows {
		fmt.Fprintf(s.out, "[ingest-profile]   %-12s p50=%-8s p95=%-8s total=%s\n",
			r.phase, fmtDur(r.p50), fmtDur(r.p95), fmtDur(r.total))
	}
}

func phaseRank(p string) int {
	for i, name := range phaseOrder {
		if name == p {
			return i
		}
	}
	return len(phaseOrder) + 1
}

func percentile(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * q)
	return sorted[idx]
}

func fmtDur(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
}

func trim(name string) string {
	// Strip directory prefix; keep filename for the one-liner.
	if i := strings.LastIndexAny(name, "/\\"); i >= 0 {
		return name[i+1:]
	}
	return name
}

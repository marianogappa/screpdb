package profile

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestModeFromEnv(t *testing.T) {
	tests := []struct {
		in   string
		want Mode
	}{
		{"", ModeOff},
		{"0", ModeOff},
		{"false", ModeOff},
		{"off", ModeOff},
		{"OFF", ModeOff},
		{"  off  ", ModeOff},
		{"verbose", ModeVerbose},
		{"VERBOSE", ModeVerbose},
		{" Verbose ", ModeVerbose},
		{"1", ModeSummary},
		{"summary", ModeSummary},
		{"true", ModeSummary},
		{"garbage", ModeSummary},
	}
	for _, tt := range tests {
		if got := ModeFromEnv(tt.in); got != tt.want {
			t.Errorf("ModeFromEnv(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestNewSinkOffIsNil(t *testing.T) {
	if s := NewSink(ModeOff); s != nil {
		t.Errorf("NewSink(ModeOff) = %v, want nil", s)
	}
	if s := NewSink(ModeSummary); s == nil {
		t.Error("NewSink(ModeSummary) = nil, want non-nil")
	}
	if s := NewSink(ModeVerbose); s == nil {
		t.Error("NewSink(ModeVerbose) = nil, want non-nil")
	}
}

func TestNilSinkIsSafe(t *testing.T) {
	var s *Sink
	s.SetWriter(&bytes.Buffer{})
	s.Reset()
	s.Aggregate()
	if got := s.PhaseTotals(); got != nil {
		t.Errorf("nil Sink PhaseTotals() = %v, want nil", got)
	}
	if r := s.Replay("x"); r != nil {
		t.Errorf("nil Sink Replay() = %v, want nil", r)
	}
}

func TestNilRunIsSafe(t *testing.T) {
	var r *Run
	closer := r.Phase("parse")
	closer()
	r.Add("parse", time.Second)
	r.Done()
}

func TestSinkPhaseTotalsAndReset(t *testing.T) {
	s := NewSink(ModeSummary)
	s.SetWriter(&bytes.Buffer{})

	r := s.Replay("rep1")
	r.Add("parse", 10*time.Millisecond)
	r.Add("commit", 5*time.Millisecond)
	r.Done()

	r2 := s.Replay("rep2")
	r2.Add("parse", 20*time.Millisecond)
	r2.Done()

	totals := s.PhaseTotals()
	if totals["parse"] != 30*time.Millisecond {
		t.Errorf("parse total = %v, want 30ms", totals["parse"])
	}
	if totals["commit"] != 5*time.Millisecond {
		t.Errorf("commit total = %v, want 5ms", totals["commit"])
	}

	s.Reset()
	if got := s.PhaseTotals(); len(got) != 0 {
		t.Errorf("after Reset PhaseTotals() = %v, want empty", got)
	}
}

func TestPhaseCloserRecordsElapsed(t *testing.T) {
	s := NewSink(ModeSummary)
	s.SetWriter(&bytes.Buffer{})
	r := s.Replay("rep")
	closer := r.Phase("parse")
	closer()
	r.Done()
	if s.PhaseTotals()["parse"] <= 0 {
		t.Error("Phase closer recorded non-positive duration")
	}
}

func TestDoneOneLinerSummary(t *testing.T) {
	var buf bytes.Buffer
	s := NewSink(ModeSummary)
	s.SetWriter(&buf)
	r := s.Replay("/some/dir/rep1.rep")
	r.Add("parse", 12*time.Millisecond)
	r.Add("commit", 3*time.Millisecond)
	r.Done()

	out := buf.String()
	if !strings.Contains(out, "rep1.rep") {
		t.Errorf("one-liner missing trimmed name: %q", out)
	}
	if strings.Contains(out, "/some/dir/") {
		t.Errorf("one-liner should strip directory: %q", out)
	}
	if !strings.Contains(out, "parse=12ms") {
		t.Errorf("one-liner missing parse phase: %q", out)
	}
	if !strings.Contains(out, "total=15ms") {
		t.Errorf("one-liner missing total: %q", out)
	}
	if strings.Contains(out, "heaviest=") {
		t.Errorf("summary mode should not emit heaviest: %q", out)
	}
}

func TestDoneVerboseEmitsBottleneck(t *testing.T) {
	var buf bytes.Buffer
	s := NewSink(ModeVerbose)
	s.SetWriter(&buf)
	r := s.Replay("rep")
	r.Add("parse", 90*time.Millisecond)
	r.Add("commit", 10*time.Millisecond)
	r.Done()

	out := buf.String()
	if !strings.Contains(out, "heaviest=parse") {
		t.Errorf("verbose one-liner missing heaviest: %q", out)
	}
	if !strings.Contains(out, "*bottleneck*") {
		t.Errorf("verbose one-liner missing bottleneck marker (parse is 90%%): %q", out)
	}
}

func TestAggregate(t *testing.T) {
	var buf bytes.Buffer
	s := NewSink(ModeSummary)
	s.SetWriter(&buf)
	for _, d := range []time.Duration{10, 20, 30} {
		r := s.Replay("rep")
		r.Add("parse", d*time.Millisecond)
		r.Done()
	}
	s.Aggregate()

	out := buf.String()
	if !strings.Contains(out, "aggregate over 3 replays") {
		t.Errorf("aggregate header wrong: %q", out)
	}
	if !strings.Contains(out, "parse") {
		t.Errorf("aggregate missing parse row: %q", out)
	}
}

func TestAggregateNoRunsIsQuiet(t *testing.T) {
	var buf bytes.Buffer
	s := NewSink(ModeSummary)
	s.SetWriter(&buf)
	s.Aggregate()
	if buf.Len() != 0 {
		t.Errorf("Aggregate with no runs wrote %q, want nothing", buf.String())
	}
}

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		sorted []time.Duration
		q      float64
		want   time.Duration
	}{
		{"empty", nil, 0.5, 0},
		{"single", []time.Duration{7}, 0.95, 7},
		{"p50", []time.Duration{1, 2, 3, 4, 5}, 0.50, 3},
		{"p95", []time.Duration{1, 2, 3, 4, 5}, 0.95, 4},
		{"p100", []time.Duration{1, 2, 3, 4, 5}, 1.0, 5},
		{"p0", []time.Duration{1, 2, 3, 4, 5}, 0.0, 1},
	}
	for _, tt := range tests {
		if got := percentile(tt.sorted, tt.q); got != tt.want {
			t.Errorf("percentile(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFmtDur(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{500 * time.Microsecond, "500µs"},
		{5 * time.Millisecond, "5ms"},
		{1500 * time.Millisecond, "1.50s"},
	}
	for _, tt := range tests {
		if got := fmtDur(tt.in); got != tt.want {
			t.Errorf("fmtDur(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPhaseRank(t *testing.T) {
	if phaseRank("parse") != 0 {
		t.Errorf("parse rank = %d, want 0", phaseRank("parse"))
	}
	if phaseRank("commit") <= phaseRank("parse") {
		t.Error("commit should rank after parse")
	}
	if phaseRank("unknown") != len(phaseOrder)+1 {
		t.Errorf("unknown rank = %d, want %d", phaseRank("unknown"), len(phaseOrder)+1)
	}
}

func TestTrim(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"rep.rep", "rep.rep"},
		{"/a/b/rep.rep", "rep.rep"},
		{`C:\a\b\rep.rep`, "rep.rep"},
	}
	for _, tt := range tests {
		if got := trim(tt.in); got != tt.want {
			t.Errorf("trim(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

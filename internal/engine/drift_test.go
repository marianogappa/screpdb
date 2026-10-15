package engine

import "testing"

func selectsEvery(from, to, step int32, missing func(frame int32) bool) []selectResult {
	var out []selectResult
	for f := from; f < to; f += step {
		out = append(out, selectResult{frame: f, missing: missing(f)})
	}
	return out
}

func TestDesyncOnset(t *testing.T) {
	never := func(int32) bool { return false }
	cases := map[string]struct {
		selects []selectResult
		want    int32
	}{
		"in sync":           {selectsEvery(0, 20000, 10, never), NoFrame},
		"stray misses only": {selectsEvery(0, 20000, 10, func(f int32) bool { return f%3000 == 0 }), NoFrame},
		"desync after stray miss": {
			selectsEvery(0, 20000, 10, func(f int32) bool { return f == 1000 || f >= 6000 && f%50 == 0 }),
			6000,
		},
		"short burst then sync": {selectsEvery(0, 20000, 10, func(f int32) bool { return f >= 5000 && f < 5050 }), NoFrame},
	}
	for name, c := range cases {
		if got := desyncOnset(c.selects); got != c.want {
			t.Errorf("%s: got %d, want %d", name, got, c.want)
		}
	}
}

func TestTrustedUntil(t *testing.T) {
	d := &Digest{LastFrame: 9000, Drift: Drift{DesyncFrame: NoFrame, ErrorFrame: NoFrame}}
	if got := d.TrustedUntil(); got != 9001 || d.Confidence() != ConfidenceHigh {
		t.Fatalf("clean: until %d confidence %d", got, d.Confidence())
	}
	d.Drift.DesyncFrame, d.Drift.ErrorFrame = 5000, 7000
	if got := d.TrustedUntil(); got != 5000 || d.Confidence() != ConfidenceLow {
		t.Fatalf("desynced: until %d confidence %d", got, d.Confidence())
	}
}

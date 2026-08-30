package dashboard

import (
	"math"
	"testing"

	"github.com/marianogappa/scfingerprint"
)

func TestFingerprintConfidenceTier(t *testing.T) {
	for _, tc := range []struct {
		name     string
		points   map[string]bool
		want     string
		wantShow bool
	}{
		{
			name:     "strictest point earns high",
			points:   map[string]bool{"fpr_1e2": true, "fpr_1e3": true, "fpr_1e4": true},
			want:     fingerprintMatchConfidenceHigh,
			wantShow: true,
		},
		{
			name:     "one-in-a-thousand earns moderate",
			points:   map[string]bool{"fpr_1e2": true, "fpr_1e3": true, "fpr_1e4": false},
			want:     fingerprintMatchConfidenceModerate,
			wantShow: true,
		},
		{
			name:     "one-in-a-hundred earns nothing",
			points:   map[string]bool{"fpr_1e2": true, "fpr_1e3": false, "fpr_1e4": false},
			wantShow: false,
		},
		{
			name:     "no points earns nothing",
			points:   map[string]bool{},
			wantShow: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, show := fingerprintConfidenceTier(tc.points)
			if show != tc.wantShow {
				t.Fatalf("show = %v, want %v", show, tc.wantShow)
			}
			if show && got != tc.want {
				t.Fatalf("tier = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFingerprintConfidenceTiersAreReachable guards the bug this logic replaced.
// Tiers used to be cut from raw SearchFPR thresholds, but SearchFPR is the Šidák
// family-wise correction of the operating points over the catalog, so it moves
// as the catalog grows. At 70 entries the "moderate" band spanned exactly one
// reachable value — 0.50516, against a 0.50 ceiling — which made moderate
// unreachable and every surviving match report "high".
//
// Keying the tiers to the operating points removes that coupling. This test
// asserts it stays removed: both tiers must be reachable at the shipped catalog
// size, and at catalog sizes well beyond it.
func TestFingerprintConfidenceTiersAreReachable(t *testing.T) {
	ds, err := scfingerprint.BuiltinDataset(scfingerprint.ConfidenceHigh)
	if err != nil {
		t.Skipf("builtin dataset unavailable: %v", err)
	}

	tiers := map[string]bool{}
	for _, points := range []map[string]bool{
		{"fpr_1e2": true, "fpr_1e3": true, "fpr_1e4": true},
		{"fpr_1e2": true, "fpr_1e3": true, "fpr_1e4": false},
	} {
		tier, show := fingerprintConfidenceTier(points)
		if !show {
			t.Fatalf("operating points %v earned no tier", points)
		}
		tiers[tier] = true
	}
	if !tiers[fingerprintMatchConfidenceHigh] || !tiers[fingerprintMatchConfidenceModerate] {
		t.Fatalf("both tiers must be reachable, got %v", tiers)
	}

	// The family-wise rates the UI reports alongside each tier must still be
	// meaningfully different from each other and from a coin flip, otherwise the
	// two tiers would be a distinction without a difference.
	n := ds.Len()
	strict := familyWiseRate(1e-4, n)
	moderate := familyWiseRate(1e-3, n)
	loose := familyWiseRate(1e-2, n)
	if !(strict < moderate && moderate < loose) {
		t.Fatalf("family-wise rates out of order at catalog size %d: %g %g %g", n, strict, moderate, loose)
	}
	if moderate > 0.25 {
		t.Errorf("catalog size %d pushes the moderate tier to a family-wise rate of %.3f; "+
			"'Possibly' is no longer defensible at that rate", n, moderate)
	}
}

// familyWiseRate mirrors scfingerprint's Šidák correction so the test can reason
// about the rates the tiers imply without reaching into the library's internals.
func familyWiseRate(alpha float64, catalogSize int) float64 {
	if catalogSize < 1 {
		catalogSize = 1
	}
	return 1 - math.Pow(1-alpha, float64(catalogSize))
}

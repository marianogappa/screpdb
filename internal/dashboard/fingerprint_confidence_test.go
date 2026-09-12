package dashboard

import (
	"math"
	"testing"

	"github.com/marianogappa/scfingerprint"
)

func TestFingerprintTier(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    scfingerprint.MatchResult
		want string
	}{
		{
			name: "confirmed: strict + identity bar + 3 evidence",
			m: scfingerprint.MatchResult{
				OperatingPoints:   map[string]bool{"fpr_1e2": true, "fpr_1e3": true, "fpr_1e4": true},
				ClearsIdentityBar: true,
				EvidenceN:         3,
			},
			want: fingerprintTierConfirmed,
		},
		{
			name: "high: strict + identity bar but only 2 evidence",
			m: scfingerprint.MatchResult{
				OperatingPoints:   map[string]bool{"fpr_1e2": true, "fpr_1e3": true, "fpr_1e4": true},
				ClearsIdentityBar: true,
				EvidenceN:         2,
			},
			want: fingerprintTierHigh,
		},
		{
			name: "high: moderate + identity bar",
			m: scfingerprint.MatchResult{
				OperatingPoints:   map[string]bool{"fpr_1e2": true, "fpr_1e3": true, "fpr_1e4": false},
				ClearsIdentityBar: true,
				EvidenceN:         5,
			},
			want: fingerprintTierHigh,
		},
		{
			name: "lead: strict but no identity bar",
			m: scfingerprint.MatchResult{
				OperatingPoints:   map[string]bool{"fpr_1e2": true, "fpr_1e3": true, "fpr_1e4": true},
				ClearsIdentityBar: false,
				EvidenceN:         5,
			},
			want: fingerprintTierLead,
		},
		{
			name: "lead: loose only",
			m: scfingerprint.MatchResult{
				OperatingPoints:   map[string]bool{"fpr_1e2": true, "fpr_1e3": false, "fpr_1e4": false},
				ClearsIdentityBar: false,
			},
			want: fingerprintTierLead,
		},
		{
			name: "none: no points",
			m:    scfingerprint.MatchResult{OperatingPoints: map[string]bool{}},
			want: "",
		},
		{
			name: "none: synthetic model",
			m: scfingerprint.MatchResult{
				OperatingPoints:   map[string]bool{"fpr_1e2": true, "fpr_1e3": true, "fpr_1e4": true},
				ClearsIdentityBar: true,
				EvidenceN:         5,
				ModelIsSynthetic:  true,
			},
			want: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := fingerprintTier(tc.m)
			if got != tc.want {
				t.Fatalf("fingerprintTier = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFingerprintTierPrecedence(t *testing.T) {
	tiers := []string{fingerprintTierConfirmed, fingerprintTierHigh, fingerprintTierLead}
	for i := 0; i < len(tiers)-1; i++ {
		if tierRank(tiers[i]) >= tierRank(tiers[i+1]) {
			t.Errorf("tier %q must outrank %q", tiers[i], tiers[i+1])
		}
	}
}

func tierRank(tier string) int {
	switch tier {
	case fingerprintTierConfirmed:
		return 0
	case fingerprintTierHigh:
		return 1
	case fingerprintTierLead:
		return 2
	}
	return 99
}

func TestFingerprintConfidenceTiersAreReachable(t *testing.T) {
	ds, err := scfingerprint.BuiltinDataset(scfingerprint.ConfidenceHigh)
	if err != nil {
		t.Skipf("builtin dataset unavailable: %v", err)
	}

	n := ds.Len()
	strict := familyWiseRate(1e-4, n)
	moderate := familyWiseRate(1e-3, n)
	loose := familyWiseRate(1e-2, n)
	if !(strict < moderate && moderate < loose) {
		t.Fatalf("family-wise rates out of order at catalog size %d: %g %g %g", n, strict, moderate, loose)
	}
	if moderate > 0.25 {
		t.Errorf("catalog size %d pushes the moderate tier to a family-wise rate of %.3f; "+
			"tier label no longer defensible at that rate", n, moderate)
	}
}

func familyWiseRate(alpha float64, catalogSize int) float64 {
	if catalogSize < 1 {
		catalogSize = 1
	}
	return 1 - math.Pow(1-alpha, float64(catalogSize))
}

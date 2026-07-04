package markers

import "testing"

// TestTierOneSubjectsAreOfInterest guards against the silent zero-match failure
// mode: a marker rule keyed on a subject missing from subjectsOfInterest never
// matches, because the detector drops that Build/Produce fact before it reaches
// the predicate state. Every subject the tier-1 openers discriminate on must be
// registered. (Discovered when the first Zerg/Protoss tech-pathway openers
// matched zero corpus replays — issue #182.)
func TestTierOneSubjectsAreOfInterest(t *testing.T) {
	required := []string{
		subjSpire, subjMutalisk, subjHydraliskDen, subjHydralisk, subjLurker,
		subjRoboticsFacility, subjReaver, subjCitadelOfAdun,
		subjTemplarArchives, subjDarkTemplar, subjStargate, subjCorsair,
		subjMachineShop, subjWraith,
	}
	for _, s := range required {
		if !IsSubjectOfInterest(s) {
			t.Errorf("subject %q is used by a tier-1 opener but not in subjectsOfInterest — its facts will be dropped and the opener will never match", s)
		}
	}
}

// TestEveryOpenerHasValidTier asserts the registry normalization leaves every
// KindInitialBuildOrder marker with a tier in {TierPreferred, TierBackup,
// TierResidual}. An unset tier on an opener is a definition bug (it would be
// silently treated as a tier and could win or lose selection unexpectedly).
func TestEveryOpenerHasValidTier(t *testing.T) {
	for _, m := range Markers() {
		if m.Kind != KindInitialBuildOrder {
			if m.Tier != 0 {
				t.Errorf("%q is a KindMarker but sets Tier=%d (tiers only apply to openers)", m.Name, m.Tier)
			}
			continue
		}
		switch m.Tier {
		case TierPreferred, TierBackup, TierResidual:
		default:
			t.Errorf("opener %q has invalid Tier=%d (want one of %d/%d/%d)",
				m.Name, m.Tier, TierPreferred, TierBackup, TierResidual)
		}
	}
}

// TestEveryRaceHasResidualTier asserts each race keeps exactly one residual
// (tier-3) opener — the floor that guarantees coverage when no preferred or
// backup opener matches.
func TestEveryRaceHasResidualTier(t *testing.T) {
	residuals := map[Race]int{}
	for _, m := range Markers() {
		if m.Kind == KindInitialBuildOrder && m.Tier == TierResidual {
			residuals[m.Race]++
		}
	}
	for _, race := range []Race{RaceZerg, RaceProtoss, RaceTerran} {
		if residuals[race] != 1 {
			t.Errorf("race %s has %d tier-residual openers, want exactly 1", race, residuals[race])
		}
	}
}

// TestBetaExemptKeysExist guards against the silent no-op failure mode: a
// beta-exempt key that doesn't match a real marker FeatureKey (a typo or a
// renamed marker) exempts nothing, so the marker keeps its beta tag and the
// bug is invisible. Every key in betaExemptFeatureKeys must name a live marker.
func TestBetaExemptKeysExist(t *testing.T) {
	valid := map[string]bool{}
	for _, m := range Markers() {
		valid[m.FeatureKey] = true
	}
	for key := range betaExemptFeatureKeys {
		if !valid[key] {
			t.Errorf("betaExemptFeatureKeys has %q, which is not a marker FeatureKey — it exempts nothing", key)
		}
	}
}

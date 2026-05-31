package spec

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/dashboard"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

// TestEconBuildTimesMatchCanonical is the headline cross-consistency check for
// issue #138: the early-game economy table's build times must equal the
// canonical models build times for every overlapping subject. This is what
// makes "stored in two places" impossible — costs.go references the models
// consts, and this test proves it.
func TestEconBuildTimesMatchCanonical(t *testing.T) {
	for _, e := range cmdenrich.AllEcon() {
		canonical, ok := models.BuildTimeOf(e.Subject)
		if !ok {
			t.Errorf("econ subject %q has no canonical models build time", e.Subject)
			continue
		}
		if e.Econ.BuildTimeS != canonical {
			t.Errorf("econ build time for %q = %v, canonical models value = %v", e.Subject, e.Econ.BuildTimeS, canonical)
		}
	}
}

// TestFeaturingOrderResolves asserts every key in the dashboard featuring strip
// resolves to either a registered marker (by FeatureKey) or a non-marker
// game-event feature. A typo or a removed marker leaves a dangling chip — this
// catches it.
func TestFeaturingOrderResolves(t *testing.T) {
	gameEventKeys := map[string]bool{}
	for _, f := range dashboard.AllGameEventFeatures() {
		gameEventKeys[f.Key] = true
	}

	for _, key := range dashboard.FeaturingOrder() {
		if markers.ByFeatureKey(key) != nil {
			continue
		}
		if gameEventKeys[key] {
			continue
		}
		t.Errorf("featuring key %q resolves to neither a marker nor a game-event feature", key)
	}
}

// TestGameEventFeaturesAreFeatured asserts every non-marker game-event feature
// is actually placed in the featuring order (no orphan chips defined but never
// shown).
func TestGameEventFeaturesAreFeatured(t *testing.T) {
	inOrder := map[string]bool{}
	for _, key := range dashboard.FeaturingOrder() {
		inOrder[key] = true
	}
	for _, f := range dashboard.AllGameEventFeatures() {
		if !inOrder[f.Key] {
			t.Errorf("game-event feature %q is defined but not in the featuring order", f.Key)
		}
	}
}

// TestOpenersHaveResolvableFeatureKeys asserts every initial-build-order marker
// carries a FeatureKey that round-trips through the marker registry.
func TestOpenersHaveResolvableFeatureKeys(t *testing.T) {
	for _, m := range openers() {
		if m.FeatureKey == "" {
			t.Errorf("opener %q has empty FeatureKey", m.Name)
			continue
		}
		if markers.ByFeatureKey(m.FeatureKey) == nil {
			t.Errorf("opener %q FeatureKey %q does not resolve in the marker registry", m.Name, m.FeatureKey)
		}
	}
}

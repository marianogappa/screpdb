package db

import (
	"strings"
)

// splitPerValueFeatureKey parses a per-value filter key into (featureKey, label).
// Returns ok=false for plain keys with no value suffix.
func splitPerValueFeatureKey(key string) (featureKey, label string, ok bool) {
	idx := strings.Index(key, perValueFeatureKeySep)
	if idx < 0 {
		return "", "", false
	}
	featureKey = key[:idx]
	label = key[idx+len(perValueFeatureKeySep):]
	if featureKey == "" || label == "" {
		return "", "", false
	}
	return featureKey, label, true
}

// uiFeatureKeyToMarkerFeatureKey bridges the frontend filter keys (short aliases
// like "nukes" / "recalls") to the canonical marker FeatureKeys. Callers pass
// either form; this map normalises to the registry's FeatureKey.
var uiFeatureKeyToMarkerFeatureKey = map[string]string{
	"nukes":   "threw_nukes",
	"recalls": "made_recalls",
}

// perValueFeatureKeySep separates a marker feature key from a resolved payload
// label value in a per-value filter key ("bo_z_fuzzy::~10 hatch").
const perValueFeatureKeySep = "::"

// leadingSupplyNumber extracts the integer from a "~N ..." fuzzy-opener label so
// filter pills sort by supply rung rather than lexically. Returns a large
// sentinel when no number is present, sinking such labels to the end.
func leadingSupplyNumber(label string) int {
	digits := strings.TrimLeft(label, "~")
	n := 0
	found := false
	for _, r := range digits {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		found = true
	}
	if !found {
		return 1 << 30
	}
	return n
}

// PerValueFeatureKey builds the filter key for one resolved value of a marker,
// e.g. a fuzzy build order's label.
func PerValueFeatureKey(featureKey, label string) string {
	return featureKey + perValueFeatureKeySep + strings.ToLower(label)
}

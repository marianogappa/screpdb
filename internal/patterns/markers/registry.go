package markers

import "strings"

// InitialBuildOrderPatternNamePrefix is the common prefix applied to every
// KindInitialBuildOrder marker's PatternName in the DB. Callers that want to
// filter "openers only" check this prefix via IsInitialBuildOrderPatternName.
// KindMarker entries use bare names and don't carry this prefix.
const InitialBuildOrderPatternNamePrefix = "Build Order: "

var (
	markerList      []Marker
	markersByPattern map[string]*Marker
	markersByFeature map[string]*Marker
)

func init() {
	markerList = allMarkers()
	markersByPattern = make(map[string]*Marker, len(markerList))
	markersByFeature = make(map[string]*Marker, len(markerList))
	for i := range markerList {
		m := &markerList[i]
		markersByPattern[strings.ToLower(m.PatternName)] = m
		markersByFeature[strings.ToLower(m.FeatureKey)] = m
	}
}

// Markers returns the full list of registered markers in display order. The
// returned slice is a shared read-only reference — do not mutate.
//
// Named "Markers" (not "All") to avoid collision with the DSL combinator
// `All(ps ...Predicate) Predicate` that lives in dsl.go.
func Markers() []Marker { return markerList }

// ByPatternName looks up a Marker by the stored pattern name. Case-
// insensitive. Returns nil if not found.
func ByPatternName(name string) *Marker {
	return markersByPattern[strings.ToLower(strings.TrimSpace(name))]
}

// ByFeatureKey looks up a Marker by its featuring filter key (e.g.
// "bo_9_pool", "carriers"). Case-insensitive. Returns nil if not found.
func ByFeatureKey(key string) *Marker {
	return markersByFeature[strings.ToLower(strings.TrimSpace(key))]
}

// IsInitialBuildOrderPatternName reports whether a stored pattern name
// belongs to the openers subset (KindInitialBuildOrder). Used by the Build
// Orders UI tab to filter its input. KindMarker entries return false.
func IsInitialBuildOrderPatternName(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), InitialBuildOrderPatternNamePrefix)
}

// AllFeatureKeys returns the canonical list of FeatureKeys across every registered
// marker. Caller may treat this as the authoritative allowlist for event_type
// strings written to replay_events under event_kind='marker'.
func AllFeatureKeys() []string {
	keys := make([]string, 0, len(markerList))
	for i := range markerList {
		keys = append(keys, markerList[i].FeatureKey)
	}
	return keys
}

// MatchupNon1v1 is a special token usable in a Marker's Matchup gate. It admits
// any replay whose team format is not 1v1 (team / FFA games), where the exact
// per-matchup string (e.g. "TvZ") is not meaningful. Build orders that apply to
// a 1v1 matchup AND to non-1v1 games list both, e.g. {"TvZ", MatchupNon1v1}.
const MatchupNon1v1 = "non-1v1"

// MatchupAdmits reports whether a Matchup gate admits a replay with the given
// matchup string and team format. Empty gate = any matchup. The MatchupNon1v1
// token matches any non-1v1 team format.
func MatchupAdmits(gate []string, matchup, teamFormat string) bool {
	if len(gate) == 0 {
		return true
	}
	for _, m := range gate {
		if m == matchup {
			return true
		}
		if m == MatchupNon1v1 && teamFormat != "1v1" {
			return true
		}
	}
	return false
}

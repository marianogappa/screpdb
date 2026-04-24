package markers

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// Initial build orders are mutually exclusive: at most one KindInitialBuildOrder BO may
// match per player per replay. These tests enforce that invariant.
//
// When a new overlap is discovered, do NOT suppress the failing test. Instead,
// tighten the broad rules in orders.go so the conflicting BOs no longer both
// match the offending stream.

type subjectKind struct {
	subject string
	kind    cmdenrich.Kind
}

var zergFuzzSubjects = []subjectKind{
	{subjSpawningPool, cmdenrich.KindMakeBuilding},
	{subjHatchery, cmdenrich.KindMakeBuilding},
	{subjEvolutionChamber, cmdenrich.KindMakeBuilding},
	{subjDrone, cmdenrich.KindMakeUnit},
	{subjOverlord, cmdenrich.KindMakeUnit},
	{subjZergling, cmdenrich.KindMakeUnit},
}

var protossFuzzSubjects = []subjectKind{
	{subjNexus, cmdenrich.KindMakeBuilding},
	{subjGateway, cmdenrich.KindMakeBuilding},
	{subjForge, cmdenrich.KindMakeBuilding},
	{subjZealot, cmdenrich.KindMakeUnit},
}

// collectInitialMatches returns the names of every KindInitialBuildOrder BO that matches
// the given facts for the supplied race.
func collectInitialMatches(race Race, facts []cmdenrich.EnrichedCommand) []string {
	var names []string
	for _, bo := range Markers() {
		if bo.Kind != KindInitialBuildOrder || bo.Race != race {
			continue
		}
		if bo.Matches(facts) {
			names = append(names, bo.Name)
		}
	}
	sort.Strings(names)
	return names
}

func formatFactsForError(facts []cmdenrich.EnrichedCommand) string {
	var b strings.Builder
	for i, f := range facts {
		if i > 0 {
			b.WriteString(", ")
		}
		kind := "B"
		if f.Kind == cmdenrich.KindMakeUnit {
			kind = "P"
		}
		fmt.Fprintf(&b, "%s(%s@%ds)", kind, f.Subject, f.Second)
	}
	return b.String()
}

// FuzzInitialBOsMutualExclusion randomly walks fact streams and asserts no
// two KindInitialBuildOrder BOs match the same stream for the same race.
//
// When the fuzzer finds a failing input, do NOT add it to a skip list —
// tighten the broad rules in orders.go so the matching BOs become mutually
// exclusive for that stream.
func FuzzInitialBOsMutualExclusion(f *testing.F) {
	// Seed corpus: a mix of typical pool-first / hatch-first / protoss
	// timings. bytes encode: first byte = race (even=Zerg, odd=Protoss);
	// then 3-byte triples (subjectIdx, second_lo, second_hi) for facts.
	f.Add([]byte{0, 0, 10, 0, 1, 85, 0, 0, 110, 0})
	f.Add([]byte{0, 3, 5, 0, 3, 15, 0, 3, 27, 0, 0, 73, 0, 5, 123, 0})
	f.Add([]byte{1, 0, 106, 0, 1, 126, 0})
	f.Add([]byte{1, 1, 70, 0, 1, 86, 0, 3, 130, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 1 {
			return
		}
		var subs []subjectKind
		var race Race
		if data[0]%2 == 0 {
			subs = zergFuzzSubjects
			race = RaceZerg
		} else {
			subs = protossFuzzSubjects
			race = RaceProtoss
		}
		facts := make([]cmdenrich.EnrichedCommand, 0, (len(data)-1)/3)
		for i := 1; i+2 < len(data); i += 3 {
			sub := subs[int(data[i])%len(subs)]
			second := int(data[i+1]) + int(data[i+2])*256
			second %= 400 // cap at ~6:40 to stay inside opener windows
			facts = append(facts, cmdenrich.EnrichedCommand{Kind: sub.kind, Subject: sub.subject, Second: second})
		}
		// Sort facts ascending by Second so the streaming monotonicity
		// invariants hold.
		sort.SliceStable(facts, func(i, j int) bool { return facts[i].Second < facts[j].Second })
		matches := collectInitialMatches(race, facts)
		if len(matches) > 1 {
			t.Fatalf("multiple %s initial BOs matched: %v — tighten broad rules so they're mutually exclusive. Facts: %s",
				race, matches, formatFactsForError(facts))
		}
	})
}

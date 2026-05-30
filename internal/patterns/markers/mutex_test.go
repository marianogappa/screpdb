package markers

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// Initial build orders are mutually exclusive PER (race, matchup) TUPLE: at most
// one KindInitialBuildOrder BO may match per player per replay, given the
// matchup gate. These tests enforce that invariant.
//
// When a new overlap is discovered, do NOT suppress the failing test. Instead,
// tighten the broad rules in definitions.go so the conflicting BOs no longer
// both match the offending stream.

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
	{subjPylon, cmdenrich.KindMakeBuilding},
	{subjGateway, cmdenrich.KindMakeBuilding},
	{subjAssimilator, cmdenrich.KindMakeBuilding},
	{subjCyberneticsCore, cmdenrich.KindMakeBuilding},
	{subjForge, cmdenrich.KindMakeBuilding},
	{subjPhotonCannon, cmdenrich.KindMakeBuilding},
	{subjZealot, cmdenrich.KindMakeUnit},
}

var terranFuzzSubjects = []subjectKind{
	{subjCommandCenter, cmdenrich.KindMakeBuilding},
	{subjSupplyDepot, cmdenrich.KindMakeBuilding},
	{subjBarracks, cmdenrich.KindMakeBuilding},
	{subjRefinery, cmdenrich.KindMakeBuilding},
	{subjFactory, cmdenrich.KindMakeBuilding},
	{subjStarport, cmdenrich.KindMakeBuilding},
	{subjAcademy, cmdenrich.KindMakeBuilding},
	{subjEngineeringBay, cmdenrich.KindMakeBuilding},
	{subjBunker, cmdenrich.KindMakeBuilding},
}

// matchupsForRace returns the canonical 1v1 matchups in which a player of the
// given race can appear (matchup strings are alphabetised on the replay side,
// e.g. PvT covers both Protoss-vs-Terran and Terran-vs-Protoss).
func matchupsForRace(race Race) []string {
	switch race {
	case RaceZerg:
		return []string{"ZvZ", "PvZ", "TvZ"}
	case RaceProtoss:
		return []string{"PvP", "PvT", "PvZ"}
	case RaceTerran:
		return []string{"TvT", "PvT", "TvZ"}
	}
	return nil
}

// matchupApplies reports whether a marker's Matchup gate admits the given
// matchup. Empty Matchup = any.
func matchupApplies(bo Marker, matchup string) bool {
	if len(bo.Matchup) == 0 {
		return true
	}
	for _, m := range bo.Matchup {
		if m == matchup {
			return true
		}
	}
	return false
}

// collectInitialMatches returns the names of every KindInitialBuildOrder BO that matches
// the given facts for the supplied (race, matchup) tuple. BOs whose Matchup
// gate excludes the tuple are skipped — mutex is enforced per-tuple.
func collectInitialMatches(race Race, matchup string, facts []cmdenrich.EnrichedCommand) []string {
	var names []string
	for _, bo := range Markers() {
		if bo.Kind != KindInitialBuildOrder || bo.Race != race {
			continue
		}
		if !matchupApplies(bo, matchup) {
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
// two KindInitialBuildOrder BOs match the same stream for the same
// (race, matchup) tuple. The fuzz exercises every (race, matchup) tuple
// at every input — a single fact stream is mutex-checked across all 9
// possible matchups so a single corpus seed exercises all of them.
//
// When the fuzzer finds a failing input, do NOT add it to a skip list —
// tighten the broad rules in definitions.go so the matching BOs become
// mutually exclusive for that (race, matchup) tuple.
//
// Byte schema: first byte selects race (mod 3 → Zerg/Protoss/Terran);
// remaining bytes form 3-byte triples (subjectIdx, second_lo, second_hi).
func FuzzInitialBOsMutualExclusion(f *testing.F) {
	// Seed corpus: a mix of typical pool-first / hatch-first / protoss /
	// terran timings.
	f.Add([]byte{0, 0, 10, 0, 1, 85, 0, 0, 110, 0})                    // Zerg early Pool
	f.Add([]byte{0, 3, 5, 0, 3, 15, 0, 3, 27, 0, 0, 73, 0, 5, 123, 0}) // Zerg 9 Pool
	f.Add([]byte{1, 0, 48, 0, 2, 86, 0, 3, 116, 0, 4, 138, 0})         // Protoss 1 Gate Core
	f.Add([]byte{1, 0, 48, 0, 0, 130, 0, 2, 175, 0})                   // Protoss Nexus First
	f.Add([]byte{1, 1, 48, 0, 5, 86, 0, 6, 130, 0, 0, 152, 0})         // Protoss FFE
	f.Add([]byte{2, 1, 60, 0, 2, 88, 0, 3, 115, 0, 4, 165, 0})         // Terran 1 Rax 1 Fac
	f.Add([]byte{2, 1, 62, 0, 0, 145, 0, 2, 165, 0})                   // Terran CC First
	f.Add([]byte{2, 2, 60, 0, 2, 80, 0, 1, 100, 0})                    // Terran BBS
	// New openers (subject indices: zerg pool=0 hatch=1 drone=3 overlord=4;
	// protoss nexus=0 gateway=2; terran depot=1 rax=2 bunker=8).
	f.Add([]byte{0, 3, 30, 0, 3, 33, 0, 3, 36, 0, 0, 60, 0})                                         // Zerg 7 Pool (3 drones)
	f.Add([]byte{0, 4, 20, 0, 3, 30, 0, 3, 33, 0, 3, 36, 0, 3, 39, 0, 3, 42, 0, 3, 45, 0, 0, 92, 0}) // Zerg 10 Pool (overlord + 6 drones)
	f.Add([]byte{1, 2, 80, 0, 0, 160, 0})                                                            // Protoss Gate Expand (gate→nexus)
	f.Add([]byte{2, 2, 55, 0, 1, 90, 0, 8, 130, 0})                                                  // Terran Bunker Rush (rax→depot→bunker)
	// Regression: rax→bunker→rax→CC@276 once both-matched Bunker Rush + 2 Rax CC
	// (real 6-player Money game). CC second = 20 + 1*256 = 276.
	f.Add([]byte{2, 1, 55, 0, 2, 80, 0, 8, 130, 0, 2, 180, 0, 0, 20, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 1 {
			return
		}
		var subs []subjectKind
		var race Race
		switch data[0] % 3 {
		case 0:
			subs = zergFuzzSubjects
			race = RaceZerg
		case 1:
			subs = protossFuzzSubjects
			race = RaceProtoss
		default:
			subs = terranFuzzSubjects
			race = RaceTerran
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
		for _, matchup := range matchupsForRace(race) {
			matches := collectInitialMatches(race, matchup, facts)
			if len(matches) > 1 {
				t.Fatalf("multiple %s initial BOs matched in %s: %v — tighten broad rules so they're mutually exclusive. Facts: %s",
					race, matchup, matches, formatFactsForError(facts))
			}
		}
	})
}

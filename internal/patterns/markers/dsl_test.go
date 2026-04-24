package markers

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// Handy fixture helpers for terse tests.
func factsBuilder() *factsB { return &factsB{} }

type factsB struct{ facts []cmdenrich.EnrichedCommand }

func (b *factsB) add(kind cmdenrich.Kind, subject string, second int) *factsB {
	b.facts = append(b.facts, cmdenrich.EnrichedCommand{Kind: kind, Subject: subject, Second: second})
	return b
}

// B is shorthand for a Build fact.
func (b *factsB) B(subject string, second int) *factsB {
	return b.add(cmdenrich.KindMakeBuilding, subject, second)
}

// P is shorthand for a Produce fact.
func (b *factsB) P(unit string, second int) *factsB {
	return b.add(cmdenrich.KindMakeUnit, unit, second)
}

func (b *factsB) list() []cmdenrich.EnrichedCommand { return b.facts }

func TestFirstBuildBefore(t *testing.T) {
	s := factsBuilder().B("Spawning Pool", 73).list()
	if !FirstBuildBefore("Spawning Pool", 120).Eval(s) {
		t.Fatalf("pool@73 should satisfy before 120")
	}
	if FirstBuildBefore("Spawning Pool", 60).Eval(s) {
		t.Fatalf("pool@73 should not satisfy before 60")
	}
	if FirstBuildBefore("Nexus", 300).Eval(s) {
		t.Fatalf("unbuilt subject must fail")
	}
}

func TestBuildBefore_RequiresA_PermitsMissingB(t *testing.T) {
	// A built, B never built: predicate passes.
	s := factsBuilder().B("Hatchery", 98).list()
	if !BuildBefore("Hatchery", "Spawning Pool").Eval(s) {
		t.Fatalf("Hatchery built, Pool missing: should pass")
	}
	// A built, B built later: passes.
	s = factsBuilder().B("Hatchery", 98).B("Spawning Pool", 116).list()
	if !BuildBefore("Hatchery", "Spawning Pool").Eval(s) {
		t.Fatalf("Hatchery@98, Pool@116: should pass")
	}
	// A built after B: fails.
	s = factsBuilder().B("Spawning Pool", 70).B("Hatchery", 118).list()
	if BuildBefore("Hatchery", "Spawning Pool").Eval(s) {
		t.Fatalf("Hatchery@118 after Pool@70: should fail")
	}
	// A never built: fails.
	s = factsBuilder().B("Spawning Pool", 70).list()
	if BuildBefore("Hatchery", "Spawning Pool").Eval(s) {
		t.Fatalf("A unbuilt: should fail")
	}
}

func TestBuildAfterWithin(t *testing.T) {
	// Pool@73, Hatch@118 -> gap 45, within 60.
	s := factsBuilder().B("Spawning Pool", 73).B("Hatchery", 118).list()
	if !BuildAfterWithin("Hatchery", "Spawning Pool", 60).Eval(s) {
		t.Fatalf("gap 45 should satisfy <=60")
	}
	// Pool@73, Hatch@150 -> gap 77, fails 60.
	s = factsBuilder().B("Spawning Pool", 73).B("Hatchery", 150).list()
	if BuildAfterWithin("Hatchery", "Spawning Pool", 60).Eval(s) {
		t.Fatalf("gap 77 should fail <=60")
	}
	// Hatch before Pool -> fails (gap <= 0).
	s = factsBuilder().B("Hatchery", 80).B("Spawning Pool", 120).list()
	if BuildAfterWithin("Hatchery", "Spawning Pool", 60).Eval(s) {
		t.Fatalf("hatch before pool should fail")
	}
}

func TestNoProduceBeforeBuild_AnchorMustExist(t *testing.T) {
	// Pool built, drone before: "no drones before pool" is false.
	s := factsBuilder().P("Drone", 10).B("Spawning Pool", 33).list()
	if NoProduceBeforeBuild("Drone", "Spawning Pool").Eval(s) {
		t.Fatalf("drone@10 before pool: predicate should be false")
	}
	// Pool built, drone AFTER pool: "no drones before pool" is true.
	s = factsBuilder().B("Spawning Pool", 33).P("Drone", 40).list()
	if !NoProduceBeforeBuild("Drone", "Spawning Pool").Eval(s) {
		t.Fatalf("drone@40 after pool: predicate should be true")
	}
	// Anchor (Pool) never built: predicate is false (needs anchor).
	s = factsBuilder().P("Drone", 10).list()
	if NoProduceBeforeBuild("Drone", "Spawning Pool").Eval(s) {
		t.Fatalf("pool unbuilt: predicate must be false")
	}
}

func TestProduceBeforeBuild(t *testing.T) {
	// Drone at 20, Pool at 73 -> drones before pool: true.
	s := factsBuilder().P("Drone", 20).B("Spawning Pool", 73).list()
	if !ProduceBeforeBuild("Drone", "Spawning Pool").Eval(s) {
		t.Fatalf("expected drones-before-pool to hold")
	}
	// No drones before pool.
	s = factsBuilder().B("Spawning Pool", 33).list()
	if ProduceBeforeBuild("Drone", "Spawning Pool").Eval(s) {
		t.Fatalf("no drones before pool: should be false")
	}
}

func TestCountBuildsBefore(t *testing.T) {
	s := factsBuilder().B("Gateway", 70).B("Gateway", 86).B("Gateway", 210).list()
	if !CountBuildsBefore("Gateway", 2, 180).Eval(s) {
		t.Fatalf("2 gates before 180 should pass")
	}
	if CountBuildsBefore("Gateway", 3, 180).Eval(s) {
		t.Fatalf("3 gates before 180 should fail (3rd is at 210)")
	}
}

func TestNthBuildBeforeAll(t *testing.T) {
	// 2nd Gate at 86 precedes Nexus (200) and Forge never built -> pass.
	s := factsBuilder().B("Gateway", 70).B("Gateway", 86).B("Nexus", 200).list()
	if !NthBuildBeforeAll("Gateway", 2, []string{"Nexus", "Forge"}).Eval(s) {
		t.Fatalf("2nd gate before Nexus, Forge absent: should pass")
	}
	// 2nd Gate at 210 after Nexus 200 -> fail.
	s = factsBuilder().B("Gateway", 70).B("Nexus", 200).B("Gateway", 210).list()
	if NthBuildBeforeAll("Gateway", 2, []string{"Nexus", "Forge"}).Eval(s) {
		t.Fatalf("2nd gate after Nexus: should fail")
	}
}

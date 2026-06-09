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

// U is shorthand for an Upgrade fact (Subject is the upgrade name).
func (b *factsB) U(name string, second int) *factsB {
	return b.add(cmdenrich.KindUpgrade, name, second)
}

// T is shorthand for a Tech fact.
func (b *factsB) T(name string, second int) *factsB {
	return b.add(cmdenrich.KindTech, name, second)
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

func TestBuildCountEqualsBefore(t *testing.T) {
	three := factsBuilder().B("Factory", 150).B("Factory", 200).B("Factory", 260).list()
	if !BuildCountEqualsBefore("Factory", 3, 600).Eval(three) {
		t.Fatalf("3 factories before 600 should match ==3")
	}
	if BuildCountEqualsBefore("Factory", 2, 600).Eval(three) {
		t.Fatalf("3 factories should not match ==2")
	}
	four := factsBuilder().B("Factory", 150).B("Factory", 200).B("Factory", 260).B("Factory", 300).list()
	if BuildCountEqualsBefore("Factory", 3, 600).Eval(four) {
		t.Fatalf("4 factories should not match ==3")
	}
	// A 4th factory after the window does not count.
	lateFourth := factsBuilder().B("Factory", 150).B("Factory", 200).B("Factory", 260).B("Factory", 650).list()
	if !BuildCountEqualsBefore("Factory", 3, 600).Eval(lateFourth) {
		t.Fatalf("4th factory after window should still match ==3 by 600")
	}
}

func TestProduceCountAtLeastBefore(t *testing.T) {
	s := factsBuilder().P("Vulture", 200).P("Vulture", 250).P("Vulture", 300).P("Vulture", 350).P("Vulture", 400).list()
	if !ProduceCountAtLeastBefore("Vulture", 5, 600).Eval(s) {
		t.Fatalf("5 vultures before 600 should match >=5")
	}
	if ProduceCountAtLeastBefore("Vulture", 6, 600).Eval(s) {
		t.Fatalf("5 vultures should not match >=6")
	}
	// Vultures after the window don't count toward the threshold.
	late := factsBuilder().P("Vulture", 200).P("Vulture", 250).P("Vulture", 700).list()
	if ProduceCountAtLeastBefore("Vulture", 3, 600).Eval(late) {
		t.Fatalf("only 2 vultures before 600 should not match >=3")
	}
}

func TestProduceCountAtMostBefore(t *testing.T) {
	two := factsBuilder().P("Vulture", 200).P("Vulture", 250).list()
	if !ProduceCountAtMostBefore("Vulture", 2, 420).Eval(two) {
		t.Fatalf("2 vultures should match <=2")
	}
	three := factsBuilder().P("Vulture", 200).P("Vulture", 250).P("Vulture", 300).list()
	if ProduceCountAtMostBefore("Vulture", 2, 420).Eval(three) {
		t.Fatalf("3 vultures should not match <=2")
	}
	// Zero of the unit also satisfies an upper bound.
	if !ProduceCountAtMostBefore("Vulture", 2, 420).Eval(factsBuilder().B("Barracks", 80).list()) {
		t.Fatalf("0 vultures should match <=2")
	}
}

func TestPredominant(t *testing.T) {
	bio := []string{"Marine", "Medic", "Firebat"}
	mech := []string{"Vulture", "Goliath", "Siege Tank (Tank Mode)"}
	bioHeavy := factsBuilder().P("Marine", 200).P("Marine", 220).P("Medic", 240).P("Vulture", 260).list()
	if !Predominant(bio, mech, 600).Eval(bioHeavy) {
		t.Fatalf("3 bio vs 1 mech should be bio-predominant")
	}
	if Predominant(mech, bio, 600).Eval(bioHeavy) {
		t.Fatalf("mech should not be predominant here")
	}
	// Tie is not predominant (strict >).
	tie := factsBuilder().P("Marine", 200).P("Vulture", 260).list()
	if Predominant(bio, mech, 600).Eval(tie) {
		t.Fatalf("1v1 tie should not be predominant")
	}
}

func TestHPUpgradeExists_OnlyTieredCount(t *testing.T) {
	// A tiered weapon/armor/shield upgrade satisfies HPUpgradeExists.
	tiered := factsBuilder().U("Protoss Ground Weapons", 300).list()
	if !HPUpgradeExists().Eval(tiered) {
		t.Fatalf("tiered upgrade should satisfy HPUpgradeExists")
	}
	if NonHPUpgradeExists().Eval(tiered) {
		t.Fatalf("tiered upgrade must not satisfy NonHPUpgradeExists")
	}
	// A one-shot upgrade is a research, not an HP upgrade.
	oneShot := factsBuilder().U("Singularity Charge (Dragoon Range)", 300).list()
	if HPUpgradeExists().Eval(oneShot) {
		t.Fatalf("one-shot upgrade must not satisfy HPUpgradeExists")
	}
	if !NonHPUpgradeExists().Eval(oneShot) {
		t.Fatalf("one-shot upgrade should satisfy NonHPUpgradeExists")
	}
}

// Never-Researched fires only when there is neither a Tech nor a non-HP
// upgrade — HP-only games must still be flagged as "never researched".
func TestNeverResearched_CountsNonHPUpgrades(t *testing.T) {
	neverResearched := Not(Any(TechExists(), NonHPUpgradeExists()))

	// Only an HP upgrade: still "never researched".
	hpOnly := factsBuilder().U("Protoss Ground Weapons", 300).list()
	if !neverResearched.Eval(hpOnly) {
		t.Fatalf("HP-upgrade-only game should count as never researched")
	}
	// A range upgrade (non-HP) counts as a research.
	withRange := factsBuilder().U("Singularity Charge (Dragoon Range)", 300).list()
	if neverResearched.Eval(withRange) {
		t.Fatalf("non-HP upgrade should clear never researched")
	}
	// A tech also counts as a research.
	withTech := factsBuilder().T("Psionic Storm", 300).list()
	if neverResearched.Eval(withTech) {
		t.Fatalf("tech should clear never researched")
	}
}

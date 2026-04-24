package markers

import "testing"

// Each test builds a minimal stream exercising one BO's broad definition,
// then asserts both positive (should match) and a close-by negative case.

func findBO(t *testing.T, name string) Marker {
	t.Helper()
	for _, bo := range Markers() {
		if bo.Name == name {
			return bo
		}
	}
	t.Fatalf("BO %q not registered", name)
	return Marker{}
}

func TestBO_4Pool(t *testing.T) {
	bo := findBO(t, "4 Pool")
	// Positive: only pool, built at 33s, before any drone/overlord.
	pos := factsBuilder().B(subjSpawningPool, 33).P(subjZergling, 85).list()
	if !bo.Matches(pos) {
		t.Fatalf("4 pool positive should match")
	}
	// Negative: drone before pool disqualifies 4 pool.
	neg := factsBuilder().P(subjDrone, 10).B(subjSpawningPool, 33).list()
	if bo.Matches(neg) {
		t.Fatalf("4 pool should fail if drone produced before pool")
	}
	// Negative: pool built too late (>= 60s).
	late := factsBuilder().B(subjSpawningPool, 65).list()
	if bo.Matches(late) {
		t.Fatalf("4 pool should fail if pool built at/after 60s")
	}
}

func TestBO_9Pool(t *testing.T) {
	bo := findBO(t, "9 Pool")
	// Positive: 9 drones then pool at 73.
	b := factsBuilder()
	for i := 0; i < 9; i++ {
		b.P(subjDrone, 5+i*3) // last drone around 29s, well before pool@73
	}
	pos := b.B(subjSpawningPool, 73).list()
	if !bo.Matches(pos) {
		t.Fatalf("9 pool should match drones-then-pool@73")
	}
	// Negative: an overlord was produced before pool.
	neg := factsBuilder().P(subjDrone, 10).P(subjOverlord, 30).B(subjSpawningPool, 73).list()
	if bo.Matches(neg) {
		t.Fatalf("9 pool should fail with overlord before pool")
	}
	// Negative: no drones produced at all before pool.
	neg2 := factsBuilder().B(subjSpawningPool, 73).list()
	if bo.Matches(neg2) {
		t.Fatalf("9 pool should fail without drones before pool")
	}
	// Negative: Hatch built before Pool — that's a hatch-first BO, not 9 Pool.
	b2 := factsBuilder()
	for i := 0; i < 9; i++ {
		b2.P(subjDrone, 5+i*3)
	}
	neg3 := b2.B(subjHatchery, 70).B(subjSpawningPool, 95).list()
	if bo.Matches(neg3) {
		t.Fatalf("9 pool should fail when Hatchery precedes Pool")
	}
	// Negative: Evolution Chamber before Pool.
	b3 := factsBuilder()
	for i := 0; i < 9; i++ {
		b3.P(subjDrone, 5+i*3)
	}
	neg4 := b3.B(subjEvolutionChamber, 60).B(subjSpawningPool, 95).list()
	if bo.Matches(neg4) {
		t.Fatalf("9 pool should fail when Evolution Chamber precedes Pool")
	}
}

func TestBO_9PoolIntoHatchery(t *testing.T) {
	bo := findBO(t, "9 Pool into Hatchery")
	// Positive: drones, pool, then hatch within 60s of pool.
	b := factsBuilder()
	for i := 0; i < 9; i++ {
		b.P(subjDrone, 5+i*3)
	}
	pos := b.B(subjSpawningPool, 73).B(subjHatchery, 118).list()
	if !bo.Matches(pos) {
		t.Fatalf("9 pool → hatch should match hatch@118 after pool@73")
	}
	// Negative: hatch built too late after pool.
	b = factsBuilder()
	for i := 0; i < 9; i++ {
		b.P(subjDrone, 5+i*3)
	}
	neg := b.B(subjSpawningPool, 73).B(subjHatchery, 200).list()
	if bo.Matches(neg) {
		t.Fatalf("9 pool → hatch should fail hatch@200 after pool@73 (gap > 60)")
	}
}

func TestBO_12Hatch(t *testing.T) {
	bo := findBO(t, "12 Hatch")
	// Positive: hatch at 98, pool at 116.
	pos := factsBuilder().B(subjHatchery, 98).B(subjSpawningPool, 116).list()
	if !bo.Matches(pos) {
		t.Fatalf("12 hatch should match hatch-before-pool")
	}
	// Negative: pool first.
	neg := factsBuilder().B(subjSpawningPool, 73).B(subjHatchery, 118).list()
	if bo.Matches(neg) {
		t.Fatalf("12 hatch should fail when pool precedes hatch")
	}
	// Negative: hatch too late.
	neg2 := factsBuilder().B(subjHatchery, 160).list()
	if bo.Matches(neg2) {
		t.Fatalf("12 hatch should fail when hatch after 2m30s")
	}
}

func TestBO_NexusFirst(t *testing.T) {
	bo := findBO(t, "Nexus First")
	pos := factsBuilder().B(subjNexus, 106).B(subjGateway, 126).list()
	if !bo.Matches(pos) {
		t.Fatalf("Nexus first should match")
	}
	neg := factsBuilder().B(subjGateway, 80).B(subjNexus, 130).list()
	if bo.Matches(neg) {
		t.Fatalf("Nexus first should fail when Gateway precedes Nexus")
	}
	// Negative: Forge precedes Nexus — that's Forge Expand, not Nexus First.
	neg2 := factsBuilder().B(subjForge, 88).B(subjNexus, 130).list()
	if bo.Matches(neg2) {
		t.Fatalf("Nexus first should fail when Forge precedes Nexus")
	}
}

func TestBO_ForgeExpa(t *testing.T) {
	bo := findBO(t, "Forge Expand")
	pos := factsBuilder().B(subjForge, 88).B(subjNexus, 130).B(subjGateway, 170).list()
	if !bo.Matches(pos) {
		t.Fatalf("Forge expa should match Forge@88, Nexus@130, Gate@170")
	}
	// Negative: Gate before Nexus.
	neg := factsBuilder().B(subjForge, 88).B(subjGateway, 120).B(subjNexus, 150).list()
	if bo.Matches(neg) {
		t.Fatalf("Forge expa should fail when Gate precedes Nexus")
	}
}

func TestBO_2Gate(t *testing.T) {
	bo := findBO(t, "2 Gate")
	pos := factsBuilder().B(subjGateway, 70).B(subjGateway, 86).list()
	if !bo.Matches(pos) {
		t.Fatalf("2 gate should match two gateways before 180s")
	}
	// Negative: 2nd gate after Nexus.
	neg := factsBuilder().B(subjGateway, 70).B(subjNexus, 100).B(subjGateway, 110).list()
	if bo.Matches(neg) {
		t.Fatalf("2 gate should fail if Nexus precedes the 2nd gate")
	}
	// Negative: only one gate.
	neg2 := factsBuilder().B(subjGateway, 70).list()
	if bo.Matches(neg2) {
		t.Fatalf("2 gate should fail with a single Gateway")
	}
}

func TestResolveExpert_ComputesDeltasAndTolerance(t *testing.T) {
	bo := findBO(t, "9 Pool")
	// Pool actual 78s (target 73, late by 5; within tol=4? No → out).
	// First Zergling at 120 (target 123, early by 3; within tol=3 → in).
	b := factsBuilder()
	for i := 0; i < 9; i++ {
		b.P(subjDrone, 5+i*3)
	}
	s := b.B(subjSpawningPool, 78).P(subjZergling, 120).list()
	res := bo.ResolveExpert(s)
	if len(res) != 2 {
		t.Fatalf("expected 2 resolutions, got %d", len(res))
	}
	if !res[0].Found || res[0].ActualSecond != 78 || res[0].DeltaSeconds != 5 || res[0].WithinTolerance {
		t.Fatalf("pool resolution wrong: %+v", res[0])
	}
	if !res[1].Found || res[1].ActualSecond != 120 || res[1].DeltaSeconds != -3 || !res[1].WithinTolerance {
		t.Fatalf("zergling resolution wrong: %+v", res[1])
	}
}

func TestRegistry_ByPatternName_CaseInsensitive(t *testing.T) {
	bo := ByPatternName("build order: 9 pool")
	if bo == nil || bo.Name != "9 Pool" {
		t.Fatalf("expected 9 Pool, got %+v", bo)
	}
	if ByPatternName("not a real BO") != nil {
		t.Fatalf("expected nil for unknown pattern")
	}
}

func TestIsInitialBuildOrderPatternName(t *testing.T) {
	if !IsInitialBuildOrderPatternName("Build Order: 4 Pool") {
		t.Fatalf("expected true for canonical name")
	}
	if IsInitialBuildOrderPatternName("Quick factory") {
		t.Fatalf("expected false for non-BO pattern")
	}
}

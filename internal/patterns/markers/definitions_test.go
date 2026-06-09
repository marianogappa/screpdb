package markers

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

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
	// Positive: only pool, no drone/overlord morphs. Timing irrelevant —
	// the rule keys off exact morph counts, the Expert events compare
	// against golden timings separately.
	pos := factsBuilder().B(subjSpawningPool, 33).P(subjZergling, 85).list()
	if !bo.Matches(pos) {
		t.Fatalf("4 pool positive should match")
	}
	// Negative: drone before pool means count != 0.
	neg := factsBuilder().P(subjDrone, 10).B(subjSpawningPool, 33).list()
	if bo.Matches(neg) {
		t.Fatalf("4 pool should fail with any drone before pool")
	}
}

func TestBO_9Pool(t *testing.T) {
	bo := findBO(t, "9 Pool")
	// Positive: exactly 5 drone morphs before pool (4 starting + 5 = 9
	// supply at Pool placement), no Overlord morph yet.
	b := factsBuilder()
	for i := 0; i < 5; i++ {
		b.P(subjDrone, 5+i*3)
	}
	pos := b.B(subjSpawningPool, 73).list()
	if !bo.Matches(pos) {
		t.Fatalf("9 pool should match exactly 5 drones then pool")
	}
	// Negative: 5 drones + Overlord + Pool is the "9 Overpool" BO, not
	// plain 9 Pool. See TestBO_9Overpool.
	b = factsBuilder()
	for i := 0; i < 5; i++ {
		b.P(subjDrone, 5+i*3)
	}
	withOvi := b.P(subjOverlord, 30).B(subjSpawningPool, 73).list()
	if bo.Matches(withOvi) {
		t.Fatalf("9 pool should NOT match when an Overlord precedes the Pool (that's 9 Overpool)")
	}
	// Negative: zero drones before pool.
	neg2 := factsBuilder().B(subjSpawningPool, 73).list()
	if bo.Matches(neg2) {
		t.Fatalf("9 pool should fail without drones before pool")
	}
	// Negative: 6 drones (would be 10 supply, not 9).
	b2 := factsBuilder()
	for i := 0; i < 6; i++ {
		b2.P(subjDrone, 5+i*3)
	}
	negCount := b2.B(subjSpawningPool, 73).list()
	if bo.Matches(negCount) {
		t.Fatalf("9 pool should fail with 6 drones (= 10 supply) before pool")
	}
	// Negative: Hatch built before Pool — that's a hatch-first BO.
	b3 := factsBuilder()
	for i := 0; i < 5; i++ {
		b3.P(subjDrone, 5+i*3)
	}
	neg3 := b3.B(subjHatchery, 70).B(subjSpawningPool, 95).list()
	if bo.Matches(neg3) {
		t.Fatalf("9 pool should fail when Hatchery precedes Pool")
	}
	// Negative: Evolution Chamber before Pool.
	b4 := factsBuilder()
	for i := 0; i < 5; i++ {
		b4.P(subjDrone, 5+i*3)
	}
	neg4 := b4.B(subjEvolutionChamber, 60).B(subjSpawningPool, 95).list()
	if bo.Matches(neg4) {
		t.Fatalf("9 pool should fail when Evolution Chamber precedes Pool")
	}
}

func TestBO_9Overpool(t *testing.T) {
	bo := findBO(t, "9 Overpool")
	// Positive: 5 drone morphs + 1 Overlord (the "over"), then Pool.
	b := factsBuilder()
	for i := 0; i < 5; i++ {
		b.P(subjDrone, 5+i*3)
	}
	pos := b.P(subjOverlord, 30).B(subjSpawningPool, 80).list()
	if !bo.Matches(pos) {
		t.Fatalf("9 overpool should match 5 drones + 1 overlord then pool")
	}
	// Negative: no Overlord — that's plain 9 Pool.
	b2 := factsBuilder()
	for i := 0; i < 5; i++ {
		b2.P(subjDrone, 5+i*3)
	}
	neg := b2.B(subjSpawningPool, 73).list()
	if bo.Matches(neg) {
		t.Fatalf("9 overpool should NOT match without an Overlord (that's 9 Pool)")
	}
}

func TestBO_12Pool(t *testing.T) {
	bo := findBO(t, "12 Pool")
	// Positive: 8 drone morphs + 1 Overlord before Pool.
	b := factsBuilder()
	for i := 0; i < 8; i++ {
		b.P(subjDrone, 5+i*3)
	}
	pos := b.P(subjOverlord, 35).B(subjSpawningPool, 104).list()
	if !bo.Matches(pos) {
		t.Fatalf("12 pool should match 8 drones + 1 overlord then pool")
	}
	// Negative: 7 drones (= 11 supply, not 12).
	b3 := factsBuilder()
	for i := 0; i < 7; i++ {
		b3.P(subjDrone, 5+i*3)
	}
	neg2 := b3.P(subjOverlord, 35).B(subjSpawningPool, 104).list()
	if bo.Matches(neg2) {
		t.Fatalf("12 pool should fail with 7 drones (= 11 supply)")
	}
	// Negative: missing Overlord (cap-blocked at 9, can't reach 12).
	b4 := factsBuilder()
	for i := 0; i < 8; i++ {
		b4.P(subjDrone, 5+i*3)
	}
	neg3 := b4.B(subjSpawningPool, 104).list()
	if bo.Matches(neg3) {
		t.Fatalf("12 pool should fail without an Overlord before pool")
	}
}

func TestBO_9PoolIntoHatchery(t *testing.T) {
	bo := findBO(t, "9 Pool into Hatchery")
	// Positive: exactly 5 drone morphs (= supply 9), pool, then hatch within
	// 60s of pool. The exact 5-Drone count is what separates this from the
	// 5–8/10–11 Pool rungs that share the "pool then fast hatch" topology.
	b := factsBuilder()
	for i := 0; i < 5; i++ {
		b.P(subjDrone, 5+i*3)
	}
	pos := b.B(subjSpawningPool, 73).B(subjHatchery, 118).list()
	if !bo.Matches(pos) {
		t.Fatalf("9 pool → hatch should match hatch@118 after pool@73")
	}
	// Negative: hatch built too late after pool.
	b = factsBuilder()
	for i := 0; i < 5; i++ {
		b.P(subjDrone, 5+i*3)
	}
	neg := b.B(subjSpawningPool, 73).B(subjHatchery, 200).list()
	if bo.Matches(neg) {
		t.Fatalf("9 pool → hatch should fail hatch@200 after pool@73 (gap > 60)")
	}
	// Negative: 7-Pool topology (3 drones) that also takes a fast hatch must
	// NOT match here — it belongs to "7 Pool".
	b = factsBuilder()
	for i := 0; i < 3; i++ {
		b.P(subjDrone, 5+i*3)
	}
	neg2 := b.B(subjSpawningPool, 73).B(subjHatchery, 110).list()
	if bo.Matches(neg2) {
		t.Fatalf("9 pool → hatch should NOT match a 3-drone (7 Pool) stream")
	}
}

func TestBO_10Hatch(t *testing.T) {
	bo := findBO(t, "10 Hatch")
	// Positive: 6 drone morphs + 1 Overlord, then Hatch (no Pool yet).
	b := factsBuilder()
	for i := 0; i < 6; i++ {
		b.P(subjDrone, 5+i*3)
	}
	pos := b.P(subjOverlord, 30).B(subjHatchery, 80).list()
	if !bo.Matches(pos) {
		t.Fatalf("10 hatch should match 6 drones + 1 overlord then hatch")
	}
	// Negative: Pool first.
	b2 := factsBuilder()
	for i := 0; i < 6; i++ {
		b2.P(subjDrone, 5+i*3)
	}
	neg := b2.P(subjOverlord, 30).B(subjSpawningPool, 60).B(subjHatchery, 80).list()
	if bo.Matches(neg) {
		t.Fatalf("10 hatch should fail when Pool precedes Hatch")
	}
}

func TestBO_11Hatch(t *testing.T) {
	bo := findBO(t, "11 Hatch")
	// Positive: 7 drone morphs + 1 Overlord, then Hatch.
	b := factsBuilder()
	for i := 0; i < 7; i++ {
		b.P(subjDrone, 5+i*3)
	}
	pos := b.P(subjOverlord, 30).B(subjHatchery, 94).list()
	if !bo.Matches(pos) {
		t.Fatalf("11 hatch should match 7 drones + 1 overlord then hatch")
	}
	// Negative: 8 drones (= 12 supply, would be 12 Hatch).
	b2 := factsBuilder()
	for i := 0; i < 8; i++ {
		b2.P(subjDrone, 5+i*3)
	}
	neg := b2.P(subjOverlord, 30).B(subjHatchery, 98).list()
	if bo.Matches(neg) {
		t.Fatalf("11 hatch should fail with 8 drones (= 12 supply)")
	}
}

func TestBO_12Hatch(t *testing.T) {
	bo := findBO(t, "12 Hatch")
	// Positive: 8 drone morphs + 1 Overlord, then Hatch.
	b := factsBuilder()
	for i := 0; i < 8; i++ {
		b.P(subjDrone, 5+i*3)
	}
	pos := b.P(subjOverlord, 35).B(subjHatchery, 98).list()
	if !bo.Matches(pos) {
		t.Fatalf("12 hatch should match 8 drones + 1 overlord then hatch")
	}
	// Negative: pool first.
	b2 := factsBuilder()
	for i := 0; i < 8; i++ {
		b2.P(subjDrone, 5+i*3)
	}
	neg := b2.P(subjOverlord, 35).B(subjSpawningPool, 73).B(subjHatchery, 118).list()
	if bo.Matches(neg) {
		t.Fatalf("12 hatch should fail when pool precedes hatch")
	}
}

func TestBO_1GateCore(t *testing.T) {
	bo := findBO(t, "1 Gate Core")
	// Positive: 1 Gate, Cyber, no 2nd Gate or Nexus before Cyber.
	pos := factsBuilder().B(subjGateway, 86).B(subjAssimilator, 116).B(subjCyberneticsCore, 138).list()
	if !bo.Matches(pos) {
		t.Fatalf("1 Gate Core should match Gate@86, Assim@116, Cyber@138")
	}
	// Negative: 2nd Gate before Cyber → that's 2 Gate.
	neg := factsBuilder().B(subjGateway, 70).B(subjGateway, 90).B(subjCyberneticsCore, 138).list()
	if bo.Matches(neg) {
		t.Fatalf("1 Gate Core should fail when 2nd Gateway precedes Cyber Core")
	}
	// Negative: Nexus before Cyber → that's Nexus First.
	neg2 := factsBuilder().B(subjGateway, 86).B(subjNexus, 120).B(subjCyberneticsCore, 138).list()
	if bo.Matches(neg2) {
		t.Fatalf("1 Gate Core should fail when Nexus precedes Cyber Core")
	}
	// Negative: Cyber too late.
	neg3 := factsBuilder().B(subjGateway, 86).B(subjCyberneticsCore, 200).list()
	if bo.Matches(neg3) {
		t.Fatalf("1 Gate Core should fail when Cyber Core is built after 180s")
	}
}

func TestBO_NexusFirst(t *testing.T) {
	bo := findBO(t, "Nexus First")
	pos := factsBuilder().B(subjNexus, 145).B(subjGateway, 175).list()
	if !bo.Matches(pos) {
		t.Fatalf("Nexus first should match")
	}
	neg := factsBuilder().B(subjGateway, 80).B(subjNexus, 145).list()
	if bo.Matches(neg) {
		t.Fatalf("Nexus first should fail when Gateway precedes Nexus")
	}
	// Negative: Forge precedes Nexus — that's Forge Expand, not Nexus First.
	neg2 := factsBuilder().B(subjForge, 88).B(subjNexus, 145).list()
	if bo.Matches(neg2) {
		t.Fatalf("Nexus first should fail when Forge precedes Nexus")
	}
	// Negative: Nexus too late (>200s).
	neg3 := factsBuilder().B(subjNexus, 220).B(subjGateway, 240).list()
	if bo.Matches(neg3) {
		t.Fatalf("Nexus first should fail when Nexus is built after 200s")
	}
}

func TestBO_GateExpand(t *testing.T) {
	bo := findBO(t, "Gate Expand")
	// Positive: Gate before Forge & Nexus, Nexus by 200s, no 2nd Gate before Nexus.
	pos := factsBuilder().B(subjGateway, 88).B(subjNexus, 165).list()
	if !bo.Matches(pos) {
		t.Fatalf("Gate Expand should match Gate@88, Nexus@165")
	}
	// Negative: Forge before Gate — that's Forge Expand.
	neg := factsBuilder().B(subjForge, 86).B(subjGateway, 120).B(subjNexus, 152).list()
	if bo.Matches(neg) {
		t.Fatalf("Gate Expand should fail when Forge precedes Gateway")
	}
	// Negative: 2 Gates before Nexus — that's 2 Gate.
	neg2 := factsBuilder().B(subjGateway, 70).B(subjGateway, 86).B(subjNexus, 165).list()
	if bo.Matches(neg2) {
		t.Fatalf("Gate Expand should fail when 2nd Gateway precedes Nexus")
	}
}

func TestBO_ForgeExpa(t *testing.T) {
	bo := findBO(t, "Forge Expand")
	pos := factsBuilder().B(subjForge, 86).B(subjNexus, 152).B(subjGateway, 200).list()
	if !bo.Matches(pos) {
		t.Fatalf("Forge expa should match Forge@86, Nexus@152, Gate@200")
	}
	// Negative: Gate before Nexus.
	neg := factsBuilder().B(subjForge, 88).B(subjGateway, 120).B(subjNexus, 150).list()
	if bo.Matches(neg) {
		t.Fatalf("Forge expa should fail when Gate precedes Nexus")
	}
	// Negative: Forge after Gate.
	neg2 := factsBuilder().B(subjGateway, 80).B(subjForge, 100).B(subjNexus, 150).list()
	if bo.Matches(neg2) {
		t.Fatalf("Forge expa should fail when Gateway precedes Forge")
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
	// Negative: 2nd gate after Cyber.
	neg2 := factsBuilder().B(subjGateway, 70).B(subjCyberneticsCore, 100).B(subjGateway, 110).list()
	if bo.Matches(neg2) {
		t.Fatalf("2 gate should fail if Cyber Core precedes the 2nd gate")
	}
	// Negative: 2nd gate after Forge.
	neg3 := factsBuilder().B(subjGateway, 70).B(subjForge, 100).B(subjGateway, 110).list()
	if bo.Matches(neg3) {
		t.Fatalf("2 gate should fail if Forge precedes the 2nd gate")
	}
	// Negative: only one gate.
	neg4 := factsBuilder().B(subjGateway, 70).list()
	if bo.Matches(neg4) {
		t.Fatalf("2 gate should fail with a single Gateway")
	}
}

// ---------------------------------------------------------------------------
// Terran openers
// ---------------------------------------------------------------------------

// marines adds n Marine produces starting at `from`, 10s apart.
func produceN(b *factsB, unit string, n, from int) *factsB {
	for i := 0; i < n; i++ {
		b.P(unit, from+i*10)
	}
	return b
}

func TestBO_Terran_Bio_ByBarracks(t *testing.T) {
	// 3 Barracks, mass Marine/Medic, no mech → 3-Rax Bio.
	b := factsBuilder().B(subjSupplyDepot, 30).B(subjBarracks, 80).B(subjBarracks, 120).B(subjBarracks, 160)
	produceN(b, subjMarine, 10, 200)
	produceN(b, subjMedic, 2, 320)
	stream := b.list()
	if !findBO(t, "3-Rax Bio").Matches(stream) {
		t.Fatalf("3 Barracks + mass M&M should be 3-Rax Bio")
	}
	for _, other := range []string{"2-Rax Bio", "4-Rax Bio", "6+ Rax Bio", "2-Fac Mech", "1-1-1"} {
		if findBO(t, other).Matches(stream) {
			t.Fatalf("bio stream must not also match %q (mutex)", other)
		}
	}
	// 6 Barracks → 6+ Rax Bio.
	b6 := factsBuilder().B(subjSupplyDepot, 30)
	for i := 0; i < 6; i++ {
		b6.B(subjBarracks, 80+i*20)
	}
	produceN(b6, subjMarine, 10, 220)
	if !findBO(t, "6+ Rax Bio").Matches(b6.list()) {
		t.Fatalf("6 Barracks + mass Marine should be 6+ Rax Bio")
	}
}

func TestBO_Terran_Mech_ByFactory_AndTankless(t *testing.T) {
	// 2 Factories, Vultures + Tanks, mech-predominant → 2-Fac Mech.
	b := factsBuilder().B(subjSupplyDepot, 30).B(subjBarracks, 80).B(subjFactory, 150).B(subjFactory, 200)
	produceN(b, subjVulture, 6, 220)
	produceN(b, subjSiegeTank, 2, 300)
	mech := b.list()
	if !findBO(t, "2-Fac Mech").Matches(mech) {
		t.Fatalf("2 Factories + Vultures + Tanks should be 2-Fac Mech")
	}
	for _, other := range []string{"3-Fac Mech", "2-Fac Tankless Mech", "3-Rax Bio", "1-1-1"} {
		if findBO(t, other).Matches(mech) {
			t.Fatalf("mech stream must not also match %q (mutex)", other)
		}
	}
	// 2 Factories, Vultures, NO Tank → 2-Fac Tankless Mech.
	tb := factsBuilder().B(subjSupplyDepot, 30).B(subjBarracks, 80).B(subjFactory, 150).B(subjFactory, 200)
	produceN(tb, subjVulture, 8, 220)
	tankless := tb.list()
	if !findBO(t, "2-Fac Tankless Mech").Matches(tankless) {
		t.Fatalf("2 Factories + Vultures, no Tank should be 2-Fac Tankless Mech")
	}
	if findBO(t, "2-Fac Mech").Matches(tankless) {
		t.Fatalf("tankless stream must not match 2-Fac Mech (mutex)")
	}
}

func TestBO_Terran_Wraith_Goliath_111(t *testing.T) {
	// 2 Starports + 5 Wraiths → Wraith.
	w := factsBuilder().B(subjSupplyDepot, 30).B(subjBarracks, 80).B(subjFactory, 150).
		B(subjStarport, 200).B(subjStarport, 260)
	produceN(w, subjWraith, 5, 300)
	if !findBO(t, "Wraith").Matches(w.list()) {
		t.Fatalf("2 Starports + 5 Wraiths should be Wraith")
	}
	// ≤2 Vultures & ≤4 Marines by 7:00, 4+ Goliaths → Goliath.
	g := factsBuilder().B(subjSupplyDepot, 30).B(subjBarracks, 80).B(subjFactory, 150).
		P(subjVulture, 200).P(subjMarine, 120)
	produceN(g, subjGoliath, 4, 300)
	if !findBO(t, "Goliath").Matches(g.list()) {
		t.Fatalf("≤2 Vult, ≤4 Marine, 4 Goliaths should be Goliath")
	}
	// Early Starport + a Wraith, balanced composition → 1-1-1.
	o := factsBuilder().B(subjSupplyDepot, 30).B(subjBarracks, 80).B(subjFactory, 150).B(subjStarport, 300).
		P(subjVulture, 320).P(subjMarine, 330).P(subjWraith, 360).list()
	if !findBO(t, "1-1-1").Matches(o) {
		t.Fatalf("early Starport + Wraith, balanced, should be 1-1-1")
	}
}

func TestBO_CCFirst(t *testing.T) {
	bo := findBO(t, "CC First")
	pos := factsBuilder().
		B(subjSupplyDepot, 62).B(subjCommandCenter, 145).B(subjBarracks, 165).list()
	if !bo.Matches(pos) {
		t.Fatalf("CC First should match Depot, CC, Rax sequence")
	}
	// Negative: Rax before CC.
	neg := factsBuilder().
		B(subjSupplyDepot, 62).B(subjBarracks, 88).B(subjCommandCenter, 180).list()
	if bo.Matches(neg) {
		t.Fatalf("CC First should fail when Barracks precedes CC")
	}
	// Negative: CC built too late (>200s).
	neg2 := factsBuilder().B(subjSupplyDepot, 62).B(subjCommandCenter, 220).list()
	if bo.Matches(neg2) {
		t.Fatalf("CC First should fail when CC is built after 200s")
	}
}

func TestBO_BBS(t *testing.T) {
	bo := findBO(t, "BBS")
	// Positive: 2 Barracks before any Depot (e.g. SST_JumJaJungJi).
	pos := factsBuilder().B(subjBarracks, 58).B(subjBarracks, 79).B(subjSupplyDepot, 100).list()
	if !bo.Matches(pos) {
		t.Fatalf("BBS should match 2 Barracks before Depot")
	}
	// Negative: Depot before 2nd Barracks.
	neg := factsBuilder().B(subjBarracks, 60).B(subjSupplyDepot, 80).B(subjBarracks, 100).list()
	if bo.Matches(neg) {
		t.Fatalf("BBS should fail when Depot precedes 2nd Barracks")
	}
	// Negative: 1st Barracks too late.
	neg2 := factsBuilder().B(subjBarracks, 110).B(subjBarracks, 130).list()
	if bo.Matches(neg2) {
		t.Fatalf("BBS should fail when 1st Barracks built after 100s")
	}
	// Negative: only one Barracks.
	neg3 := factsBuilder().B(subjBarracks, 60).list()
	if bo.Matches(neg3) {
		t.Fatalf("BBS should fail with a single Barracks")
	}
}

// ---------------------------------------------------------------------------
// New ladder rungs / openers / residuals
// ---------------------------------------------------------------------------

// zergPool builds a stream of n drone morphs then a Pool.
func zergPoolStream(drones, poolSec int) []cmdenrich.EnrichedCommand {
	b := factsBuilder()
	for i := 0; i < drones; i++ {
		b.P(subjDrone, 5+i*3)
	}
	return b.B(subjSpawningPool, poolSec).list()
}

func TestBO_PoolLadder_5to11(t *testing.T) {
	// supply → expected BO name. Supply = 4 + drone morphs before Pool.
	cases := map[int]string{5: "5 Pool", 6: "6 Pool", 7: "7 Pool", 8: "8 Pool", 10: "10 Pool", 11: "11 Pool"}
	for supply, name := range cases {
		bo := findBO(t, name)
		drones := supply - 4
		if !bo.Matches(zergPoolStream(drones, 50+drones*6)) {
			t.Fatalf("%s should match exactly %d drones before Pool", name, drones)
		}
		// One extra drone is the next rung up, not this one.
		if bo.Matches(zergPoolStream(drones+1, 50+drones*6)) {
			t.Fatalf("%s should NOT match with %d drones", name, drones+1)
		}
		// A Hatchery before the Pool makes it hatch-first, not a pool BO.
		hatchFirst := factsBuilder()
		for i := 0; i < drones; i++ {
			hatchFirst.P(subjDrone, 5+i*3)
		}
		neg := hatchFirst.B(subjHatchery, 40).B(subjSpawningPool, 80).list()
		if bo.Matches(neg) {
			t.Fatalf("%s should fail when a Hatchery precedes the Pool", name)
		}
	}
}

func TestBO_HatchLadder_4to8(t *testing.T) {
	cases := map[int]string{4: "4 Hatch", 5: "5 Hatch", 6: "6 Hatch", 7: "7 Hatch", 8: "8 Hatch"}
	for supply, name := range cases {
		bo := findBO(t, name)
		drones := supply - 4
		b := factsBuilder()
		for i := 0; i < drones; i++ {
			b.P(subjDrone, 5+i*3)
		}
		pos := b.B(subjHatchery, 50+drones*8).list()
		if !bo.Matches(pos) {
			t.Fatalf("%s should match exactly %d drones before the Hatchery", name, drones)
		}
		// A Pool before the Hatchery makes it pool-first, not hatch-first.
		b2 := factsBuilder()
		for i := 0; i < drones; i++ {
			b2.P(subjDrone, 5+i*3)
		}
		neg := b2.B(subjSpawningPool, 40).B(subjHatchery, 80).list()
		if bo.Matches(neg) {
			t.Fatalf("%s should fail when a Pool precedes the Hatchery", name)
		}
	}
}

func TestBO_ProtossNoExpa(t *testing.T) {
	gate := findBO(t, "1 Gate (no expa)")
	// Positive: lone Gateway, no fast Cyber, no expansion.
	if !gate.Matches(factsBuilder().B(subjPylon, 48).B(subjGateway, 90).list()) {
		t.Fatalf("1 Gate (no expa) should match a lone Gateway with no expand/cyber")
	}
	// Negative: expands (Nexus) — that's Gate Expand.
	if gate.Matches(factsBuilder().B(subjGateway, 90).B(subjNexus, 180).list()) {
		t.Fatalf("1 Gate (no expa) should fail when the player expands")
	}
	fc := findBO(t, "Forge Cannon (no expa)")
	// Positive: Forge + Cannon, no expansion.
	if !fc.Matches(factsBuilder().B(subjForge, 90).B(subjPhotonCannon, 130).list()) {
		t.Fatalf("Forge Cannon (no expa) should match Forge + Cannon with no expand")
	}
	// Negative: Forge then Nexus is FFE, not this.
	if fc.Matches(factsBuilder().B(subjForge, 90).B(subjPhotonCannon, 130).B(subjNexus, 200).list()) {
		t.Fatalf("Forge Cannon (no expa) should fail when the player expands (FFE)")
	}
}

func TestBO_BunkerRush(t *testing.T) {
	bo := findBO(t, "Bunker Rush")
	// Positive: Rax → Bunker, all-in (no CC, no Factory). A defensive gas is
	// now fine — the build is defined by the absence of an expansion/tech.
	pos := factsBuilder().B(subjBarracks, 55).B(subjSupplyDepot, 90).B(subjBunker, 130).list()
	if !bo.Matches(pos) {
		t.Fatalf("Bunker Rush should match Rax → Bunker all-in (no CC, no Factory)")
	}
	posGas := factsBuilder().B(subjBarracks, 55).B(subjRefinery, 100).B(subjBunker, 140).list()
	if !bo.Matches(posGas) {
		t.Fatalf("Bunker Rush should still match when a defensive gas precedes the Bunker")
	}
	// Negative: expands (CC) — that's 1 Rax FE, not an all-in.
	negCC := factsBuilder().B(subjBarracks, 55).B(subjBunker, 130).B(subjCommandCenter, 200).list()
	if bo.Matches(negCC) {
		t.Fatalf("Bunker Rush should fail when the player expands (CC)")
	}
	// Negative: techs to Factory — that's 1 Rax 1 Fac / 1-1-1.
	negFac := factsBuilder().B(subjBarracks, 55).B(subjBunker, 130).B(subjRefinery, 140).B(subjFactory, 200).list()
	if bo.Matches(negFac) {
		t.Fatalf("Bunker Rush should fail when the player techs to a Factory")
	}
	// Negative: 2 Rax before the bunker — that's BBS.
	neg2 := factsBuilder().B(subjBarracks, 55).B(subjBarracks, 80).B(subjBunker, 130).list()
	if bo.Matches(neg2) {
		t.Fatalf("Bunker Rush should fail with 2 Barracks before the Bunker (that's BBS)")
	}
}

func TestBO_GateExpand_NonPvZ(t *testing.T) {
	bo := findBO(t, "Gate Expand")
	// Positive: Gateway → Nexus → Cyber (the PvT/PvP 1-Gate expand).
	pos := factsBuilder().B(subjGateway, 80).B(subjNexus, 160).B(subjCyberneticsCore, 185).list()
	if !bo.Matches(pos) {
		t.Fatalf("Gate Expand should match Gateway → Nexus → Cyber")
	}
	// Negative: Cyber before Nexus — that's 1 Gate Core, not an expand.
	neg := factsBuilder().B(subjGateway, 80).B(subjCyberneticsCore, 130).B(subjNexus, 180).list()
	if bo.Matches(neg) {
		t.Fatalf("Gate Expand should fail when Cyber precedes Nexus")
	}
}

func TestBO_ForgeExpand_Loosened(t *testing.T) {
	bo := findBO(t, "Forge Expand")
	// Positive: slower FFE — Forge@130 (was rejected by the old <100 bound),
	// Nexus@240 (was rejected by the old <200 bound).
	pos := factsBuilder().B(subjForge, 130).B(subjNexus, 240).B(subjGateway, 260).list()
	if !bo.Matches(pos) {
		t.Fatalf("Forge Expand should match a slow FFE (Forge@130, Nexus@240)")
	}
}

func TestBO_Residuals_Complement(t *testing.T) {
	// Zerg residual: greedy Pool at supply ≥13 (≥9 drones), no named rung.
	zerg := findBO(t, "Pool/Hatch (Other)")
	if !zerg.Matches(zergPoolStream(9, 140)) {
		t.Fatalf("Zerg residual should match a 9-drone (supply 13) Pool")
	}
	if zerg.Matches(zergPoolStream(5, 73)) {
		t.Fatalf("Zerg residual should NOT match a named rung (9 Pool)")
	}
	// Protoss residual: a leftover Gateway+Forge opener (no Cannon, no
	// expansion) that matches none of the named builds — not 1 Gate (no expa)
	// (which excludes a Forge), not Forge Cannon (no Cannon), not FFE (no Nexus).
	prot := findBO(t, "Gateway (Other)")
	if !prot.Matches(factsBuilder().B(subjGateway, 70).B(subjForge, 120).list()) {
		t.Fatalf("Protoss residual should match a leftover Gateway+Forge opener")
	}
	if prot.Matches(factsBuilder().B(subjGateway, 70).B(subjCyberneticsCore, 138).list()) {
		t.Fatalf("Protoss residual should NOT match a 1 Gate Core")
	}
	// A lone slow Gateway is now its own named build, not the residual.
	if prot.Matches(factsBuilder().B(subjGateway, 70).B(subjCyberneticsCore, 200).list()) {
		t.Fatalf("Protoss residual should NOT match a lone slow Gateway (that's 1 Gate (no expa))")
	}
	// Terran residual: a Barracks opener with no real army (tiny/short game)
	// that matches no composition BO nor a kept topology opener.
	terr := findBO(t, "Terran (Other)")
	if !terr.Matches(factsBuilder().B(subjSupplyDepot, 60).B(subjBarracks, 88).B(subjRefinery, 120).B(subjAcademy, 160).list()) {
		t.Fatalf("Terran (Other) should match a Barracks opener with no defining composition")
	}
}

func TestMarker_OpenerUnresolved(t *testing.T) {
	m := findBO(t, "Opener unresolved")
	// Positive: no defining building ever placed (only workers/supply).
	if !m.Matches(factsBuilder().P(subjDrone, 10).P(subjDrone, 20).list()) {
		t.Fatalf("Opener unresolved should match when no defining building is placed")
	}
	// Negative: a Pool (Zerg defining building) was placed.
	if m.Matches(factsBuilder().B(subjSpawningPool, 40).list()) {
		t.Fatalf("Opener unresolved should NOT match once a Pool is placed")
	}
	// Negative: a Gateway (Protoss defining building) was placed.
	if m.Matches(factsBuilder().B(subjGateway, 70).list()) {
		t.Fatalf("Opener unresolved should NOT match once a Gateway is placed")
	}
}

// ---------------------------------------------------------------------------
// 10+ Scouts (Money-map signature)
// ---------------------------------------------------------------------------

func TestMarker_TenPlusScouts_PositiveAtTenth(t *testing.T) {
	m := findBO(t, "10+ Scouts")
	b := factsBuilder()
	for i := 0; i < 10; i++ {
		b.P("Scout", 600+i*5)
	}
	if !m.Matches(b.list()) {
		t.Fatalf("10+ Scouts should match exactly 10 scouts produced")
	}
}

func TestMarker_TenPlusScouts_NegativeAtNine(t *testing.T) {
	m := findBO(t, "10+ Scouts")
	b := factsBuilder()
	for i := 0; i < 9; i++ {
		b.P("Scout", 600+i*5)
	}
	if m.Matches(b.list()) {
		t.Fatalf("10+ Scouts should NOT match with only 9 scouts")
	}
}

// ---------------------------------------------------------------------------
// Double Stargate (PvZ signature)
// ---------------------------------------------------------------------------

func TestMarker_DoubleStargate_PositiveTwoGatesSixSairs(t *testing.T) {
	m := findBO(t, "Double Stargate")
	b := factsBuilder()
	b.B(subjStargate, 300).B(subjStargate, 360)
	produceN(b, subjCorsair, 6, 380)
	if !m.Matches(b.list()) {
		t.Fatalf("2 Stargates + 6 Corsairs should match Double Stargate")
	}
}

func TestMarker_DoubleStargate_NegativeOneStargate(t *testing.T) {
	m := findBO(t, "Double Stargate")
	b := factsBuilder()
	b.B(subjStargate, 300)
	produceN(b, subjCorsair, 8, 380)
	if m.Matches(b.list()) {
		t.Fatalf("a single Stargate should NOT match Double Stargate")
	}
}

func TestMarker_DoubleStargate_NegativeFewCorsairs(t *testing.T) {
	m := findBO(t, "Double Stargate")
	b := factsBuilder()
	b.B(subjStargate, 300).B(subjStargate, 360)
	produceN(b, subjCorsair, 5, 380)
	if m.Matches(b.list()) {
		t.Fatalf("only 5 Corsairs should NOT match Double Stargate")
	}
}

func TestResolveExpert_ComputesDeltasAndTolerance(t *testing.T) {
	bo := findBO(t, "9 Pool")
	// Pool actual 78s (target 73, late by 5; within tol=4? No → out).
	// First Zergling at 120 (target 123, early by 3; within tol=3 → in).
	b := factsBuilder()
	for i := 0; i < 5; i++ {
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

package moments

import (
	"testing"

	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

func missileDeath(sec int, reason openbw.DeathReason, x, y int32) engine.Death {
	return engine.Death{Frame: f(sec), Unit: 9000, Type: ut(bwgame.UnitTypesTerranNuclearMissile), Owner: 0, X: x, Y: y, Reason: uint8(reason), KillerOwner: NoPlayer}
}

func nukeAt(sec int) *engine.Digest {
	d := digest()
	d.Casts = []engine.Cast{{Frame: f(sec), Owner: 0, Order: orderNukePaint, X: 3650, Y: 3650}}
	return d
}

func TestNukeLandsAndKills(t *testing.T) {
	d := nukeAt(900)
	d.Deaths = append(d.Deaths, missileDeath(912, openbw.ReasonDetonated, 3650, 3650))
	for range 3 {
		addDeath(d, 912, 1, bwgame.UnitTypesProtossProbe, 3700, 3640, 0, bwgame.UnitTypesTerranNuclearMissile)
	}
	addDeath(d, 912, 1, bwgame.UnitTypesProtossProbe, 1000, 1000, 0, bwgame.UnitTypesTerranNuclearMissile)
	got := ofType(Detect(d, testBases, tvp), "nuke")
	if len(got) != 1 || got[0].Result != NukeLanded || got[0].SnapshotSecond != 911 {
		t.Fatalf("a nuke that lands is shown the second before: %+v", got)
	}
	if o := got[0].Outcome; o.KilledUnits != 3 || o.Workers != 3 {
		t.Fatalf("the blast kills the three Probes under it, not the one across the map: %+v", o)
	}
}

func TestNukeWithTheGhostKilledOrStopped(t *testing.T) {
	d := nukeAt(900)
	d.Deaths = append(d.Deaths, missileDeath(907, openbw.ReasonLinked, 3776, 14))
	addDeath(d, 907, 0, bwgame.UnitTypesTerranGhost, 3400, 3650, 1, bwgame.UnitTypesProtossDragoon)
	if got := ofType(Detect(d, testBases, tvp), "nuke"); len(got) != 1 || got[0].Result != NukeGhostKilled || got[0].SnapshotSecond != 906 {
		t.Fatalf("the missile dies with its Ghost: %+v", got)
	}
	d.Deaths = d.Deaths[:1]
	if got := ofType(Detect(d, testBases, tvp), "nuke"); len(got) != 1 || got[0].Result != NukeCancelled {
		t.Fatalf("the missile dies with the Ghost alive: %+v", got)
	}
}

func TestNukeThatNeverFlew(t *testing.T) {
	d := nukeAt(900)
	addDeath(d, 905, 0, bwgame.UnitTypesTerranGhost, 3300, 3500, 1, bwgame.UnitTypesProtossDragoon)
	if got := ofType(Detect(d, testBases, tvp), "nuke"); len(got) != 1 || got[0].Result != NukeGhostKilled {
		t.Fatalf("a Ghost killed walking into range: %+v", got)
	}
	d.Deaths = nil
	if got := ofType(Detect(d, testBases, tvp), "nuke"); len(got) != 1 || got[0].Result != NukeCancelled || got[0].SnapshotSecond != 900 {
		t.Fatalf("no missile and no dead Ghost: %+v", got)
	}
	d = nukeAt(1190)
	if got := ofType(Detect(d, testBases, tvp), "nuke"); len(got) != 1 || got[0].Result != "" {
		t.Fatalf("the game ends before the nuke could land: %+v", got)
	}
}

func recallInto(d *engine.Digest, sec int, n int) {
	units := make([]uint16, n)
	for i := range units {
		units[i] = ut(bwgame.UnitTypesProtossZealot)
	}
	d.Casts = append(d.Casts, engine.Cast{Frame: f(sec - 2), Owner: 1, Order: orderRecall, X: 3600, Y: 3600})
	d.Teleports = append(d.Teleports, engine.Teleport{Frame: f(sec), Owner: 1, FromX: 3600, FromY: 3600, ToX: 400, ToY: 400, Units: units})
}

func TestRecallsMergeAndCountUnits(t *testing.T) {
	d := digest()
	recallInto(d, 700, 8)
	recallInto(d, 702, 4)
	recallInto(d, 800, 3)
	got := ofType(Detect(d, testBases, zvp), "recall")
	if len(got) != 2 || got[0].Count != 12 || got[1].Count != 3 {
		t.Fatalf("two Arbiters recalling together are one recall; one later is another: %+v", got)
	}
}

func TestRecallWithHallucinatedArbiters(t *testing.T) {
	d := digest()
	recallInto(d, 700, 6)
	d.Casts = append(d.Casts, engine.Cast{Frame: f(670), Owner: 1, Order: orderHallucination, X: 3000, Y: 3000})
	for _, sec := range []int{705, 720, 760} {
		d.Deaths = append(d.Deaths, engine.Death{Frame: f(sec), Type: ut(bwgame.UnitTypesProtossArbiter), Owner: 1, X: 400, Y: 400, Class: uint8(openbw.ClassHallucination), Reason: uint8(openbw.ReasonDamage), KillerOwner: 0})
	}
	got := ofType(Detect(d, testBases, zvp), "recall")
	if len(got) != 1 || got[0].Hallucinated != 2 {
		t.Fatalf("two hallucinated Arbiters die within a hallucination's life of the cast: %+v", got)
	}
	if got[0].Outcome.Lost != 0 {
		t.Fatalf("hallucinations are not losses: %+v", got[0].Outcome)
	}
}

func TestRecallResults(t *testing.T) {
	cases := []struct {
		name  string
		setup func(d *engine.Digest)
		want  string
	}{
		{"razes the main", func(d *engine.Digest) {
			addDeath(d, 720, 0, bwgame.UnitTypesZergHatchery, 300, 300, 1, bwgame.UnitTypesProtossZealot)
		}, RecallRazedBase},
		{"razes buildings", func(d *engine.Digest) {
			addDeath(d, 720, 0, bwgame.UnitTypesZergSpawningPool, 420, 420, 1, bwgame.UnitTypesProtossZealot)
			addDeath(d, 725, 0, bwgame.UnitTypesZergEvolutionChamber, 460, 420, 1, bwgame.UnitTypesProtossZealot)
		}, RecallRazed},
		{"dies at once", func(d *engine.Digest) {
			for i := range 6 {
				addDeath(d, 705+i, 1, bwgame.UnitTypesProtossZealot, 400, 400, 0, bwgame.UnitTypesZergLurker)
			}
		}, RecallWiped},
		{"trades well", func(d *engine.Digest) {
			for i := range 8 {
				addDeath(d, 705+i, 0, bwgame.UnitTypesZergDrone, 400, 400, 1, bwgame.UnitTypesProtossZealot)
			}
			addDeath(d, 712, 1, bwgame.UnitTypesProtossZealot, 400, 400, 0, bwgame.UnitTypesZergZergling)
		}, RecallTradedWell},
		{"trades poorly", func(d *engine.Digest) {
			addDeath(d, 705, 0, bwgame.UnitTypesZergDrone, 400, 400, 1, bwgame.UnitTypesProtossZealot)
			for i := range 4 {
				addDeath(d, 730+i, 1, bwgame.UnitTypesProtossZealot, 400, 400, 0, bwgame.UnitTypesZergZergling)
			}
		}, RecallTradedPoorly},
		{"nothing happens", func(d *engine.Digest) {}, ""},
	}
	for _, c := range cases {
		d := digest()
		recallInto(d, 700, 6)
		c.setup(d)
		got := ofType(Detect(d, testBases, zvp), "recall")
		if len(got) != 1 || got[0].Result != c.want {
			t.Errorf("%s: %+v, want %q", c.name, got, c.want)
		}
	}
}

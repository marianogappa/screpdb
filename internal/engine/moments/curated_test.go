package moments

import (
	"testing"

	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

func f(sec int) int32 { return int32(sec * framesPerSecond) }

// A two-player map: mains at the top-left and bottom-right, a natural beside each, one open expansion.
var testBases = []Base{
	{Kind: BaseStart, X: 300, Y: 300},
	{Kind: BaseNatural, X: 900, Y: 400},
	{Kind: BaseStart, X: 3700, Y: 3700},
	{Kind: BaseNatural, X: 3100, Y: 3600},
	{Kind: BaseExpansion, X: 2000, Y: 400},
}

var zvp = []Player{{Slot: 0, Race: openbw.RaceZerg, Human: true}, {Slot: 1, Race: openbw.RaceProtoss, Human: true}}

func digest() *engine.Digest {
	return &engine.Digest{
		LastFrame: f(1200),
		Players:   []engine.Player{{Slot: 0}, {Slot: 1}},
		Initial: []engine.InitialUnit{
			{Unit: 1, Type: ut(bwgame.UnitTypesZergHatchery), Owner: 0, X: 300, Y: 300},
			{Unit: 2, Type: ut(bwgame.UnitTypesProtossNexus), Owner: 1, X: 3700, Y: 3700},
		},
		Drift: engine.Drift{DesyncFrame: engine.NoFrame, ErrorFrame: engine.NoFrame},
	}
}

func cell(v int32) int32 { return v / engine.ArmyCellPixels }

func army(sec int, owner int8, t bwgame.UnitTypes, n int, x, y int32) engine.ArmySnapshot {
	s := engine.ArmySnapshot{Frame: f(sec)}
	for range n {
		s.Units = append(s.Units, engine.ArmyUnit{Owner: owner, Type: ut(t), X: cell(x), Y: cell(y)})
	}
	return s
}

func pool(start int) engine.Production {
	return engine.Production{Owner: 0, Type: ut(bwgame.UnitTypesZergSpawningPool), From: ut(bwgame.UnitTypesZergDrone), X: 400, Y: 400, Start: f(start), Complete: f(start + 50), Cancel: engine.NoFrame}
}

func types(events []Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.Type)
	}
	return out
}

func TestZerglingRushNeedsANinePoolAndCommitment(t *testing.T) {
	d := digest()
	d.Productions = []engine.Production{pool(60)}
	d.Army = []engine.ArmySnapshot{army(190, 0, bwgame.UnitTypesZergZergling, 4, 3650, 3650)}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("four lings poking and leaving are not a rush: %v", types(got))
	}
	d.Deaths = []engine.Death{
		{Frame: f(200), Type: ut(bwgame.UnitTypesProtossProbe), Owner: 1, X: 3640, Y: 3640, Reason: uint8(openbw.ReasonDamage), KillerOwner: 0},
		{Frame: f(205), Type: ut(bwgame.UnitTypesProtossProbe), Owner: 1, X: 3640, Y: 3640, Reason: uint8(openbw.ReasonDamage), KillerOwner: 0},
	}
	got := Detect(d, testBases, zvp)
	if len(got) != 1 || got[0].Type != "zergling_rush" || got[0].Target != 1 || got[0].Base != 2 || got[0].Second != 190 {
		t.Fatalf("early lings that killed two Probes: %+v", got)
	}
	d.Productions = []engine.Production{pool(76)}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("overpool lings harassing are not a rush: %v", types(got))
	}
}

func TestDropsNeedDangerOrNumbersOrDamage(t *testing.T) {
	d := digest()
	d.Unloads = []engine.Unload{{Frame: f(600), TransportType: ut(bwgame.UnitTypesProtossShuttle), Owner: 1, X: 350, Y: 350, Cargo: []uint16{ut(bwgame.UnitTypesProtossReaver)}}}
	got := Detect(d, testBases, zvp)
	if len(got) != 1 || got[0].Type != "drop" || got[0].Units[1] != ut(bwgame.UnitTypesProtossReaver) || got[0].SnapshotSecond != 601 {
		t.Fatalf("a Reaver drop: %+v", got)
	}
	d.Unloads[0].Cargo = []uint16{ut(bwgame.UnitTypesProtossZealot), ut(bwgame.UnitTypesProtossZealot)}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("two Zealots that killed nothing are not a drop worth showing: %v", types(got))
	}
	d.Deaths = []engine.Death{{Frame: f(610), Type: ut(bwgame.UnitTypesZergDrone), Owner: 0, X: 340, Y: 340, Reason: uint8(openbw.ReasonDamage), KillerOwner: 1}}
	if got := Detect(d, testBases, zvp); len(got) != 1 || got[0].Type != "drop" {
		t.Fatalf("a drop that killed a Drone: %v", types(got))
	}
}

func TestRecallNeedsTheCast(t *testing.T) {
	d := digest()
	d.Teleports = []engine.Teleport{{Frame: f(700), Owner: 1, FromX: 3600, FromY: 3600, ToX: 400, ToY: 400, Units: []uint16{65, 65, 66}}}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("a teleport without a Recall cast: %v", types(got))
	}
	d.Casts = []engine.Cast{{Frame: f(695), Owner: 1, Order: orderRecall, X: 410, Y: 410}}
	got := Detect(d, testBases, zvp)
	if len(got) != 1 || got[0].Type != "recall" || got[0].Base != 0 || got[0].SourceBase != 2 || got[0].Target != 0 {
		t.Fatalf("recall: %+v", got)
	}
}

func TestAttacksNeedBloodAndFoldWithinAMinute(t *testing.T) {
	d := digest()
	for sec := 500; sec <= 530; sec += 10 {
		d.Army = append(d.Army, army(sec, 1, bwgame.UnitTypesProtossZealot, 6, 350, 350))
	}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("an army that drew no blood: %v", types(got))
	}
	for i := range 4 {
		d.Deaths = append(d.Deaths, engine.Death{Frame: f(505 + i), Type: ut(bwgame.UnitTypesZergDrone), Owner: 0, X: 340, Y: 340, Reason: uint8(openbw.ReasonDamage), KillerOwner: 1})
	}
	d.Army = append(d.Army, army(540, 1, bwgame.UnitTypesProtossZealot, 6, 950, 420))
	d.Initial = append(d.Initial, engine.InitialUnit{Unit: 3, Type: ut(bwgame.UnitTypesZergHatchery), Owner: 0, X: 900, Y: 400})
	for i := range 4 {
		d.Deaths = append(d.Deaths, engine.Death{Frame: f(541 + i), Type: ut(bwgame.UnitTypesZergDrone), Owner: 0, X: 910, Y: 410, Reason: uint8(openbw.ReasonDamage), KillerOwner: 1})
	}
	got := Detect(d, testBases, zvp)
	if len(got) != 1 || got[0].Type != "attack" || got[0].Actor != 1 || got[0].Target != 0 || got[0].Second != 500 {
		t.Fatalf("want one attack at 8:20, the natural a few seconds later folded in: %+v", got)
	}
}

func TestExpansionsSkipBasesAlreadyHeldAndMacroHatcheries(t *testing.T) {
	d := digest()
	d.Productions = []engine.Production{
		{Owner: 0, Type: ut(bwgame.UnitTypesZergHatchery), From: ut(bwgame.UnitTypesZergDrone), Unit: 10, X: 320, Y: 330, Start: f(100), Complete: f(170), Cancel: engine.NoFrame},
		{Owner: 0, Type: ut(bwgame.UnitTypesZergHatchery), From: ut(bwgame.UnitTypesZergDrone), Unit: 11, X: 900, Y: 400, Start: f(120), Complete: f(190), Cancel: engine.NoFrame},
		{Owner: 0, Type: ut(bwgame.UnitTypesZergLair), From: ut(bwgame.UnitTypesZergHatchery), Unit: 1, X: 300, Y: 300, Start: f(300), Complete: f(400), Cancel: engine.NoFrame},
		{Owner: 1, Type: ut(bwgame.UnitTypesProtossNexus), From: uint16(openbw.UnitTypeNone), Unit: 12, X: 2000, Y: 400, Start: f(600), Complete: engine.NoFrame, Cancel: f(620)},
	}
	got := Detect(d, testBases, zvp)
	if len(got) != 1 || got[0].Type != "expansion" || got[0].Base != 1 || got[0].Second != 120 || got[0].SnapshotSecond != 122 {
		t.Fatalf("want only the natural: %+v", got)
	}
}

func TestEventsStopAtTheTrustedFrame(t *testing.T) {
	d := digest()
	d.Drift.DesyncFrame = f(100)
	d.Productions = []engine.Production{{Owner: 0, Type: ut(bwgame.UnitTypesZergHatchery), From: ut(bwgame.UnitTypesZergDrone), X: 900, Y: 400, Start: f(120), Complete: f(190), Cancel: engine.NoFrame}}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("events after the desync: %v", types(got))
	}
}

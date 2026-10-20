//go:build !js

package dashboard

import (
	"encoding/base64"
	"testing"

	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/moments"
)

const (
	unitMarine        = 0
	unitCommandCenter = 106
	unitSupplyDepot   = 109
	unitHatchery      = 131
)

func TestFrameSecondUsesFastestSpeed(t *testing.T) {
	cases := map[int32]int64{0: 0, 23: 0, 24: 1, 1000: 42, 1429: 60}
	for frame, want := range cases {
		if got := frameSecond(frame); got != want {
			t.Errorf("frameSecond(%d) = %d, want %d", frame, got, want)
		}
	}
}

func TestPopulateSupplyCurvesHalvesDedupesAndCaps(t *testing.T) {
	dg := &engine.Digest{Economy: []engine.EconomySample{
		{Frame: 0, Owner: 0, SupplyUsed: 8, SupplyMax: 18},
		{Frame: 24, Owner: 0, SupplyUsed: 8, SupplyMax: 18},
		{Frame: 48, Owner: 0, SupplyUsed: 10, SupplyMax: 18},
		{Frame: 0, Owner: 1, SupplyUsed: 380, SupplyMax: 420},
		{Frame: 0, Owner: 5, SupplyUsed: 2, SupplyMax: 2},
	}}
	detail := workflowGameDetail{Players: []workflowGamePlayer{{PlayerID: 10}, {PlayerID: 11}, {PlayerID: 12}}}
	populateSupplyCurves(&detail, dg, map[int8]int64{0: 10, 1: 11})

	p0 := detail.Players[0].SupplyCurve
	if len(p0) != 2 || p0[0] != (workflowSupplyPoint{Second: 0, Used: 4, Cap: 9}) || p0[1] != (workflowSupplyPoint{Second: 2, Used: 5, Cap: 9}) {
		t.Fatalf("player 0 curve = %+v", p0)
	}
	p1 := detail.Players[1].SupplyCurve
	if len(p1) != 1 || p1[0].Used != 190 || p1[0].Cap != 200 {
		t.Fatalf("player 1 curve = %+v, want cap clamped to 200", p1)
	}
	if detail.Players[2].SupplyCurve != nil {
		t.Fatalf("player without samples got %+v", detail.Players[2].SupplyCurve)
	}
}

func creepGrid(w, h int, tiles ...[2]int) engine.Snapshot {
	bits := make([]byte, (w*h+7)/8)
	for _, tl := range tiles {
		i := tl[1]*w + tl[0]
		bits[i/8] |= 1 << (i % 8)
	}
	return engine.Snapshot{CreepW: w, Creep: bits}
}

func TestCreepEdgesRingASingleCreepTile(t *testing.T) {
	if got := creepEdges(engine.Snapshot{}); got != nil {
		t.Fatalf("no creep width: got %v", got)
	}
	edges := creepEdges(creepGrid(5, 5, [2]int{2, 2}))
	if len(edges)%3 != 0 {
		t.Fatalf("edges are not triples: %v", edges)
	}
	seen := map[[2]int]int{}
	for i := 0; i < len(edges); i += 3 {
		tx, ty, frame := edges[i], edges[i+1], edges[i+2]
		if tx < 1 || tx > 3 || ty < 1 || ty > 3 || (tx == 2 && ty == 2) {
			t.Errorf("edge at (%d,%d) is not next to the creep tile", tx, ty)
		}
		if frame <= 0 {
			t.Errorf("edge at (%d,%d) has frame %d", tx, ty, frame)
		}
		seen[[2]int{tx, ty}] = frame
	}
	if len(seen) == 0 {
		t.Fatal("a lone creep tile drew no edges")
	}
	north := scmapanalyzer.CreepEdgeFrame(1 << 1)
	if f, ok := seen[[2]int{2, 1}]; ok && f != north {
		t.Errorf("tile north of creep: frame %d, want %d", f, north)
	}
}

func TestCreepEdgesSkipFullyCoveredMap(t *testing.T) {
	snap := engine.Snapshot{CreepW: 4, Creep: []byte{0xFF}}
	if got := creepEdges(snap); len(got) != 0 {
		t.Fatalf("all creep: got edges %v", got)
	}
}

func TestUnitName(t *testing.T) {
	if got := unitName(unitMarine); got != "Marine" {
		t.Errorf("unitName(marine) = %q", got)
	}
	if got := unitName(60000); got == "Marine" {
		t.Errorf("unknown id resolved to %q", got)
	}
}

func TestSnapshotBuildingFootprintAndTownHall(t *testing.T) {
	cc, ok := snapshotBuilding(unitCommandCenter, 7, 100, 200, true)
	if !ok {
		t.Fatal("command center is a building")
	}
	if cc.UnitType != "Command Center" || cc.PlayerID != 7 || cc.X != 100 || cc.Y != 200 || cc.W != 128 || cc.H != 96 || !cc.Complete || !cc.TownHall {
		t.Fatalf("command center = %+v", cc)
	}
	depot, ok := snapshotBuilding(unitSupplyDepot, 7, 0, 0, false)
	if !ok || depot.TownHall || depot.Complete || depot.W != 96 || depot.H != 64 {
		t.Fatalf("supply depot = %+v ok=%v", depot, ok)
	}
	if _, ok := snapshotBuilding(unitMarine, 7, 0, 0, true); ok {
		t.Fatal("a marine is not a building")
	}
}

func TestCoarseSnapshotWire(t *testing.T) {
	snap := moments.Snapshot{
		Second: 120, ArmySecond: 110,
		Buildings: []moments.Building{
			{Type: unitCommandCenter, Owner: 0, X: 10, Y: 20, Complete: true},
			{Type: unitSupplyDepot, Owner: 0, X: 30, Y: 40, Progress: 1.5},
			{Type: unitHatchery, Owner: 1, X: 50, Y: 60, Progress: -0.2},
			{Type: unitMarine, Owner: 0},
			{Type: unitCommandCenter, Owner: 9},
		},
		Units: []moments.Unit{{Type: unitMarine, Owner: 0, X: 5, Y: 6}, {Type: unitMarine, Owner: 9}},
	}
	out := coarseSnapshotWire(snap, map[int8]int64{0: 100, 1: 101})
	if out.Second != 120 || out.ArmySecond != 110 || out.Exact {
		t.Fatalf("header = %+v", out)
	}
	if len(out.Buildings) != 3 {
		t.Fatalf("buildings = %+v", out.Buildings)
	}
	if out.Buildings[0].Progress != nil {
		t.Error("complete building has progress")
	}
	if p := out.Buildings[1].Progress; p == nil || *p != 1 {
		t.Errorf("progress not clamped to 1: %v", p)
	}
	if p := out.Buildings[2].Progress; p == nil || *p != 0 || out.Buildings[2].PlayerID != 101 {
		t.Errorf("progress not clamped to 0: %+v", out.Buildings[2])
	}
	if len(out.Units) != 1 || out.Units[0] != (engineSnapshotUnit{UnitType: "Marine", PlayerID: 100, X: 5, Y: 6}) {
		t.Fatalf("units = %+v", out.Units)
	}
}

func TestExactSnapshotWire(t *testing.T) {
	creep := creepGrid(4, 2, [2]int{0, 0})
	snap := engine.Snapshot{
		Second: 300,
		Units: []engine.SnapshotUnit{
			{Type: unitCommandCenter, Owner: 0, X: 10, Y: 20, Completed: true, Lifted: true, Progress: engine.ProgressDone},
			{Type: unitSupplyDepot, Owner: 0, X: 30, Y: 40, Progress: 127},
			{Type: unitMarine, Owner: 1, X: 7, Y: 8, Burrowed: true, Progress: engine.ProgressDone},
			{Type: unitMarine, Owner: 4},
		},
		CreepW:  creep.CreepW,
		Creep:   creep.Creep,
		Tileset: 4,
	}
	out := exactSnapshotWire(snap, map[int8]int64{0: 100, 1: 101})
	if !out.Exact || out.Second != 300 || out.ArmySecond != 300 {
		t.Fatalf("header = %+v", out)
	}
	if len(out.Buildings) != 2 || !out.Buildings[0].Lifted || out.Buildings[0].Progress != nil {
		t.Fatalf("buildings = %+v", out.Buildings)
	}
	if p := out.Buildings[1].Progress; p == nil || *p != 127.0/254 {
		t.Errorf("depot progress = %v", p)
	}
	if len(out.Units) != 1 || out.Units[0] != (engineSnapshotUnit{UnitType: "Marine", PlayerID: 101, X: 7, Y: 8, Burrowed: true}) {
		t.Fatalf("units = %+v", out.Units)
	}
	if out.Creep == nil || out.Creep.W != 4 || out.Creep.Tileset != "jungle" || out.Creep.Bits != base64.StdEncoding.EncodeToString(snap.Creep) || len(out.Creep.Edges) == 0 {
		t.Fatalf("creep = %+v", out.Creep)
	}

	if bare := exactSnapshotWire(engine.Snapshot{Second: 1}, nil); bare.Creep != nil || len(bare.Buildings) != 0 || len(bare.Units) != 0 {
		t.Fatalf("empty snapshot = %+v", bare)
	}
}

func TestDigestMemoCachesAndEvictsOldest(t *testing.T) {
	var m digestMemo
	loads := map[[32]byte]int{}
	loader := func(k [32]byte) func() (*engine.Digest, bool) {
		return func() (*engine.Digest, bool) {
			loads[k]++
			return &engine.Digest{}, true
		}
	}
	key := func(i int) [32]byte { return [32]byte{byte(i)} }

	first := m.get(key(0), loader(key(0)))
	if first == nil {
		t.Fatal("first load returned nil")
	}
	if again := m.get(key(0), loader(key(0))); again != first || loads[key(0)] != 1 {
		t.Fatalf("cached hit reloaded: loads=%d", loads[key(0)])
	}
	for i := 1; i <= recentDigests; i++ {
		m.get(key(i), loader(key(i)))
	}
	if len(m.order) != recentDigests || len(m.byKey) != recentDigests {
		t.Fatalf("memo holds %d/%d, want %d", len(m.order), len(m.byKey), recentDigests)
	}
	if _, ok := m.byKey[key(0)]; ok {
		t.Fatal("oldest digest was not evicted")
	}
	if m.get(key(0), loader(key(0))) == first || loads[key(0)] != 2 {
		t.Fatalf("evicted digest loads = %d, want 2", loads[key(0)])
	}

	if got := m.get(key(99), func() (*engine.Digest, bool) { return nil, false }); got != nil {
		t.Fatalf("failed load returned %+v", got)
	}
	if _, ok := m.byKey[key(99)]; ok {
		t.Fatal("failed load was cached")
	}
}

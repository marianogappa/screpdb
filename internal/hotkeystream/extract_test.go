package hotkeystream

import (
	"testing"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/rep/repcore"
)

func unitByName(t *testing.T, name string) *repcmd.Unit {
	t.Helper()
	for id := uint16(0); id < 250; id++ {
		if u := repcmd.UnitByID(id); u != nil && u.Name == name {
			return u
		}
	}
	t.Fatalf("unit %q not found", name)
	return nil
}

func techByName(t *testing.T, name string) *repcmd.Tech {
	t.Helper()
	for id := byte(0); id < 60; id++ {
		if x := repcmd.TechByID(id); x != nil && x.Name == name {
			return x
		}
	}
	t.Fatalf("tech %q not found", name)
	return nil
}

func upgradeByName(t *testing.T, name string) *repcmd.Upgrade {
	t.Helper()
	for id := byte(0); id < 70; id++ {
		if x := repcmd.UpgradeByID(id); x != nil && x.Name == name {
			return x
		}
	}
	t.Fatalf("upgrade %q not found", name)
	return nil
}

type replayBuilder struct {
	t    *testing.T
	cmds []repcmd.Cmd
}

func (b *replayBuilder) base(pid byte, sec int, typeID byte) *repcmd.Base {
	return &repcmd.Base{
		Frame:    repcore.Frame(sec * 1000 / 42),
		PlayerID: pid,
		Type:     repcmd.TypeByID(typeID),
	}
}

func (b *replayBuilder) sel(pid byte, sec int, tags ...uint16) {
	unitTags := make([]repcmd.UnitTag, len(tags))
	for i, tag := range tags {
		unitTags[i] = repcmd.UnitTag(tag)
	}
	b.cmds = append(b.cmds, &repcmd.SelectCmd{Base: b.base(pid, sec, repcmd.TypeIDSelect), UnitTags: unitTags})
}

func (b *replayBuilder) selRemove(pid byte, sec int, tags ...uint16) {
	unitTags := make([]repcmd.UnitTag, len(tags))
	for i, tag := range tags {
		unitTags[i] = repcmd.UnitTag(tag)
	}
	b.cmds = append(b.cmds, &repcmd.SelectCmd{Base: b.base(pid, sec, repcmd.TypeIDSelectRemove), UnitTags: unitTags})
}

func (b *replayBuilder) selAdd(pid byte, sec int, tags ...uint16) {
	unitTags := make([]repcmd.UnitTag, len(tags))
	for i, tag := range tags {
		unitTags[i] = repcmd.UnitTag(tag)
	}
	b.cmds = append(b.cmds, &repcmd.SelectCmd{Base: b.base(pid, sec, repcmd.TypeIDSelectAdd), UnitTags: unitTags})
}

func (b *replayBuilder) hotkey(pid byte, sec int, hotkeyTypeID byte, group byte) {
	b.cmds = append(b.cmds, &repcmd.HotkeyCmd{
		Base:       b.base(pid, sec, repcmd.TypeIDHotkey),
		HotkeyType: repcmd.HotkeyTypes[hotkeyTypeID],
		Group:      group,
	})
}

func (b *replayBuilder) build(pid byte, sec int, unitName string, x, y uint16) {
	b.cmds = append(b.cmds, &repcmd.BuildCmd{
		Base: b.base(pid, sec, repcmd.TypeIDBuild),
		Pos:  repcore.Point{X: x, Y: y},
		Unit: unitByName(b.t, unitName),
	})
}

func (b *replayBuilder) train(pid byte, sec int, unitName string) {
	b.cmds = append(b.cmds, &repcmd.TrainCmd{
		Base: b.base(pid, sec, repcmd.TypeIDTrain),
		Unit: unitByName(b.t, unitName),
	})
}

func (b *replayBuilder) unitMorph(pid byte, sec int, unitName string) {
	b.cmds = append(b.cmds, &repcmd.TrainCmd{
		Base: b.base(pid, sec, repcmd.TypeIDUnitMorph),
		Unit: unitByName(b.t, unitName),
	})
}

func (b *replayBuilder) buildingMorph(pid byte, sec int, unitName string) {
	b.cmds = append(b.cmds, &repcmd.BuildingMorphCmd{
		Base: b.base(pid, sec, repcmd.TypeIDBuildingMorph),
		Unit: unitByName(b.t, unitName),
	})
}

func (b *replayBuilder) tech(pid byte, sec int, name string) {
	b.cmds = append(b.cmds, &repcmd.TechCmd{Base: b.base(pid, sec, repcmd.TypeIDTech), Tech: techByName(b.t, name)})
}

func (b *replayBuilder) upgrade(pid byte, sec int, name string) {
	b.cmds = append(b.cmds, &repcmd.UpgradeCmd{Base: b.base(pid, sec, repcmd.TypeIDUpgrade), Upgrade: upgradeByName(b.t, name)})
}

func (b *replayBuilder) scan(pid byte, sec int) {
	b.cmds = append(b.cmds, &repcmd.TargetedOrderCmd{
		Base:  b.base(pid, sec, repcmd.TypeIDTargetedOrder),
		Order: repcmd.OrderByID(repcmd.OrderIDCastScannerSweep),
	})
}

func (b *replayBuilder) replay() *rep.Replay {
	terran := &rep.Player{ID: 1, SlotID: 0}
	zerg := &rep.Player{ID: 2, SlotID: 1}
	return &rep.Replay{
		Header: &rep.Header{
			Players:    []*rep.Player{terran, zerg},
			PIDPlayers: map[byte]*rep.Player{1: terran, 2: zerg},
		},
		Commands: &rep.Commands{Cmds: b.cmds},
		MapData: &rep.MapData{
			StartLocations: []rep.StartLocation{
				{Point: repcore.Point{X: 64 * 32, Y: 32 * 32}, SlotID: 0},
				{Point: repcore.Point{X: 200 * 32, Y: 200 * 32}, SlotID: 1},
			},
		},
	}
}

func eventsFor(events []Event, group byte) []Event {
	var out []Event
	for _, e := range events {
		if e.Group == group {
			out = append(out, e)
		}
	}
	return out
}

func TestExtractAnnotatesAssigns(t *testing.T) {
	b := &replayBuilder{t: t}
	const terran, zergP = byte(1), byte(2)

	// Terran: tag 10 is a worker (single-selected Build), assigned to group 1.
	b.sel(terran, 1, 10)
	b.build(terran, 2, "Barracks", 30, 30)
	b.hotkey(terran, 3, repcmd.HotkeyTypeIDAssign, 1)
	// Tag 20 trains a Marine, so it is the Barracks placed at (30,30).
	b.sel(terran, 10, 20)
	b.train(terran, 11, "Marine")
	b.hotkey(terran, 12, repcmd.HotkeyTypeIDAssign, 2)
	// Tag 30 builds the Comsat add-on, proving it is the (spawn-seeded, no
	// Build command) Command Center: location falls back to the start tile.
	b.sel(terran, 20, 30)
	b.build(terran, 21, "ComSat", 66, 31)
	b.hotkey(terran, 22, repcmd.HotkeyTypeIDAssign, 3)
	// Tag 40 casts a scan, so it is the Comsat placed at (66,31).
	b.sel(terran, 30, 40)
	b.scan(terran, 31)
	b.hotkey(terran, 32, repcmd.HotkeyTypeIDAssign, 4)
	// Tag 50 researches Stim, so it is the Academy; no Academy Build command,
	// and it is no town hall, so its tile stays unknown.
	b.sel(terran, 40, 50)
	b.tech(terran, 41, "Stim Packs")
	b.hotkey(terran, 42, repcmd.HotkeyTypeIDAssign, 5)
	// Tag 60 starts +1 infantry weapons, so it is the Engineering Bay at (10,10).
	b.build(terran, 49, "Engineering Bay", 10, 10)
	b.sel(terran, 50, 60)
	b.upgrade(terran, 51, "Terran Infantry Weapons")
	b.hotkey(terran, 52, repcmd.HotkeyTypeIDAssign, 6)
	// A three-unit army on group 7, then recalled and added.
	b.sel(terran, 60, 70, 71, 72)
	b.hotkey(terran, 61, repcmd.HotkeyTypeIDAssign, 7)
	b.hotkey(terran, 62, repcmd.HotkeyTypeIDSelect, 7)
	b.hotkey(terran, 63, repcmd.HotkeyTypeIDAssign+2, 2) // Add group 2
	// SelectRemove down to a single unknown tag; assign is a 1-unit group.
	b.sel(terran, 70, 300, 301)
	b.selRemove(terran, 71, 301)
	b.hotkey(terran, 72, repcmd.HotkeyTypeIDAssign, 8)
	// Group 16 exercises the wire escape.
	b.hotkey(terran, 73, repcmd.HotkeyTypeIDAssign, 16)
	// SelectAdd grows the selection before an assign.
	b.selAdd(terran, 74, 302)
	b.hotkey(terran, 75, repcmd.HotkeyTypeIDAssign, 9)

	// Zerg: the hatch-tap pattern proves tag 200 is a town hall, anchored to
	// the start location; the Lair morph proves tag 210 is a second hall whose
	// tile stays unknown (only one start location to hand out).
	b.sel(zergP, 5, 200)
	b.sel(zergP, 6, 201, 202)
	b.unitMorph(zergP, 7, "Drone")
	b.sel(zergP, 8, 200)
	b.hotkey(zergP, 9, repcmd.HotkeyTypeIDAssign, 5)
	b.sel(zergP, 15, 210)
	b.buildingMorph(zergP, 16, "Lair")
	b.hotkey(zergP, 17, repcmd.HotkeyTypeIDAssign, 6)

	streams := Extract(b.replay())
	terranEvents, zergEvents := streams[terran], streams[zergP]
	if len(terranEvents) == 0 || len(zergEvents) == 0 {
		t.Fatalf("missing streams: terran=%d zerg=%d", len(terranEvents), len(zergEvents))
	}

	check := func(t *testing.T, events []Event, group byte, wantType byte, wantBuilding string, wantX, wantY byte, wantCount byte) {
		t.Helper()
		got := eventsFor(events, group)
		if len(got) == 0 {
			t.Fatalf("group %d: no events", group)
		}
		e := got[0]
		if e.Type != wantType {
			t.Fatalf("group %d: type = %s, want %s", group, TypeName(e.Type), TypeName(wantType))
		}
		if wantType == TypeAssignBuilding {
			if BuildingName(e.Building) != wantBuilding {
				t.Fatalf("group %d: building = %q, want %q", group, BuildingName(e.Building), wantBuilding)
			}
			if e.TileX != wantX || e.TileY != wantY {
				t.Fatalf("group %d: tile = (%d,%d), want (%d,%d)", group, e.TileX, e.TileY, wantX, wantY)
			}
		}
		if wantType == TypeAssignUnits && e.Count != wantCount {
			t.Fatalf("group %d: count = %d, want %d", group, e.Count, wantCount)
		}
	}

	check(t, terranEvents, 1, TypeAssignUnits, "", 0, 0, 1)
	check(t, terranEvents, 2, TypeAssignBuilding, "Barracks", 30, 30, 0)
	// Start location (64,32) pixels/32 minus half a 4x3 footprint => (62,31).
	check(t, terranEvents, 3, TypeAssignBuilding, "Command Center", 62, 31, 0)
	check(t, terranEvents, 4, TypeAssignBuilding, "Comsat Station", 66, 31, 0)
	check(t, terranEvents, 5, TypeAssignBuilding, "Academy", TileUnknown, TileUnknown, 0)
	check(t, terranEvents, 6, TypeAssignBuilding, "Engineering Bay", 10, 10, 0)
	check(t, terranEvents, 7, TypeAssignUnits, "", 0, 0, 3)
	check(t, terranEvents, 8, TypeAssignUnits, "", 0, 0, 1)
	check(t, terranEvents, 16, TypeAssignUnits, "", 0, 0, 1)
	check(t, terranEvents, 9, TypeAssignUnits, "", 0, 0, 2)

	check(t, zergEvents, 5, TypeAssignBuilding, "Hatchery", 198, 199, 0)
	check(t, zergEvents, 6, TypeAssignBuilding, "Hatchery", TileUnknown, TileUnknown, 0)

	// Select/Add events survive with their type and second.
	g7 := eventsFor(terranEvents, 7)
	// Seconds round-trip through frames (sec -> frame -> sec floors twice), so
	// assert the recall's second within that tolerance rather than exactly.
	if len(g7) != 2 || g7[1].Type != TypeSelect || g7[1].Sec < 60 || g7[1].Sec > 62 {
		t.Fatalf("group 7 recall wrong: %+v", g7)
	}
	g2 := eventsFor(terranEvents, 2)
	if len(g2) != 2 || g2[1].Type != TypeAdd {
		t.Fatalf("group 2 add wrong: %+v", g2)
	}

	// The extracted stream must round-trip through the wire format.
	decoded, err := Decode(Encode(append([]Event(nil), terranEvents...)))
	if err != nil || len(decoded) != len(terranEvents) {
		t.Fatalf("round trip failed: %v (%d vs %d)", err, len(decoded), len(terranEvents))
	}
}

func TestExtractNilAndEmpty(t *testing.T) {
	if got := Extract(nil); len(got) != 0 {
		t.Fatalf("Extract(nil) = %v", got)
	}
	if got := Extract(&rep.Replay{}); len(got) != 0 {
		t.Fatalf("Extract(empty) = %v", got)
	}
	b := &replayBuilder{t: t}
	b.sel(1, 1, 10) // selection but no hotkey events: no stream
	if got := Extract(b.replay()); len(got) != 0 {
		t.Fatalf("player without hotkey events must produce no stream, got %v", got)
	}
}

func TestExtractRecycledTagNearestEvidenceWins(t *testing.T) {
	b := &replayBuilder{t: t}
	// Tag 10 is a worker early (issues a Build), then long after is proven a
	// Barracks; an assign near the late evidence must classify as the building.
	b.sel(1, 1, 10)
	b.build(1, 2, "Supply Depot", 5, 5)
	b.build(1, 500, "Barracks", 40, 40)
	b.sel(1, 600, 10)
	b.train(1, 601, "Marine")
	b.hotkey(1, 602, repcmd.HotkeyTypeIDAssign, 1)
	events := Extract(b.replay())[1]
	got := eventsFor(events, 1)
	if len(got) != 1 || got[0].Type != TypeAssignBuilding || BuildingName(got[0].Building) != "Barracks" {
		t.Fatalf("recycled tag must classify by nearest evidence, got %+v", got)
	}
	if got[0].TileX != 40 || got[0].TileY != 40 {
		t.Fatalf("recycled tag location = (%d,%d), want (40,40)", got[0].TileX, got[0].TileY)
	}
}

func TestClampTile(t *testing.T) {
	for _, tc := range []struct {
		in   int
		want byte
	}{{-3, 0}, {0, 0}, {100, 100}, {254, 254}, {255, 254}, {400, 254}} {
		if got := clampTile(tc.in); got != tc.want {
			t.Fatalf("clampTile(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

package engine

import (
	"reflect"
	"testing"

	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/rep/repcore"
	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/screpsim"
)

func bareState() *passState {
	return &passState{
		d:            &Digest{Drift: Drift{ErrorFrame: NoFrame, FirstMissing: NoFrame, DesyncFrame: NoFrame}},
		hotkeyGroups: map[hotkeyGroupKey][]openbw.UnitID{},
	}
}

func cmdResult(frame int32, owner int8, actionID byte, cmd repcmd.Cmd, selection ...openbw.UnitID) *screpsim.CommandResult {
	return &screpsim.CommandResult{
		CommandResult: &openbw.CommandResult{EventBase: openbw.EventBase{Frame: frame}, Owner: owner, ActionID: actionID, Selection: selection},
		Cmd:           cmd,
	}
}

func hotkey(typeID, group byte) *repcmd.HotkeyCmd {
	return &repcmd.HotkeyCmd{HotkeyType: repcmd.HotkeyTypeByID(typeID), Group: group}
}

func TestUnionUnits(t *testing.T) {
	a := []openbw.UnitID{1, 2, 3}
	got := unionUnits(a, []openbw.UnitID{3, 4, 1, 5, 4})
	if !reflect.DeepEqual(got, []openbw.UnitID{1, 2, 3, 4, 5}) {
		t.Fatalf("union %v", got)
	}
	got[0] = 99
	if a[0] != 1 {
		t.Fatal("union aliases its first argument")
	}
	if got := unionUnits(nil, nil); len(got) != 0 {
		t.Fatalf("empty union %v", got)
	}
}

func TestRecordHotkeyIgnoresWhatItCannotKey(t *testing.T) {
	cases := map[string]*screpsim.CommandResult{
		"not a hotkey":        cmdResult(10, 0, 0x13, &repcmd.GeneralCmd{}, 1),
		"no command":          cmdResult(10, 0, 0x13, nil, 1),
		"nil hotkey type":     cmdResult(10, 0, 0x13, &repcmd.HotkeyCmd{Group: 1}, 1),
		"group past 9":        cmdResult(10, 0, 0x13, hotkey(repcmd.HotkeyTypeIDAssign, 10), 1),
		"observer slot":       cmdResult(10, maxPlayerSlot+1, 0x13, hotkey(repcmd.HotkeyTypeIDAssign, 1), 1),
		"unknown hotkey type": cmdResult(10, 0, 0x13, hotkey(7, 1), 1),
	}
	for name, e := range cases {
		p := bareState()
		p.recordHotkey(e)
		if len(p.hotkeyGroups) != 0 || len(p.d.Hotkeys) != 0 {
			t.Errorf("%s: groups %v hotkeys %v", name, p.hotkeyGroups, p.d.Hotkeys)
		}
	}
}

func TestRecordHotkeySelectReplacesGroupWithoutRecording(t *testing.T) {
	p := bareState()
	key := hotkeyGroupKey{owner: 2, group: 4}
	p.hotkeyGroups[key] = []openbw.UnitID{9}
	sel := []openbw.UnitID{5, 6}
	p.recordHotkey(cmdResult(10, 2, 0x13, hotkey(repcmd.HotkeyTypeIDSelect, 4), sel...))
	if !reflect.DeepEqual(p.hotkeyGroups[key], sel) {
		t.Fatalf("group %v, want %v", p.hotkeyGroups[key], sel)
	}
	sel[0] = 99
	if p.hotkeyGroups[key][0] != 5 {
		t.Fatal("group aliases the selection")
	}
	if len(p.d.Hotkeys) != 0 {
		t.Fatalf("a recall recorded an assign: %v", p.d.Hotkeys)
	}
}

func TestRecordHotkeyAssignAndAddWithEmptySelection(t *testing.T) {
	p := bareState()
	key := hotkeyGroupKey{owner: 1, group: 0}
	p.hotkeyGroups[key] = []openbw.UnitID{7}
	p.recordHotkey(cmdResult(10, 1, 0x13, hotkey(repcmd.HotkeyTypeIDAssign, 0)))
	if len(p.hotkeyGroups[key]) != 0 || len(p.d.Hotkeys) != 0 {
		t.Fatalf("assigning nothing: group %v hotkeys %v", p.hotkeyGroups[key], p.d.Hotkeys)
	}
	p.recordHotkey(cmdResult(20, 1, 0x13, hotkey(repcmd.HotkeyTypeIDAdd, 0)))
	if len(p.hotkeyGroups[key]) != 0 || len(p.d.Hotkeys) != 0 {
		t.Fatalf("adding nothing: group %v hotkeys %v", p.hotkeyGroups[key], p.d.Hotkeys)
	}
}

func targeted(orderID byte, x, y uint16) *repcmd.TargetedOrderCmd {
	return &repcmd.TargetedOrderCmd{Order: repcmd.OrderByID(orderID), Pos: repcore.Point{X: x, Y: y}}
}

func TestRecordCast(t *testing.T) {
	p := bareState()
	accepted := cmdResult(500, 3, 0x14, targeted(0x8e, 1000, 2000))
	rejected := cmdResult(510, 3, 0x14, targeted(0x8e, 1000, 2000))
	rejected.Outcome = openbw.Rejected
	for _, e := range []*screpsim.CommandResult{
		accepted,
		rejected,
		cmdResult(520, 3, 0x14, targeted(0x0a, 5, 5)),
		cmdResult(530, 3, 0x14, &repcmd.TargetedOrderCmd{}),
		cmdResult(540, 3, 0x14, hotkey(repcmd.HotkeyTypeIDSelect, 1)),
		cmdResult(550, 0, 0x14, targeted(0x89, 64, 96)),
	} {
		p.recordCast(e)
	}
	want := []Cast{{Frame: 500, Owner: 3, Order: 0x8e, X: 1000, Y: 2000}, {Frame: 550, Owner: 0, Order: 0x89, X: 64, Y: 96}}
	if !reflect.DeepEqual(p.d.Casts, want) {
		t.Fatalf("casts %+v, want %+v", p.d.Casts, want)
	}
}

func TestCommandResultCountsSelects(t *testing.T) {
	p := bareState()
	sel := func(frame int32, action byte, missing uint8) *screpsim.CommandResult {
		e := cmdResult(frame, 0, action, &repcmd.GeneralCmd{})
		e.MissingTags = missing
		return e
	}
	for _, e := range []*screpsim.CommandResult{
		sel(10, actionSelect, 0),
		sel(20, 0x13, 3),
		sel(30, actionShiftSelect, 2),
		sel(40, actionSelect, 1),
		sel(50, actionShiftSelect, 0),
	} {
		p.commandResult(e)
	}
	dr := p.d.Drift
	if dr.Selects != 4 || dr.MissingSelects != 2 || dr.FirstMissing != 30 {
		t.Fatalf("drift %+v", dr)
	}
	want := []selectResult{{10, false}, {30, true}, {40, true}, {50, false}}
	if !reflect.DeepEqual(p.selects, want) {
		t.Fatalf("selects %v, want %v", p.selects, want)
	}
}

func TestCmdOf(t *testing.T) {
	p := &passState{actionToCmd: []int32{NoCmd, 4, 9}}
	cases := map[int32]int32{-1: NoCmd, 0: NoCmd, 1: 4, 2: 9, 3: NoCmd}
	for action, want := range cases {
		if got := p.cmdOf(&openbw.EventBase{Cmd: action}); got != want {
			t.Errorf("action %d: got %d, want %d", action, got, want)
		}
	}
}

func TestSnapshotHasCreep(t *testing.T) {
	s := Snapshot{CreepW: 4, Creep: []byte{0b1010_0110, 0b0000_0001}}
	set := map[[2]int]bool{{1, 0}: true, {2, 0}: true, {1, 1}: true, {3, 1}: true, {0, 2}: true}
	for ty := range 4 {
		for tx := range 4 {
			if got := s.HasCreep(tx, ty); got != set[[2]int{tx, ty}] {
				t.Errorf("tile %d,%d: got %v", tx, ty, got)
			}
		}
	}
	for _, c := range [][2]int{{-1, 0}, {0, -1}, {4, 0}, {5, 1}, {0, 100}, {-4, 1}} {
		if s.HasCreep(c[0], c[1]) {
			t.Errorf("out of bounds %v has creep", c)
		}
	}
	if (Snapshot{CreepW: 4}).HasCreep(1, 0) {
		t.Error("a snapshot without creep bits has creep")
	}
	if (Snapshot{Creep: []byte{0xff}}).HasCreep(0, 0) {
		t.Error("a zero-width snapshot has creep")
	}
}

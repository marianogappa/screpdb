package unittags

import (
	"testing"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/rep/repcore"
)

// fr returns a frame whose Frame.Seconds() truncates to exactly sec. One frame
// is 42ms, so sec*1000/42 frames lands inside second sec (rounded to mid-second
// to avoid boundary truncation drift).
func fr(sec int) repcore.Frame { return repcore.Frame((sec*1000 + 500) / 42) }

func base(pid byte, sec int, typeID byte) *repcmd.Base {
	return &repcmd.Base{Frame: fr(sec), PlayerID: pid, Type: &repcmd.Type{ID: typeID}}
}

func sel(pid byte, sec int, tags ...uint16) *repcmd.SelectCmd {
	ut := make([]repcmd.UnitTag, len(tags))
	for i, t := range tags {
		ut[i] = repcmd.UnitTag(t)
	}
	return &repcmd.SelectCmd{Base: base(pid, sec, repcmd.TypeIDSelect), UnitTags: ut}
}

func selAdd(pid byte, sec int, tags ...uint16) *repcmd.SelectCmd {
	c := sel(pid, sec, tags...)
	c.Base.Type = &repcmd.Type{ID: repcmd.TypeIDSelectAdd}
	return c
}

func selRemove(pid byte, sec int, tags ...uint16) *repcmd.SelectCmd {
	c := sel(pid, sec, tags...)
	c.Base.Type = &repcmd.Type{ID: repcmd.TypeIDSelectRemove}
	return c
}

func hotkey(pid byte, sec int, htype string, group byte) *repcmd.HotkeyCmd {
	var id byte
	switch htype {
	case "Assign":
		id = repcmd.HotkeyTypeIDAssign
	case "Select":
		id = repcmd.HotkeyTypeIDSelect
	case "Add":
		id = repcmd.HotkeyTypeIDAdd
	}
	return &repcmd.HotkeyCmd{
		Base:       base(pid, sec, repcmd.TypeIDHotkey),
		HotkeyType: &repcmd.HotkeyType{Enum: repcore.Enum{Name: htype}, ID: id},
		Group:      group,
	}
}

func build(pid byte, sec int, name string, x, y uint16) *repcmd.BuildCmd {
	return &repcmd.BuildCmd{
		Base: base(pid, sec, repcmd.TypeIDBuild),
		Pos:  repcore.Point{X: x, Y: y},
		Unit: &repcmd.Unit{Enum: repcore.Enum{Name: name}},
	}
}

func train(pid byte, sec int, name string) *repcmd.TrainCmd {
	return &repcmd.TrainCmd{
		Base: base(pid, sec, repcmd.TypeIDTrain),
		Unit: &repcmd.Unit{Enum: repcore.Enum{Name: name}},
	}
}

func morph(pid byte, sec int, name string) *repcmd.TrainCmd {
	c := train(pid, sec, name)
	c.Base.Type = &repcmd.Type{ID: repcmd.TypeIDUnitMorph}
	return c
}

func replayOf(cmds ...repcmd.Cmd) *rep.Replay {
	return &rep.Replay{Commands: &rep.Commands{Cmds: cmds}}
}

func TestAnalyze_NilGuards(t *testing.T) {
	if got := Analyze(nil); got == nil || len(got.Players) != 0 {
		t.Errorf("Analyze(nil) should return empty non-nil evidence, got %+v", got)
	}
	if got := Analyze(&rep.Replay{}); got == nil || len(got.Players) != 0 {
		t.Errorf("Analyze(no commands) should return empty evidence, got %+v", got)
	}
}

func TestAnalyze_TrainAttribution(t *testing.T) {
	tests := []struct {
		name       string
		cmds       []repcmd.Cmd
		wantBldg   string
		wantTag    uint16
		wantUnits  int
		wantNoProd bool
	}{
		{
			name:      "single-select gateway trains zealot",
			cmds:      []repcmd.Cmd{sel(1, 10, 0xAA), train(1, 11, "Zealot")},
			wantBldg:  "Gateway",
			wantTag:   0xAA,
			wantUnits: 1,
		},
		{
			name:      "repeat trains from same tag accumulate",
			cmds:      []repcmd.Cmd{sel(1, 10, 0xAA), train(1, 11, "Dragoon"), train(1, 20, "Dragoon")},
			wantBldg:  "Gateway",
			wantTag:   0xAA,
			wantUnits: 2,
		},
		{
			name:      "SCV attributed to Command Center",
			cmds:      []repcmd.Cmd{sel(1, 5, 0x01), train(1, 6, "SCV")},
			wantBldg:  "Command Center",
			wantTag:   0x01,
			wantUnits: 1,
		},
		{
			// A multi-unit selection cannot name the single producing building.
			name:       "multi-select train records no producer",
			cmds:       []repcmd.Cmd{sel(1, 10, 0xAA, 0xBB), train(1, 11, "Zealot")},
			wantNoProd: true,
		},
		{
			// Train with no prior select at all: empty selection.
			name:       "no selection train records no producer",
			cmds:       []repcmd.Cmd{train(1, 11, "Zealot")},
			wantNoProd: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := Analyze(replayOf(tt.cmds...))
			pe := ev.Players[1]
			if pe == nil {
				t.Fatal("expected player 1 evidence")
			}
			if tt.wantNoProd {
				for bldg, tags := range pe.Producers {
					if len(tags) > 0 {
						t.Errorf("expected no producers, got %s: %+v", bldg, tags)
					}
				}
				return
			}
			p := pe.Producers[tt.wantBldg][tt.wantTag]
			if p == nil {
				t.Fatalf("expected producer %s tag %#x, got %+v", tt.wantBldg, tt.wantTag, pe.Producers)
			}
			if p.Units != tt.wantUnits {
				t.Errorf("units: got %d, want %d", p.Units, tt.wantUnits)
			}
		})
	}
}

func TestAnalyze_ZergLarvaMorphAttribution(t *testing.T) {
	// Larva morph selects larvae, not the hall. The hall is recovered from the
	// single-unit selection immediately preceding the larvae select.
	cmds := []repcmd.Cmd{
		sel(2, 10, 0x100), // tap the Hatchery (single) -> becomes prevSingle
		sel(2, 11, 0x200), // select larvae (this is single too, replaces prevSingle before... see below)
		morph(2, 12, "Zergling"),
	}
	ev := Analyze(replayOf(cmds...))
	pe := ev.Players[2]
	// When the larvae select is itself single, prevSingle after snap == the hall
	// tap (0x100), because snap runs on the larvae-select recording the prior
	// (hall) single. So the morph attributes to 0x100.
	p := pe.Producers[zergTownHall][0x100]
	if p == nil || p.Units != 1 {
		t.Fatalf("expected zerg town-hall producer 0x100, got %+v", pe.Producers[zergTownHall])
	}
}

func TestAnalyze_ZergLarvaMorphNoPrevSingle(t *testing.T) {
	// Multi-select immediately before the larvae select: prevSingle invalid,
	// so the morph cannot be attributed.
	cmds := []repcmd.Cmd{
		sel(2, 10, 0x100, 0x101), // multi -> prevSingleValid=false after snap
		sel(2, 11, 0x200),        // larvae
		morph(2, 12, "Drone"),
	}
	ev := Analyze(replayOf(cmds...))
	pe := ev.Players[2]
	if len(pe.Producers[zergTownHall]) != 0 {
		t.Errorf("expected no town-hall attribution without a prior single tap, got %+v", pe.Producers[zergTownHall])
	}
}

func TestAnalyze_NonLarvaMorphIgnored(t *testing.T) {
	// Lurker is not a larva-morph unit and is not in producerBuilding -> ignored.
	cmds := []repcmd.Cmd{sel(2, 10, 0x100), sel(2, 11, 0x200), morph(2, 12, "Lurker")}
	ev := Analyze(replayOf(cmds...))
	pe := ev.Players[2]
	for bldg, tags := range pe.Producers {
		if len(tags) > 0 {
			t.Errorf("non-larva morph should record no producer, got %s: %+v", bldg, tags)
		}
	}
}

func TestAnalyze_WorkerBuildsAndAddons(t *testing.T) {
	cmds := []repcmd.Cmd{
		sel(1, 10, 0xD1),                     // single worker
		build(1, 11, "Barracks", 20, 30),     // worker build recorded
		sel(1, 20, 0xF1),                     // single factory
		build(1, 21, "Machine Shop", 22, 30), // add-on proves Factory 0xF1 exists
		sel(1, 30, 0xD1, 0xD2),               // multi-select
		build(1, 31, "Supply Depot", 40, 40), // not a worker-build (multi)
	}
	ev := Analyze(replayOf(cmds...))
	pe := ev.Players[1]

	if len(pe.WorkerBuilds) != 2 {
		t.Fatalf("expected 2 single-worker builds, got %d: %+v", len(pe.WorkerBuilds), pe.WorkerBuilds)
	}
	wb := pe.WorkerBuilds[0]
	if wb.Building != "Barracks" || wb.Worker != 0xD1 || wb.X != 20 || wb.Y != 30 {
		t.Errorf("first worker build wrong: %+v", wb)
	}
	if !pe.Addons["Factory"][0xF1] {
		t.Errorf("Machine Shop should prove Factory 0xF1 exists, addons=%+v", pe.Addons)
	}
	// Builds map records all Build commands regardless of selection.
	if len(pe.Builds["Supply Depot"]) != 1 || len(pe.Builds["Barracks"]) != 1 {
		t.Errorf("Builds map missing entries: %+v", pe.Builds)
	}
}

func TestAnalyze_HotkeyGroupSelect(t *testing.T) {
	// Assign a group, then reselect via hotkey and train: production binds to
	// the hotkey-restored single tag.
	cmds := []repcmd.Cmd{
		sel(1, 10, 0x55),           // select gateway
		hotkey(1, 11, "Assign", 1), // bind group 1 = {0x55}
		sel(1, 20, 0x99),           // select something else
		hotkey(1, 21, "Select", 1), // restore group 1 -> {0x55}
		train(1, 22, "Zealot"),
	}
	ev := Analyze(replayOf(cmds...))
	pe := ev.Players[1]
	if p := pe.Producers["Gateway"][0x55]; p == nil || p.Units != 1 {
		t.Fatalf("hotkey-restored gateway should produce, got %+v", pe.Producers["Gateway"])
	}
}

func TestAnalyze_SelectAddRemove(t *testing.T) {
	// Add then remove leaves a single tag; a train then attributes to it.
	cmds := []repcmd.Cmd{
		sel(1, 10, 0x11),       // {0x11}
		selAdd(1, 11, 0x22),    // {0x11, 0x22}
		selRemove(1, 12, 0x11), // {0x22}
		train(1, 13, "Marine"), // single -> Barracks 0x22
	}
	ev := Analyze(replayOf(cmds...))
	pe := ev.Players[1]
	if p := pe.Producers["Barracks"][0x22]; p == nil || p.Units != 1 {
		t.Fatalf("expected Barracks 0x22 after add/remove, got %+v", pe.Producers["Barracks"])
	}
	if _, dup := pe.Producers["Barracks"][0x11]; dup {
		t.Errorf("removed tag 0x11 must not be a producer")
	}
}

func TestAnalyze_HotkeyAddUnion(t *testing.T) {
	// Hotkey "Add" unions the group into the current selection; the result is
	// multi so a train records no single producer.
	cmds := []repcmd.Cmd{
		sel(1, 10, 0xA1),
		hotkey(1, 11, "Assign", 2), // group 2 = {0xA1}
		sel(1, 20, 0xB2),           // {0xB2}
		hotkey(1, 21, "Add", 2),    // {0xB2, 0xA1}
		train(1, 22, "Dragoon"),
	}
	ev := Analyze(replayOf(cmds...))
	pe := ev.Players[1]
	if len(pe.Producers["Gateway"]) != 0 {
		t.Errorf("multi selection after hotkey Add must not attribute a producer, got %+v", pe.Producers["Gateway"])
	}
}

func TestAnalyze_RecycledTagDistinctByBuilding(t *testing.T) {
	// BW recycles unit tag 0x30: it names a Gateway early, then a Stargate later.
	// Each production must land under its own building type (recording keys on
	// building type, so the incarnations are kept distinct).
	cmds := []repcmd.Cmd{
		sel(1, 10, 0x30), train(1, 11, "Zealot"),
		sel(1, 200, 0x30), train(1, 201, "Corsair"),
	}
	ev := Analyze(replayOf(cmds...))
	pe := ev.Players[1]
	if p := pe.Producers["Gateway"][0x30]; p == nil || p.Units != 1 {
		t.Errorf("Gateway incarnation of 0x30 missing: %+v", pe.Producers["Gateway"])
	}
	if p := pe.Producers["Stargate"][0x30]; p == nil || p.Units != 1 {
		t.Errorf("Stargate incarnation of 0x30 missing: %+v", pe.Producers["Stargate"])
	}
}

func TestProducedPlacements(t *testing.T) {
	// Two Command Centers: the start hall (produces before any Build -> unmatched)
	// and an expansion built at (60,40) that produces after -> matched. The
	// expansion placement must appear; the unmatched tag contributes no tile.
	cmds := []repcmd.Cmd{
		sel(1, 5, 0x01), train(1, 6, "SCV"), // start hall, no Build
		build(1, 100, "Command Center", 60, 40),
		sel(1, 110, 0x02), train(1, 111, "SCV"), // expansion produces after build
	}
	ev := Analyze(replayOf(cmds...))
	pe := ev.Players[1]
	placements := pe.ProducedPlacements()
	set := placements["Command Center"]
	if !set[[2]int{60, 40}] {
		t.Errorf("expansion placement (60,40) should be a produced placement, got %+v", set)
	}
	if len(set) != 1 {
		t.Errorf("only the matched expansion should yield a placement, got %+v", set)
	}
}

func TestProducedPlacements_NoBuildsNoEntry(t *testing.T) {
	// Producer whose building type has no Build commands -> no placement entry.
	cmds := []repcmd.Cmd{sel(1, 5, 0x09), train(1, 6, "Marine")}
	ev := Analyze(replayOf(cmds...))
	pe := ev.Players[1]
	if _, ok := pe.ProducedPlacements()["Barracks"]; ok {
		t.Errorf("no Build for Barracks => no ProducedPlacements entry")
	}
}

func TestAnalyze_ProductionSignals_AnchoredVsFallback(t *testing.T) {
	cmds := []repcmd.Cmd{
		sel(1, 5, 0x01), train(1, 6, "SCV"), // start hall, unmatched -> Anchored:false
		build(1, 100, "Command Center", 60, 40),
		sel(1, 110, 0x02), train(1, 111, "SCV"), // matched -> Anchored:true @ (60,40)
	}
	ev := Analyze(replayOf(cmds...))
	pe := ev.Players[1]
	var anchored, unanchored int
	for _, s := range pe.ProductionSignals {
		if s.Anchored {
			anchored++
			if s.X != 60 || s.Y != 40 {
				t.Errorf("anchored signal at wrong tile: %+v", s)
			}
		} else {
			unanchored++
			if s.X != 0 || s.Y != 0 {
				t.Errorf("unanchored signal must have zero coords, got %+v", s)
			}
		}
	}
	if anchored != 1 || unanchored != 1 {
		t.Errorf("expected 1 anchored + 1 unanchored signal, got %d/%d", anchored, unanchored)
	}
	// Signals must be sorted by second.
	for i := 1; i < len(pe.ProductionSignals); i++ {
		if pe.ProductionSignals[i-1].Sec > pe.ProductionSignals[i].Sec {
			t.Errorf("signals not sorted by sec: %+v", pe.ProductionSignals)
		}
	}
}

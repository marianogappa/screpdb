package commands

import (
	"testing"

	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/rep/repcore"
)

func TestProcessCommandDispatchesToHandler(t *testing.T) {
	r := NewCommandRegistry()
	base := baseCmd(repcmd.TypeIDBuild, "Build", 1, 24)
	cmd := &repcmd.BuildCmd{
		Base: base,
		Pos:  repcore.Point{X: 5, Y: 6},
		Unit: &repcmd.Unit{Enum: repcore.Enum{Name: "Barracks"}, ID: 106},
	}

	got := r.ProcessCommand(cmd, 0)

	if got == nil {
		t.Fatal("ProcessCommand returned nil for a handled Build command")
	}
	if got.ActionType != "Build" {
		t.Errorf("ActionType: want Build, got %q", got.ActionType)
	}
	if got.UnitType == nil || *got.UnitType != "Barracks" {
		t.Errorf("UnitType: want Barracks, got %v", got.UnitType)
	}
}

func TestProcessCommandSetsSecondsFromFrame(t *testing.T) {
	r := NewCommandRegistry()
	// StarCraft runs at ~23.81 frames/sec; frame 240 ≈ 10s.
	base := baseCmd(repcmd.TypeIDTrain, "Train", 1, 240)
	cmd := &repcmd.TrainCmd{Base: base, Unit: &repcmd.Unit{Enum: repcore.Enum{Name: "Marine"}}}

	got := r.ProcessCommand(cmd, 0)

	if got == nil {
		t.Fatal("ProcessCommand returned nil")
	}
	wantSec := int(repcore.Frame(240).Seconds())
	if got.SecondsFromGameStart != wantSec {
		t.Errorf("SecondsFromGameStart: want %d, got %d", wantSec, got.SecondsFromGameStart)
	}
}

func TestProcessCommandFallsBackToGeneralHandler(t *testing.T) {
	r := NewCommandRegistry()
	// A type ID with no registered handler and not in the ignore list.
	base := baseCmd(0xCC, "SomeUnhandled", 1, 0)
	cmd := &repcmd.GeneralCmd{Base: base, Data: []byte{0x01, 0x02}}

	got := r.ProcessCommand(cmd, 0)

	if got == nil {
		t.Fatal("unhandled effective command should fall back to general handler, got nil")
	}
	if got.ActionType != "SomeUnhandled" {
		t.Errorf("ActionType: want SomeUnhandled, got %q", got.ActionType)
	}
	if got.GeneralData == nil || *got.GeneralData != "0102" {
		t.Errorf("GeneralData: want 0102, got %v", got.GeneralData)
	}
}

func TestProcessCommandIgnoresIgnoredType(t *testing.T) {
	r := NewCommandRegistry()
	base := baseCmd(repcmd.TypeIDSelect, "Select", 1, 0)
	cmd := &repcmd.GeneralCmd{Base: base}

	if got := r.ProcessCommand(cmd, 0); got != nil {
		t.Errorf("ignored command type should return nil, got %+v", got)
	}
}

func TestProcessCommandIgnoresIneffective(t *testing.T) {
	r := NewCommandRegistry()
	base := baseCmd(repcmd.TypeIDBuild, "Build", 1, 0)
	base.IneffKind = repcore.IneffKindUnitQueueOverflow
	cmd := &repcmd.BuildCmd{Base: base, Pos: repcore.Point{X: 1, Y: 1}}

	if got := r.ProcessCommand(cmd, 0); got != nil {
		t.Errorf("ineffective command should return nil regardless of type, got %+v", got)
	}
}

func TestGetSupportedCommandTypesExcludesIgnored(t *testing.T) {
	r := NewCommandRegistry()
	types := r.GetSupportedCommandTypes()

	if len(types) == 0 {
		t.Fatal("GetSupportedCommandTypes returned nothing")
	}

	seen := map[byte]bool{}
	for _, tp := range types {
		if ignoredCommandTypes[tp] {
			t.Errorf("supported types must not include ignored type %d", tp)
		}
		if seen[tp] {
			t.Errorf("duplicate type %d in supported types", tp)
		}
		seen[tp] = true
	}

	if !seen[repcmd.TypeIDBuild] {
		t.Error("expected Build to be a supported command type")
	}
	if !seen[repcmd.VirtualTypeIDLand] {
		t.Error("expected virtual Land to be a supported command type")
	}
}

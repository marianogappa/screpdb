package engine

import (
	"testing"

	openbw "github.com/marianogappa/openbw-go"

	"github.com/marianogappa/openbw-go/bwgame"
)

func TestPassRecordsHotkeyGroups(t *testing.T) {
	d := passReplay(t, "SomaJyJ.rep")
	if len(d.Hotkeys) == 0 {
		t.Fatal("no hotkey assigns recorded")
	}
	byType := map[uint16]int{}
	for i, h := range d.Hotkeys {
		if h.Count == 0 || h.Count > 12 {
			t.Fatalf("hotkey %d: count %d", i, h.Count)
		}
		if i > 0 && h.Frame < d.Hotkeys[i-1].Frame {
			t.Fatalf("hotkeys out of order at %d", i)
		}
		byType[h.MainType]++
		if testing.Verbose() && i < 400 {
			t.Logf("%5ds slot %d group %d: %s x%d", h.Frame*42/1000, h.Owner, h.Group, openbw.UnitType(h.MainType).Name(), h.Count)
		}
	}
	if byType[uint16(bwgame.UnitTypesZergZergling)]+byType[uint16(bwgame.UnitTypesZergMutalisk)]+byType[uint16(bwgame.UnitTypesZergHydralisk)] == 0 {
		t.Fatalf("a ZvT with no Zerg army group: %v", byType)
	}
}

func TestSummarizeGroupPrefersValue(t *testing.T) {
	if CostOf(uint16(bwgame.UnitTypesZergLurker)).Value()*3 <= CostOf(uint16(bwgame.UnitTypesZergZergling)).Value()*8 {
		t.Fatal("three Lurkers must outweigh eight Zerglings")
	}
}

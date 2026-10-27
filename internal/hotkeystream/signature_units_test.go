package hotkeystream

import (
	"testing"
)

func TestSignatureSplitsUnitRunsByTheEngineUnitType(t *testing.T) {
	ling, muta := uint16(0x25), uint16(0x2B)
	game := func() []Event {
		var events []Event
		events = append(events, Event{Sec: 180, Type: TypeAssignUnits, Group: 1, Count: 12, Unit: ling})
		for s := int32(181); s < 420; s += 5 {
			events = append(events, Event{Sec: s, Type: TypeSelect, Group: 1})
		}
		events = append(events, Event{Sec: 420, Type: TypeAssignUnits, Group: 1, Count: 9, Unit: muta})
		for s := int32(421); s < 720; s += 5 {
			events = append(events, Event{Sec: s, Type: TypeSelect, Group: 1})
		}
		return events
	}
	sig := ComputeSignature("p", "Zerg", [][]Event{game(), game(), game()})
	if len(sig.Keys) != 1 {
		t.Fatalf("keys %+v", sig.Keys)
	}
	runs := sig.Keys[0].Runs
	if len(runs) != 2 || runs[0].Unit != "Zergling" || runs[1].Unit != "Mutalisk" || runs[0].Category != CategoryUnits || runs[0].MedianCount != 12 || runs[1].MedianCount != 9 {
		t.Fatalf("runs %+v", runs)
	}

	plain := game()
	for i := range plain {
		plain[i].Unit = 0
	}
	if runs := ComputeSignature("p", "Zerg", [][]Event{plain, plain, plain}).Keys[0].Runs; len(runs) != 1 || runs[0].Unit != "" {
		t.Fatalf("without the engine, one units run: %+v", runs)
	}
}

package hotkeystream

import "testing"

func layoutSig(hallKey, prodKey int) *Signature {
	return &Signature{Race: "Protoss", Keys: []KeySignature{
		{Key: 1, Runs: []KeyRun{{StartMin: 0, EndMin: 14, Category: CategoryUnits + unitSep + "Zealot", Presses: 300}}},
		{Key: hallKey, Runs: []KeyRun{{StartMin: 0, EndMin: 5, Category: CategoryHall, Presses: 120}}},
		{Key: prodKey, Runs: []KeyRun{{StartMin: 2, EndMin: 14, Category: CategoryProd, Presses: 200}}},
	}}
}

func TestLayoutSimilarity(t *testing.T) {
	same := LayoutSimilarity(layoutSig(5, 4), layoutSig(5, 4))
	swapped := LayoutSimilarity(layoutSig(5, 4), layoutSig(2, 3))
	if same < 0.999 {
		t.Fatalf("identical layouts: similarity %.3f, want 1", same)
	}
	if swapped >= same || swapped <= 0 {
		t.Fatalf("different hall/prod keys: similarity %.3f, want between 0 and %.3f", swapped, same)
	}
	if got := LayoutSimilarity(nil, layoutSig(5, 4)); got != 0 {
		t.Fatalf("empty signature: similarity %.3f, want 0", got)
	}
}

func TestGameRates(t *testing.T) {
	events := []Event{
		{Sec: 10, Type: TypeAssignUnits, Group: 1},
		{Sec: 20, Type: TypeSelect, Group: 1},
		{Sec: 30, Type: TypeSelect, Group: 1},
		{Sec: 40, Type: TypeAssignBuilding, Group: 4},
		{Sec: 50, Type: TypeSelect, Group: 12},
		{Sec: 800, Type: TypeSelect, Group: 1},
	}
	got, ok := GameRates(events, 1200)
	if !ok || got.PressesPerMinute != 2.0/12 || got.RebindsPerMinute != 1.0/12 {
		t.Fatalf("rates %+v ok=%v, want 2 presses and 1 rebind over 12 minutes", got, ok)
	}
	short, _ := GameRates(events[:3], 60)
	if short.PressesPerMinute != 2 || short.RebindsPerMinute != 1 {
		t.Fatalf("a one-minute game's rates %+v, want per its own minute", short)
	}
	if _, ok := GameRates(events, 0); ok {
		t.Fatal("a game with no length has no rates")
	}
	if m := MedianRates([]Rates{{1, 1}, {3, 9}, {2, 4}}); m.PressesPerMinute != 2 || m.RebindsPerMinute != 4 {
		t.Fatalf("median %+v", m)
	}
}

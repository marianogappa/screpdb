package gamerules

import "testing"

func TestCompositionSpellsAreUniqueAndLookupable(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range CompositionSpells {
		if s.Order == "" || s.Unit == "" || s.Spell == "" {
			t.Fatalf("incomplete spell entry: %+v", s)
		}
		if seen[s.Order] {
			t.Fatalf("duplicate order %q", s.Order)
		}
		seen[s.Order] = true
		if !IsCompositionSpellOrder(s.Order) {
			t.Fatalf("IsCompositionSpellOrder(%q) = false", s.Order)
		}
	}
	if IsCompositionSpellOrder("CastScannerSweep") {
		t.Fatal("comsat scan must not be a composition spell")
	}
	if len(CompositionSpells) != 25 {
		t.Fatalf("expected 25 spells, got %d", len(CompositionSpells))
	}
}

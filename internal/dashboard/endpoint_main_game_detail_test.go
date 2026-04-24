package dashboard

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

// TestOverlayBaseMetas_StampsNaturalOfClock ensures natural expansions in the
// layout carry a NaturalOfClock pointer derived from the start base's
// NaturalExpansion name pointer. This is what lets lookupOverlayBase
// distinguish a natural from a coincident expa at the same clock.
func TestOverlayBaseMetas_StampsNaturalOfClock(t *testing.T) {
	layout := &models.MapContextLayout{
		Bases: []models.MapContextBase{
			{Name: "start-a", Kind: "start", Clock: 11, NaturalExpansion: "nat-a"},
			{Name: "nat-a", Kind: "expa", Clock: 9},
			{Name: "other-expa-at-9", Kind: "expa", Clock: 9},
			{Name: "start-b", Kind: "start", Clock: 5, NaturalExpansion: "nat-b"},
			{Name: "nat-b", Kind: "expa", Clock: 7},
		},
	}
	metas := overlayBaseMetasFromLayout(layout)
	if len(metas) != len(layout.Bases) {
		t.Fatalf("expected %d metas, got %d", len(layout.Bases), len(metas))
	}

	byName := map[string]overlayBaseMeta{}
	for _, meta := range metas {
		byName[meta.Base.Name] = meta
	}

	natA := byName["nat-a"]
	if natA.Base.NaturalOfClock == nil {
		t.Fatalf("nat-a: expected NaturalOfClock to be set (start-a is at 11), got nil")
	}
	if *natA.Base.NaturalOfClock != 11 {
		t.Fatalf("nat-a: expected NaturalOfClock=11, got %d", *natA.Base.NaturalOfClock)
	}

	other := byName["other-expa-at-9"]
	if other.Base.NaturalOfClock != nil {
		t.Fatalf("other-expa-at-9: expected nil NaturalOfClock (not a natural), got %d", *other.Base.NaturalOfClock)
	}

	natB := byName["nat-b"]
	if natB.Base.NaturalOfClock == nil || *natB.Base.NaturalOfClock != 5 {
		t.Fatalf("nat-b: expected NaturalOfClock=5, got %v", natB.Base.NaturalOfClock)
	}
}

// TestLookupOverlayBase_DisambiguatesNaturalVsExpaAtSameClock is the
// regression test for the primary natural-misclassification bug. Previously
// a natural and an expa sharing the same clock collapsed to the same lookup
// key and the painted polygon depended on iteration order.
func TestLookupOverlayBase_DisambiguatesNaturalVsExpaAtSameClock(t *testing.T) {
	layout := &models.MapContextLayout{
		Bases: []models.MapContextBase{
			{Name: "start-a", Kind: "start", Clock: 11, NaturalExpansion: "nat-a"},
			// Natural and plain expansion share clock 9.
			{Name: "nat-a", Kind: "expa", Clock: 9},
			{Name: "other-expa-at-9", Kind: "expa", Clock: 9},
		},
	}
	metas := overlayBaseMetasFromLayout(layout)

	naturalType := "natural"
	expansionType := "expansion"
	clock := int64(9)
	startAClock := int64(11)

	// Natural of start-a (at 11) at clock 9 → must select "nat-a".
	got, ok := lookupOverlayBase(metas, &naturalType, &clock, &startAClock)
	if !ok {
		t.Fatalf("natural lookup failed")
	}
	if got.Name != "nat-a" {
		t.Fatalf("expected nat-a, got %q", got.Name)
	}

	// Expansion at clock 9 with no natural_of → must select "other-expa-at-9".
	got, ok = lookupOverlayBase(metas, &expansionType, &clock, nil)
	if !ok {
		t.Fatalf("expansion lookup failed")
	}
	if got.Name != "other-expa-at-9" {
		t.Fatalf("expected other-expa-at-9, got %q", got.Name)
	}
}

// TestBaseKeyForEvent_NaturalIncludesOwnerClock verifies ownership keys
// disambiguate naturals belonging to different players. Otherwise
// applyOwnershipTransition would overwrite one player's natural ownership
// with another's whenever they land on the same dial position.
func TestBaseKeyForEvent_NaturalIncludesOwnerClock(t *testing.T) {
	clock9 := int64(9)
	startA := int64(11)
	startB := int64(5)

	eventA := &workflowGameEvent{Base: &workflowGameEventBase{Kind: "natural", Clock: clock9, NaturalOfClock: &startA}}
	eventB := &workflowGameEvent{Base: &workflowGameEventBase{Kind: "natural", Clock: clock9, NaturalOfClock: &startB}}

	if baseKeyForEvent(eventA) == baseKeyForEvent(eventB) {
		t.Fatalf("expected distinct keys for naturals of different players, got %q == %q",
			baseKeyForEvent(eventA), baseKeyForEvent(eventB))
	}
}

// TestBaseLabel_CenterBase covers the label rendering for scmapanalyzer's
// Clock=0 marker. The templated "at 0" / "an expa near 0" strings read
// wrong; the UI expects the literal "center base".
func TestBaseLabel_CenterBase(t *testing.T) {
	starting := "starting"
	expansion := "expansion"
	natural := "natural"
	zero := int64(0)
	six := int64(6)

	if got := baseLabel(&starting, &zero, nil); got != "center base" {
		t.Fatalf("starting center: expected \"center base\", got %q", got)
	}
	if got := baseLabel(&expansion, &zero, nil); got != "center base" {
		t.Fatalf("expansion center: expected \"center base\", got %q", got)
	}
	// Player at 6's natural happens to be the center base.
	if got := baseLabel(&natural, &zero, &six); got != "6's natural (center base)" {
		t.Fatalf("natural center (owner at 6): expected \"6's natural (center base)\", got %q", got)
	}
	// Both the natural AND its owner are center (rare — self-referential).
	if got := baseLabel(&natural, &zero, &zero); got != "center base" {
		t.Fatalf("natural center (owner also center): expected \"center base\", got %q", got)
	}
}

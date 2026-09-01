package dashboard

import (
	"testing"

	db "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
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
// Clock=0 marker. The templated "0" / "an expansion near 0" strings read
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

// TestFuzzyZergSchemaFromLabel covers the label → simplified-Zerg schema
// derivation the fuzzy opener chart is built from, including that every
// measured golden-band label resolves to a schema whose defining-building row
// actually renders the band (subject present in the schema).
func TestFuzzyZergSchemaFromLabel(t *testing.T) {
	cases := []struct {
		label  string
		ok     bool
		schema zergBOEventSchema
	}{
		{"~11 Hatch", true, zergBOEventSchema{Drones: 7, HasOverlord: true, HasHatchery: true}},
		{"~5 Hatch", true, zergBOEventSchema{Drones: 1, HasHatchery: true}},
		{"~10 Overpool", true, zergBOEventSchema{Drones: 6, HasOverlord: true, HasPool: true}},
		{"~12 Pool", true, zergBOEventSchema{Drones: 8, HasPool: true}},
		{"Zerg opening (approximate)", false, zergBOEventSchema{}},
		{"~3 Hatch", false, zergBOEventSchema{}},
	}
	for _, c := range cases {
		got, ok := fuzzyZergSchemaFromLabel("bo_z_fuzzy", c.label)
		if ok != c.ok || got != c.schema {
			t.Fatalf("fuzzyZergSchemaFromLabel(%q) = %+v/%v, want %+v/%v", c.label, got, ok, c.schema, c.ok)
		}
	}
	if _, ok := fuzzyZergSchemaFromLabel("bo_9_pool", "~11 Hatch"); ok {
		t.Fatalf("non-fuzzy feature key must not resolve a schema")
	}

	for _, label := range []string{"~11 Hatch", "~10 Hatch", "~10 Overpool", "~5 Hatch"} {
		expert := markers.FuzzyZergExpertEvents(label)
		if len(expert) == 0 {
			t.Fatalf("measured label %q has no golden band", label)
		}
		schema, ok := fuzzyZergSchemaFromLabel("bo_z_fuzzy", label)
		if !ok {
			t.Fatalf("measured label %q does not parse", label)
		}
		subject := expert[0].Match.Subject
		renders := (subject == models.GeneralUnitSpawningPool && schema.HasPool) ||
			(subject == models.GeneralUnitHatchery && schema.HasHatchery)
		if !renders {
			t.Fatalf("label %q band subject %q is not rendered by schema %+v", label, subject, schema)
		}
	}
}

// TestBuildZergBOEvents_FuzzyBand exercises the render path the fuzzy opener
// chart uses: a label-derived schema plus the per-label golden band, over raw
// command timings. Drone/Overlord rows stay actual-only; the defining building
// carries the band and the in-band verdict.
func TestBuildZergBOEvents_FuzzyBand(t *testing.T) {
	label := "~10 Overpool"
	schema, ok := fuzzyZergSchemaFromLabel("bo_z_fuzzy", label)
	if !ok {
		t.Fatalf("label %q should parse", label)
	}
	expert := markers.FuzzyZergExpertEvents(label)
	if len(expert) == 0 {
		t.Fatalf("label %q should carry a band", label)
	}
	poolSec := expert[0].TargetSecond
	overlord := 55
	events := buildZergBOEvents(schema, expert, db.EarlyZergTimingsRow{
		DroneMorphSecs:   []int{10, 18, 26, 34, 42, 50},
		FirstOverlordSec: &overlord,
		FirstPoolSec:     &poolSec,
	})
	if len(events) != schema.Drones+2 {
		t.Fatalf("expected %d events, got %d", schema.Drones+2, len(events))
	}
	for _, ev := range events[:schema.Drones] {
		if !ev.NoExpert || !ev.Found {
			t.Fatalf("drone row should be found and actual-only: %+v", ev)
		}
	}
	pool := events[len(events)-1]
	if pool.Key != "Spawning Pool" || pool.NoExpert || !pool.Found {
		t.Fatalf("pool row should carry the band: %+v", pool)
	}
	if pool.TargetSecond != int64(expert[0].TargetSecond) || !pool.WithinTolerance {
		t.Fatalf("pool at the target second should be in band: %+v", pool)
	}
}

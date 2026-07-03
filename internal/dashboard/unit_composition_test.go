package dashboard

import (
	"testing"

	db "github.com/marianogappa/screpdb/internal/dashboard/db"
)

func TestPhaseForSecond_BothBoundariesSet(t *testing.T) {
	cases := []struct {
		name               string
		second, early, mid int
		want               string
	}{
		{"early", 100, 400, 800, "early"},
		{"on early boundary -> mid", 400, 400, 800, "mid"},
		{"mid", 600, 400, 800, "mid"},
		{"on mid boundary -> late", 800, 400, 800, "late"},
		{"late", 1000, 400, 800, "late"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := phaseForSecond(tc.second, tc.early, tc.mid); got != tc.want {
				t.Fatalf("phaseForSecond(%d, %d, %d): want %s, got %s",
					tc.second, tc.early, tc.mid, tc.want, got)
			}
		})
	}
}

func TestPhaseForSecond_NoBoundaries_AllEarly(t *testing.T) {
	for _, s := range []int{0, 60, 600, 6000} {
		if got := phaseForSecond(s, 0, 0); got != "early" {
			t.Fatalf("phaseForSecond(%d, 0, 0): want early, got %s", s, got)
		}
	}
}

func TestPhaseForSecond_OnlyMidEnd_SplitsAtMidEnd(t *testing.T) {
	// Regression test: the Protoss-fast-Carrier case where neither player
	// reached the tier-2 signals (Muta/Lurker/Wraith/SiegeArmed/
	// DragoonArmed/Reaver/DT). Carrier sets midEnd at 1000 but earlyEnd
	// stays 0. Carriers must NOT bin to "early" — they must bin to
	// "late". Pre-bug: phaseForSecond unconditionally returned "early"
	// when earlyEnd <= 0.
	cases := []struct {
		name   string
		second int
		want   string
	}{
		{"pre-midEnd is early", 500, "early"},
		{"first carrier (== midEnd) is late", 1000, "late"},
		{"post-midEnd is late", 1200, "late"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := phaseForSecond(tc.second, 0, 1000); got != tc.want {
				t.Fatalf("second=%d, earlyEnd=0, midEnd=1000: want %s, got %s",
					tc.second, tc.want, got)
			}
		})
	}
}

func TestPhaseForSecond_OnlyEarlyEnd_NoLatePhase(t *testing.T) {
	// midEnd = 0, earlyEnd > 0 → tier-2 fired but tier-3 didn't. Late
	// phase should never appear.
	if got := phaseForSecond(100, 400, 0); got != "early" {
		t.Fatalf("pre-earlyEnd: want early, got %s", got)
	}
	if got := phaseForSecond(400, 400, 0); got != "mid" {
		t.Fatalf("on earlyEnd: want mid, got %s", got)
	}
	if got := phaseForSecond(9999, 400, 0); got != "mid" {
		t.Fatalf("far past earlyEnd with midEnd=0: want mid (no late phase), got %s", got)
	}
}

func TestComputeComposition_CarriersGoToLateWhenOnlyMidEndSet(t *testing.T) {
	// End-to-end regression of the user-reported bug: Protoss player
	// produces Carriers at second 1000. Boundaries: earlyEnd=0,
	// midEnd=1000 (set by carrierSec via phases.Compute). Carriers must
	// surface in the "late" pill, not "early".
	playerID := int64(7)
	rows := []db.UnitProductionOrCastRow{
		{
			PlayerID:             playerID,
			ActionType:           "Train",
			UnitType:             ptr("Carrier"),
			SecondsFromGameStart: 1000,
		},
		{
			PlayerID:             playerID,
			ActionType:           "Train",
			UnitType:             ptr("Carrier"),
			SecondsFromGameStart: 1100,
		},
	}
	got := computeCompositionForReplay(rows, db.PhaseBoundaries{EarlyEndsAtSecond: 0, MidEndsAtSecond: 1000})
	if len(got) != 1 {
		t.Fatalf("want 1 row (single phase), got %d: %+v", len(got), got)
	}
	if got[0].Phase != "late" {
		t.Fatalf("Carriers should bin to late, got phase=%q (full row: %+v)", got[0].Phase, got[0])
	}
	// Carrier is a regular attacking unit — appears in the slot strip
	// (units list) with its real count, not in the spells strip. Reaver
	// behaves the same way (see test below).
	if len(got[0].Spells) != 0 {
		t.Fatalf("Carrier must not appear in spells strip, got %+v", got[0].Spells)
	}
	if len(got[0].Units) != 1 || got[0].Units[0].Name != "Carrier" || got[0].Units[0].Count != 2 {
		t.Fatalf("Carrier should be in units with count 2, got %+v", got[0].Units)
	}
}

func TestComputeComposition_ReaverIsRegularAttackingUnit(t *testing.T) {
	// Reaver was previously in the signature-non-caster set (right
	// strip, count discarded). Per user feedback, Reavers are produced
	// in meaningful numbers (2-6) and read as primary army composition,
	// so they belong in the slot strip on the left.
	playerID := int64(3)
	rows := []db.UnitProductionOrCastRow{
		{PlayerID: playerID, ActionType: "Train", UnitType: ptr("Reaver"), SecondsFromGameStart: 700},
		{PlayerID: playerID, ActionType: "Train", UnitType: ptr("Reaver"), SecondsFromGameStart: 800},
		{PlayerID: playerID, ActionType: "Train", UnitType: ptr("Reaver"), SecondsFromGameStart: 900},
	}
	got := computeCompositionForReplay(rows, db.PhaseBoundaries{EarlyEndsAtSecond: 600, MidEndsAtSecond: 0})
	if len(got) != 1 {
		t.Fatalf("want 1 row, got %d: %+v", len(got), got)
	}
	if len(got[0].Spells) != 0 {
		t.Fatalf("Reaver must not appear in spells strip, got %+v", got[0].Spells)
	}
	if len(got[0].Units) != 1 || got[0].Units[0].Name != "Reaver" || got[0].Units[0].Count != 3 {
		t.Fatalf("Reaver should be in units with count 3, got %+v", got[0].Units)
	}
}

func TestComputeComposition_SpellsByCastNotUnit(t *testing.T) {
	// A Science Vessel that casts both Irradiate and EMP yields two
	// distinct spell entries sharing the same unit icon. The Vessel never
	// appears in the Units histogram. Casts arrive as 'Targeted Order'
	// commands (mirroring the SQL). A spell-only phase still needs a
	// non-zero productionCount, so the Vessel build is present too.
	playerID := int64(5)
	rows := []db.UnitProductionOrCastRow{
		{PlayerID: playerID, ActionType: "Train", UnitType: ptr("Marine"), SecondsFromGameStart: 100},
		{PlayerID: playerID, ActionType: "Train", UnitType: ptr("Science Vessel"), SecondsFromGameStart: 110},
		{PlayerID: playerID, ActionType: "Targeted Order", OrderName: ptr("CastIrradiate"), SecondsFromGameStart: 120},
		{PlayerID: playerID, ActionType: "Targeted Order", OrderName: ptr("CastEMPShockwave"), SecondsFromGameStart: 130},
		{PlayerID: playerID, ActionType: "Targeted Order", OrderName: ptr("CastIrradiate"), SecondsFromGameStart: 140},
	}
	got := computeCompositionForReplay(rows, db.PhaseBoundaries{EarlyEndsAtSecond: 0, MidEndsAtSecond: 0})
	if len(got) != 1 {
		t.Fatalf("want 1 row, got %d: %+v", len(got), got)
	}
	if len(got[0].Units) != 1 || got[0].Units[0].Name != "Marine" {
		t.Fatalf("only Marine should be in units (Vessel excluded), got %+v", got[0].Units)
	}
	if len(got[0].Spells) != 2 {
		t.Fatalf("want 2 distinct spells (Irradiate deduped), got %+v", got[0].Spells)
	}
	for _, s := range got[0].Spells {
		if s.Unit != "Science Vessel" {
			t.Fatalf("spell %q should carry the Science Vessel icon, got unit %q", s.Spell, s.Unit)
		}
	}
}

func TestComputeComposition_BattlecruiserInHistogramPlusYamato(t *testing.T) {
	// Battlecruiser is primarily an attacker: it always counts in the
	// Units histogram. A Yamato cast ('Targeted Order' / FireYamatoGun —
	// not a "Cast*" order) additionally surfaces it under Spells, so the
	// BC appears in both places.
	playerID := int64(9)
	rows := []db.UnitProductionOrCastRow{
		{PlayerID: playerID, ActionType: "Train", UnitType: ptr("Battlecruiser"), SecondsFromGameStart: 600},
		{PlayerID: playerID, ActionType: "Train", UnitType: ptr("Battlecruiser"), SecondsFromGameStart: 610},
		{PlayerID: playerID, ActionType: "Targeted Order", OrderName: ptr("FireYamatoGun"), SecondsFromGameStart: 620},
	}
	got := computeCompositionForReplay(rows, db.PhaseBoundaries{})
	if len(got) != 1 {
		t.Fatalf("want 1 row, got %d: %+v", len(got), got)
	}
	if len(got[0].Units) != 1 || got[0].Units[0].Name != "Battlecruiser" || got[0].Units[0].Count != 2 {
		t.Fatalf("Battlecruiser should be in units with count 2, got %+v", got[0].Units)
	}
	if len(got[0].Spells) != 1 || got[0].Spells[0].Spell != "Yamato Gun" || got[0].Spells[0].Unit != "Battlecruiser" {
		t.Fatalf("Yamato cast should also surface as a Yamato Gun spell, got %+v", got[0].Spells)
	}
}

func TestComputeComposition_VultureMineIsACast(t *testing.T) {
	// Spider Mine is a non-"Cast*" ability (PlaceMine / VultureMine). The
	// Vulture stays in the histogram as an attacker and the mine surfaces
	// under Spells — proving the SQL/map fix isn't Yamato-specific.
	playerID := int64(11)
	rows := []db.UnitProductionOrCastRow{
		{PlayerID: playerID, ActionType: "Train", UnitType: ptr("Vulture"), SecondsFromGameStart: 200},
		{PlayerID: playerID, ActionType: "Targeted Order", OrderName: ptr("PlaceMine"), SecondsFromGameStart: 210},
		{PlayerID: playerID, ActionType: "Targeted Order", OrderName: ptr("VultureMine"), SecondsFromGameStart: 220},
	}
	got := computeCompositionForReplay(rows, db.PhaseBoundaries{})
	if len(got) != 1 || len(got[0].Units) != 1 || got[0].Units[0].Name != "Vulture" {
		t.Fatalf("Vulture should be in the histogram, got %+v", got)
	}
	if len(got[0].Spells) != 1 || got[0].Spells[0].Spell != "Spider Mine" || got[0].Spells[0].Unit != "Vulture" {
		t.Fatalf("Spider Mine should surface once under Spells, got %+v", got[0].Spells)
	}
}

func ptr(s string) *string { return &s }

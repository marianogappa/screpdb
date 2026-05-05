package dashboard

import (
	"testing"

	db "github.com/marianogappa/screpdb/internal/dashboard/db"
)

func TestPhaseForSecond_BothBoundariesSet(t *testing.T) {
	cases := []struct {
		name              string
		second, early, mid int
		want              string
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
	// (units list) with its real count, not in the right-side casters
	// strip. Reaver behaves the same way (see test below).
	if len(got[0].Casters) != 0 {
		t.Fatalf("Carrier must not appear in casters strip, got %+v", got[0].Casters)
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
	if len(got[0].Casters) != 0 {
		t.Fatalf("Reaver must not appear in casters strip, got %+v", got[0].Casters)
	}
	if len(got[0].Units) != 1 || got[0].Units[0].Name != "Reaver" || got[0].Units[0].Count != 3 {
		t.Fatalf("Reaver should be in units with count 3, got %+v", got[0].Units)
	}
}

func ptr(s string) *string { return &s }

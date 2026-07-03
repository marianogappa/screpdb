package phases

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

func unit(subject string, second int, playerID int64) cmdenrich.EnrichedCommand {
	return cmdenrich.EnrichedCommand{Kind: cmdenrich.KindMakeUnit, Subject: subject, Second: second, PlayerID: playerID}
}
func tech(subject string, second int, playerID int64) cmdenrich.EnrichedCommand {
	return cmdenrich.EnrichedCommand{Kind: cmdenrich.KindTech, Subject: subject, Second: second, PlayerID: playerID}
}
func upgrade(subject string, second int, playerID int64) cmdenrich.EnrichedCommand {
	return cmdenrich.EnrichedCommand{Kind: cmdenrich.KindUpgrade, Subject: subject, Second: second, PlayerID: playerID}
}

func TestCompute_NoSignals(t *testing.T) {
	earlyEnd, midEnd := Compute(nil)
	if earlyEnd != 0 || midEnd != 0 {
		t.Fatalf("expected (0,0); got (%d,%d)", earlyEnd, midEnd)
	}
}

func TestCompute_MutaliskOnly(t *testing.T) {
	stream := []cmdenrich.EnrichedCommand{
		unit("Mutalisk", 360, 1),
		unit("Mutalisk", 380, 1),
	}
	earlyEnd, midEnd := Compute(stream)
	if earlyEnd != 360 {
		t.Fatalf("earlyEnd: want 360, got %d", earlyEnd)
	}
	if midEnd != 0 {
		t.Fatalf("midEnd: want 0, got %d", midEnd)
	}
}

func TestCompute_SiegeTankNeedsTechToCount(t *testing.T) {
	// First Siege Tank at 240s but no Siege Mode researched → siegeArmedSec is -1.
	// No other early signals → earlyEnd stays 0.
	stream := []cmdenrich.EnrichedCommand{
		unit("Siege Tank (Tank Mode)", 240, 1),
	}
	earlyEnd, _ := Compute(stream)
	if earlyEnd != 0 {
		t.Fatalf("earlyEnd: want 0 (no siege mode tech), got %d", earlyEnd)
	}

	// Add Siege Mode tech at 360s → siegeArmedSec = max(240, 360) = 360.
	stream = append(stream, tech("Tank Siege Mode", 360, 1))
	earlyEnd, _ = Compute(stream)
	if earlyEnd != 360 {
		t.Fatalf("earlyEnd with siege mode: want 360, got %d", earlyEnd)
	}
}

func TestCompute_DragoonNeedsRangeToCount(t *testing.T) {
	stream := []cmdenrich.EnrichedCommand{
		unit("Dragoon", 200, 1),
		upgrade("Singularity Charge (Dragoon Range)", 320, 1),
	}
	earlyEnd, _ := Compute(stream)
	if earlyEnd != 320 {
		t.Fatalf("earlyEnd: want 320 (max of dragoon=200, range=320), got %d", earlyEnd)
	}
}

func TestCompute_MidPickEarliestDefiler(t *testing.T) {
	stream := []cmdenrich.EnrichedCommand{
		unit("Mutalisk", 360, 1),  // early end
		unit("Defiler", 720, 1),   // mid candidate
		unit("Ultralisk", 900, 1), // mid candidate (later)
	}
	earlyEnd, midEnd := Compute(stream)
	if earlyEnd != 360 {
		t.Fatalf("earlyEnd: want 360, got %d", earlyEnd)
	}
	if midEnd != 720 {
		t.Fatalf("midEnd: want 720, got %d", midEnd)
	}
}

func TestCompute_MidClampedToEarly(t *testing.T) {
	// Defiler "first" comes before Mutalisk "first" — mid clamps to earlyEnd.
	stream := []cmdenrich.EnrichedCommand{
		unit("Defiler", 300, 1),
		unit("Mutalisk", 600, 1),
	}
	earlyEnd, midEnd := Compute(stream)
	if earlyEnd != 600 {
		t.Fatalf("earlyEnd: want 600, got %d", earlyEnd)
	}
	if midEnd != 600 {
		t.Fatalf("midEnd: want clamped 600, got %d", midEnd)
	}
}

func TestCompute_ReaverIsAnEarlyEndSignal(t *testing.T) {
	// Protoss-only game where neither Toss player researched Singularity
	// Charge but one of them produced a Reaver at second 480. Pre-fix
	// this returned earlyEnd=0 — leaving any later tier-3 unit (Carrier)
	// to fall into the "early" phase pill.
	stream := []cmdenrich.EnrichedCommand{
		unit("Reaver", 480, 1),
		unit("Carrier", 1100, 1), // mid signal
	}
	earlyEnd, midEnd := Compute(stream)
	if earlyEnd != 480 {
		t.Fatalf("earlyEnd: want 480 (first Reaver), got %d", earlyEnd)
	}
	if midEnd != 1100 {
		t.Fatalf("midEnd: want 1100, got %d", midEnd)
	}
}

func TestCompute_DarkTemplarIsAnEarlyEndSignal(t *testing.T) {
	// PvP via Citadel/Templar Archives where DT comes before any other
	// tier-2 signal. DT is gas-heavy enough to be a meaningful tier-2
	// inflection without an upgrade gate.
	stream := []cmdenrich.EnrichedCommand{
		unit("Dark Templar", 360, 1),
	}
	earlyEnd, _ := Compute(stream)
	if earlyEnd != 360 {
		t.Fatalf("earlyEnd: want 360 (first Dark Templar), got %d", earlyEnd)
	}
}

func TestCompute_TerranPlusTwoUpgrades(t *testing.T) {
	// Two Terran Infantry Weapons commands by player 1 — secs[1] = 800 is
	// "level 2 finished". With nothing else mid, midEnd should pick 800.
	stream := []cmdenrich.EnrichedCommand{
		unit("Wraith", 360, 1), // earlyEnd
		upgrade("Terran Infantry Weapons", 500, 1),
		upgrade("Terran Infantry Weapons", 800, 1),
	}
	earlyEnd, midEnd := Compute(stream)
	if earlyEnd != 360 {
		t.Fatalf("earlyEnd: want 360, got %d", earlyEnd)
	}
	if midEnd != 800 {
		t.Fatalf("midEnd: want 800 (level-2 from 2nd occurrence), got %d", midEnd)
	}
}

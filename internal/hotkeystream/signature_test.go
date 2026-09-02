package hotkeystream

import (
	"strings"
	"testing"
)

// syntheticGame builds a stream where key 5 holds a Hatchery all game, key 1
// holds a 9-unit army, and key 4 is tech in minutes 3-5 then units after.
func syntheticGame() []Event {
	events := []Event{
		{Sec: 5, Type: TypeAssignBuilding, Group: 5, Building: BuildingID("Hatchery"), TileX: 10, TileY: 10},
		{Sec: 20, Type: TypeAssignUnits, Group: 1, Count: 9},
		{Sec: 180, Type: TypeAssignBuilding, Group: 4, Building: BuildingID("Evolution Chamber"), TileX: 12, TileY: 12},
		{Sec: 370, Type: TypeAssignUnits, Group: 4, Count: 6},
	}
	for sec := int32(10); sec < 1200; sec += 15 {
		events = append(events,
			Event{Sec: sec, Type: TypeSelect, Group: 5},
			Event{Sec: sec + 3, Type: TypeSelect, Group: 1},
		)
	}
	for sec := int32(185); sec < 360; sec += 10 {
		events = append(events, Event{Sec: sec, Type: TypeSelect, Group: 4})
	}
	for sec := int32(380); sec < 1200; sec += 20 {
		events = append(events, Event{Sec: sec, Type: TypeSelect, Group: 4})
	}
	return events
}

func TestComputeSignatureTemporalRuns(t *testing.T) {
	games := [][]Event{syntheticGame(), syntheticGame(), syntheticGame(), syntheticGame()}
	sig := ComputeSignature("TestPlayer", "Zerg", games)
	if sig.Games != 4 {
		t.Fatalf("games = %d, want 4", sig.Games)
	}
	if sig.TemporalScore < 0.99 {
		t.Fatalf("identical games must score ~1.0, got %f", sig.TemporalScore)
	}
	byKey := map[int]KeySignature{}
	for _, k := range sig.Keys {
		byKey[k.Key] = k
	}
	if k5, ok := byKey[5]; !ok || len(k5.Runs) != 1 || k5.Runs[0].Category != CategoryHall {
		t.Fatalf("key 5 must be a single hall run, got %+v", byKey[5])
	}
	if k1, ok := byKey[1]; !ok || len(k1.Runs) != 1 || k1.Runs[0].Category != CategoryUnits || k1.MedianCount != 9 {
		t.Fatalf("key 1 must be a units run with median 9, got %+v", byKey[1])
	}
	k4, ok := byKey[4]
	if !ok || len(k4.Runs) != 2 {
		t.Fatalf("key 4 must have two runs (tech then units), got %+v", k4)
	}
	if k4.Runs[0].Category != CategoryTech || k4.Runs[1].Category != CategoryUnits {
		t.Fatalf("key 4 runs out of order: %+v", k4.Runs)
	}
	if k4.Runs[1].StartMin < 6 {
		t.Fatalf("key 4 units run must start around minute 6, got %d", k4.Runs[1].StartMin)
	}
	prose := sig.Prose
	for _, want := range []string{"TestPlayer", "Hatchery on 5", "Key 4 is repurposed", "tech early, then army from ~min 6"} {
		if !strings.Contains(prose, want) {
			t.Fatalf("prose missing %q: %s", want, prose)
		}
	}
}

func TestComputeSignatureEmpty(t *testing.T) {
	sig := ComputeSignature("X", "Terran", nil)
	if sig.Games != 0 || len(sig.Keys) != 0 || sig.Prose != "" {
		t.Fatalf("empty signature not empty: %+v", sig)
	}
}

func TestKeyRanges(t *testing.T) {
	for _, tc := range []struct {
		in   []int
		want string
	}{
		{[]int{1, 2, 3, 7, 8, 9, 0}, "1–3, 7–0"},
		{[]int{5}, "5"},
		{[]int{9, 0}, "9 and 0"},
		{[]int{2, 4}, "2, 4"},
	} {
		if got := keyRanges(tc.in); got != tc.want {
			t.Fatalf("keyRanges(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

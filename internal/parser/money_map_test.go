package parser

import (
	"testing"

	scraprep "github.com/icza/screp/rep"
)

func mapDataWith(amounts ...int) *scraprep.Replay {
	fields := make([]scraprep.Resource, 0, len(amounts))
	for _, amount := range amounts {
		fields = append(fields, scraprep.Resource{Amount: uint32(amount)})
	}
	return &scraprep.Replay{MapData: &scraprep.MapData{MineralFields: fields}}
}

func TestIsMoneyMap(t *testing.T) {
	for _, tc := range []struct {
		name    string
		amounts []int
		want    bool
	}{
		{name: "nil replay", amounts: nil, want: false},
		{
			// A stock melee map: every patch is a standard 1500.
			name:    "standard melee map",
			amounts: []int{1500, 1500, 1500, 1500, 1500},
			want:    false,
		},
		{
			// Late in a game a regular map is full of half-mined patches. The
			// median must stay pinned to the standard amount.
			name:    "partially mined melee map",
			amounts: []int{196, 318, 329, 428, 1500, 1500, 1500, 1500, 1500},
			want:    false,
		},
		{
			// Big Game Hunters - Remastered. Its first stored field is exactly
			// 10000, which the old "MineralFields[0] > 10000" rule rejected on
			// both counts: wrong sample, and a strict comparison against a
			// value that is itself a money amount.
			name:    "money map whose first field is exactly 10000",
			amounts: []int{10000, 20000, 20000, 20000, 20000},
			want:    true,
		},
		{
			name:    "plain big game hunters",
			amounts: []int{20000, 10000, 20000, 20000, 10000},
			want:    true,
		},
		{
			// A modestly enriched map still counts: anything above a standard
			// patch is a deliberate choice by the map maker.
			name:    "lightly enriched map",
			amounts: []int{2500, 2500, 2500},
			want:    true,
		},
		{
			// A depleted money map is still a money map by median.
			name:    "money map with depleted patches",
			amounts: []int{0, 0, 20000, 20000, 20000},
			want:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var replay *scraprep.Replay
			if tc.amounts != nil {
				replay = mapDataWith(tc.amounts...)
			}
			if got := isMoneyMap(replay); got != tc.want {
				t.Errorf("isMoneyMap(%v) = %v, want %v", tc.amounts, got, tc.want)
			}
		})
	}
}

func TestIsMoneyMap_NoMineralFields(t *testing.T) {
	if isMoneyMap(&scraprep.Replay{}) {
		t.Error("a replay with no map data must not classify as a money map")
	}
	if isMoneyMap(mapDataWith()) {
		t.Error("a map with no mineral fields must not classify as a money map")
	}
}

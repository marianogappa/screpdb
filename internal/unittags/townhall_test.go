package unittags

import (
	"reflect"
	"testing"
)

func TestCollapsedBuildSeconds(t *testing.T) {
	tests := []struct {
		name   string
		builds []Build
		want   []int
	}{
		{name: "empty", builds: nil, want: nil},
		{
			name:   "single build",
			builds: []Build{{Sec: 100, X: 10, Y: 10}},
			want:   []int{100},
		},
		{
			// Two distinct expansions >= a footprint apart are both kept.
			name: "distinct bases kept",
			builds: []Build{
				{Sec: 100, X: 10, Y: 10},
				{Sec: 300, X: 50, Y: 60},
			},
			want: []int{100, 300},
		},
		{
			// A re-place lands 46s later at the same tile: footprint overlaps the
			// first placement, so it collapses to one base (issue #245).
			name: "footprint-overlapping replace collapses",
			builds: []Build{
				{Sec: 100, X: 10, Y: 10},
				{Sec: 146, X: 10, Y: 10},
			},
			want: []int{100},
		},
		{
			// Overlap within the footprint (dx<4, dy<3) collapses; the earliest
			// placement is the one kept regardless of stream order.
			name: "partial footprint overlap collapses to earliest",
			builds: []Build{
				{Sec: 200, X: 12, Y: 11}, // later in time, overlaps
				{Sec: 100, X: 10, Y: 10}, // earlier -> kept
			},
			want: []int{100},
		},
		{
			// Exactly one footprint apart (dx==4) does NOT overlap -> distinct.
			name: "adjacent-but-non-overlapping kept",
			builds: []Build{
				{Sec: 100, X: 10, Y: 10},
				{Sec: 200, X: 14, Y: 10},
			},
			want: []int{100, 200},
		},
		{
			// Three placements: two overlap the first, third is distinct.
			name: "mixed overlap and distinct",
			builds: []Build{
				{Sec: 100, X: 10, Y: 10},
				{Sec: 120, X: 11, Y: 11}, // overlaps first
				{Sec: 130, X: 12, Y: 10}, // overlaps first
				{Sec: 400, X: 80, Y: 80}, // distinct
			},
			want: []int{100, 400},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collapsedBuildSeconds(tt.builds)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collapsedBuildSeconds = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTownHallBuildSeconds(t *testing.T) {
	if got := TownHallBuildSeconds(nil); len(got) != 0 {
		t.Errorf("nil evidence should yield empty map, got %+v", got)
	}

	ev := &Evidence{Players: map[byte]*PlayerEvidence{
		1: {Builds: map[string][]Build{zergTownHall: {
			{Sec: 100, X: 10, Y: 10},
			{Sec: 140, X: 10, Y: 10}, // re-place, collapses
			{Sec: 400, X: 80, Y: 80}, // distinct expansion
		}}},
		2: {Builds: map[string][]Build{"Barracks": {{Sec: 50, X: 5, Y: 5}}}}, // no town halls
	}}
	got := TownHallBuildSeconds(ev)
	if want := []int{100, 400}; !reflect.DeepEqual(got[1], want) {
		t.Errorf("player 1 town-hall seconds = %v, want %v", got[1], want)
	}
	if _, ok := got[2]; ok {
		t.Errorf("player with no town-hall builds must be absent from result, got %+v", got[2])
	}
}

func TestTownHallBuildSeconds_FromRawStream(t *testing.T) {
	// End-to-end through Analyze: two Hatchery builds one footprint-overlapping
	// re-place apart collapse to one; the spawn hall (no Build) is absent.
	r := replayOf(
		build(2, 100, zergTownHall, 30, 40),
		build(2, 146, zergTownHall, 31, 41), // overlapping re-place
		build(2, 600, zergTownHall, 90, 90), // real 3rd base
	)
	ev := Analyze(r)
	got := TownHallBuildSeconds(ev)
	if want := []int{100, 600}; !reflect.DeepEqual(got[2], want) {
		t.Errorf("raw-stream town-hall seconds = %v, want %v", got[2], want)
	}
}

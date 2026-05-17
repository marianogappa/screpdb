package dashboard

import "testing"

func TestBuildTimeFor(t *testing.T) {
	cases := []struct {
		name string
		want float64
	}{
		{"Marine", 24},
		{"Siege Tank (Tank Mode)", 50},
		{"Zealot", 25.2},
		{"Dark Templar", 50},
		{"Mutalisk", 25},
		{"Guardian", 40},
		{"Battlecruiser", 133},
		{"Arbiter", 160},
		// Unknown / hero unit → 0 (caller treats as "count at command time").
		{"Sarah Kerrigan (Ghost)", 0},
		{"", 0},
	}
	for _, c := range cases {
		got := buildTimeFor(c.name)
		if got != c.want {
			t.Errorf("buildTimeFor(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

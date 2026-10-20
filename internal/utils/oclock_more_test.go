package utils

import "testing"

func TestCalculateStartLocationOclock(t *testing.T) {
	// A 128x128 tile map is 4096x4096 pixels, centred at 2048,2048.
	cases := []struct {
		name string
		x, y int
		want int
	}{
		{"east", 4000, 2048, 3},
		{"southeast", 4000, 4000, 5},
		{"south", 2048, 4000, 6},
		{"southwest", 100, 4000, 7},
		{"west", 100, 2048, 9},
		{"northwest", 100, 100, 11},
		{"north", 2048, 100, 12},
		{"northeast", 4000, 100, 1},
		{"centre falls back to east", 2048, 2048, 3},
		{"just below east boundary", 4000, 2048 - 10, 3},
	}
	for _, c := range cases {
		if got := CalculateStartLocationOclock(128, 128, c.x, c.y); got != c.want {
			t.Errorf("%s (%d,%d): got %d, want %d", c.name, c.x, c.y, got, c.want)
		}
	}
}

func TestCalculateStartLocationOclockNonSquareMap(t *testing.T) {
	// 192x64 tiles is 6144x2048 pixels; the angle is in pixels, so a wide map's top-left corner reads as west.
	if got := CalculateStartLocationOclock(192, 64, 0, 0); got != 9 {
		t.Fatalf("top-left corner: got %d, want 9", got)
	}
	if got := CalculateStartLocationOclock(192, 64, 6144, 1024); got != 3 {
		t.Fatalf("right edge middle: got %d, want 3", got)
	}
}

func TestIsBigGameHuntersMap(t *testing.T) {
	cases := map[string]bool{
		"Big Game Hunters":     true,
		"  big game hunters  ": true,
		"(8)Big Game Hunters":  true,
		"(4) BGH":              true,
		"BigGameHunters":       true,
		"Big-Game-Hunters 2.1": true,
		"BGH_extra":            true,
		"Fighting Spirit":      false,
		"Big Game Hunterss":    false,
		"Hunters":              false,
		"(2)Lost Temple BGH":   false,
		"":                     false,
	}
	for name, want := range cases {
		if got := IsBigGameHuntersMap(name); got != want {
			t.Errorf("%q: got %v, want %v", name, got, want)
		}
	}
}

func TestIsCliffDropPosition(t *testing.T) {
	const w, h = 4096, 4096
	cases := []struct {
		name         string
		x, y, mw, mh int
		want         bool
	}{
		{"top-left corner", 10, 10, w, h, true},
		{"top-left box edge excluded", CliffDropCornerWidthPx, 10, w, h, false},
		{"bottom-right corner", w - 10, h - 10, w, h, true},
		{"bottom-right exact map edge", w, h, w, h, true},
		{"bottom-right box edge included", w - CliffDropCornerWidthPx, h - CliffDropCornerHeightPx, w, h, true},
		{"past map edge", w + 1, h, w, h, false},
		{"top-right corner is not a cliff", w - 10, 10, w, h, false},
		{"bottom-left corner is not a cliff", 10, h - 10, w, h, false},
		{"centre", 2048, 2048, w, h, false},
		{"negative x", -1, 10, w, h, false},
		{"zero width map", 10, 10, 0, h, false},
		{"negative height map", 10, 10, w, -5, false},
	}
	for _, c := range cases {
		if got := IsCliffDropPosition(c.x, c.y, c.mw, c.mh); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

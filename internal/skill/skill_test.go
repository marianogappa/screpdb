package skill

import "testing"

func TestModelLoadsWithEveryRace(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, race := range []string{"T", "P", "Z"} {
		if _, ok := m.Combined.Races[race]; !ok {
			t.Fatalf("no combined score for %s", race)
		}
		for _, key := range []string{EAPM, HotkeyRecalls, ScreenSwitches, SpendingQuotient, BankedShare} {
			c, ok := m.Metrics[key].Cuts[race]
			if !ok {
				t.Fatalf("no %s cuts for %s", key, race)
			}
			if m.Metrics[key].HigherIsBetter && (c[0] > c[1] || c[1] > c[2]) || !m.Metrics[key].HigherIsBetter && (c[0] < c[1] || c[1] < c[2]) {
				t.Fatalf("%s %s cuts out of order: %v", key, race, c)
			}
		}
	}
}

func TestPlaceIsLinearBetweenCuts(t *testing.T) {
	cuts := [3]float64{100, 200, 300}
	for _, tc := range []struct {
		v    float64
		pos  float64
		tier int
	}{{-50, 0, TierBeginner}, {50, 0.5, TierBeginner}, {150, 1.5, TierIntermediate}, {300, 3, TierPro}, {350, 3.5, TierPro}, {999, 4, TierPro}} {
		if p := place(tc.v, cuts, true); p.Position != tc.pos || p.Tier != tc.tier {
			t.Errorf("place(%v) = %+v, want position %v tier %d", tc.v, p, tc.pos, tc.tier)
		}
	}
	if p := place(0.05, [3]float64{0.2, 0.1, 0.05}, false); p.Tier != TierPro {
		t.Errorf("lower is better: %+v", p)
	}
}

func TestEvaluateOrdersPlayers(t *testing.T) {
	m, _ := Load()
	slow := m.Evaluate("T", map[string]float64{EAPM: 90, HotkeyRecalls: 10, ScreenSwitches: 8})
	fast := m.Evaluate("T", map[string]float64{EAPM: 260, HotkeyRecalls: 160, ScreenSwitches: 26})
	if slow.Combined == nil || fast.Combined == nil || slow.Combined.Tier != TierBeginner || fast.Combined.Tier != TierPro {
		t.Fatalf("slow %+v fast %+v", slow.Combined, fast.Combined)
	}
	if missing := m.Evaluate("T", map[string]float64{EAPM: 150}); missing.Combined != nil {
		t.Fatal("a combined score needs every combined feature")
	}
}

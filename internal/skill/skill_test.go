package skill

import (
	"fmt"
	"math"
	"sort"
	"testing"
)

func TestModelLoadsWithEveryRace(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, race := range []string{"T", "P", "Z"} {
		if _, ok := m.Combined.Races[race]; !ok {
			t.Fatalf("no combined score for %s", race)
		}
		for _, key := range m.Combined.Features {
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

func TestEveryShownMetricCountsTowardTheCombinedScore(t *testing.T) {
	m, _ := Load()
	if len(m.Combined.Features) != len(m.Metrics) {
		t.Fatalf("combined features %v, shown metrics %d", m.Combined.Features, len(m.Metrics))
	}
	for race, cr := range m.Combined.Races {
		total := 0.0
		for _, w := range cr.Weights {
			total += math.Abs(w)
		}
		for i, w := range cr.Weights {
			if share := math.Abs(w) / total; share < 0.099 || share > 0.301 {
				t.Errorf("%s %s carries %.3f of the weight, outside 10%% to 30%%", race, m.Combined.Features[i], share)
			}
			if higher := m.Metrics[m.Combined.Features[i]].HigherIsBetter; higher != (w > 0) {
				t.Errorf("%s %s weight %v has the wrong sign", race, m.Combined.Features[i], w)
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
		if p := place(tc.v, cuts, [2]float64{}, [4]float64{}, true); p.Position != tc.pos || p.Tier != tc.tier {
			t.Errorf("place(%v) = %+v, want position %v tier %d", tc.v, p, tc.pos, tc.tier)
		}
	}
	if p := place(0.05, [3]float64{0.2, 0.1, 0.05}, [2]float64{1, 0}, [4]float64{}, false); p.Tier != TierPro {
		t.Errorf("lower is better: %+v", p)
	}
}

func TestEvaluateOrdersPlayers(t *testing.T) {
	m, _ := Load()
	slow := m.Evaluate("T", "", ContextOneVsOne, map[string]float64{EAPM: 90, UsedRecalls: 3, CheckRecalls: 4, ArmyCadence: 0.2, UnspentPerIncome: 0.9, ProductionIdle: 0.6, IdleArmy: 0.4, MicroActions: 0.3})
	fast := m.Evaluate("T", "", ContextOneVsOne, map[string]float64{EAPM: 260, UsedRecalls: 45, CheckRecalls: 90, ArmyCadence: 0.6, UnspentPerIncome: 0.25, ProductionIdle: 0.25, IdleArmy: 0.05, MicroActions: 4})
	if slow.Combined == nil || fast.Combined == nil || slow.Combined.Tier != TierBeginner || fast.Combined.Tier != TierPro {
		t.Fatalf("slow %+v fast %+v", slow.Combined, fast.Combined)
	}
	if missing := m.Evaluate("T", "", ContextOneVsOne, map[string]float64{EAPM: 150}); missing.Combined != nil {
		t.Fatal("a combined score needs every combined feature")
	}
}

func TestEndsSpreadPlayersBelowTheCuts(t *testing.T) {
	cuts, ends := [3]float64{160, 194, 218}, [2]float64{40, 300}
	a, b := place(60, cuts, ends, [4]float64{}, true), place(115, cuts, ends, [4]float64{}, true)
	if !(a.Position > 0 && a.Position < b.Position && b.Position < 1) {
		t.Fatalf("60 at %v and 115 at %v should both sit inside Beginner, apart", a.Position, b.Position)
	}
}

func TestStripsShareOneScaleAcrossRaces(t *testing.T) {
	m, _ := Load()
	z := m.Evaluate("Z", "", ContextOneVsOne, map[string]float64{EAPM: 81})
	p := m.Evaluate("P", "", ContextOneVsOne, map[string]float64{EAPM: 83})
	if z.Metrics[EAPM].Position >= p.Metrics[EAPM].Position {
		t.Fatalf("EAPM 81 (Zerg) at %v must sit left of 83 (Protoss) at %v", z.Metrics[EAPM].Position, p.Metrics[EAPM].Position)
	}
	low := m.Evaluate("T", "", ContextOneVsOne, map[string]float64{EAPM: 49})
	if low.Metrics[EAPM].Position <= 0 {
		t.Fatalf("EAPM 49 should sit inside Beginner, not on its edge: %v", low.Metrics[EAPM].Position)
	}
	none := m.Evaluate("T", "", ContextOneVsOne, map[string]float64{UsedRecalls: 0})
	if none.Metrics[UsedRecalls].Position != 0 {
		t.Fatalf("no recalls at all is the very start of the axis, got %v", none.Metrics[UsedRecalls].Position)
	}
}

func TestRaceSpecificMetricsUseTheirRaceCuts(t *testing.T) {
	m, _ := Load()
	for _, key := range []string{ArmyCadence, ProductionIdle} {
		if !m.Metrics[key].PerRace {
			t.Fatalf("%s should place each race on its own cuts", key)
		}
	}
	for _, race := range []string{"T", "P", "Z"} {
		mid := m.Metrics[ProductionIdle].Cuts[race][0]
		if p := m.Evaluate(race, "", ContextOneVsOne, map[string]float64{ProductionIdle: mid}).Metrics[ProductionIdle]; math.Abs(p.Position-1) > 0.05 {
			t.Errorf("%s idle production at its own Beginner cut sits at %v, want 1 give or take a printed percent", race, p.Position)
		}
	}
}

func TestATiersMedianSitsMidBand(t *testing.T) {
	cuts, ends, medians := [3]float64{100, 200, 300}, [2]float64{0, 400}, [4]float64{80, 150, 260, 330}
	for _, tc := range []struct{ v, pos float64 }{{80, 0.5}, {40, 0.25}, {90, 0.75}, {260, 2.5}, {330, 3.5}, {400, 4}} {
		if p := place(tc.v, cuts, ends, medians, true); math.Abs(p.Position-tc.pos) > 1e-9 {
			t.Errorf("place(%v) at %v, want %v", tc.v, p.Position, tc.pos)
		}
	}
	if p := place(-80, [3]float64{-100, -200, -300}, [2]float64{0, -400}, [4]float64{-80, -150, -260, -330}, false); math.Abs(p.Position-0.5) > 1e-9 {
		t.Errorf("lower is better: %+v", p)
	}
}

// TestStripsPrintInScale is the strip contract: whatever race, matchup or kind of game a player's value came
// from, on one metric's strip a better printed value sits further right, and equal printed values sit together.
func TestStripsPrintInScale(t *testing.T) {
	m, _ := Load()
	type dot struct {
		who string
		p   Placement
	}
	opponents := map[string][]string{"T": {"", "T", "P", "Z"}, "P": {"", "T", "P", "Z"}, "Z": {"", "T", "P", "Z"}}
	contexts := []string{ContextOneVsOne, ContextTeamRegular, ContextMoney}
	for key, mc := range m.Metrics {
		ends := mc.Ends[AllRaces]
		lo, hi := min(ends[0], ends[1]), max(ends[0], ends[1])
		var dots []dot
		for race, opps := range opponents {
			for _, opp := range opps {
				for _, ctx := range contexts {
					if ctx != ContextOneVsOne && opp != "" {
						continue
					}
					for i := 0; i <= 200; i++ {
						v := lo + (hi-lo)*1.2*float64(i)/200 - (hi-lo)*0.1
						if p, ok := m.Evaluate(race, opp, ctx, map[string]float64{key: max(0, v)}).Metrics[key]; ok {
							dots = append(dots, dot{fmt.Sprintf("%s v%s %q value %v", race, opp, ctx, v), p})
						}
					}
				}
			}
		}
		if len(dots) == 0 {
			t.Fatalf("%s placed nobody", key)
		}
		sign := 1.0
		if !mc.HigherIsBetter {
			sign = -1
		}
		sort.Slice(dots, func(i, j int) bool { return sign*dots[i].p.Value < sign*dots[j].p.Value })
		for i, d := range dots {
			if d.p.Tier != min(TierPro, int(d.p.Position)) {
				t.Fatalf("%s: %s tier %d disagrees with position %v", key, d.who, d.p.Tier, d.p.Position)
			}
			if i == 0 {
				continue
			}
			prev := dots[i-1]
			switch {
			case prev.p.Value == d.p.Value && prev.p.Position != d.p.Position:
				t.Fatalf("%s: %s and %s both print %v but sit at %v and %v", key, prev.who, d.who, d.p.Value, prev.p.Position, d.p.Position)
			case prev.p.Position > d.p.Position:
				t.Fatalf("%s: %s prints %v, better than %s's %v, yet sits left (%v < %v)", key, d.who, d.p.Value, prev.who, prev.p.Value, d.p.Position, prev.p.Position)
			}
			if step := Step[key]; step > 0 && math.Abs(d.p.Value/step-math.Round(d.p.Value/step)) > 1e-6 {
				t.Fatalf("%s: %s prints %v, not a multiple of %v", key, d.who, d.p.Value, step)
			}
		}
	}
}

func TestEveryStripMetricHasAPrintedStep(t *testing.T) {
	m, _ := Load()
	for key := range m.Metrics {
		if Step[key] == 0 {
			t.Errorf("%s has no Step: the tab would print it rounded while placing it unrounded", key)
		}
	}
}

func TestOnStripIsStable(t *testing.T) {
	m, _ := Load()
	for key := range m.Metrics {
		for _, race := range []string{"T", "P", "Z"} {
			for _, v := range []float64{0, 0.05, 0.3, 0.47, 0.8, 2, 40, 150, 300} {
				p, ok := m.Evaluate(race, "", ContextMoney, map[string]float64{key: v}).Metrics[key]
				if !ok {
					continue
				}
				if again := m.OnStrip(key, p); math.Abs(again.Value-p.Value) > 1e-9 || math.Abs(again.Position-p.Position) > 1e-9 {
					t.Errorf("%s %s %v: %+v moves to %+v when put on its strip again", key, race, v, p, again)
				}
			}
		}
	}
}

func TestABurstOfHoldPositionStaysBelowAdvanced(t *testing.T) {
	m, _ := Load()
	burst := ScaledMicro(2.0/13, 2*microArmyValue)
	for _, race := range []string{"T", "P", "Z"} {
		if p := m.Evaluate(race, "", ContextOneVsOne, map[string]float64{MicroActions: burst}).Metrics[MicroActions]; p.Tier >= TierAdvanced {
			t.Errorf("%s: 20 Hold Position presses in 20 seconds of a 15 minute game read %+v", race, p)
		}
	}
}

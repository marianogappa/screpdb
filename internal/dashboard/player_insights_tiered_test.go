package dashboard

import (
	"math"
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/skill"
)

func repeat(v float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v + float64(i%5)*0.01
	}
	return out
}

func TestWilsonLower(t *testing.T) {
	if got := wilsonLower(0, 0); got != 0 {
		t.Fatalf("no games: got %v", got)
	}
	if got := wilsonLower(25, 30); math.Abs(got-0.70) > 0.01 {
		t.Fatalf("25 of 30: got %.3f, want about 0.70", got)
	}
	if wilsonLower(6, 10) >= insightMinShortShare {
		t.Fatal("6 of 10 short is not confidently most games")
	}
	if wilsonLower(28, 30) < insightMinShortShare {
		t.Fatal("28 of 30 short is confidently most games")
	}
}

func TestPositionTrend(t *testing.T) {
	cases := []struct {
		name         string
		newer, older []float64
		want         string
	}{
		{"too few games", repeat(0.9, 5), repeat(0.3, 30), ""},
		{"reliably higher", repeat(0.9, 20), repeat(0.3, 20), trendBetter},
		{"reliably lower", repeat(0.3, 20), repeat(0.9, 20), trendWorse},
		{"same level", repeat(0.5, 20), repeat(0.5, 20), ""},
		{"significant but tiny shift", repeat(0.51, 20), repeat(0.45, 20), ""},
	}
	for _, c := range cases {
		if got := positionTrend(c.newer, c.older); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPositionTrendIgnoresOneOutlier(t *testing.T) {
	newer := repeat(0.4, 20)
	newer[0] = 3.5
	if got := positionTrend(newer, repeat(0.4, 20)); got != "" {
		t.Fatalf("one spammy game moved the trend: %q", got)
	}
}

func TestTopByFamily(t *testing.T) {
	got := topByFamily([]workflowPlayerInsight{
		{Key: "keep_producing", family: "production", Reliability: 0.3},
		{Key: "hotkeys", family: "hotkeys", Reliability: 0.5},
		{Key: "steady_army", family: "production", Reliability: 0.4},
		{Key: "use_army", family: "army", Reliability: 0.2},
		{Key: "micro", family: "micro", Reliability: 0.1},
	})
	want := []string{"hotkeys", "steady_army", "use_army"}
	if len(got) != len(want) {
		t.Fatalf("got %d insights, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Key != want[i] {
			t.Errorf("position %d: got %s, want %s", i, got[i].Key, want[i])
		}
	}
}

func TestMeasuredGamesConfidence(t *testing.T) {
	var games measuredGames
	for i := range 20 {
		p := 0.4
		if i%4 == 0 {
			p = 1.4
		}
		games = append(games, measuredGame{tieredMeasure: tieredMeasure{position: p}})
	}
	if short := games.short(1); len(short) != 15 {
		t.Fatalf("got %d short, want 15", len(short))
	}
	if conf := games.confidence(1); conf < insightMinShortShare {
		t.Fatalf("15 of 20 short should be confident, got %.2f", conf)
	}
}

func TestBankMeasure(t *testing.T) {
	g := tieredGame{race: "T", context: "", values: map[string]float64{"unspent_per_income": 0.409}}
	g.game.replay = &library.Replay{MapKind: library.MapKindRegular}
	bank := barMeasure("unspent_per_income", bankBars, true)
	m, ok := bank(nil, g, 1)
	if !ok || math.Abs(m.position-1) > 1e-9 || m.next != 0.352 {
		t.Fatalf("at the Beginner|Intermediate cut: got %+v, %v", m, ok)
	}
	g.game.replay = &library.Replay{MapKind: library.MapKindMoney}
	if _, ok := bank(nil, g, 1); ok {
		t.Fatal("money maps have no bank bars")
	}
}

func TestNoHotkeysMeasure(t *testing.T) {
	none, _ := noHotkeysMeasure(nil, tieredGame{values: map[string]float64{"used_recalls_per_min": 0.3, "check_recalls_per_min": 0.2}}, 1)
	some, _ := noHotkeysMeasure(nil, tieredGame{values: map[string]float64{"used_recalls_per_min": 4, "check_recalls_per_min": 1}}, 1)
	if none.position >= 1 || some.position < 1 {
		t.Fatalf("got %v and %v", none.position, some.position)
	}
}

func TestBarMeasureHigherIsBetter(t *testing.T) {
	army := barMeasure("army_recalls_per_min", armyRecallBars, false)
	g := tieredGame{race: "T", values: map[string]float64{"army_recalls_per_min": 16.78}}
	g.game.replay = &library.Replay{MapKind: library.MapKindMoney}
	m, ok := army(nil, g, 1)
	if !ok || math.Abs(m.position-2) > 1e-9 || m.next != 11.08 {
		t.Fatalf("at the Intermediate|Advanced cut: got %+v, %v", m, ok)
	}
	prod := barMeasure("production_recalls_per_min", productionRecallBars, false)
	if _, ok := prod(nil, tieredGame{race: "Z", values: map[string]float64{"production_recalls_per_min": 3}}, 1); ok {
		t.Fatal("Zerg has no production hotkey bars")
	}
}

func TestFigureAgrees(t *testing.T) {
	idle := tieredInsightRule{stat: percentStat, lowerIsBetter: true}
	games := func(value, next float64) measuredGames {
		return measuredGames{{tieredMeasure: tieredMeasure{value: value, next: next}}}
	}
	cases := []struct {
		name  string
		rule  tieredInsightRule
		games measuredGames
		want  bool
	}{
		{"behind", idle, games(0.55, 0.45), true},
		{"rounds equal", idle, games(0.36, 0.35), false},
		{"ahead", idle, games(0.30, 0.45), false},
		{"higher is better and behind", tieredInsightRule{stat: perMinuteStat}, games(3, 12), true},
		{"no figure", tieredInsightRule{}, games(0, 0), true},
	}
	for _, c := range cases {
		if got := c.rule.figureAgrees(c.games); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestOKStrengthsSkipListedFamilies(t *testing.T) {
	model, err := skill.Load()
	if err != nil {
		t.Fatal(err)
	}
	var games []tieredGame
	for range 20 {
		g := tieredGame{race: "T", values: map[string]float64{"army_recalls_per_min": 30, "production_recalls_per_min": 10}}
		g.game.replay = &library.Replay{}
		games = append(games, g)
	}
	got := okStrengths(model, games, 0.5, nil)
	if len(got) != 2 {
		t.Fatalf("got %d strengths, want both hotkey ones", len(got))
	}
	got = okStrengths(model, games, 0.5, []workflowPlayerInsight{{Key: "no_hotkeys", family: "hotkeys"}})
	if len(got) != 0 {
		t.Fatalf("Start using hotkeys must silence hotkey strengths, got %v", got)
	}
	got = okStrengths(model, games, 0.5, []workflowPlayerInsight{{Key: "army_hotkeys", family: "army_hotkeys"}})
	if len(got) != 1 || got[0].Key != "production_hotkeys" {
		t.Fatalf("an army hotkeys insight leaves only the production strength, got %v", got)
	}
}

func TestOKStrengthsKeepToTheirLevel(t *testing.T) {
	model, err := skill.Load()
	if err != nil {
		t.Fatal(err)
	}
	var games []tieredGame
	for range 20 {
		g := tieredGame{race: "T", values: map[string]float64{"army_recalls_per_min": 40, "production_recalls_per_min": 12}}
		g.game.replay = &library.Replay{}
		games = append(games, g)
	}
	if got := okStrengths(model, games, 2.5, nil); len(got) != 0 {
		t.Fatalf("an Advanced player got hotkey praise: %v", got)
	}
}

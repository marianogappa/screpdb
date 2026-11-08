package dashboard

import (
	"math"
	"testing"

	"github.com/marianogappa/screpdb/internal/skill"
)

func ratedAt(t *testing.T, position float64, values map[string]float64) ratedPlayer {
	p := testModel(t).StripAt(skill.EAPM, position)
	return ratedPlayer{race: "T", values: values, result: skill.Result{Combined: &p, Metrics: map[string]skill.Placement{skill.EAPM: p}}}
}

func testModel(t *testing.T) *skill.Model {
	m, err := skill.Load()
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPlayerSkillNeedsAFewGamesAndTakesTheMedianPosition(t *testing.T) {
	noHotkeys := map[string]float64{skill.UsedRecalls: 0, skill.CheckRecalls: 0.5}
	busy := map[string]float64{skill.UsedRecalls: 20, skill.CheckRecalls: 40}
	var rated []ratedPlayer
	for range minSkillGames - 1 {
		rated = append(rated, ratedAt(t, 1.2, noHotkeys))
	}
	if summarizeSkill(testModel(t), rated) != nil {
		t.Fatal("fewer than minSkillGames rated games must not show an average")
	}
	rated = append(rated, ratedAt(t, 2.2, busy))
	got := summarizeSkill(testModel(t), rated)
	if got == nil || got.Games != minSkillGames {
		t.Fatalf("got %+v", got)
	}
	if math.Abs(got.Combined.Position-1.2) > 0.05 || got.Combined.Tier != skill.TierIntermediate {
		t.Fatalf("combined %+v, want the median game at 1.2 in Intermediate, unmoved by one stray game", got.Combined)
	}
	if len(got.Pills) != 1 || got.Pills[0].Key != "no_hotkeys" || got.Pills[0].Games != minSkillGames-1 || got.Pills[0].Of != minSkillGames {
		t.Fatalf("pills %+v, want no_hotkeys in 4 of 5 games", got.Pills)
	}
}

func TestAMetricFewGamesMeasureIsLeftOut(t *testing.T) {
	var rated []ratedPlayer
	for i := range minSkillGames {
		r := ratedAt(t, 1.5, nil)
		if i < minSkillGames-1 {
			r.result.Metrics[skill.IdleArmy] = testModel(t).StripAt(skill.IdleArmy, 3.8)
		}
		rated = append(rated, r)
	}
	got := summarizeSkill(testModel(t), rated)
	if _, ok := got.Metrics[skill.IdleArmy]; ok {
		t.Fatalf("idle army measured in %d games should be left out", minSkillGames-1)
	}
	if _, ok := got.Metrics[skill.EAPM]; !ok {
		t.Fatal("a metric every game measures stays")
	}
}

func TestTheHeadlineFollowsTheStrips(t *testing.T) {
	m := testModel(t)
	var rated []ratedPlayer
	for range minSkillGames {
		metrics := map[string]skill.Placement{}
		for _, k := range m.Combined.Features {
			metrics[k] = m.StripAt(k, 1.5)
		}
		metrics[skill.ArmyCadence], metrics[skill.ProductionIdle] = m.StripAt(skill.ArmyCadence, 0.4), m.StripAt(skill.ProductionIdle, 0.4)
		p := skill.Placement{Position: 0.6}
		rated = append(rated, ratedPlayer{race: "P", result: skill.Result{Combined: &p, Metrics: metrics}})
	}
	got := summarizeSkill(m, rated)
	shares := m.Shares("P")
	want := 0.0
	for k, share := range shares {
		want += share * got.Metrics[k].Position
	}
	if math.Abs(got.Combined.Position-want) > 1e-9 || math.Abs(want-(1.5-1.1*(shares[skill.ArmyCadence]+shares[skill.ProductionIdle]))) > 0.05 || got.Combined.Tier != skill.TierIntermediate {
		t.Fatalf("headline %+v, want the strips' weighted average %v in Intermediate", got.Combined, want)
	}
}

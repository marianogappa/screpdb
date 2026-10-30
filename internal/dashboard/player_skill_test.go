package dashboard

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/skill"
)

func ratedAt(position float64, values map[string]float64) ratedPlayer {
	p := skill.Placement{Position: position, Tier: int(position)}
	return ratedPlayer{values: values, result: skill.Result{Combined: &p, Metrics: map[string]skill.Placement{skill.EAPM: p}}}
}

func TestPlayerSkillNeedsAFewGamesAndTakesTheMedianPosition(t *testing.T) {
	noHotkeys := map[string]float64{skill.UsedRecalls: 0, skill.CheckRecalls: 0.5}
	busy := map[string]float64{skill.UsedRecalls: 20, skill.CheckRecalls: 40}
	var rated []ratedPlayer
	for range minSkillGames - 1 {
		rated = append(rated, ratedAt(1.2, noHotkeys))
	}
	if summarizeSkill(rated) != nil {
		t.Fatal("fewer than minSkillGames rated games must not show an average")
	}
	rated = append(rated, ratedAt(2.2, busy))
	got := summarizeSkill(rated)
	if got == nil || got.Games != minSkillGames {
		t.Fatalf("got %+v", got)
	}
	if got.Combined.Position != 1.2 || got.Combined.Tier != skill.TierIntermediate {
		t.Fatalf("combined %+v, want the median game at 1.2 in Intermediate, unmoved by one stray game", got.Combined)
	}
	if len(got.Pills) != 1 || got.Pills[0].Key != "no_hotkeys" || got.Pills[0].Games != minSkillGames-1 || got.Pills[0].Of != minSkillGames {
		t.Fatalf("pills %+v, want no_hotkeys in 4 of 5 games", got.Pills)
	}
}

func TestAMetricFewGamesMeasureIsLeftOut(t *testing.T) {
	var rated []ratedPlayer
	for i := range minSkillGames {
		r := ratedAt(1.5, nil)
		if i < minSkillGames-1 {
			r.result.Metrics[skill.IdleArmy] = skill.Placement{Position: 3.8, Tier: skill.TierPro}
		}
		rated = append(rated, r)
	}
	got := summarizeSkill(rated)
	if _, ok := got.Metrics[skill.IdleArmy]; ok {
		t.Fatalf("idle army measured in %d games should be left out", minSkillGames-1)
	}
	if _, ok := got.Metrics[skill.EAPM]; !ok {
		t.Fatal("a metric every game measures stays")
	}
}

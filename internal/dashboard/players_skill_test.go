package dashboard

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/skill"
)

func TestSkillMediansTakeTheMiddlePlayerPerMetric(t *testing.T) {
	summary := func(combined float64, eapm float64) workflowLibrarySkill {
		return workflowLibrarySkill{
			Combined: skill.Placement{Position: combined},
			Metrics:  map[string]skill.Placement{skill.EAPM: {Value: eapm, Position: combined}},
		}
	}
	model, err := skill.Load()
	if err != nil {
		t.Fatal(err)
	}
	combined, metrics := skillMedians(model, []workflowLibrarySkill{summary(1.2, 60), summary(0.4, 40), summary(2.5, 120), summary(1.8, 90)})
	if combined == nil || combined.Players != 4 || combined.Position != 1.5 {
		t.Fatalf("combined %+v", combined)
	}
	if m := metrics[skill.EAPM]; m.Value != 75 || m.Players != 4 {
		t.Fatalf("eapm %+v", m)
	}
	if c, m := skillMedians(model, nil); c != nil || len(m) != 0 {
		t.Fatalf("no players: %+v %+v", c, m)
	}
}

// TestMediansPrintInScale: on the Players tab, players of different races share each strip, so the medians of
// a per-race metric must print values in the order of their positions.
func TestMediansPrintInScale(t *testing.T) {
	model, err := skill.Load()
	if err != nil {
		t.Fatal(err)
	}
	game := func(race string, idle float64) ratedPlayer {
		return ratedPlayer{race: race, result: model.Evaluate(race, "", skill.ContextTeamRegular, map[string]float64{skill.ProductionIdle: idle})}
	}
	var summaries []workflowLibrarySkill
	for _, tc := range []struct {
		race string
		idle float64
	}{{"Z", 0.37}, {"P", 0.48}, {"P", 0.54}, {"T", 0.70}, {"P", 0.72}, {"T", 0.83}} {
		games := []ratedPlayer{game(tc.race, tc.idle-0.02), game(tc.race, tc.idle), game(tc.race, tc.idle+0.02)}
		ps := make([]skill.Placement, len(games))
		for i, g := range games {
			ps[i] = g.result.Metrics[skill.ProductionIdle]
		}
		summaries = append(summaries, workflowLibrarySkill{Metrics: map[string]skill.Placement{skill.ProductionIdle: gamesPlacement(model, skill.ProductionIdle, ps)}})
	}
	for i, a := range summaries {
		for _, b := range summaries[i+1:] {
			pa, pb := a.Metrics[skill.ProductionIdle], b.Metrics[skill.ProductionIdle]
			if (pa.Value < pb.Value) != (pa.Position > pb.Position) && pa.Value != pb.Value {
				t.Errorf("idle production %v at %v and %v at %v are out of scale", pa.Value, pa.Position, pb.Value, pb.Position)
			}
		}
	}
}

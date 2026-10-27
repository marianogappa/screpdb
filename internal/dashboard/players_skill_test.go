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
	combined, metrics := skillMedians([]workflowLibrarySkill{summary(1.2, 60), summary(0.4, 40), summary(2.5, 120), summary(1.8, 90)})
	if combined == nil || combined.Players != 4 || combined.Position != 1.5 {
		t.Fatalf("combined %+v", combined)
	}
	if m := metrics[skill.EAPM]; m.Value != 75 || m.Players != 4 {
		t.Fatalf("eapm %+v", m)
	}
	if c, m := skillMedians(nil); c != nil || len(m) != 0 {
		t.Fatalf("no players: %+v %+v", c, m)
	}
}

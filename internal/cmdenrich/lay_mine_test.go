package cmdenrich

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func TestClassify_LayMine(t *testing.T) {
	// The parser emits the spider-mine placement as a Targeted Order named
	// "PlaceMine" (the "VultureMine" ability order also maps here). Both must
	// classify as KindLayMine so the first-mine timing marker can see them.
	for _, order := range []string{models.UnitOrderPlaceMine, models.UnitOrderVultureMine} {
		cmd := &models.Command{
			PlayerID:             1,
			ActionType:           "Targeted Order",
			OrderName:            strp(order),
			SecondsFromGameStart: 300,
		}
		fact, ok := Classify(cmd)
		if !ok {
			t.Fatalf("expected %s to classify, got dropped", order)
		}
		if fact.Kind != KindLayMine {
			t.Fatalf("%s: kind = %d, want KindLayMine (%d)", order, fact.Kind, KindLayMine)
		}
		if fact.Aggression != Aggressive {
			t.Fatalf("%s: aggression = %d, want Aggressive (%d)", order, fact.Aggression, Aggressive)
		}
		if fact.Subject != order {
			t.Fatalf("%s: subject = %q, want %q", order, fact.Subject, order)
		}
	}
}

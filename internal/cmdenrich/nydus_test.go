package cmdenrich

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }

func TestClassify_NydusOrders(t *testing.T) {
	tests := []struct {
		name       string
		actionType string
		orderName  string
		wantKind   Kind
		wantAggr   Aggression
		inX        int
		wantX      int // expected pixel X after normalization
	}{
		// BuildNydusExit arrives as an ActionType="Build" command (the canal's
		// build-exit ability) with a tile position, converted to pixels (×32+16).
		// The OrderName check must win over the "Build" ActionType.
		{"build exit", "Build", models.UnitOrderBuildNydusExit, KindBuildNydusExit, Ambiguous, 40, 40*32 + 16},
		// EnterNydusCanal targets a unit: position is already pixels.
		{"enter canal", "TargetedOrder", models.UnitOrderEnterNydusCanal, KindEnterNydusCanal, Aggressive, 1296, 1296},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &models.Command{
				PlayerID:             1,
				ActionType:           tt.actionType,
				OrderName:            strp(tt.orderName),
				X:                    intp(tt.inX),
				Y:                    intp(tt.inX),
				SecondsFromGameStart: 200,
			}
			fact, ok := Classify(cmd)
			if !ok {
				t.Fatalf("expected %s to classify, got dropped", tt.orderName)
			}
			if fact.Kind != tt.wantKind {
				t.Fatalf("kind = %d, want %d", fact.Kind, tt.wantKind)
			}
			if fact.Aggression != tt.wantAggr {
				t.Fatalf("aggression = %d, want %d", fact.Aggression, tt.wantAggr)
			}
			if fact.X == nil || *fact.X != tt.wantX || fact.Y == nil || *fact.Y != tt.wantX {
				t.Fatalf("coords = (%v,%v), want (%d,%d)", fact.X, fact.Y, tt.wantX, tt.wantX)
			}
		})
	}
}

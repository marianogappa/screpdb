package markers

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

func ptr(i int) *int { return &i }

func makeUnit(second int, subject string) cmdenrich.EnrichedCommand {
	return cmdenrich.EnrichedCommand{Kind: cmdenrich.KindMakeUnit, Subject: subject, Second: second}
}

func unloadAll(second, x, y int) cmdenrich.EnrichedCommand {
	return cmdenrich.EnrichedCommand{Kind: cmdenrich.KindUnloadAll, Second: second, X: ptr(x), Y: ptr(y)}
}

// 128x128-tile BGH map => 4096x4096 px; (100,50) is inside the top-left corner box.
func bghReplay() *models.Replay {
	return &models.Replay{MapName: "(8)Big Game Hunters", MapWidth: 128, MapHeight: 128}
}

func TestCliffDrop_RequiresDropship(t *testing.T) {
	cornerX, cornerY := 100, 50
	tests := []struct {
		name    string
		cmds    []cmdenrich.EnrichedCommand
		matched bool
	}{
		{
			name: "bunker unload with tank but no dropship is not a cliff drop",
			cmds: []cmdenrich.EnrichedCommand{
				makeUnit(299, models.GeneralUnitSiegeTankTankMode),
				unloadAll(313, cornerX, cornerY),
			},
			matched: false,
		},
		{
			name: "tank dropped from a dropship into the corner is a cliff drop",
			cmds: []cmdenrich.EnrichedCommand{
				makeUnit(280, models.GeneralUnitDropship),
				makeUnit(299, models.GeneralUnitSiegeTankTankMode),
				unloadAll(313, cornerX, cornerY),
			},
			matched: true,
		},
		{
			name: "dropship without a tank is not a cliff drop",
			cmds: []cmdenrich.EnrichedCommand{
				makeUnit(280, models.GeneralUnitDropship),
				unloadAll(313, cornerX, cornerY),
			},
			matched: false,
		},
		{
			name: "dropship + tank unloaded mid-map is not a cliff drop",
			cmds: []cmdenrich.EnrichedCommand{
				makeUnit(280, models.GeneralUnitDropship),
				makeUnit(299, models.GeneralUnitSiegeTankTankMode),
				unloadAll(313, 2048, 2048),
			},
			matched: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &cliffDropEvaluator{}
			for _, c := range tc.cmds {
				e.Observe(c)
			}
			got := e.Finalize(CustomEvalContext{Replay: bghReplay()})
			if got.Matched != tc.matched {
				t.Fatalf("Matched = %v, want %v", got.Matched, tc.matched)
			}
		})
	}
}

package markers

import (
	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/utils"
)

type cliffDropCandidate struct {
	second int
	x, y   int
}

// cliffDropEvaluator: presence marker for Terran cliff drops on Big
// Game Hunters. Fires once per (replay, player) at the second of the
// first qualifying drop. Conditions:
//
//  1. Map name matches the BGH family (gated at Finalize via
//     utils.IsBigGameHuntersMap).
//  2. Player has produced at least one Siege Tank by the drop second.
//  3. The UnloadAll command lands within the top-left or bottom-right
//     corner box (utils.IsCliffDropPosition).
//
// The worldstate engine emits a parallel `cliff_drop` game_event for
// every qualifying drop in a replay; this marker is the per-player
// presence pill (game list / summary / filter chips).
type cliffDropEvaluator struct {
	hasTank    bool
	candidates []cliffDropCandidate
}

func (e *cliffDropEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	switch f.Kind {
	case cmdenrich.KindMakeUnit:
		if f.Subject == models.GeneralUnitSiegeTankTankMode {
			e.hasTank = true
		}
	case cmdenrich.KindUnloadAll:
		if !e.hasTank || f.X == nil || f.Y == nil {
			return
		}
		e.candidates = append(e.candidates, cliffDropCandidate{
			second: f.Second,
			x:      *f.X,
			y:      *f.Y,
		})
	}
}

func (e *cliffDropEvaluator) Finalize(ctx CustomEvalContext) CustomResult {
	if ctx.Replay == nil {
		return CustomResult{}
	}
	if !utils.IsBigGameHuntersMap(ctx.Replay.MapName) {
		return CustomResult{}
	}
	mapWidthPx := int(ctx.Replay.MapWidth) * 32
	mapHeightPx := int(ctx.Replay.MapHeight) * 32
	for _, c := range e.candidates {
		if utils.IsCliffDropPosition(c.x, c.y, mapWidthPx, mapHeightPx) {
			return CustomResult{
				Matched:          true,
				DetectedAtSecond: c.second,
			}
		}
	}
	return CustomResult{}
}

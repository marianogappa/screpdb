package markers

import "github.com/marianogappa/screpdb/internal/cmdenrich"

// mutaHitnRunEvaluator flags a Zerg player who ran a high-confidence Mutalisk
// hit-and-run campaign (issue #194). Detection lives in internal/unittags and
// is threaded through the worldstate engine; this evaluator only reads the
// engine's conservative per-player confidence flag and surfaces it as a
// presence-only pill (no timing — see worldstate/muta_harass_pass.go for why).
type mutaHitnRunEvaluator struct{}

func (e *mutaHitnRunEvaluator) Observe(_ cmdenrich.EnrichedCommand) {}

func (e *mutaHitnRunEvaluator) Finalize(ctx CustomEvalContext) CustomResult {
	if ctx.WorldState == nil {
		return CustomResult{}
	}
	if !ctx.WorldState.HasMutaHitnRun(ctx.ReplayPlayerID) {
		return CustomResult{}
	}
	return CustomResult{Matched: true}
}

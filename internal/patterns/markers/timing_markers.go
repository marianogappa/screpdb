package markers

import "github.com/marianogappa/screpdb/internal/cmdenrich"

// firstUnitTiming / firstUpgradeTiming build the Custom factory for a timing
// marker that fires at the first Train/Morph (resp. Upgrade) of subject, gated
// to strictly before maxSecond (0 = no deadline).
func firstUnitTiming(subject string, maxSecond int) func() CustomEvaluator {
	return func() CustomEvaluator {
		return &firstFactTimingEvaluator{kind: cmdenrich.KindMakeUnit, subject: subject, maxSecond: maxSecond}
	}
}

func firstUpgradeTiming(subject string, maxSecond int) func() CustomEvaluator {
	return func() CustomEvaluator {
		return &firstFactTimingEvaluator{kind: cmdenrich.KindUpgrade, subject: subject, maxSecond: maxSecond}
	}
}

func firstTechTiming(subject string, maxSecond int) func() CustomEvaluator {
	return func() CustomEvaluator {
		return &firstFactTimingEvaluator{kind: cmdenrich.KindTech, subject: subject, maxSecond: maxSecond}
	}
}

// firstMineTiming fires at the first second a Vulture lays a Spider Mine. The
// mine-lay order surfaces as a KindLayMine fact (see cmdenrich); the order name
// varies (PlaceMine / VultureMine), so this matches any KindLayMine via the
// evaluator's empty-subject wildcard.
func firstMineTiming(maxSecond int) func() CustomEvaluator {
	return func() CustomEvaluator {
		return &firstFactTimingEvaluator{kind: cmdenrich.KindLayMine, subject: "", maxSecond: maxSecond}
	}
}

// firstFactTimingEvaluator commits at the first second a fact of the given Kind
// and Subject appears, provided that second is strictly before maxSecond. Used
// by the first-Reaver / first-Corsair / Speedlot timing markers: the pill's
// {timestamp} placeholder resolves from DetectedAtSecond. A maxSecond of 0
// disables the deadline.
type firstFactTimingEvaluator struct {
	kind      cmdenrich.Kind
	subject   string
	maxSecond int
	firstSec  int
	matched   bool
}

func (e *firstFactTimingEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	if e.matched {
		return
	}
	// Empty subject = wildcard: match any fact of this Kind (used by the
	// mine-lay timing, whose order name varies).
	if f.Kind != e.kind || (e.subject != "" && f.Subject != e.subject) {
		return
	}
	e.firstSec = f.Second
	e.matched = true
}

func (e *firstFactTimingEvaluator) Finalize(_ CustomEvalContext) CustomResult {
	if !e.matched {
		return CustomResult{}
	}
	if e.maxSecond > 0 && e.firstSec >= e.maxSecond {
		return CustomResult{}
	}
	return CustomResult{Matched: true, DetectedAtSecond: e.firstSec}
}

// crazyZergEvaluator matches a TvZ Zerg that transitions Mutalisk -> Ultralisk
// with Zerg Carapace upgraded and without morphing any Lurker before the first
// Ultralisk. Committed at the first Ultralisk's second.
type crazyZergEvaluator struct {
	hasMuta       bool
	hasCarapace   bool
	lurkerBefore  bool
	firstUltraSec int
	hasUltra      bool
}

func (e *crazyZergEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	switch {
	case f.Kind == cmdenrich.KindMakeUnit && f.Subject == subjMutalisk:
		e.hasMuta = true
	case f.Kind == cmdenrich.KindMakeUnit && f.Subject == subjLurker:
		if !e.hasUltra {
			e.lurkerBefore = true
		}
	case f.Kind == cmdenrich.KindMakeUnit && f.Subject == subjUltralisk:
		if !e.hasUltra {
			e.hasUltra = true
			e.firstUltraSec = f.Second
		}
	case f.Kind == cmdenrich.KindUpgrade && f.Subject == subjZergCarapace:
		e.hasCarapace = true
	}
}

func (e *crazyZergEvaluator) Finalize(_ CustomEvalContext) CustomResult {
	if !e.hasMuta || !e.hasUltra || !e.hasCarapace || e.lurkerBefore {
		return CustomResult{}
	}
	return CustomResult{Matched: true, DetectedAtSecond: e.firstUltraSec}
}

// sairSpeedlotEvaluator commits when a player has produced at least two Corsairs
// AND started Zealot leg-speed (Citadel of Adun is implied by the upgrade). It
// is the composition marker that replaced the former Sair/Speedlot opener:
// presence-only, surfaced as a pill, not a build order. DetectedAtSecond is the
// later of the two qualifying seconds.
type sairSpeedlotEvaluator struct {
	corsairSecs []int
	speedSec    int
	hasSpeed    bool
}

func (e *sairSpeedlotEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	switch {
	case f.Kind == cmdenrich.KindMakeUnit && f.Subject == subjCorsair:
		e.corsairSecs = append(e.corsairSecs, f.Second)
	case f.Kind == cmdenrich.KindUpgrade && f.Subject == subjLegEnhancement && !e.hasSpeed:
		e.speedSec = f.Second
		e.hasSpeed = true
	}
}

func (e *sairSpeedlotEvaluator) Finalize(_ CustomEvalContext) CustomResult {
	if !e.hasSpeed || len(e.corsairSecs) < 2 {
		return CustomResult{}
	}
	sec := e.corsairSecs[1]
	if e.speedSec > sec {
		sec = e.speedSec
	}
	return CustomResult{Matched: true, DetectedAtSecond: sec}
}

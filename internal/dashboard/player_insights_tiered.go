package dashboard

import (
	"math"
	"sort"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/skill"
)

// A tiered insight compares a player's games against the next skill tier up, never against progamers, and only
// for players whose combined position falls in its tiers range: advice gets harder as the player gets better.
//
// validity is the metric's partial rank correlation with MMR holding EAPM fixed, measured by scripts/skill-mine on
// 1,380 ladder sides: what the metric says about skill beyond plain speed. Micro is left out because it scored
// lowest there (0.16) and counts actions rather than judging them.
type tieredInsightRule struct {
	key string
	// family groups rules that give overlapping advice; only the most reliable of a family is shown.
	family string
	// replaces are rules this one supersedes when both fire: the more basic advice comes first.
	replaces []string
	// noStrength keeps a rule out of the strengths: it only knows when something is missing.
	noStrength bool
	tiers      [2]float64
	validity   float64
	measure    func(m *skill.Model, g tieredGame, bar float64) (tieredMeasure, bool)
	// stat turns the median value and the next tier's median into the card's vague figure.
	stat          func(you, next float64) map[string]any
	lowerIsBetter bool
	learn         string
}

// tieredMeasure is one game's place on the tier axis, its value and the next tier's median value, both in the
// 1v1 regular-map terms the bars are fitted in.
type tieredMeasure struct {
	position, value, next float64
}

const (
	learnMacro         = "https://liquipedia.net/starcraft/Macromanagement"
	learnControlGroups = "https://classic.battle.net/scc/GS/control.shtml#control"
)

var tieredInsightRules = []tieredInsightRule{
	{key: "no_hotkeys", family: "hotkeys", replaces: []string{"army_hotkeys", "production_hotkeys"}, tiers: [2]float64{0, 1}, validity: 0.37, measure: noHotkeysMeasure, noStrength: true, learn: learnControlGroups},
	{key: "army_hotkeys", family: "army_hotkeys", tiers: [2]float64{0, 2}, validity: 0.25, measure: barMeasure(skill.ArmyRecalls, armyRecallBars, false), stat: perMinuteStat, learn: learnControlGroups},
	{key: "production_hotkeys", family: "production_hotkeys", tiers: [2]float64{0, 2}, validity: 0.30, measure: barMeasure(skill.ProdRecalls, productionRecallBars, false), stat: perMinuteStat, learn: learnControlGroups},
	{key: "keep_producing", family: "production", tiers: [2]float64{0, 2}, validity: 0.19, measure: modelMeasure(skill.ProductionIdle), stat: percentStat, learn: learnMacro, lowerIsBetter: true},
	{key: "steady_army", family: "production", tiers: [2]float64{0.5, 2.5}, validity: 0.17, measure: modelMeasure(skill.ArmyCadence), stat: cadenceStat, learn: learnMacro},
	{key: "use_army", family: "army", tiers: [2]float64{0, 2.5}, validity: 0.22, measure: modelMeasure(skill.IdleArmy), stat: percentStat, lowerIsBetter: true},
	{key: "bank", family: "spending", tiers: [2]float64{0, 2}, validity: 0.26, measure: barMeasure(skill.UnspentPerIncome, bankBars, true), stat: bankStat, learn: learnMacro, lowerIsBetter: true},
}

const (
	maxShownInsights = 3
	// insightSpanGames is how many of the newest rated games an insight reads, and how many before them its
	// trend compares against.
	insightSpanGames = 30
	insightMinGames  = 12
	// insightMinShortShare is the share of games that must fall short of the next tier, at 95% confidence.
	insightMinShortShare = 0.5
	// okBelowBar puts the strengths' bar at the middle of the player's own tier: doing fine for their level.
	okBelowBar    = 0.5
	trendMinZ     = 1.96
	trendMinShift = 0.15
)

const (
	trendBetter = "better"
	trendWorse  = "worse"
)

type tieredGame struct {
	game      insightGame
	race      string
	opponent  string
	context   string
	values    map[string]float64
	positions map[string]float64
}

// tieredInsights are the insights shown now, most reliable first, the ones the player has grown out of, and the
// strengths, said without figures.
func (d *Dashboard) tieredInsights(playerKey string, s *workflowPlayerSkillSummary) (shown, improved, strengths []workflowPlayerInsight) {
	model, err := skill.Load()
	if err != nil || s.Combined.Tier >= skill.TierPro {
		return nil, nil, nil
	}
	games := tieredGames(model, d.library.lib.View().PlayerGames(playerKey))
	recent, older := games[:min(len(games), insightSpanGames)], games[min(len(games), insightSpanGames):]
	older = older[:min(len(older), insightSpanGames)]
	level := s.Combined.Position
	bar := math.Floor(math.Max(0, level)) + 1
	replaced := map[string]bool{}
	for _, rule := range tieredInsightRules {
		now := measureGames(model, recent, rule, bar)
		if level < rule.tiers[0] || level >= rule.tiers[1] || replaced[rule.key] {
			continue
		}
		before := measureGames(model, older, rule, bar)
		nowConf, beforeConf := now.confidence(bar), before.confidence(bar)
		trend := positionTrend(now.positions(), before.positions())
		switch {
		case len(now) >= insightMinGames && nowConf >= insightMinShortShare && rule.figureAgrees(now):
			in := tieredInsight(rule, now, bar, rule.validity*nowConf)
			in.Trend = trend
			shown = append(shown, in)
			for _, r := range rule.replaces {
				replaced[r] = true
			}
		case len(before) >= insightMinGames && beforeConf >= insightMinShortShare && trend == trendBetter:
			in := tieredInsight(rule, before, bar, rule.validity*beforeConf)
			in.Games, in.Examples, in.Params = len(now)+len(before), nil, map[string]any{}
			improved = append(improved, in)
		}
	}
	shown, improved = topByFamily(shown), topByFamily(improved)
	return shown, improved, okStrengths(model, recent, level, append(shown, improved...))
}

// okStrengths are what the player already does at least as well as the middle of their own tier, in most recent
// games at 95% confidence, among the rules for their level: the "OK" side of an insight they could get, so an
// Advanced player is never praised for using hotkeys. A family with an insight or an improvement never also shows as a strength, and neither
// does one an insight replaces: "Start using hotkeys" next to "You command your army with hotkeys" would contradict.
func okStrengths(model *skill.Model, recent []tieredGame, level float64, listed []workflowPlayerInsight) []workflowPlayerInsight {
	bar := math.Floor(math.Max(0, level)) + 1
	family := map[string]string{}
	for _, rule := range tieredInsightRules {
		family[rule.key] = rule.family
	}
	blocked := map[string]bool{}
	for _, in := range listed {
		blocked[in.family] = true
		for _, rule := range tieredInsightRules {
			if rule.key == in.Key {
				for _, r := range rule.replaces {
					blocked[family[r]] = true
				}
			}
		}
	}
	okBar := bar - okBelowBar
	var out []workflowPlayerInsight
	for _, rule := range tieredInsightRules {
		if rule.noStrength || blocked[rule.family] || level < rule.tiers[0] || level >= rule.tiers[1] {
			continue
		}
		now := measureGames(model, recent, rule, bar)
		if ok := now.aheadConfidence(okBar); len(now) >= insightMinGames && ok >= insightMinShortShare {
			in := tieredInsight(rule, now, bar, rule.validity*ok)
			in.Examples, in.Params = nil, map[string]any{}
			out = append(out, in)
		}
	}
	return topByFamily(out)
}

// figureAgrees says the card's own rounded figure shows the player behind the next tier: a card reading "35%, vs
// 35% one level up" would undercut itself, so it is not shown.
func (rule tieredInsightRule) figureAgrees(games measuredGames) bool {
	if rule.stat == nil || len(games) == 0 {
		return true
	}
	values, nexts := make([]float64, len(games)), make([]float64, len(games))
	for i, g := range games {
		values[i], nexts[i] = g.value, g.next
	}
	you, next := medianOf(values), medianOf(nexts)
	shown := rule.stat(you, next)
	if shown["you"] == shown["next"] {
		return false
	}
	return rule.lowerIsBetter == (you > next)
}

type measuredGame struct {
	game tieredGame
	tieredMeasure
}

type measuredGames []measuredGame

func measureGames(model *skill.Model, games []tieredGame, rule tieredInsightRule, bar float64) measuredGames {
	var out measuredGames
	for _, g := range games {
		if m, ok := rule.measure(model, g, bar); ok {
			out = append(out, measuredGame{g, m})
		}
	}
	return out
}

func (ms measuredGames) positions() []float64 {
	out := make([]float64, len(ms))
	for i, m := range ms {
		out[i] = m.position
	}
	return out
}

func (ms measuredGames) short(bar float64) measuredGames {
	var out measuredGames
	for _, m := range ms {
		if m.position < bar {
			out = append(out, m)
		}
	}
	return out
}

// aheadConfidence is the share of games at or past bar at 95% confidence.
func (ms measuredGames) aheadConfidence(bar float64) float64 {
	return wilsonLower(len(ms)-len(ms.short(bar)), len(ms))
}

// confidence is the share of games that fall short of bar at 95% confidence (the lower Wilson bound).
func (ms measuredGames) confidence(bar float64) float64 {
	return wilsonLower(len(ms.short(bar)), len(ms))
}

func tieredInsight(rule tieredInsightRule, games measuredGames, bar float64, reliability float64) workflowPlayerInsight {
	in := workflowPlayerInsight{Key: rule.key, Games: len(games), Params: map[string]any{}, Reliability: math.Round(reliability*100) / 100, Learn: rule.learn, family: rule.family}
	var oldest *library.Replay
	values, nexts := make([]float64, 0, len(games)), make([]float64, 0, len(games))
	for _, g := range games {
		if r := g.game.game.replay; oldest == nil || r.Date.Before(oldest.Date) {
			oldest = r
		}
		values, nexts = append(values, g.value), append(nexts, g.next)
	}
	if oldest != nil {
		in.From = oldest.Date.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	short := games.short(bar)
	if rule.stat != nil && len(games) > 0 {
		in.Params = rule.stat(medianOf(values), medianOf(nexts))
	} else {
		in.Params = map[string]any{"count": len(short), "of": len(games)}
	}
	plain := make([]insightGame, 0, len(short))
	worst := map[int64]float64{}
	for _, g := range short {
		plain = append(plain, g.game.game)
		worst[g.game.game.replay.ID] = -g.position
	}
	in.Examples = examplesBy(plain, "skill", func(g insightGame) (float64, bool) { return worst[g.replay.ID], true })
	return in
}

// tieredGames are the player's rated games, newest first.
func tieredGames(model *skill.Model, refs []library.PlayerRef) []tieredGame {
	var out []tieredGame
	for _, ref := range refs {
		r := ref.Replay
		rated, ok := rateGame(model, r).players[int(ref.Ordinal)]
		if !ok {
			continue
		}
		var humans []int
		for i := range r.Players {
			if p := &r.Players[i]; p.IsHuman() && !p.IsObserver() {
				humans = append(humans, i)
			}
		}
		g := tieredGame{
			game:      insightGame{replay: r, won: ref.Player().PlayerOutcome() == models.OutcomeWon, duration: int(r.Duration)},
			race:      rated.race,
			opponent:  opponentRace(r, humans, int(ref.Ordinal)),
			context:   skillContext(r, len(humans)),
			values:    rated.values,
			positions: map[string]float64{},
		}
		if r.Flags.Has(library.FlagIsOneOnOne) {
			for i := range r.Players {
				if i != int(ref.Ordinal) && !r.Players[i].IsObserver() {
					g.game.oppName = r.Players[i].Name
				}
			}
		}
		for k, p := range rated.result.Metrics {
			g.positions[k] = p.Position
		}
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].game.replay.Date.After(out[j].game.replay.Date) })
	return out
}

// modelMeasure reads a skill model metric: its placement, its value moved to 1v1 regular-map terms, and the median
// of the next tier on the same cuts.
func modelMeasure(metric string) func(*skill.Model, tieredGame, float64) (tieredMeasure, bool) {
	return func(m *skill.Model, g tieredGame, bar float64) (tieredMeasure, bool) {
		p, ok1 := g.positions[metric]
		v, ok2 := g.values[metric]
		mc := m.Metrics[metric]
		medians, ok3 := mc.Medians[mc.Scale(g.race, g.opponent)]
		if !ok1 || !ok2 || !ok3 {
			return tieredMeasure{}, false
		}
		v += m.ContextShift[skill.AdjustKey(g.context, g.race)][metric]
		return tieredMeasure{position: p, value: v, next: medians[min(3, int(bar))]}, true
	}
}

// noHotkeysMeasure is a fact rather than a model reading: a game with under one hotkey recall a minute counts as
// one without hotkeys.
func noHotkeysMeasure(_ *skill.Model, g tieredGame, bar float64) (tieredMeasure, bool) {
	used, ok1 := g.values[skill.UsedRecalls]
	checks, ok2 := g.values[skill.CheckRecalls]
	if !ok1 || !ok2 {
		return tieredMeasure{}, false
	}
	if used+checks < 1 {
		return tieredMeasure{position: 0}, true
	}
	return tieredMeasure{position: bar}, true
}

// tierBars are, per race, the Beginner|Intermediate and Intermediate|Advanced cuts and the Beginner, Intermediate
// and Advanced medians of a metric the skill model does not hold, fitted the way it fits its cuts on 1v1 ladder
// games with known MMR.
type tierBars map[string]struct {
	cuts    [2]float64
	medians [3]float64
}

// Army and production hotkey uses a minute, on 4,776 sides of 2,486 ladder games. Zerg has no production bars: a
// Hatchery recall is followed by selecting larvae, so it never reads as production (partial rho 0.06).
var armyRecallBars = tierBars{
	"T": {[2]float64{6.73, 16.78}, [3]float64{3.46, 11.08, 23.83}},
	"P": {[2]float64{5.60, 13.95}, [3]float64{2.77, 7.58, 20.85}},
	"Z": {[2]float64{6.92, 18.39}, [3]float64{4.09, 10.45, 27.10}},
}

var productionRecallBars = tierBars{
	"T": {[2]float64{1.78, 4.50}, [3]float64{0.63, 2.59, 7.06}},
	"P": {[2]float64{1.31, 4.58}, [3]float64{0.44, 2.73, 6.67}},
}

// Minutes of income left unspent, on 1,380 skill-mine ladder sides.
var bankBars = tierBars{
	"T": {[2]float64{0.409, 0.311}, [3]float64{0.477, 0.352, 0.283}},
	"P": {[2]float64{0.369, 0.310}, [3]float64{0.399, 0.347, 0.270}},
	"Z": {[2]float64{0.445, 0.362}, [3]float64{0.520, 0.411, 0.334}},
}

// barMeasure places a game on tierBars, linear through the cuts; regularOneVsOne keeps to the games the bars
// hold for, when a metric shifts in team games and on money maps.
func barMeasure(metric string, bars tierBars, regularOneVsOne bool) func(*skill.Model, tieredGame, float64) (tieredMeasure, bool) {
	return func(_ *skill.Model, g tieredGame, bar float64) (tieredMeasure, bool) {
		v, ok := g.values[metric]
		b, known := bars[g.race]
		if !ok || !known || regularOneVsOne && (g.context != skill.ContextOneVsOne || g.game.replay.MapKind != library.MapKindRegular) {
			return tieredMeasure{}, false
		}
		position := math.Max(0, math.Min(4, 1+(v-b.cuts[0])/(b.cuts[1]-b.cuts[0])))
		return tieredMeasure{position: position, value: v, next: b.medians[min(2, int(bar))]}, true
	}
}

func perMinuteStat(you, next float64) map[string]any {
	return map[string]any{"you": roughCount(you), "next": roughCount(next)}
}

func percentStat(you, next float64) map[string]any {
	return map[string]any{"you": roundTo(you*100, 5), "next": roundTo(next*100, 5)}
}

// cadenceStat turns the share of 10-second stretches that start army units into seconds between new units.
func cadenceStat(you, next float64) map[string]any {
	return map[string]any{"you": roundTo(10/math.Max(you, 0.05), 5), "next": roundTo(10/math.Max(next, 0.05), 5)}
}

func bankStat(you, next float64) map[string]any {
	return map[string]any{"you": roundTo(you*60, 5), "next": roundTo(next*60, 5)}
}

func roundTo(v, step float64) float64 { return math.Round(v/step) * step }

func roughCount(v float64) float64 {
	if v < 3 {
		return math.Round(v*2) / 2
	}
	return math.Round(v)
}

func wilsonLower(k, n int) float64 {
	if n == 0 {
		return 0
	}
	const z = 1.645
	p, nf := float64(k)/float64(n), float64(n)
	centre := p + z*z/(2*nf)
	spread := z * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf))
	return (centre - spread) / (1 + z*z/nf)
}

// positionTrend says whether the newer games sit reliably higher or lower on the tier axis than the older ones:
// a rank test, so one game of spam cannot decide it, and a minimum shift, so a tiny steady change does not either.
func positionTrend(newer, older []float64) string {
	if len(newer) < insightMinGames || len(older) < insightMinGames {
		return ""
	}
	shift := medianOf(newer) - medianOf(older)
	z := mannWhitneyZ(newer, older)
	switch {
	case z >= trendMinZ && shift >= trendMinShift:
		return trendBetter
	case z <= -trendMinZ && shift <= -trendMinShift:
		return trendWorse
	}
	return ""
}

func mannWhitneyZ(a, b []float64) float64 {
	u := 0.0
	for _, x := range a {
		for _, y := range b {
			switch {
			case x > y:
				u++
			case x == y:
				u += 0.5
			}
		}
	}
	na, nb := float64(len(a)), float64(len(b))
	mean := na * nb / 2
	sd := math.Sqrt(na * nb * (na + nb + 1) / 12)
	if sd == 0 {
		return 0
	}
	return (u - mean) / sd
}

func topByFamily(ins []workflowPlayerInsight) []workflowPlayerInsight {
	sort.SliceStable(ins, func(i, j int) bool { return ins[i].Reliability > ins[j].Reliability })
	seen := map[string]bool{}
	var out []workflowPlayerInsight
	for _, in := range ins {
		if seen[in.family] || len(out) == maxShownInsights {
			continue
		}
		seen[in.family] = true
		out = append(out, in)
	}
	return out
}

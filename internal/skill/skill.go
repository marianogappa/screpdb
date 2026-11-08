// Package skill places a player's game on four tiers fitted to ladder MMR: Beginner (under 1,400),
// Intermediate (1,400 to 1,799), Advanced (1,800 and up) and Pro (labelled progamers).
//
// model.json is written by scripts/skill-fit from scripts/skill-mine's measurements of thousands of 1v1
// ladder games with known MMR. Only metrics that followed MMR there are in it; each has cut values per race
// between neighbouring tiers. The combined score is a per-race regression on every one of them, with each
// weight held between 10% and 30% so that no single metric (EAPM above all) decides the tier, and it is
// only called confident when it sits well away from a cut.
package skill

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sync"
)

//go:embed model.json
var modelJSON []byte

const (
	TierBeginner = iota
	TierIntermediate
	TierAdvanced
	TierPro
	tierCount
)

// AllRaces keys the cuts every race shares. Metric strips place players on them, so on one strip a higher
// value always sits further right; the combined estimate stays per race, and so do the strips of metrics
// measured differently per race (MetricCuts.PerRace).
const AllRaces = "all"

// Metric keys, as the miner names them.
const (
	EAPM             = "eapm"
	UsedRecalls      = "used_recalls_per_min"
	CheckRecalls     = "check_recalls_per_min"
	ArmyCadence      = "army_cadence"
	UnspentPerIncome = "unspent_per_income"
	ProductionIdle   = "production_idle_share"
	IdleArmy         = "idle_army_share"
	MicroActions     = "micro_actions_per_min"
)

// MinGameSeconds and MinAPM bound what the model was fitted on: shorter games and idle players are not
// measurable. Only 1v1 games were measured.
const (
	MinGameSeconds = 480
	MinAPM         = 20
)

type Model struct {
	Games           int                   `json:"games"`
	Players         int                   `json:"players"`
	ConfidentMargin float64               `json:"confident_margin"`
	Metrics         map[string]MetricCuts `json:"metrics"`
	Combined        Combined              `json:"combined"`
	// ContextShift is added to macro values from team games and money maps to make them their 1v1
	// regular-map equivalents, keyed by AdjustKey and metric.
	ContextShift map[string]map[string]float64 `json:"context_shift,omitempty"`
}

// Game contexts whose macro values are adjusted: team games on regular maps, and money maps (mostly team
// games; their few 1v1s share the table).
const (
	ContextOneVsOne    = ""
	ContextTeamRegular = "team_regular"
	ContextMoney       = "money"
)

// AdjustKey is the ContextShift key of a context and a race.
func AdjustKey(context, race string) string { return context + "/" + race }

// MetricCuts are, per race ("T", "P", "Z"), the values between Beginner|Intermediate, Intermediate|Advanced
// and Advanced|Pro, where as many of the lower tier score past the cut as of the upper tier fall short of it.
type MetricCuts struct {
	HigherIsBetter bool `json:"higher_is_better"`
	// PerRace places each race on its own cuts: the metric depends on what the race's production makes, so
	// its values only compare within a race.
	PerRace bool `json:"per_race,omitempty"`
	// PerMatchup also places 1v1 games on cuts of their matchup ("PvT"): the metric depends on what the
	// opponent's race makes a player do, like Protoss holding a position against Terran.
	PerMatchup bool                  `json:"per_matchup,omitempty"`
	Rho        float64               `json:"rho"`
	Cuts       map[string][3]float64 `json:"cuts"`
	// Ends are, per race, the worst and best values the axis reaches (the 2nd and 98th percentile of the
	// fitted players), so players far below or above the cuts still spread out instead of piling up.
	Ends map[string][2]float64 `json:"ends,omitempty"`
	// Medians are, per race, each tier's median value: the middle of its band on the axis, so a typical
	// player of a tier sits mid-band and the band's edges are for players past the tier's middle.
	Medians map[string][4]float64 `json:"medians,omitempty"`
}

type Combined struct {
	Features []string                   `json:"features"`
	Races    map[string]CombinedForRace `json:"races"`
}

type CombinedForRace struct {
	Mean      []float64  `json:"mean"`
	SD        []float64  `json:"sd"`
	Weights   []float64  `json:"weights"`
	Intercept float64    `json:"intercept"`
	Cuts      [3]float64 `json:"cuts"`
	Ends      [2]float64 `json:"ends"`
	Medians   [4]float64 `json:"medians"`
}

var (
	loadOnce sync.Once
	loaded   *Model
	loadErr  error
)

// Load returns the embedded model.
func Load() (*Model, error) {
	loadOnce.Do(func() {
		var m Model
		if err := json.Unmarshal(modelJSON, &m); err != nil {
			loadErr = fmt.Errorf("skill model: %w", err)
			return
		}
		loaded = &m
	})
	return loaded, loadErr
}

// Placement is a value placed on the tier axis: 0 to 1 is Beginner, 1 to 2 Intermediate, 2 to 3 Advanced and
// 3 to 4 Pro, linear between cuts. The outer tiers are as wide as their neighbour, then clamped.
type Placement struct {
	Value    float64 `json:"value"`
	Position float64 `json:"position"`
	Tier     int     `json:"tier"`
}

func place(value float64, cuts [3]float64, ends [2]float64, medians [4]float64, higherIsBetter bool) Placement {
	a := newAxis(cuts, ends, medians, higherIsBetter)
	v := a.sign * value
	tier := 0
	for _, cut := range a.cuts {
		if v >= cut {
			tier++
		}
	}
	return Placement{Value: value, Position: axisPosition(v, a.points), Tier: tier}
}

// axis maps values to positions. Lower-is-better metrics are negated (sign -1) so that position always grows
// with sign*value.
type axis struct {
	points []axisPoint
	cuts   [3]float64
	sign   float64
}

func newAxis(cuts [3]float64, ends [2]float64, medians [4]float64, higherIsBetter bool) axis {
	sign, c, e, m := 1.0, cuts, ends, medians
	if !higherIsBetter {
		sign, c, e = -1, [3]float64{-cuts[0], -cuts[1], -cuts[2]}, [2]float64{-ends[0], -ends[1]}
		m = [4]float64{-medians[0], -medians[1], -medians[2], -medians[3]}
	}
	// The outer tiers reach the data's ends; without ends past the cuts, they are as wide as their neighbour.
	lo, hi := c[0]-(c[1]-c[0]), c[2]+(c[2]-c[1])
	if e[0] < c[0] {
		lo = e[0]
	}
	if e[1] > c[2] {
		hi = e[1]
	}
	return axis{points: axisPoints(lo, c, hi, m, medians != [4]float64{}), cuts: c, sign: sign}
}

func (a axis) position(value float64) float64 { return axisPosition(a.sign*value, a.points) }

func (a axis) value(position float64) float64 {
	ps := a.points
	if position <= ps[0].position {
		return a.sign * ps[0].value
	}
	for i := 1; i < len(ps); i++ {
		if position <= ps[i].position {
			lo, hi := ps[i-1], ps[i]
			return a.sign * (lo.value + (position-lo.position)/(hi.position-lo.position)*(hi.value-lo.value))
		}
	}
	return a.sign * ps[len(ps)-1].value
}

type axisPoint struct{ value, position float64 }

// axisPoints are the values the axis passes through: its ends, the cuts, and each tier's median mid-band
// when that median lies inside its band.
func axisPoints(lo float64, c [3]float64, hi float64, m [4]float64, haveMedians bool) []axisPoint {
	edges := [5]float64{lo, c[0], c[1], c[2], hi}
	points := []axisPoint{{lo, 0}}
	for i := 0; i < 4; i++ {
		if haveMedians && m[i] > edges[i] && m[i] < edges[i+1] {
			points = append(points, axisPoint{m[i], float64(i) + 0.5})
		}
		points = append(points, axisPoint{edges[i+1], float64(i + 1)})
	}
	return points
}

func axisPosition(v float64, points []axisPoint) float64 {
	if v <= points[0].value {
		return 0
	}
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		if v <= b.value {
			if b.value <= a.value {
				return b.position
			}
			return a.position + (v-a.value)/(b.value-a.value)*(b.position-a.position)
		}
	}
	return 4
}

// Step is the precision the Skill tab prints each metric's value at.
var Step = map[string]float64{
	EAPM:           1,
	UsedRecalls:    1,
	CheckRecalls:   1,
	ArmyCadence:    0.01,
	ProductionIdle: 0.01,
	IdleArmy:       0.001,
	MicroActions:   0.1,
}

// OnStrip puts a placement on its metric's strip, where every player of every race and game shares one
// scale: a value printed further right is always better, and two players printed alike sit together. Metrics
// placed on per-race or per-matchup cuts, or shifted for team games and money maps, print the value the shared
// scale reads at their position, so their tier is unchanged; the others print what was measured. Either way
// the value is rounded to Step and the position follows the rounded value.
func (m *Model) OnStrip(key string, p Placement) Placement {
	mc, ok := m.Metrics[key]
	if !ok {
		return p
	}
	cuts, ok := mc.Cuts[AllRaces]
	if !ok {
		return p
	}
	a := newAxis(cuts, mc.Ends[AllRaces], mc.Medians[AllRaces], mc.HigherIsBetter)
	v := p.Value
	if m.convertedOnStrip(key) {
		v = a.value(p.Position)
	}
	return m.onStripAxis(key, a, v)
}

// StripAt is the placement at position on key's strip.
func (m *Model) StripAt(key string, position float64) Placement {
	mc := m.Metrics[key]
	a := newAxis(mc.Cuts[AllRaces], mc.Ends[AllRaces], mc.Medians[AllRaces], mc.HigherIsBetter)
	return m.onStripAxis(key, a, a.value(position))
}

func (m *Model) onStripAxis(key string, a axis, v float64) Placement {
	if step := Step[key]; step > 0 {
		v = math.Round(v/step) * step
	}
	pos := a.position(v)
	return Placement{Value: v, Position: pos, Tier: min(TierPro, int(pos))}
}

// convertedOnStrip says whether a metric's placements come from somewhere else than the shared scale.
func (m *Model) convertedOnStrip(key string) bool {
	mc := m.Metrics[key]
	if mc.PerRace || mc.PerMatchup {
		return true
	}
	for _, shifts := range m.ContextShift {
		if shifts[key] != 0 {
			return true
		}
	}
	return false
}

// Result is one player's game.
type Result struct {
	Metrics map[string]Placement `json:"metrics"`
	// Combined is the Headline of the metrics; nil when they carry less than minMeasuredShare of the weight.
	Combined *Placement `json:"combined,omitempty"`
	// Confident is true when the combined score is ConfidentMargin or more from every cut: on held-out games
	// that tier was right 70% of the time, against 55% for any score.
	Confident bool `json:"confident"`
}

// Evaluate places a player's measured values; race is "T", "P" or "Z", opponent is the 1v1 opponent's
// race ("" in team games), and context says what kind of game it was. Macro values of team games and money maps are placed as their 1v1 regular-map equivalents;
// Placement.Value stays the measured value.
func (m *Model) Evaluate(race, opponent, context string, values map[string]float64) Result {
	res := Result{Metrics: map[string]Placement{}}
	adjusted := map[string]float64{}
	for key, v := range values {
		adjusted[key] = v + m.ContextShift[AdjustKey(context, race)][key]
	}
	for key, mc := range m.Metrics {
		scale := AllRaces
		if _, ok := mc.Cuts[scale]; !ok || mc.PerRace {
			scale = race
		}
		if matchup := race + "v" + opponent; mc.PerMatchup && opponent != "" {
			if _, ok := mc.Cuts[matchup]; ok {
				scale = matchup
			}
		}
		cuts, ok := mc.Cuts[scale]
		v, has := values[key]
		if !ok || !has {
			continue
		}
		p := place(adjusted[key], cuts, mc.Ends[scale], mc.Medians[scale], mc.HigherIsBetter)
		p.Value = v
		res.Metrics[key] = m.OnStrip(key, p)
	}
	cr, ok := m.Combined.Races[race]
	if !ok {
		return res
	}
	shares := m.Shares(race)
	measured := 0.0
	for key, share := range shares {
		if _, ok := res.Metrics[key]; ok {
			measured += share
		}
	}
	if measured < minMeasuredShare {
		return res
	}
	res.Combined = m.Headline(shares, res.Metrics)
	score := cr.Intercept
	for i, f := range m.Combined.Features {
		v, has := adjusted[f]
		if !has {
			return res
		}
		z := (v - cr.Mean[i]) / cr.SD[i]
		score += cr.Weights[i] * max(-3, min(3, z))
	}
	res.Confident = true
	for _, c := range cr.Cuts {
		if abs(score-c) < m.ConfidentMargin {
			res.Confident = false
		}
	}
	return res
}

// minMeasuredShare is how much of the estimate's weight a game must measure to get a Skill tier. Idle army
// needs a real army for a minute after 8:00, which many games never field; the other metrics are enough.
const minMeasuredShare = 0.85

// Shares is how much each combined metric weighs in the fitted estimate for race, summing to 1.
func (m *Model) Shares(race string) map[string]float64 {
	cr, ok := m.Combined.Races[race]
	if !ok {
		return nil
	}
	total := 0.0
	for _, w := range cr.Weights {
		total += abs(w)
	}
	out := map[string]float64{}
	for i, f := range m.Combined.Features {
		if total > 0 && i < len(cr.Weights) {
			out[f] = abs(cr.Weights[i]) / total
		}
	}
	return out
}

// Headline is the tier the strips add up to: the average of the metrics' positions, each weighted by its
// share of the estimate. The fitted score ranks players almost exactly as well, but it can call a player
// Beginner whose strips mostly read Intermediate, and a headline that disagrees with the strips under it
// reads as a bug.
func (m *Model) Headline(shares map[string]float64, metrics map[string]Placement) *Placement {
	pos, weight := 0.0, 0.0
	for key, share := range shares {
		if p, ok := metrics[key]; ok {
			pos += share * p.Position
			weight += share
		}
	}
	if weight == 0 {
		return nil
	}
	pos /= weight
	return &Placement{Value: pos, Position: pos, Tier: min(TierPro, int(pos))}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

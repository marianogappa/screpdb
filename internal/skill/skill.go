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
	// ContextAdjust maps macro values from team games and money maps onto their 1v1 regular-map
	// equivalents, keyed by AdjustKey and metric.
	ContextAdjust map[string]map[string]QuantileMap `json:"context_adjust,omitempty"`
}

// Game contexts whose macro values are adjusted: team games on regular maps, and money maps (mostly team
// games; their few 1v1s share the table).
const (
	ContextOneVsOne    = ""
	ContextTeamRegular = "team_regular"
	ContextMoney       = "money"
)

// AdjustKey is the ContextAdjust key of a context and a race.
func AdjustKey(context, race string) string { return context + "/" + race }

// QuantileMap holds the same quantiles of one metric in a context (From) and in 1v1 regular-map games (To),
// both among players of the same mechanics level, so a value converts at equal percentile.
type QuantileMap struct {
	From []float64 `json:"from"`
	To   []float64 `json:"to"`
}

func (q QuantileMap) apply(v float64) float64 {
	n := len(q.From)
	if n < 2 || len(q.To) != n {
		return v
	}
	if v <= q.From[0] {
		return q.To[0]
	}
	if v >= q.From[n-1] {
		return q.To[n-1]
	}
	for i := 1; i < n; i++ {
		if v <= q.From[i] {
			span := q.From[i] - q.From[i-1]
			f := 0.0
			if span > 0 {
				f = (v - q.From[i-1]) / span
			}
			return q.To[i-1] + f*(q.To[i]-q.To[i-1])
		}
	}
	return v
}

// MetricCuts are, per race ("T", "P", "Z"), the values between Beginner|Intermediate, Intermediate|Advanced
// and Advanced|Pro, where as many of the lower tier score past the cut as of the upper tier fall short of it.
type MetricCuts struct {
	HigherIsBetter bool `json:"higher_is_better"`
	// PerRace places each race on its own cuts: the metric depends on what the race's production makes, so
	// its values only compare within a race.
	PerRace bool                  `json:"per_race,omitempty"`
	Rho     float64               `json:"rho"`
	Cuts    map[string][3]float64 `json:"cuts"`
	// Ends are, per race, the worst and best values the axis reaches (the 2nd and 98th percentile of the
	// fitted players), so players far below or above the cuts still spread out instead of piling up.
	Ends map[string][2]float64 `json:"ends,omitempty"`
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

func place(value float64, cuts [3]float64, ends [2]float64, higherIsBetter bool) Placement {
	v, c, e := value, cuts, ends
	if !higherIsBetter {
		v, c, e = -value, [3]float64{-cuts[0], -cuts[1], -cuts[2]}, [2]float64{-ends[0], -ends[1]}
	}
	// The outer tiers reach the data's ends; without ends past the cuts, they are as wide as their neighbour.
	lo, hi := c[0]-(c[1]-c[0]), c[2]+(c[2]-c[1])
	if e[0] < c[0] {
		lo = e[0]
	}
	if e[1] > c[2] {
		hi = e[1]
	}
	edges := [5]float64{lo, c[0], c[1], c[2], hi}
	pos := 0.0
	switch {
	case v <= edges[0]:
		pos = 0
	case v >= edges[4]:
		pos = 4
	default:
		for i := 0; i < 4; i++ {
			if v <= edges[i+1] {
				span := edges[i+1] - edges[i]
				if span <= 0 {
					pos = float64(i + 1)
				} else {
					pos = float64(i) + (v-edges[i])/span
				}
				break
			}
		}
	}
	tier := 0
	for _, cut := range c {
		if v >= cut {
			tier++
		}
	}
	return Placement{Value: value, Position: pos, Tier: tier}
}

// Result is one player's game.
type Result struct {
	Metrics map[string]Placement `json:"metrics"`
	// Combined is the regression score placed on the tier axis; nil when a combined feature is missing.
	Combined *Placement `json:"combined,omitempty"`
	// Confident is true when the combined score is ConfidentMargin or more from every cut: on held-out games
	// that tier was right 70% of the time, against 55% for any score.
	Confident bool `json:"confident"`
}

// Evaluate places a player's measured values; race is "T", "P" or "Z", and context says what kind of game
// it was. Macro values of team games and money maps are placed as their 1v1 regular-map equivalents;
// Placement.Value stays the measured value.
func (m *Model) Evaluate(race, context string, values map[string]float64) Result {
	res := Result{Metrics: map[string]Placement{}}
	adjusted := map[string]float64{}
	for key, v := range values {
		adjusted[key] = v
		if q, ok := m.ContextAdjust[AdjustKey(context, race)][key]; ok {
			adjusted[key] = q.apply(v)
		}
	}
	for key, mc := range m.Metrics {
		scale := AllRaces
		if _, ok := mc.Cuts[scale]; !ok || mc.PerRace {
			scale = race
		}
		cuts, ok := mc.Cuts[scale]
		v, has := values[key]
		if !ok || !has {
			continue
		}
		p := place(adjusted[key], cuts, mc.Ends[scale], mc.HigherIsBetter)
		p.Value = v
		res.Metrics[key] = p
	}
	cr, ok := m.Combined.Races[race]
	if !ok {
		return res
	}
	score := cr.Intercept
	for i, f := range m.Combined.Features {
		v, has := adjusted[f]
		if !has {
			return res
		}
		z := (v - cr.Mean[i]) / cr.SD[i]
		score += cr.Weights[i] * max(-3, min(3, z))
	}
	p := place(score, cr.Cuts, cr.Ends, true)
	res.Combined = &p
	res.Confident = true
	for _, c := range cr.Cuts {
		if abs(score-c) < m.ConfidentMargin {
			res.Confident = false
		}
	}
	return res
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

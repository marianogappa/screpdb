// Package skill places a player's game on four tiers fitted to ladder MMR: Beginner (under 1,400),
// Intermediate (1,400 to 1,799), Advanced (1,800 and up) and Pro (labelled progamers).
//
// model.json is written by scripts/skill-fit from scripts/skill-mine's measurements of thousands of 1v1
// ladder games with known MMR. Only metrics that followed MMR there are in it; each has cut values per race
// between neighbouring tiers. The combined score is a per-race regression on the three mechanics metrics
// that worked for every race, and it is only called confident when it sits well away from a cut.
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

// Metric keys, as the miner names them.
const (
	EAPM             = "eapm"
	HotkeyRecalls    = "hotkey_recalls_per_min"
	ScreenSwitches   = "viewport_switches_per_min"
	SpendingQuotient = "spending_quotient"
	HallBusy         = "hall_worker_busy"
	LarvaPerHatch    = "larva_per_hatch"
	BankedShare      = "float_share"
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
}

// MetricCuts are, per race ("T", "P", "Z"), the values between Beginner|Intermediate, Intermediate|Advanced
// and Advanced|Pro, where as many of the lower tier score past the cut as of the upper tier fall short of it.
type MetricCuts struct {
	HigherIsBetter bool                  `json:"higher_is_better"`
	Rho            float64               `json:"rho"`
	Cuts           map[string][3]float64 `json:"cuts"`
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

func place(value float64, cuts [3]float64, higherIsBetter bool) Placement {
	v, c := value, cuts
	if !higherIsBetter {
		v, c = -value, [3]float64{-cuts[0], -cuts[1], -cuts[2]}
	}
	edges := [5]float64{c[0] - (c[1] - c[0]), c[0], c[1], c[2], c[2] + (c[2] - c[1])}
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

// Evaluate places a player's measured values; race is "T", "P" or "Z".
func (m *Model) Evaluate(race string, values map[string]float64) Result {
	res := Result{Metrics: map[string]Placement{}}
	for key, mc := range m.Metrics {
		cuts, ok := mc.Cuts[race]
		v, has := values[key]
		if ok && has {
			res.Metrics[key] = place(v, cuts, mc.HigherIsBetter)
		}
	}
	cr, ok := m.Combined.Races[race]
	if !ok {
		return res
	}
	score := cr.Intercept
	for i, f := range m.Combined.Features {
		v, has := values[f]
		if !has {
			return res
		}
		z := (v - cr.Mean[i]) / cr.SD[i]
		score += cr.Weights[i] * max(-3, min(3, z))
	}
	p := place(score, cr.Cuts, true)
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

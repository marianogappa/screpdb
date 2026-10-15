package dashboard

import (
	"math"

	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/propack"
)

// Every replay has the same shape in both modes: engine-only fields are null until the engine pass
// upgrades it, so the browser reserves their space from the first render.
const (
	modeFast   = "fast"
	modeEngine = "engine"

	engineConfidenceHigh = "high"
	engineConfidenceLow  = "low"

	framesPerSecond = 1000.0 / 42.0
)

type workflowEngineFields struct {
	Mode                      string  `json:"mode"`
	EngineConfidence          *string `json:"engine_confidence"`
	EngineTrustedUntilSeconds *int64  `json:"engine_trusted_until_seconds"`
}

func engineFieldsOf(r *library.Replay) workflowEngineFields {
	if r == nil || r.EngineData == nil {
		return workflowEngineFields{Mode: modeFast}
	}
	confidence := engineConfidenceHigh
	if r.EngineData.LowConfidence {
		confidence = engineConfidenceLow
	}
	f := workflowEngineFields{Mode: modeEngine, EngineConfidence: &confidence}
	if r.EngineData.LowConfidence {
		seconds := int64(math.Floor(float64(r.EngineData.TrustedUntil) / framesPerSecond))
		f.EngineTrustedUntilSeconds = &seconds
	}
	return f
}

func (d *Dashboard) engineFields(replayID int64) workflowEngineFields {
	if d.library == nil {
		return workflowEngineFields{Mode: modeFast}
	}
	r, _ := d.library.lib.Snapshot().ByID(replayID)
	return engineFieldsOf(r)
}

// engineCoverage is the "n of N games with engine data" every aggregate shows beside its numbers.
type engineCoverage struct {
	Games int `json:"games"`
	Total int `json:"total"`
}

func coverageOf(replays []*library.Replay) engineCoverage {
	c := engineCoverage{Total: len(replays)}
	for _, r := range replays {
		if r.EngineData != nil {
			c.Games++
		}
	}
	return c
}

func (d *Dashboard) engineCoverage() engineCoverage {
	if d.library == nil {
		return engineCoverage{}
	}
	return coverageOf(d.library.lib.View().Replays())
}

func (d *Dashboard) playerEngineCoverage(playerKey string) engineCoverage {
	if d.library == nil {
		return engineCoverage{}
	}
	var c engineCoverage
	for _, ref := range d.library.lib.View().PlayerGames(playerKey) {
		if ref.Player().IsObserver() {
			continue
		}
		c.Total++
		if ref.Replay.EngineData != nil {
			c.Games++
		}
	}
	return c
}

// workflowPlayerMacro is one player's macro scorecard; null in fast mode, and a ratio is null when the
// game gives it no basis (a Zerg has no idle worker production, a short game no blocks per 100 supply).
type workflowPlayerMacro struct {
	AnalysedSeconds      int64    `json:"analysed_seconds"`
	SupplyBlockedSeconds int64    `json:"supply_blocked_seconds"`
	SupplyBlocks         int64    `json:"supply_blocks"`
	BlocksPer100Supply   *float64 `json:"blocks_per_100_supply"`
	SpareSupply          *float64 `json:"spare_supply"`
	SpendingQuotient     *float64 `json:"spending_quotient"`
	IncomePerMinute      *float64 `json:"income_per_minute"`
	AverageUnspent       *float64 `json:"average_unspent"`
	WorkerIdleShare      *float64 `json:"worker_idle_share"`
	ProductionIdleShare  *float64 `json:"production_idle_share"`
	// WorkersBy6Min is null for a game shorter than 6:00.
	WorkersBy6Min      *int64   `json:"workers_by_6min"`
	SecondsTo40Workers *float64 `json:"seconds_to_40_workers"`
	FloatSeconds       int64    `json:"float_seconds"`
	IncomeCurve        []int64  `json:"income_curve"`
	WorkersCurve       []int64  `json:"workers_curve"`
	// ProRanges is the pro spread of each metric for the player's race, keyed by the metric's JSON name.
	ProRanges map[string]propack.MacroRange `json:"pro_ranges,omitempty"`
}

func finite(v float32) *float64 {
	f := float64(v)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil
	}
	return &f
}

func int64s(values []uint16) []int64 {
	out := make([]int64, len(values))
	for i, v := range values {
		out[i] = int64(v)
	}
	return out
}

func macroOf(p library.EnginePlayer) *workflowPlayerMacro {
	var workersBy6Min *int64
	if p.Seconds >= 360 {
		n := int64(p.WorkersBy6Min)
		workersBy6Min = &n
	}
	return &workflowPlayerMacro{
		AnalysedSeconds:      int64(p.Seconds),
		SupplyBlockedSeconds: int64(p.SupplyBlockedSeconds),
		SupplyBlocks:         int64(p.SupplyBlocks),
		BlocksPer100Supply:   finite(p.BlocksPer100Supply),
		SpareSupply:          finite(p.SpareSupply),
		SpendingQuotient:     finite(p.SpendingQuotient),
		IncomePerMinute:      finite(p.IncomePerMinute),
		AverageUnspent:       finite(p.AverageUnspent),
		WorkerIdleShare:      finite(p.WorkerIdleShare),
		ProductionIdleShare:  finite(p.ProductionIdleShare),
		WorkersBy6Min:        workersBy6Min,
		SecondsTo40Workers:   finite(p.SecondsTo40Workers),
		FloatSeconds:         int64(p.FloatSeconds),
		IncomeCurve:          int64s(p.Income),
		WorkersCurve:         int64s(p.Workers),
	}
}

func (d *Dashboard) populateEngineMacro(detail *workflowGameDetail) {
	if d.library == nil {
		return
	}
	rec, ok := d.library.lib.Snapshot().ByID(detail.ReplayID)
	if !ok || rec.EngineData == nil {
		return
	}
	ranges, err := propack.LoadMacroRanges()
	if err != nil {
		crashreport.Handle(err, nil, false)
	}
	bySlot := map[uint8]library.EnginePlayer{}
	for _, p := range rec.EngineData.Players {
		bySlot[p.Slot] = p
	}
	for i := range detail.Players {
		for ordinal, p := range rec.Players {
			if library.PlayerID(rec.ID, uint8(ordinal)) != detail.Players[i].PlayerID {
				continue
			}
			if m, ok := bySlot[p.Slot]; ok {
				detail.Players[i].Macro = macroOf(m)
				detail.Players[i].Macro.ProRanges = ranges.ForRace(detail.Players[i].Race)
			}
		}
	}
}

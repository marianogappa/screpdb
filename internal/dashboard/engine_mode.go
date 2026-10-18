package dashboard

import (
	"math"

	"github.com/marianogappa/screpdb/internal/library"
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

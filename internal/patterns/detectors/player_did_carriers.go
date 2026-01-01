package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// DidCarriersPlayerDetector detects if a player created at least 3 Carriers
type DidCarriersPlayerDetector struct {
	BasePlayerDetector
	countDetector CountDetector
}

// NewDidCarriersPlayerDetector creates a new player-level Carriers detector
func NewDidCarriersPlayerDetector() *DidCarriersPlayerDetector {
	return &DidCarriersPlayerDetector{
		countDetector: *NewCountDetector(),
	}
}

func (d *DidCarriersPlayerDetector) Name() string {
	return "Did Carriers"
}

func (d *DidCarriersPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}

	matcher := MatchUnitType("Carrier")
	if d.countDetector.ProcessCount(command, matcher, 3) {
		d.SetFinished(true)
		return true
	}
	return false
}

func (d *DidCarriersPlayerDetector) GetResult() *core.PatternResult {
	if !d.IsFinished() {
		return nil
	}
	// Only return a result if we detected at least 3 carriers (result is true)
	counts := d.countDetector.GetCounts()
	for _, count := range counts {
		if count >= 3 {
			valueBool := true
			return d.BuildPlayerResult(d.Name(), &valueBool, nil, nil, nil)
		}
	}
	return nil
}

func (d *DidCarriersPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && d.countDetector.HasAnyCountAbove(3)
}

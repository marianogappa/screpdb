package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// HadCarriersReplayDetector detects if any player in the replay created at least 3 Carriers
type HadCarriersReplayDetector struct {
	BaseReplayDetector
	countDetector CountDetector
}

// NewHadCarriersReplayDetector creates a new replay-level Carriers detector
func NewHadCarriersReplayDetector() *HadCarriersReplayDetector {
	return &HadCarriersReplayDetector{
		countDetector: *NewCountDetector(),
	}
}

func (d *HadCarriersReplayDetector) Name() string {
	return "Had Carriers"
}

func (d *HadCarriersReplayDetector) ProcessCommand(command *models.Command) bool {
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

func (d *HadCarriersReplayDetector) GetResult() *core.PatternResult {
	if !d.IsFinished() {
		return nil
	}
	// Only return a result if any player has 3+ Carriers (result is true)
	if d.countDetector.HasAnyCountAbove(3) {
		valueBool := true
		return d.BuildReplayResult(d.Name(), &valueBool, nil, nil, nil)
	}
	return nil
}

func (d *HadCarriersReplayDetector) ShouldSave() bool {
	return d.IsFinished() && d.countDetector.HasAnyCountAbove(3)
}

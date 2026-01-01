package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstCarrierBuildTriggeredReplayDetector detects the seconds to first Carrier build triggered in the replay
type SecondsToFirstCarrierBuildTriggeredReplayDetector struct {
	BaseReplayDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstCarrierBuildTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstCarrierBuildTriggeredReplayDetector() *SecondsToFirstCarrierBuildTriggeredReplayDetector {
	return &SecondsToFirstCarrierBuildTriggeredReplayDetector{}
}

func (d *SecondsToFirstCarrierBuildTriggeredReplayDetector) Name() string {
	return "Seconds to First Carrier Build Triggered"
}

func (d *SecondsToFirstCarrierBuildTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}

	matcher := MatchActionAndUnit(models.ActionTypeTrain, models.GeneralUnitCarrier)
	if d.firstOccurrence.ProcessFirstOccurrence(command, matcher) {
		d.SetFinished(true)
		return true
	}
	return false
}

func (d *SecondsToFirstCarrierBuildTriggeredReplayDetector) GetResult() *core.PatternResult {
	return d.BuildReplayResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstCarrierBuildTriggeredReplayDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}


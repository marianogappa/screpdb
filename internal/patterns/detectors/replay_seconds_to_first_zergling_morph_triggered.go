package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstZerglingMorphTriggeredReplayDetector detects the seconds to first Zergling morph triggered in the replay
type SecondsToFirstZerglingMorphTriggeredReplayDetector struct {
	BaseReplayDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstZerglingMorphTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstZerglingMorphTriggeredReplayDetector() *SecondsToFirstZerglingMorphTriggeredReplayDetector {
	return &SecondsToFirstZerglingMorphTriggeredReplayDetector{}
}

func (d *SecondsToFirstZerglingMorphTriggeredReplayDetector) Name() string {
	return "Seconds to First Zergling Morph Triggered"
}

func (d *SecondsToFirstZerglingMorphTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}

	matcher := MatchActionAndUnit(models.ActionTypeUnitMorph, models.GeneralUnitZergling)
	if d.firstOccurrence.ProcessFirstOccurrence(command, matcher) {
		d.SetFinished(true)
		return true
	}
	return false
}

func (d *SecondsToFirstZerglingMorphTriggeredReplayDetector) GetResult() *core.PatternResult {
	return d.BuildReplayResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstZerglingMorphTriggeredReplayDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}


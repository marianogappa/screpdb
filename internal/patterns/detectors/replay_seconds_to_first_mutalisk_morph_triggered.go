package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstMutaliskMorphTriggeredReplayDetector detects the seconds to first Mutalisk morph triggered in the replay
type SecondsToFirstMutaliskMorphTriggeredReplayDetector struct {
	BaseReplayDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstMutaliskMorphTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstMutaliskMorphTriggeredReplayDetector() *SecondsToFirstMutaliskMorphTriggeredReplayDetector {
	return &SecondsToFirstMutaliskMorphTriggeredReplayDetector{}
}

func (d *SecondsToFirstMutaliskMorphTriggeredReplayDetector) Name() string {
	return "Seconds to First Mutalisk Morph Triggered"
}

func (d *SecondsToFirstMutaliskMorphTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}

	matcher := MatchActionAndUnit(models.ActionTypeUnitMorph, models.GeneralUnitMutalisk)
	if d.firstOccurrence.ProcessFirstOccurrence(command, matcher) {
		d.SetFinished(true)
		return true
	}
	return false
}

func (d *SecondsToFirstMutaliskMorphTriggeredReplayDetector) GetResult() *core.PatternResult {
	return d.BuildReplayResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstMutaliskMorphTriggeredReplayDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}


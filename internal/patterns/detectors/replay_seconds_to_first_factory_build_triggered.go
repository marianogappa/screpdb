package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstFactoryBuildTriggeredReplayDetector detects the seconds to first Factory build triggered in the replay
type SecondsToFirstFactoryBuildTriggeredReplayDetector struct {
	BaseReplayDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstFactoryBuildTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstFactoryBuildTriggeredReplayDetector() *SecondsToFirstFactoryBuildTriggeredReplayDetector {
	return &SecondsToFirstFactoryBuildTriggeredReplayDetector{}
}

func (d *SecondsToFirstFactoryBuildTriggeredReplayDetector) Name() string {
	return "Seconds to First Factory Build Triggered"
}

func (d *SecondsToFirstFactoryBuildTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}

	matcher := MatchActionAndUnit(models.ActionTypeBuild, models.GeneralUnitFactory)
	if d.firstOccurrence.ProcessFirstOccurrence(command, matcher) {
		d.SetFinished(true)
		return true
	}
	return false
}

func (d *SecondsToFirstFactoryBuildTriggeredReplayDetector) GetResult() *core.PatternResult {
	return d.BuildReplayResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstFactoryBuildTriggeredReplayDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}

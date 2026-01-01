package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstGatewayBuildTriggeredReplayDetector detects the seconds to first Gateway build triggered in the replay
type SecondsToFirstGatewayBuildTriggeredReplayDetector struct {
	BaseReplayDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstGatewayBuildTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstGatewayBuildTriggeredReplayDetector() *SecondsToFirstGatewayBuildTriggeredReplayDetector {
	return &SecondsToFirstGatewayBuildTriggeredReplayDetector{}
}

func (d *SecondsToFirstGatewayBuildTriggeredReplayDetector) Name() string {
	return "Seconds to First Gateway Build Triggered"
}

func (d *SecondsToFirstGatewayBuildTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}

	matcher := MatchActionAndUnit(models.ActionTypeBuild, models.GeneralUnitGateway)
	if d.firstOccurrence.ProcessFirstOccurrence(command, matcher) {
		d.SetFinished(true)
		return true
	}
	return false
}

func (d *SecondsToFirstGatewayBuildTriggeredReplayDetector) GetResult() *core.PatternResult {
	return d.BuildReplayResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstGatewayBuildTriggeredReplayDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}


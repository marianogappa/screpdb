package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstGatewayBuildTriggeredPlayerDetector detects the seconds to first Gateway build triggered for a player
type SecondsToFirstGatewayBuildTriggeredPlayerDetector struct {
	BasePlayerDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstGatewayBuildTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstGatewayBuildTriggeredPlayerDetector() *SecondsToFirstGatewayBuildTriggeredPlayerDetector {
	return &SecondsToFirstGatewayBuildTriggeredPlayerDetector{}
}

func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) Name() string {
	return "Seconds to First Gateway Build Triggered"
}

func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
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

func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) GetResult() *core.PatternResult {
	return d.BuildPlayerResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}


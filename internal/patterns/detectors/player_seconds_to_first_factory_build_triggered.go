package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstFactoryBuildTriggeredPlayerDetector detects the seconds to first Factory build triggered for a player
type SecondsToFirstFactoryBuildTriggeredPlayerDetector struct {
	BasePlayerDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstFactoryBuildTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstFactoryBuildTriggeredPlayerDetector() *SecondsToFirstFactoryBuildTriggeredPlayerDetector {
	return &SecondsToFirstFactoryBuildTriggeredPlayerDetector{}
}

func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) Name() string {
	return "Seconds to First Factory Build Triggered"
}

func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
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

func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) GetResult() *core.PatternResult {
	return d.BuildPlayerResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}

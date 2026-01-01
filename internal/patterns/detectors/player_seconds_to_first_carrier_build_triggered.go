package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstCarrierBuildTriggeredPlayerDetector detects the seconds to first Carrier build triggered for a player
type SecondsToFirstCarrierBuildTriggeredPlayerDetector struct {
	BasePlayerDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstCarrierBuildTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstCarrierBuildTriggeredPlayerDetector() *SecondsToFirstCarrierBuildTriggeredPlayerDetector {
	return &SecondsToFirstCarrierBuildTriggeredPlayerDetector{}
}

func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) Name() string {
	return "Seconds to First Carrier Build Triggered"
}

func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
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

func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) GetResult() *core.PatternResult {
	return d.BuildPlayerResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}

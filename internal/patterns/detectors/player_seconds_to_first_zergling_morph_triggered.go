package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstZerglingMorphTriggeredPlayerDetector detects the seconds to first Zergling morph triggered for a player
type SecondsToFirstZerglingMorphTriggeredPlayerDetector struct {
	BasePlayerDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstZerglingMorphTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstZerglingMorphTriggeredPlayerDetector() *SecondsToFirstZerglingMorphTriggeredPlayerDetector {
	return &SecondsToFirstZerglingMorphTriggeredPlayerDetector{}
}

func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) Name() string {
	return "Seconds to First Zergling Morph Triggered"
}

func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
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

func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) GetResult() *core.PatternResult {
	return d.BuildPlayerResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}

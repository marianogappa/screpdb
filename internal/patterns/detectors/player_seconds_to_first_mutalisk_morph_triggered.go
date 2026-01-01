package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstMutaliskMorphTriggeredPlayerDetector detects the seconds to first Mutalisk morph triggered for a player
type SecondsToFirstMutaliskMorphTriggeredPlayerDetector struct {
	BasePlayerDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstMutaliskMorphTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstMutaliskMorphTriggeredPlayerDetector() *SecondsToFirstMutaliskMorphTriggeredPlayerDetector {
	return &SecondsToFirstMutaliskMorphTriggeredPlayerDetector{}
}

func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) Name() string {
	return "Seconds to First Mutalisk Morph Triggered"
}

func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
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

func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) GetResult() *core.PatternResult {
	return d.BuildPlayerResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}


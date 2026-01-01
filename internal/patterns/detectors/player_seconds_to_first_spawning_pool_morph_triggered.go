package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector detects the seconds to first Spawning Pool morph triggered for a player
type SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector struct {
	BasePlayerDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstSpawningPoolMorphTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstSpawningPoolMorphTriggeredPlayerDetector() *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector {
	return &SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector{}
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) Name() string {
	return "Seconds to First Spawning Pool Morph Triggered"
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}

	matcher := MatchActionAndUnit(models.ActionTypeBuild, models.GeneralUnitSpawningPool)
	if d.firstOccurrence.ProcessFirstOccurrence(command, matcher) {
		d.SetFinished(true)
		return true
	}
	return false
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) GetResult() *core.PatternResult {
	return d.BuildPlayerResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}


package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstSpawningPoolMorphTriggeredReplayDetector detects the seconds to first Spawning Pool morph triggered in the replay
type SecondsToFirstSpawningPoolMorphTriggeredReplayDetector struct {
	BaseReplayDetector
	firstOccurrence FirstOccurrenceDetector
}

// NewSecondsToFirstSpawningPoolMorphTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstSpawningPoolMorphTriggeredReplayDetector() *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector {
	return &SecondsToFirstSpawningPoolMorphTriggeredReplayDetector{}
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector) Name() string {
	return "Seconds to First Spawning Pool Morph Triggered"
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
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

func (d *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector) GetResult() *core.PatternResult {
	return d.BuildReplayResult(d.Name(), nil, d.firstOccurrence.GetSeconds(), nil, nil)
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector) ShouldSave() bool {
	return d.IsFinished() && d.firstOccurrence.IsMatched()
}


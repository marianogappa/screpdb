package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// DidCarriersTeamDetector detects if any player on a team created at least 3 Carriers
type DidCarriersTeamDetector struct {
	BaseTeamDetector
	countDetector CountDetector
}

// NewDidCarriersTeamDetector creates a new team-level Carriers detector
func NewDidCarriersTeamDetector() *DidCarriersTeamDetector {
	return &DidCarriersTeamDetector{
		countDetector: *NewCountDetector(),
	}
}

func (d *DidCarriersTeamDetector) Name() string {
	return "Did Carriers"
}

func (d *DidCarriersTeamDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}

	matcher := MatchUnitType("Carrier")
	if d.countDetector.ProcessCount(command, matcher, 3) {
		d.SetFinished(true)
		return true
	}
	return false
}

func (d *DidCarriersTeamDetector) GetResult() *core.PatternResult {
	if !d.IsFinished() {
		return nil
	}
	// Only return a result if any player on the team has 3+ Carriers (result is true)
	if d.countDetector.HasAnyCountAbove(3) {
		valueBool := true
		return d.BuildTeamResult(d.Name(), &valueBool, nil, nil, nil)
	}
	return nil
}

func (d *DidCarriersTeamDetector) ShouldSave() bool {
	return d.IsFinished() && d.countDetector.HasAnyCountAbove(3)
}

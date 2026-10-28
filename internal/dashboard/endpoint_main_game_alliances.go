package dashboard

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/parser"
)

// populateAllianceTimelineForGameDetail attaches the alliance topology
// timeline ingest computed for the game. Games it did not analyse (not Melee,
// or two players or fewer) have none, and the field stays empty.
func (d *Dashboard) populateAllianceTimelineForGameDetail(detail *workflowGameDetail) error {
	if detail.GameType != "Melee" {
		return nil
	}
	rows, err := d.dbStore.ListReplayAllianceSnapshots(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to list alliance snapshots: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	out := make([]workflowAllianceSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, workflowAllianceSnapshot{Sec: row.Second, Teams: row.Teams, Stacking: row.Stacking})
	}
	detail.AllianceTimeline = out
	detail.AllianceStackingThresholdSeconds = parser.StackingThresholdSec
	return nil
}

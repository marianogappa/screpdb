package db

import (
	"context"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

// EarlyZergTimingsRow is one Zerg player's morph / build timings in the
// early game window. DroneMorphSecs is the full ordered list (1st, 2nd,
// ...); the building / unit times are first-only.
type EarlyZergTimingsRow struct {
	PlayerID         int64
	DroneMorphSecs   []int
	FirstOverlordSec *int
	FirstPoolSec     *int
	FirstHatcherySec *int
}

// LoadEarlyZergTimings returns one row per Zerg player in the replay
// containing the in-game seconds of their early Drone morphs and the
// first Overlord / Spawning Pool / Hatchery commands. Used by the
// build-orders detail tab to render BO milestones (drone-numbered ticks,
// pool/hatch/overlord) without persisting them per detection.
func (s *Store) LoadEarlyZergTimings(ctx context.Context, replayID int64) ([]EarlyZergTimingsRow, error) {
	rows, err := sqlcgen.New(Trace(s.replayScoped())).ListEarlyZergMorphsForBOTimings(ctx, replayID)
	if err != nil {
		return nil, err
	}

	byPlayer := map[int64]*EarlyZergTimingsRow{}
	for _, r := range rows {
		row, ok := byPlayer[r.PlayerID]
		if !ok {
			row = &EarlyZergTimingsRow{PlayerID: r.PlayerID}
			byPlayer[r.PlayerID] = row
		}
		unit := ""
		if r.UnitType != nil {
			unit = *r.UnitType
		}
		sec := int(r.SecondsFromGameStart)
		switch {
		case r.ActionType == "Unit Morph" && unit == "Drone":
			row.DroneMorphSecs = append(row.DroneMorphSecs, sec)
		case r.ActionType == "Unit Morph" && unit == "Overlord":
			if row.FirstOverlordSec == nil {
				v := sec
				row.FirstOverlordSec = &v
			}
		case r.ActionType == "Build" && unit == "Spawning Pool":
			if row.FirstPoolSec == nil {
				v := sec
				row.FirstPoolSec = &v
			}
		case r.ActionType == "Build" && unit == "Hatchery":
			if row.FirstHatcherySec == nil {
				v := sec
				row.FirstHatcherySec = &v
			}
		}
	}

	out := make([]EarlyZergTimingsRow, 0, len(byPlayer))
	for _, r := range byPlayer {
		out = append(out, *r)
	}
	return out, nil
}

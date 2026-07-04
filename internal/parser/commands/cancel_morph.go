package commands

import "github.com/marianogappa/screpdb/internal/models"

// DropCancelledMorphs accounts for a larva morph the player cancelled before
// their first Overlord. While no Overlord has morphed, a cancelled egg can only
// be a Drone (there is nothing else in an egg that early), so it refunds one
// supply and must not inflate the "N Pool" / "N Hatch" opener count. After the
// first Overlord morph a cancel is ambiguous (Drone or Overlord) and is left
// untouched — the opener supply is already fixed by then.
//
// Each Cancel Morph cancels one unit of the most recent still-standing Drone
// morph (LIFO): a single morph is dropped from the stream, a multi-larva morph
// (MorphUnitCount > 1) has its count decremented by one. Operates per player
// over the time-ordered command stream and returns a fresh slice; the input
// *models.Command pointers are reused (MorphUnitCount may be mutated).
func DropCancelledMorphs(commands []*models.Command) []*models.Command {
	type droneMorph struct {
		idx       int
		remaining int
	}
	stacks := map[int64][]*droneMorph{}
	overlordSeen := map[int64]bool{}
	var tracked []*droneMorph
	dropped := map[int]bool{}

	for i, cmd := range commands {
		if cmd == nil {
			continue
		}
		pid := commandPlayerID(cmd)
		switch cmd.ActionType {
		case models.ActionTypeUnitMorph:
			if overlordSeen[pid] || cmd.UnitType == nil {
				continue
			}
			switch *cmd.UnitType {
			case models.UnitNameOverlord:
				overlordSeen[pid] = true
			case models.UnitNameDrone:
				count := cmd.MorphUnitCount
				if count < 1 {
					count = 1
				}
				dm := &droneMorph{idx: i, remaining: count}
				stacks[pid] = append(stacks[pid], dm)
				tracked = append(tracked, dm)
			}
		case models.ActionTypeCancelMorph:
			if overlordSeen[pid] {
				continue
			}
			stack := stacks[pid]
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			top.remaining--
			if top.remaining <= 0 {
				dropped[top.idx] = true
				stacks[pid] = stack[:len(stack)-1]
			}
		}
	}

	// Apply count reductions for partially-cancelled multi-larva morphs. For a
	// fully-cancelled single morph this is a no-op (it is excluded below).
	for _, dm := range tracked {
		if dropped[dm.idx] {
			continue
		}
		commands[dm.idx].MorphUnitCount = dm.remaining
	}

	if len(dropped) == 0 {
		return commands
	}
	out := make([]*models.Command, 0, len(commands))
	for i, cmd := range commands {
		if dropped[i] {
			continue
		}
		out = append(out, cmd)
	}
	return out
}

func commandPlayerID(cmd *models.Command) int64 {
	if cmd.Player != nil {
		return int64(cmd.Player.PlayerID)
	}
	return cmd.PlayerID
}

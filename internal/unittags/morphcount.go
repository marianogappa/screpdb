package unittags

import (
	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
)

// MorphSelectionSizes reconstructs, for every Zerg larva-morph command, the
// number of units selected when the command fired — keyed by the command's
// index into r.Commands.Cmds.
//
// A single Brood War larva-morph command morphs every selected larva at once
// (select three larvae, press Drone → one command, three Drones), but screp
// records it as one command carrying one unit type. Counting commands therefore
// undercounts the player's true supply and misclassifies the opening (an 11
// Hatch read as 10 Hatch). This selection size is the *intended* multiplicity;
// the early-game filter caps it by the larva and minerals actually available.
//
// Selection is tracked exactly as Analyze does: direct Select/SelectAdd/
// SelectRemove plus hotkey assign/select/add. Only larva-morph commands get an
// entry; everything else is absent (callers treat absent as 1).
func MorphSelectionSizes(r *rep.Replay) map[int]int {
	out := map[int]int{}
	if r == nil || r.Commands == nil {
		return out
	}
	type sel struct {
		cur    []uint16
		groups map[byte][]uint16
	}
	states := map[byte]*sel{}
	get := func(pid byte) *sel {
		if states[pid] == nil {
			states[pid] = &sel{groups: map[byte][]uint16{}}
		}
		return states[pid]
	}
	for i, c := range r.Commands.Cmds {
		b := c.BaseCmd()
		if b == nil || b.Type == nil {
			continue
		}
		s := get(b.PlayerID)
		switch b.Type.ID {
		case repcmd.TypeIDSelect, repcmd.TypeIDSelect121:
			if sc, ok := c.(*repcmd.SelectCmd); ok {
				s.cur = tagsOf(sc.UnitTags)
			}
		case repcmd.TypeIDSelectAdd, repcmd.TypeIDSelectAdd121:
			if sc, ok := c.(*repcmd.SelectCmd); ok {
				s.cur = unionTags(s.cur, tagsOf(sc.UnitTags))
			}
		case repcmd.TypeIDSelectRemove, repcmd.TypeIDSelectRemove121:
			if sc, ok := c.(*repcmd.SelectCmd); ok {
				s.cur = removeTags(s.cur, tagsOf(sc.UnitTags))
			}
		case repcmd.TypeIDHotkey:
			if hc, ok := c.(*repcmd.HotkeyCmd); ok && hc.HotkeyType != nil {
				switch hc.HotkeyType.Name {
				case "Assign":
					s.groups[hc.Group] = append([]uint16(nil), s.cur...)
				case "Select":
					s.cur = append([]uint16(nil), s.groups[hc.Group]...)
				case "Add":
					s.cur = unionTags(s.cur, s.groups[hc.Group])
				}
			}
		case repcmd.TypeIDUnitMorph:
			tc, ok := c.(*repcmd.TrainCmd)
			if !ok || tc.Unit == nil {
				continue
			}
			if larvaMorphUnits[tc.Unit.Name] && len(s.cur) > 0 {
				out[i] = len(s.cur)
			}
		}
	}
	return out
}

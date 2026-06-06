// Package unittags reconstructs per-player selection state from a StarCraft:
// Brood War replay's raw command stream and attributes production and worker
// actions to the specific in-game unit tags that issued them.
//
// Build / Train / Unit-Morph commands do not name the structure that produced
// the unit, but Select / Hotkey commands carry the selected units' tags. By
// replaying selection state we can bind, with high confidence, each single-unit
// selection to the building tag that produced from it — the evidence the
// internal/builddedup strategies consume.
//
// This operates on the raw screp stream (rep.Commands.Cmds) because screpdb's
// normal parser discards Select commands and their tags.
package unittags

import (
	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
)

// producerBuilding maps a produced unit's name to the building type that
// produces it. Protoss and Terran only: their Train command operates on the
// selected production building, so a single-unit selection at train time names
// that building's tag. (Zerg unit-morphs select larva, not the Hatchery, so
// they are deliberately absent — see package builddedup scope notes.)
var producerBuilding = map[string]string{
	"Probe": "Nexus", "Zealot": "Gateway", "Dragoon": "Gateway",
	"High Templar": "Gateway", "Dark Templar": "Gateway",
	"Reaver": "Robotics Facility", "Shuttle": "Robotics Facility", "Observer": "Robotics Facility",
	"Scout": "Stargate", "Carrier": "Stargate", "Arbiter": "Stargate", "Corsair": "Stargate",
	"SCV":    "Command Center",
	"Marine": "Barracks", "Firebat": "Barracks", "Ghost": "Barracks", "Medic": "Barracks",
	"Vulture": "Factory", "Siege Tank": "Factory", "Goliath": "Factory",
	"Wraith": "Starport", "Dropship": "Starport", "Science Vessel": "Starport",
	"Valkyrie": "Starport", "Battlecruiser": "Starport",
}

// addonParent maps a Terran add-on to its parent building type. Building an
// add-on proves the parent existed (the "add-on ⇒ parent" existence law).
var addonParent = map[string]string{
	"Machine Shop": "Factory", "Control Tower": "Starport",
	"Comsat Station": "Command Center", "Nuclear Silo": "Command Center",
}

// WorkerBuild records one Build command issued with a single worker selected.
type WorkerBuild struct {
	PlayerID byte
	Frame    int32
	Sec      int
	Building string
	Worker   uint16 // the selected worker's unit tag
	X, Y     int    // build placement tile
}

// Build records one Build command (any selection state).
type Build struct {
	Frame int32
	Sec   int
	X, Y  int
}

// Production records what a single producing building tag did.
type Production struct {
	FirstSec int
	Units    int
}

// PlayerEvidence is the per-player evidence extracted from selection state.
type PlayerEvidence struct {
	// WorkerBuilds is every single-worker-selected Build command, in stream order.
	WorkerBuilds []WorkerBuild
	// Builds maps building name -> all Build commands for it.
	Builds map[string][]Build
	// Producers maps building type -> producing tag -> what it produced.
	Producers map[string]map[uint16]*Production
	// Addons maps building type -> set of tags proven to exist by an add-on.
	Addons map[string]map[uint16]bool
}

// Evidence holds per-player evidence keyed by replay PlayerID.
type Evidence struct {
	Players map[byte]*PlayerEvidence
}

// Analyze walks the raw command stream and returns selection-derived evidence.
func Analyze(r *rep.Replay) *Evidence {
	ev := &Evidence{Players: map[byte]*PlayerEvidence{}}
	if r == nil || r.Commands == nil {
		return ev
	}

	type selState struct {
		cur    []uint16
		groups map[byte][]uint16
	}
	states := map[byte]*selState{}
	get := func(pid byte) (*selState, *PlayerEvidence) {
		if states[pid] == nil {
			states[pid] = &selState{groups: map[byte][]uint16{}}
			ev.Players[pid] = &PlayerEvidence{
				Builds:    map[string][]Build{},
				Producers: map[string]map[uint16]*Production{},
				Addons:    map[string]map[uint16]bool{},
			}
		}
		return states[pid], ev.Players[pid]
	}

	for _, c := range r.Commands.Cmds {
		b := c.BaseCmd()
		if b == nil || b.Type == nil {
			continue
		}
		s, pe := get(b.PlayerID)
		sec := int(b.Frame.Seconds())

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
		case repcmd.TypeIDBuild:
			bc, ok := c.(*repcmd.BuildCmd)
			if !ok || bc.Unit == nil {
				continue
			}
			name := bc.Unit.Name
			pe.Builds[name] = append(pe.Builds[name], Build{
				Frame: int32(b.Frame), Sec: sec, X: int(bc.Pos.X), Y: int(bc.Pos.Y),
			})
			if len(s.cur) == 1 {
				pe.WorkerBuilds = append(pe.WorkerBuilds, WorkerBuild{
					PlayerID: b.PlayerID, Frame: int32(b.Frame), Sec: sec, Building: name, Worker: s.cur[0],
					X: int(bc.Pos.X), Y: int(bc.Pos.Y),
				})
				if pt, ok := addonParent[name]; ok {
					if pe.Addons[pt] == nil {
						pe.Addons[pt] = map[uint16]bool{}
					}
					pe.Addons[pt][s.cur[0]] = true
				}
			}
		case repcmd.TypeIDTrain, repcmd.TypeIDUnitMorph:
			tc, ok := c.(*repcmd.TrainCmd)
			if !ok || tc.Unit == nil || len(s.cur) != 1 {
				continue
			}
			bldg, ok := producerBuilding[tc.Unit.Name]
			if !ok {
				continue
			}
			tag := s.cur[0]
			if pe.Producers[bldg] == nil {
				pe.Producers[bldg] = map[uint16]*Production{}
			}
			p := pe.Producers[bldg][tag]
			if p == nil {
				p = &Production{FirstSec: sec}
				pe.Producers[bldg][tag] = p
			}
			p.Units++
		}
	}
	return ev
}

func tagsOf(ut []repcmd.UnitTag) []uint16 {
	out := make([]uint16, 0, len(ut))
	for _, t := range ut {
		out = append(out, uint16(t))
	}
	return out
}

func unionTags(a, b []uint16) []uint16 {
	seen := make(map[uint16]bool, len(a)+len(b))
	out := make([]uint16, 0, len(a)+len(b))
	for _, x := range a {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	for _, x := range b {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func removeTags(a, b []uint16) []uint16 {
	drop := make(map[uint16]bool, len(b))
	for _, x := range b {
		drop[x] = true
	}
	out := make([]uint16, 0, len(a))
	for _, x := range a {
		if !drop[x] {
			out = append(out, x)
		}
	}
	return out
}

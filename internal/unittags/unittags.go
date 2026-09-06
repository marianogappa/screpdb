// Package unittags reconstructs per-player selection state from the raw command
// stream and attributes production and worker actions to the in-game unit tags
// that issued them.
//
// Build / Train / Unit-Morph commands don't name the structure that produced the
// unit, but Select / Hotkey commands carry the selected units' tags — so
// replaying selection state binds each single-unit selection to the producing
// building's tag, which is the evidence internal/builddedup consumes.
//
// It works on the raw screp stream because screpdb's normal parser discards
// Select commands and their tags.
package unittags

import (
	"sort"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
)

// Protoss and Terran only: their Train operates on the selected production
// building, so a single-unit selection at train time names that building's tag.
// Zerg unit-morphs select larva, not the Hatchery, so they are absent.
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

// Building an add-on proves the parent existed (the "add-on ⇒ parent" law).
var addonParent = map[string]string{
	"Machine Shop": "Factory", "Control Tower": "Starport",
	"Comsat Station": "Command Center", "Nuclear Silo": "Command Center",
}

// larvaMorphUnits morph from a town hall's larvae. Unlike Terran/Protoss
// trains, a larva-morph selects the LARVAE, not the hall, so the producing hall
// is recovered from the selection just before the larvae select. Non-larva
// morphs (Lurker, Guardian, building morphs) select the morphing unit and are
// deliberately absent.
var larvaMorphUnits = map[string]bool{
	"Drone": true, "Zergling": true, "Hydralisk": true, "Mutalisk": true,
	"Overlord": true, "Defiler": true, "Ultralisk": true, "Queen": true,
	"Scourge": true, "Infested Terran": true,
}

// The synthetic producer key for Zerg town-hall larva production. Larva morphs
// are attributed to the hall tag selected just before the larvae, so the same
// tag→Build-location correlation used for Terran/Protoss applies. The key is
// "Hatchery" so it correlates against Hatchery Build placements — Lair and Hive
// are tag-preserving morphs of an existing Hatchery.
const zergTownHall = "Hatchery"

type WorkerBuild struct {
	PlayerID byte
	Frame    int32
	Sec      int
	Building string
	Worker   uint16 // the selected worker's unit tag
	X, Y     int    // build placement tile
}

// Build records one Build command, at any selection state.
type Build struct {
	Frame int32
	Sec   int
	X, Y  int
}

type Production struct {
	FirstSec int
	Units    int
	// Secs is every Train/Morph second from this tag, in stream order. The
	// ownership pass refreshes base ownership at each: a producing building proves
	// the base is still alive.
	Secs []int
}

// ProductionSignal is one "the producing building is alive here" datapoint. The
// location is the build tile when Anchored, and the player's start base when not
// — the spawn-seeded starting hall, or a tag that matched no Build command.
type ProductionSignal struct {
	Sec      int
	X, Y     int // build placement tile; meaningful only when Anchored
	Anchored bool
}

type PlayerEvidence struct {
	// Every single-worker-selected Build command, in stream order.
	WorkerBuilds []WorkerBuild
	// building name -> all Build commands for it.
	Builds map[string][]Build
	// building type -> producing tag -> what it produced.
	Producers map[string]map[uint16]*Production
	// building type -> tags proven to exist by an add-on.
	Addons map[string]map[uint16]bool
	// Time-ordered production-location datapoints, populated by
	// attributeProductionLocations at the end of Analyze.
	ProductionSignals []ProductionSignal
}

// Evidence is keyed by replay PlayerID.
type Evidence struct {
	Players map[byte]*PlayerEvidence
}

func Analyze(r *rep.Replay) *Evidence {
	ev := &Evidence{Players: map[byte]*PlayerEvidence{}}
	if r == nil || r.Commands == nil {
		return ev
	}

	type selState struct {
		cur    []uint16
		groups map[byte][]uint16
		// prevSingle is the tag selected immediately before the current selection, when
		// that prior selection held exactly one unit. A Zerg larva morph selects larvae,
		// not the hall, so the hall is recovered from this prior single-select (the
		// macro-cycle hatch tap).
		prevSingle      uint16
		prevSingleValid bool
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

	// Record the single-ness of the current selection before it is replaced, so a
	// following larva morph can attribute to the prior hall tap.
	snap := func(s *selState) {
		if len(s.cur) == 1 {
			s.prevSingle, s.prevSingleValid = s.cur[0], true
		} else {
			s.prevSingleValid = false
		}
	}

	recordProduction := func(pe *PlayerEvidence, bldg string, tag uint16, sec int) {
		if pe.Producers[bldg] == nil {
			pe.Producers[bldg] = map[uint16]*Production{}
		}
		p := pe.Producers[bldg][tag]
		if p == nil {
			p = &Production{FirstSec: sec}
			pe.Producers[bldg][tag] = p
		}
		p.Units++
		p.Secs = append(p.Secs, sec)
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
				snap(s)
				s.cur = tagsOf(sc.UnitTags)
			}
		case repcmd.TypeIDSelectAdd, repcmd.TypeIDSelectAdd121:
			if sc, ok := c.(*repcmd.SelectCmd); ok {
				snap(s)
				s.cur = unionTags(s.cur, tagsOf(sc.UnitTags))
			}
		case repcmd.TypeIDSelectRemove, repcmd.TypeIDSelectRemove121:
			if sc, ok := c.(*repcmd.SelectCmd); ok {
				snap(s)
				s.cur = removeTags(s.cur, tagsOf(sc.UnitTags))
			}
		case repcmd.TypeIDHotkey:
			if hc, ok := c.(*repcmd.HotkeyCmd); ok && hc.HotkeyType != nil {
				switch hc.HotkeyType.Name {
				case "Assign":
					s.groups[hc.Group] = append([]uint16(nil), s.cur...)
				case "Select":
					snap(s)
					s.cur = append([]uint16(nil), s.groups[hc.Group]...)
				case "Add":
					// Shift+number ADDS the current selection to the group; the selection itself
					// is unchanged.
					s.groups[hc.Group] = unionTags(s.groups[hc.Group], s.cur)
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
			if !ok || tc.Unit == nil {
				continue
			}
			name := tc.Unit.Name
			if bldg, ok := producerBuilding[name]; ok {
				// Terran/Protoss: the producing building IS the single-selected unit.
				if len(s.cur) == 1 {
					recordProduction(pe, bldg, s.cur[0], sec)
				}
				continue
			}
			if larvaMorphUnits[name] && s.prevSingleValid {
				// Zerg larva morph: attribute to the hall tag tapped just before the larvae.
				recordProduction(pe, zergTownHall, s.prevSingle, sec)
			}
		}
	}

	for _, pe := range ev.Players {
		attributeProductionLocations(pe)
	}
	return ev
}

// A Hatchery footprint is 4 build-tiles by 3. Two Build(Hatchery) commands whose
// footprints overlap can't both be standing bases — they are one intended
// Hatchery placed then re-placed. Overlap, not a time gap, is the reliable
// signal: a re-place can land tens of seconds later (issue #245: 46s at the same
// tile), while two genuinely distinct bases are always ≥ a footprint apart.
const (
	hatcheryTileWidth  = 4
	hatcheryTileHeight = 3
)

// TownHallBuildSeconds returns per-player sorted seconds of distinct expansion
// town-hall Builds, with footprint-overlapping re-placements collapsed. The count
// drives the "N Hatch <tech>" base tally (issue #245).
//
// Builds come from the RAW stream, not screpdb's deduped one, deliberately: the
// standard build-dedup pass can drop a genuine expansion — its tag attribution
// is tuned for ownership, not exact counting — which would under-count N.
//
// The spawn-seeded starting hall has no Build command and is absent; callers
// count it as base 1. Keying the count on which tags morphed larvae was tried
// and rejected: tag recycling inflates that set while larva attribution misses
// halls the player never tap-selected, so it both over- and under-counts.
func TownHallBuildSeconds(ev *Evidence) map[byte][]int {
	out := map[byte][]int{}
	if ev == nil {
		return out
	}
	for pid, pe := range ev.Players {
		secs := collapsedBuildSeconds(pe.Builds[zergTownHall])
		if len(secs) > 0 {
			out[pid] = secs
		}
	}
	return out
}

// collapsedBuildSeconds drops any build whose footprint overlaps an earlier kept
// one (a re-placement of the same intended structure). Processed in time order,
// so the first placement at a spot is the one kept.
func collapsedBuildSeconds(builds []Build) []int {
	if len(builds) == 0 {
		return nil
	}
	sorted := append([]Build(nil), builds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Sec < sorted[j].Sec })
	var kept []Build
	for _, b := range sorted {
		overlap := false
		for _, k := range kept {
			if abs(b.X-k.X) < hatcheryTileWidth && abs(b.Y-k.Y) < hatcheryTileHeight {
				overlap = true
				break
			}
		}
		if !overlap {
			kept = append(kept, b)
		}
	}
	secs := make([]int, len(kept))
	for i, b := range kept {
		secs[i] = b.Sec
	}
	sort.Ints(secs)
	return secs
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// attributeProductionLocations maps each producing tag to a build placement, so
// production refreshes the right base, then emits one signal per production
// second. Tags matching no Build — the spawn-seeded starting hall, or unmatched
// extras — emit Anchored:false for the ownership pass to resolve to the start.
func attributeProductionLocations(pe *PlayerEvidence) {
	var signals []ProductionSignal
	for bldg, tags := range pe.Producers {
		loc := matchProducerTagsToBuilds(tags, pe.Builds[bldg])
		for tag, p := range tags {
			b, anchored := loc[tag]
			for _, sec := range p.Secs {
				sig := ProductionSignal{Sec: sec, Anchored: anchored}
				if anchored {
					sig.X, sig.Y = b.X, b.Y
				}
				signals = append(signals, sig)
			}
		}
	}
	sort.Slice(signals, func(i, j int) bool { return signals[i].Sec < signals[j].Sec })
	pe.ProductionSignals = signals
}

// matchProducerTagsToBuilds greedily assigns each tag (earliest first producer
// wins) to the earliest unclaimed Build placed at or before its first production,
// since a building cannot produce before it is commanded. Unmatched tags are
// absent: the starting hall has no Build, and surplus tags fall back to the start
// base in the ownership pass.
func matchProducerTagsToBuilds(tags map[uint16]*Production, builds []Build) map[uint16]Build {
	type tagFirst struct {
		tag      uint16
		firstSec int
	}
	ordered := make([]tagFirst, 0, len(tags))
	for tag, p := range tags {
		ordered = append(ordered, tagFirst{tag: tag, firstSec: p.FirstSec})
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].firstSec < ordered[j].firstSec })

	sorted := append([]Build(nil), builds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Sec < sorted[j].Sec })
	claimed := make([]bool, len(sorted))

	out := map[uint16]Build{}
	for _, t := range ordered {
		for j := range sorted {
			if !claimed[j] && sorted[j].Sec <= t.firstSec {
				claimed[j] = true
				out[t.tag] = sorted[j]
				break
			}
		}
	}
	return out
}

// ProducedPlacements returns the build tiles a producing tag was matched to —
// buildings that demonstrably stood and produced. A produced building was not
// abandoned, so build-dedup must never drop it however fragile the unit-tag
// attribution (issue #244: the real expansion Command Center was dropped as a
// worker "one-at-a-time" redirect even though it produced SCVs).
func (pe *PlayerEvidence) ProducedPlacements() map[string]map[[2]int]bool {
	out := map[string]map[[2]int]bool{}
	for bldg, tags := range pe.Producers {
		loc := matchProducerTagsToBuilds(tags, pe.Builds[bldg])
		if len(loc) == 0 {
			continue
		}
		set := map[[2]int]bool{}
		for _, b := range loc {
			set[[2]int{b.X, b.Y}] = true
		}
		out[bldg] = set
	}
	return out
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

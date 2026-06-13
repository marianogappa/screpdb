package unittags

import (
	"sort"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// CmdCoord is an inferred location for a command that the raw replay stream
// leaves spatially blank. X, Y are PIXEL coordinates (tile*32 + 16), matching
// the coordinate space worldstate uses for every non-building Kind, so the
// enriched value flows through cmdenrich unchanged.
type CmdCoord struct {
	X, Y int
}

// hatchFreqThreshold is the minimum number of larva-morph attributions a tag
// must collect (as the single unit selected immediately before the larvae) to
// be accepted as a Hatchery/Lair/Hive when it is not already proven by a
// Lair/Hive building-morph. Empirically real town halls accumulate tens of
// morphs across a game while stray single-selects appear once or twice; the
// threshold rejects that noise. See issue #175.
const hatchFreqThreshold = 3

// townHalls are the buildings that exist from frame 0 without a Build command,
// so an unmatched producing tag of these types resolves to the player's start
// location rather than being dropped. (Lair/Hive are tag-preserving morphs of
// a Hatchery, so they correlate against Hatchery builds and start under it.)
var townHalls = map[string]bool{
	models.GeneralUnitNexus:         true,
	models.GeneralUnitCommandCenter: true,
	models.GeneralUnitHatchery:      true,
}

// researchParent maps a Terran add-on that researches a tech to the building a
// player actually single-selects to issue that research — the add-on hangs off
// its parent and the parent's tag is the one observed in the selection. Used so
// add-on techs (Siege Mode, Spider Mines, Yamato, Lockdown, …) resolve against
// the parent's Build placement. Non-add-on research buildings map to themselves
// implicitly.
var researchParent = map[string]string{
	models.GeneralUnitMachineShop:  models.GeneralUnitFactory,
	models.GeneralUnitControlTower: models.GeneralUnitStarport,
	models.GeneralUnitPhysicsLab:   models.GeneralUnitScienceFacility,
	models.GeneralUnitCovertOps:    models.GeneralUnitScienceFacility,
}

// morphSourceBuild maps a building-morph target to the Build command whose
// placement names its location. Hive is a morph of a Lair, which is itself a
// tag-preserving morph of a Hatchery, so both resolve against Hatchery builds.
var morphSourceBuild = map[string]string{
	"Lair":          models.GeneralUnitHatchery,
	"Hive":          models.GeneralUnitHatchery,
	"Greater Spire": models.GeneralUnitSpire,
	"Sunken Colony": models.GeneralUnitCreepColony,
	"Spore Colony":  models.GeneralUnitCreepColony,
}

type point struct{ X, Y int }

// ttKey identifies a producing building by type and unit tag. BW recycles unit
// tags, so the same tag can name a Barracks early and a Factory later; keying
// locations on (type, tag) keeps those distinct instead of collapsing them.
type ttKey struct {
	bldg string
	tag  uint16
}

// coordRec is one command awaiting a resolved location.
type coordRec struct {
	idx      int    // index into r.Commands.Cmds
	bldgType string // building type whose tag location applies; "" for tag-only (CancelTrain)
	tag      uint16
	sec      int
}

// playerAcc accumulates one player's evidence during the selection walk.
type playerAcc struct {
	builds    map[string][]Build        // building type -> Build placements (tiles)
	firstSec  map[string]map[uint16]int // building type -> tag -> first event second
	larvaPrev map[uint16]int            // hatch-tap tag -> larva-morph count
	hatchSeed map[uint16]bool           // tags proven to be a town hall by a Lair/Hive morph
	recs      []coordRec
}

func newPlayerAcc() *playerAcc {
	return &playerAcc{
		builds:    map[string][]Build{},
		firstSec:  map[string]map[uint16]int{},
		larvaPrev: map[uint16]int{},
		hatchSeed: map[uint16]bool{},
	}
}

func (a *playerAcc) note(bldg string, tag uint16, sec int) {
	if a.firstSec[bldg] == nil {
		a.firstSec[bldg] = map[uint16]int{}
	}
	if _, seen := a.firstSec[bldg][tag]; !seen {
		a.firstSec[bldg][tag] = sec
	}
}

// Coordinates infers per-command pixel locations for the production / research
// / cancel commands the raw stream leaves blank, by binding each to the
// producing building's tag (recovered from selection state) and that tag to its
// Build placement. A frame-0 town hall has no Build command, so an unmatched
// town-hall tag falls back to the player's start location. The result is keyed
// by index into r.Commands.Cmds. See issue #175.
func Coordinates(r *rep.Replay, players []*models.Player) map[int]CmdCoord {
	out := map[int]CmdCoord{}
	if r == nil || r.Commands == nil {
		return out
	}

	starts := map[byte]point{}
	for _, p := range players {
		if p != nil && p.StartLocationX != nil && p.StartLocationY != nil {
			starts[p.PlayerID] = point{*p.StartLocationX, *p.StartLocationY}
		}
	}

	type selState struct {
		cur             []uint16
		groups          map[byte][]uint16
		prevSingle      uint16
		prevSingleValid bool
	}
	states := map[byte]*selState{}
	accs := map[byte]*playerAcc{}
	get := func(pid byte) (*selState, *playerAcc) {
		if states[pid] == nil {
			states[pid] = &selState{groups: map[byte][]uint16{}}
			accs[pid] = newPlayerAcc()
		}
		return states[pid], accs[pid]
	}
	snap := func(s *selState) {
		if len(s.cur) == 1 {
			s.prevSingle, s.prevSingleValid = s.cur[0], true
		} else {
			s.prevSingleValid = false
		}
	}

	// larvaRec defers a larva morph until the hatchery tag set is known: only
	// morphs whose hatch-tap tag is a confirmed town hall get a coordinate.
	type larvaRec struct {
		pid byte
		idx int
		tag uint16
		sec int
	}
	var larvaRecs []larvaRec

	for i, c := range r.Commands.Cmds {
		b := c.BaseCmd()
		if b == nil || b.Type == nil {
			continue
		}
		s, a := get(b.PlayerID)
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
					snap(s)
					s.cur = unionTags(s.cur, s.groups[hc.Group])
				}
			}
		case repcmd.TypeIDBuild:
			if bc, ok := c.(*repcmd.BuildCmd); ok && bc.Unit != nil {
				a.builds[bc.Unit.Name] = append(a.builds[bc.Unit.Name], Build{
					Frame: int32(b.Frame), Sec: sec, X: int(bc.Pos.X), Y: int(bc.Pos.Y),
				})
			}
		case repcmd.TypeIDTrain, repcmd.TypeIDUnitMorph:
			tc, ok := c.(*repcmd.TrainCmd)
			if !ok || tc.Unit == nil {
				continue
			}
			name := tc.Unit.Name
			if bldg, ok := producerBuilding[name]; ok {
				if len(s.cur) == 1 {
					a.note(bldg, s.cur[0], sec)
					a.recs = append(a.recs, coordRec{idx: i, bldgType: bldg, tag: s.cur[0], sec: sec})
				}
				continue
			}
			if larvaMorphUnits[name] && s.prevSingleValid {
				a.larvaPrev[s.prevSingle]++
				larvaRecs = append(larvaRecs, larvaRec{pid: b.PlayerID, idx: i, tag: s.prevSingle, sec: sec})
			}
		case repcmd.TypeIDBuildingMorph:
			if bm, ok := c.(*repcmd.BuildingMorphCmd); ok && bm.Unit != nil && len(s.cur) == 1 {
				if src, ok := morphSourceBuild[bm.Unit.Name]; ok {
					a.note(src, s.cur[0], sec)
					a.recs = append(a.recs, coordRec{idx: i, bldgType: src, tag: s.cur[0], sec: sec})
					if bm.Unit.Name == "Lair" || bm.Unit.Name == "Hive" {
						a.hatchSeed[s.cur[0]] = true
					}
				}
			}
		case repcmd.TypeIDTech:
			if tc, ok := c.(*repcmd.TechCmd); ok && tc.Tech != nil && len(s.cur) == 1 {
				if meta, ok := models.LookupTech(tc.Tech.Name); ok {
					bldg := selectableResearchBuilding(meta.BuildingSubject)
					a.note(bldg, s.cur[0], sec)
					a.recs = append(a.recs, coordRec{idx: i, bldgType: bldg, tag: s.cur[0], sec: sec})
				}
			}
		case repcmd.TypeIDUpgrade:
			if uc, ok := c.(*repcmd.UpgradeCmd); ok && uc.Upgrade != nil && len(s.cur) == 1 {
				if meta, ok := models.LookupUpgrade(uc.Upgrade.Name); ok {
					bldg := selectableResearchBuilding(meta.BuildingSubject)
					a.note(bldg, s.cur[0], sec)
					a.recs = append(a.recs, coordRec{idx: i, bldgType: bldg, tag: s.cur[0], sec: sec})
				}
			}
		case repcmd.TypeIDCancelTrain:
			// The cancelling building is single-selected; its tag is recovered
			// here and resolved against whatever location it earned from other
			// evidence (it produced or researched something). No type is known
			// from the cancel itself, so bldgType is left blank.
			if len(s.cur) == 1 {
				a.recs = append(a.recs, coordRec{idx: i, tag: s.cur[0], sec: sec})
			}
		}
	}

	// Fold confirmed larva morphs into Hatchery evidence (Phase A → B): a hatch
	// tap is trusted only when its tag is Lair/Hive-proven or crosses the
	// frequency threshold, so a stray single-select before larvae is ignored.
	for _, lr := range larvaRecs {
		a := accs[lr.pid]
		if !a.hatchSeed[lr.tag] && a.larvaPrev[lr.tag] < hatchFreqThreshold {
			continue
		}
		a.note(models.GeneralUnitHatchery, lr.tag, lr.sec)
		a.recs = append(a.recs, coordRec{idx: lr.idx, bldgType: models.GeneralUnitHatchery, tag: lr.tag, sec: lr.sec})
	}

	for pid, a := range accs {
		loc := resolveTagLocations(a, starts[pid])
		for _, rec := range a.recs {
			key := ttKey{bldg: rec.bldgType, tag: rec.tag}
			if rec.bldgType == "" {
				// CancelTrain names no building type; pick the producing type
				// whose tag was most recently first-seen at or before the
				// cancel (tags recycle, so the latest incarnation wins).
				key.bldg = a.latestTypeForTag(rec.tag, rec.sec)
				if key.bldg == "" {
					continue
				}
			}
			p, ok := loc[key]
			if !ok {
				continue
			}
			out[rec.idx] = CmdCoord{X: p.X*32 + 16, Y: p.Y*32 + 16}
		}
	}
	return out
}

// latestTypeForTag returns the building type under which tag was first seen at
// or before sec, preferring the most recent first-seen (BW recycles tags). Ties
// break on type name for determinism. Returns "" when the tag names no producer.
func (a *playerAcc) latestTypeForTag(tag uint16, sec int) string {
	best := ""
	bestSec := -1
	for bldg, tags := range a.firstSec {
		fs, ok := tags[tag]
		if !ok || fs > sec {
			continue
		}
		if fs > bestSec || (fs == bestSec && bldg < best) {
			bestSec = fs
			best = bldg
		}
	}
	return best
}

// resolveTagLocations maps every (building-type, tag) to a build tile: each
// building type's tags are greedily matched to its Build placements (earliest
// producer to earliest preceding Build), and unmatched town-hall tags fall back
// to the player's start location. Keyed on (type, tag) so a recycled tag that
// named two different buildings keeps a distinct location for each.
func resolveTagLocations(a *playerAcc, start point) map[ttKey]point {
	loc := map[ttKey]point{}
	for bldg, tags := range a.firstSec {
		matched := matchTagsToBuilds(tags, a.builds[bldg])
		isTownHall := townHalls[bldg]
		for tag := range tags {
			if b, ok := matched[tag]; ok {
				loc[ttKey{bldg, tag}] = point{b.X, b.Y}
			} else if isTownHall && (start != point{}) {
				// Start location is already in pixels; store as a tile so the
				// caller's uniform tile→pixel step lands back on it.
				loc[ttKey{bldg, tag}] = point{(start.X - 16) / 32, (start.Y - 16) / 32}
			}
		}
	}
	return loc
}

// matchTagsToBuilds greedily assigns each tag (earliest first-event first) to
// the earliest unclaimed Build placed at or before that event — a building
// cannot produce before it is commanded.
func matchTagsToBuilds(firstSec map[uint16]int, builds []Build) map[uint16]Build {
	type tf struct {
		tag uint16
		sec int
	}
	ordered := make([]tf, 0, len(firstSec))
	for tag, sec := range firstSec {
		ordered = append(ordered, tf{tag, sec})
	}
	// Tie-break on tag so map iteration order can't make the greedy assignment
	// (and thus the inferred coordinate) non-deterministic between ingests.
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].sec != ordered[j].sec {
			return ordered[i].sec < ordered[j].sec
		}
		return ordered[i].tag < ordered[j].tag
	})

	sorted := append([]Build(nil), builds...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Sec != sorted[j].Sec {
			return sorted[i].Sec < sorted[j].Sec
		}
		if sorted[i].X != sorted[j].X {
			return sorted[i].X < sorted[j].X
		}
		return sorted[i].Y < sorted[j].Y
	})
	claimed := make([]bool, len(sorted))

	out := map[uint16]Build{}
	for _, t := range ordered {
		for j := range sorted {
			if !claimed[j] && sorted[j].Sec <= t.sec {
				claimed[j] = true
				out[t.tag] = sorted[j]
				break
			}
		}
	}
	return out
}

func selectableResearchBuilding(researchBuilding string) string {
	if parent, ok := researchParent[researchBuilding]; ok {
		return parent
	}
	return researchBuilding
}

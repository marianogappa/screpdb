package hotkeystream

import (
	"sort"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
)

// The evidence maps below overlap with internal/unittags on purpose: unittags
// answers "which building produced this unit" for build dedup, while this pass
// answers "what did a hotkey group hold" — the two evolve independently.

// producerBuilding maps a trained unit to the building type that trains it.
// A Train with a single-unit selection proves the selected tag is that
// building (Brood War allows at most one building per selection).
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

// larvaMorphUnits morph from larvae; the producing hall is the single-selected
// tag just before the larvae select (the macro-cycle hatch tap).
var larvaMorphUnits = map[string]bool{
	"Drone": true, "Zergling": true, "Hydralisk": true, "Mutalisk": true,
	"Overlord": true, "Defiler": true, "Ultralisk": true, "Queen": true,
	"Scourge": true, "Infested Terran": true,
}

// addonParent: building an add-on proves the single-selected tag is the parent.
var addonParent = map[string]string{
	"Machine Shop": "Factory", "Control Tower": "Starport",
	"Comsat Station": "Command Center", "Nuclear Silo": "Command Center",
	"Covert Ops": "Science Facility", "Physics Lab": "Science Facility",
}

// techBuilding: researching a tech proves the single-selected tag is the
// building that researches it.
var techBuilding = map[string]string{
	"Stim Packs": "Academy", "Restoration": "Academy", "Optical Flare": "Academy",
	"Lockdown": "Covert Ops", "Personnel Cloaking": "Covert Ops",
	"Spider Mines": "Machine Shop", "Tank Siege Mode": "Machine Shop",
	"EMP Shockwave": "Science Facility", "Irradiate": "Science Facility",
	"Yamato Gun": "Physics Lab", "Cloaking Field": "Control Tower",
	"Burrowing":        "Hatchery",
	"Lurker Aspect":    "Hydralisk Den",
	"Spawn Broodlings": "Queen's Nest", "Ensnare": "Queen's Nest",
	"Plague": "Defiler Mound", "Consume": "Defiler Mound",
	"Psionic Storm": "Templar Archives", "Hallucination": "Templar Archives",
	"Mind Control": "Templar Archives", "Maelstrom": "Templar Archives",
	"Recall": "Arbiter Tribunal", "Stasis Field": "Arbiter Tribunal",
	"Disruption Web": "Fleet Beacon",
}

// upgradeBuilding: like techBuilding, for upgrades.
var upgradeBuilding = map[string]string{
	"Terran Infantry Armor": "Engineering Bay", "Terran Infantry Weapons": "Engineering Bay",
	"Terran Vehicle Plating": "Armory", "Terran Vehicle Weapons": "Armory",
	"Terran Ship Plating": "Armory", "Terran Ship Weapons": "Armory",
	"U-238 Shells (Marine Range)": "Academy", "Caduceus Reactor (Medic Energy)": "Academy",
	"Ion Thrusters (Vulture Speed)": "Machine Shop", "Charon Boosters (Goliath Range)": "Machine Shop",
	"Titan Reactor (Science Vessel Energy)": "Science Facility",
	"Ocular Implants (Ghost Sight)":         "Covert Ops", "Moebius Reactor (Ghost Energy)": "Covert Ops",
	"Apollo Reactor (Wraith Energy)":           "Control Tower",
	"Colossus Reactor (Battle Cruiser Energy)": "Physics Lab",
	"Zerg Carapace":                            "Evolution Chamber", "Zerg Melee Attacks": "Evolution Chamber",
	"Zerg Missile Attacks": "Evolution Chamber",
	"Zerg Flyer Carapace":  "Spire", "Zerg Flyer Attacks": "Spire",
	"Ventral Sacs (Overlord Transport)": "Hatchery", "Antennae (Overlord Sight)": "Hatchery",
	"Pneumatized Carapace (Overlord Speed)": "Hatchery",
	"Metabolic Boost (Zergling Speed)":      "Spawning Pool", "Adrenal Glands (Zergling Attack)": "Spawning Pool",
	"Muscular Augments (Hydralisk Speed)": "Hydralisk Den", "Grooved Spines (Hydralisk Range)": "Hydralisk Den",
	"Gamete Meiosis (Queen Energy)": "Queen's Nest", "Defiler Energy": "Defiler Mound",
	"Chitinous Plating (Ultralisk Armor)": "Ultralisk Cavern", "Anabolic Synthesis (Ultralisk Speed)": "Ultralisk Cavern",
	"Protoss Ground Armor": "Forge", "Protoss Ground Weapons": "Forge", "Protoss Plasma Shields": "Forge",
	"Protoss Air Armor": "Cybernetics Core", "Protoss Air Weapons": "Cybernetics Core",
	"Singularity Charge (Dragoon Range)": "Cybernetics Core",
	"Leg Enhancement (Zealot Speed)":     "Citadel of Adun",
	"Scarab Damage":                      "Robotics Support Bay", "Reaver Capacity": "Robotics Support Bay",
	"Gravitic Drive (Shuttle Speed)": "Robotics Support Bay",
	"Sensor Array (Observer Sight)":  "Observatory", "Gravitic Booster (Observer Speed)": "Observatory",
	"Khaydarin Amulet (Templar Energy)": "Templar Archives", "Argus Talisman (Dark Archon Energy)": "Templar Archives",
	"Khaydarin Core (Arbiter Energy)": "Arbiter Tribunal",
	"Argus Jewel (Corsair Energy)":    "Fleet Beacon", "Carrier Capacity": "Fleet Beacon",
	"Apial Sensors (Scout Sight)": "Fleet Beacon", "Gravitic Thrusters (Scout Speed)": "Fleet Beacon",
}

// canonicalBuildingName translates the screp unit names that differ from the
// canonical building names used across this package (and the wire enum).
func canonicalBuildingName(screpName string) string {
	switch screpName {
	case "ComSat":
		return "Comsat Station"
	case "Queens Nest":
		return "Queen's Nest"
	}
	return screpName
}

// townHalls have no Build command when they are the spawn-seeded main; their
// location falls back to the player's start location.
var townHalls = map[string]bool{"Hatchery": true, "Command Center": true, "Nexus": true}

type bldgEvidence struct {
	frame int32
	bldg  string
}

type tagEvidence struct {
	bldgEvs []bldgEvidence
	unitEvs []int32
}

type rawHotkeyEvent struct {
	frame   int32
	typ     byte // TypeSelect / TypeAssignUnits (pre-classification) / TypeAdd
	group   byte
	selSize int
	selTag  uint16 // valid when selSize == 1
}

type buildPlacement struct {
	frame int32
	x, y  int
}

type playerExtract struct {
	events []rawHotkeyEvent
	tags   map[uint16]*tagEvidence
	builds map[string][]buildPlacement
}

// Extract reconstructs each player's annotated hotkey stream from the raw
// replay command stream: selection state binds hotkey groups to unit tags,
// production/research/add-on/scanner commands prove which tags are buildings
// (classified by the evidence nearest in time to each assign), and Build
// placements plus the start location provide building tile coordinates.
func Extract(r *rep.Replay) map[byte][]Event {
	out := map[byte][]Event{}
	if r == nil || r.Commands == nil || r.Header == nil {
		return out
	}

	type selState struct {
		cur             []uint16
		groups          map[byte][]uint16
		prevSingle      uint16
		prevSingleValid bool
	}
	states := map[byte]*selState{}
	extracts := map[byte]*playerExtract{}
	get := func(pid byte) (*selState, *playerExtract) {
		if states[pid] == nil {
			states[pid] = &selState{groups: map[byte][]uint16{}}
			extracts[pid] = &playerExtract{tags: map[uint16]*tagEvidence{}, builds: map[string][]buildPlacement{}}
		}
		return states[pid], extracts[pid]
	}
	tagOf := func(pe *playerExtract, tag uint16) *tagEvidence {
		t := pe.tags[tag]
		if t == nil {
			t = &tagEvidence{}
			pe.tags[tag] = t
		}
		return t
	}
	noteMulti := func(pe *playerExtract, sel []uint16, frame int32) {
		if len(sel) <= 1 {
			return
		}
		for _, tag := range sel {
			t := tagOf(pe, tag)
			if n := len(t.unitEvs); n == 0 || t.unitEvs[n-1] != frame {
				t.unitEvs = append(t.unitEvs, frame)
			}
		}
	}
	addBldgEv := func(pe *playerExtract, tag uint16, frame int32, bldg string) {
		t := tagOf(pe, tag)
		t.bldgEvs = append(t.bldgEvs, bldgEvidence{frame: frame, bldg: bldg})
	}

	for _, c := range r.Commands.Cmds {
		b := c.BaseCmd()
		if b == nil || b.Type == nil {
			continue
		}
		s, pe := get(b.PlayerID)
		frame := int32(b.Frame)
		snap := func() {
			if len(s.cur) == 1 {
				s.prevSingle, s.prevSingleValid = s.cur[0], true
			} else {
				s.prevSingleValid = false
			}
		}

		switch b.Type.ID {
		case repcmd.TypeIDSelect, repcmd.TypeIDSelect121:
			if sc, ok := c.(*repcmd.SelectCmd); ok {
				snap()
				s.cur = tagsOf(sc.UnitTags)
				noteMulti(pe, s.cur, frame)
			}
		case repcmd.TypeIDSelectAdd, repcmd.TypeIDSelectAdd121:
			if sc, ok := c.(*repcmd.SelectCmd); ok {
				snap()
				s.cur = unionTags(s.cur, tagsOf(sc.UnitTags))
				noteMulti(pe, s.cur, frame)
			}
		case repcmd.TypeIDSelectRemove, repcmd.TypeIDSelectRemove121:
			if sc, ok := c.(*repcmd.SelectCmd); ok {
				snap()
				s.cur = removeTags(s.cur, tagsOf(sc.UnitTags))
			}
		case repcmd.TypeIDHotkey:
			hc, ok := c.(*repcmd.HotkeyCmd)
			if !ok || hc.HotkeyType == nil {
				continue
			}
			e := rawHotkeyEvent{frame: frame, group: hc.Group}
			switch hc.HotkeyType.Name {
			case "Assign":
				e.typ = TypeAssignUnits
				s.groups[hc.Group] = append([]uint16(nil), s.cur...)
			case "Select":
				e.typ = TypeSelect
				snap()
				s.cur = append([]uint16(nil), s.groups[hc.Group]...)
				noteMulti(pe, s.cur, frame)
			case "Add":
				e.typ = TypeAdd
				snap()
				s.cur = unionTags(s.cur, s.groups[hc.Group])
				noteMulti(pe, s.cur, frame)
			default:
				continue
			}
			e.selSize = len(s.cur)
			if e.selSize == 1 {
				e.selTag = s.cur[0]
			}
			pe.events = append(pe.events, e)
		case repcmd.TypeIDBuild:
			bc, ok := c.(*repcmd.BuildCmd)
			if !ok || bc.Unit == nil {
				continue
			}
			name := canonicalBuildingName(bc.Unit.Name)
			pe.builds[name] = append(pe.builds[name], buildPlacement{frame: frame, x: int(bc.Pos.X), y: int(bc.Pos.Y)})
			if len(s.cur) == 1 {
				if parent, isAddon := addonParent[name]; isAddon {
					addBldgEv(pe, s.cur[0], frame, parent)
				} else {
					// A single-selected tag issuing a Build is a worker.
					t := tagOf(pe, s.cur[0])
					t.unitEvs = append(t.unitEvs, frame)
				}
			}
		case repcmd.TypeIDTrain, repcmd.TypeIDUnitMorph:
			tc, ok := c.(*repcmd.TrainCmd)
			if !ok || tc.Unit == nil {
				continue
			}
			if bldg, ok := producerBuilding[tc.Unit.Name]; ok && len(s.cur) == 1 {
				addBldgEv(pe, s.cur[0], frame, bldg)
			} else if larvaMorphUnits[tc.Unit.Name] && s.prevSingleValid {
				addBldgEv(pe, s.prevSingle, frame, "Hatchery")
			}
		case repcmd.TypeIDBuildingMorph:
			if bm, ok := c.(*repcmd.BuildingMorphCmd); ok && bm.Unit != nil && len(s.cur) == 1 {
				addBldgEv(pe, s.cur[0], frame, "Hatchery")
			}
		case repcmd.TypeIDTech:
			if tc, ok := c.(*repcmd.TechCmd); ok && tc.Tech != nil && len(s.cur) == 1 {
				if bldg, ok := techBuilding[tc.Tech.Name]; ok {
					addBldgEv(pe, s.cur[0], frame, bldg)
				}
			}
		case repcmd.TypeIDUpgrade:
			if uc, ok := c.(*repcmd.UpgradeCmd); ok && uc.Upgrade != nil && len(s.cur) == 1 {
				if bldg, ok := upgradeBuilding[uc.Upgrade.Name]; ok {
					addBldgEv(pe, s.cur[0], frame, bldg)
				}
			}
		case repcmd.TypeIDTargetedOrder, repcmd.TypeIDTargetedOrder121:
			toc, ok := c.(*repcmd.TargetedOrderCmd)
			if !ok || toc.Order == nil {
				continue
			}
			if toc.Order.ID == repcmd.OrderIDCastScannerSweep && len(s.cur) == 1 {
				addBldgEv(pe, s.cur[0], frame, "Comsat Station")
			}
		}
	}

	for pid, pe := range extracts {
		if len(pe.events) == 0 {
			continue
		}
		out[pid] = annotate(pe, startLocationTile(r, pid))
	}
	return out
}

// classifyNearest returns the building a tag most plausibly was at the given
// frame: the nearest piece of building evidence wins unless unit evidence
// (multi-selection membership, or issuing a Build) sits strictly closer.
// Nearest-in-time beats whole-game classification because Brood War recycles
// unit tags.
func (t *tagEvidence) classifyNearest(frame int32) (string, bool) {
	if t == nil || len(t.bldgEvs) == 0 {
		return "", false
	}
	bestDist, best := int64(1)<<62, ""
	for _, ev := range t.bldgEvs {
		if d := absI64(int64(ev.frame) - int64(frame)); d < bestDist {
			bestDist, best = d, ev.bldg
		}
	}
	unitDist := int64(1) << 62
	for _, f := range t.unitEvs {
		if d := absI64(int64(f) - int64(frame)); d < unitDist {
			unitDist = d
		}
	}
	if bestDist <= unitDist {
		return best, true
	}
	return "", false
}

// annotate classifies every assign and resolves building locations, producing
// the final wire events.
func annotate(pe *playerExtract, startTile *buildPlacement) []Event {
	type bldgTag struct {
		tag        uint16
		bldg       string
		firstFrame int32
	}
	var bldgTags []bldgTag
	tagBldg := map[uint16]string{}
	for _, e := range pe.events {
		if e.typ != TypeAssignUnits || e.selSize != 1 {
			continue
		}
		if _, seen := tagBldg[e.selTag]; seen {
			continue
		}
		if bldg, ok := pe.tags[e.selTag].classifyNearest(e.frame); ok {
			tagBldg[e.selTag] = bldg
			first := int32(1) << 30
			for _, ev := range pe.tags[e.selTag].bldgEvs {
				if ev.frame < first {
					first = ev.frame
				}
			}
			bldgTags = append(bldgTags, bldgTag{tag: e.selTag, bldg: bldg, firstFrame: first})
		} else {
			tagBldg[e.selTag] = ""
		}
	}

	// Greedily bind each building tag (earliest evidence first) to the
	// earliest unclaimed Build placement of that type issued no later than the
	// tag's first evidence — a building cannot act before it was placed. The
	// spawn-seeded main town hall has no Build command; the first unmatched
	// hall tag anchors to the start location.
	sort.Slice(bldgTags, func(i, j int) bool { return bldgTags[i].firstFrame < bldgTags[j].firstFrame })
	claimed := map[string][]bool{}
	tagLoc := map[uint16]buildPlacement{}
	mainUsed := false
	for _, bt := range bldgTags {
		list := pe.builds[bt.bldg]
		if claimed[bt.bldg] == nil {
			sort.Slice(list, func(i, j int) bool { return list[i].frame < list[j].frame })
			pe.builds[bt.bldg] = list
			claimed[bt.bldg] = make([]bool, len(list))
		}
		matched := false
		for i, b := range list {
			if !claimed[bt.bldg][i] && b.frame <= bt.firstFrame {
				claimed[bt.bldg][i] = true
				tagLoc[bt.tag] = b
				matched = true
				break
			}
		}
		if !matched && townHalls[bt.bldg] && !mainUsed && startTile != nil {
			tagLoc[bt.tag] = *startTile
			mainUsed = true
		}
	}

	events := make([]Event, 0, len(pe.events))
	for _, e := range pe.events {
		out := Event{Sec: e.frame * frameMillis / 1000, Type: e.typ, Group: e.group}
		if e.typ == TypeAssignUnits {
			if e.selSize == 1 {
				if bldg := tagBldg[e.selTag]; bldg != "" {
					out.Type = TypeAssignBuilding
					out.Building = BuildingID(bldg)
					out.TileX, out.TileY = TileUnknown, TileUnknown
					if loc, ok := tagLoc[e.selTag]; ok {
						out.TileX, out.TileY = clampTile(loc.x), clampTile(loc.y)
					}
				} else {
					out.Count = 1
				}
			} else {
				out.Count = byte(min(e.selSize, 255))
			}
		}
		events = append(events, out)
	}
	return events
}

// startLocationTile returns the player's start location in build tiles, or
// nil when map data or the slot's start location is unavailable. The point is
// the location center in pixels; shift by half a town-hall footprint (4x3
// tiles) so the tile matches Build-command anchoring.
func startLocationTile(r *rep.Replay, pid byte) *buildPlacement {
	if r.MapData == nil || r.Header == nil {
		return nil
	}
	p := r.Header.PIDPlayers[pid]
	if p == nil {
		return nil
	}
	for _, sl := range r.MapData.StartLocations {
		if sl.SlotID == byte(p.SlotID) {
			return &buildPlacement{x: int(sl.X)/32 - 2, y: int(sl.Y)/32 - 1}
		}
	}
	return nil
}

func clampTile(v int) byte {
	if v < 0 {
		return 0
	}
	if v >= int(TileUnknown) {
		return TileUnknown - 1
	}
	return byte(v)
}

func absI64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
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

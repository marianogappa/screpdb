// Package builddedup removes redundant Build commands using selection-derived
// evidence (see internal/unittags), so build-order building counts reflect what
// actually happened. It applies two strategies of different confidence:
//
//   - Tier A — worker one-at-a-time (provable, whole game): if the same worker
//     is redirected to another build before its previous building could finish,
//     the previous build never completed. Terran SCV / Zerg Drone only (Protoss
//     probes warp-in and are freed, so the law does not apply). BW does not
//     allow queuing building construction, so a second build by the same worker
//     within the first's build time is conclusive.
//
//   - Tier B — never-produced production buildings (inferential, BO window): a
//     production-capable building that never produced a unit is dropped. Guards
//     keep at least one instance of a built type (tech-tree safety) and never
//     drop a building proven to exist by an add-on.
//
// Compute returns a Plan whose ShouldDrop predicate is handed to earlyfilter.
// The dedup runs first (it is higher-confidence than earlyfilter's resource
// simulation); earlyfilter then filters whatever remains.
package builddedup

import (
	"sort"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/unittags"
)

// boWindowSec bounds Tier B (the inferential rule) to the build-order phase.
// Tier A is provable and is never windowed.
const boWindowSec = 10 * 60

// productionBuildings are the unit-producing buildings Tier B reasons about.
// Zerg production buildings (Hatchery/Lair/Hive) are absent: unit-morphs select
// larva, not the building, so their tags are not building identities.
var productionBuildings = map[string]bool{
	"Nexus": true, "Gateway": true, "Robotics Facility": true, "Stargate": true,
	"Command Center": true, "Barracks": true, "Factory": true, "Starport": true,
}

// startInstances are buildings every player starts with one of (melee); the
// starting one produces but has no Build command, so it is seeded.
var startInstances = map[string]int{"Nexus": 1, "Command Center": 1}

type buildKey struct {
	pid   byte
	frame int32
}

// Plan is the set of Build commands to drop, with the reason per command.
type Plan struct {
	drops      map[buildKey]string
	TierADrops int
	TierBDrops int
}

// ShouldDrop reports whether cmd is a Build command this plan removes.
func (p *Plan) ShouldDrop(cmd *models.Command) bool {
	if p == nil || cmd == nil || cmd.Player == nil || cmd.ActionType != models.ActionTypeBuild {
		return false
	}
	_, ok := p.drops[buildKey{cmd.Player.PlayerID, cmd.Frame}]
	return ok
}

// Reason returns the drop reason for a Build command, or "" if not dropped.
func (p *Plan) Reason(pid byte, frame int32) string {
	if p == nil {
		return ""
	}
	return p.drops[buildKey{pid, frame}]
}

// Compute builds a drop Plan from selection evidence and player races.
func Compute(ev *unittags.Evidence, players []*models.Player) *Plan {
	pl := &Plan{drops: map[buildKey]string{}}
	if ev == nil {
		return pl
	}
	raceByPID := map[byte]string{}
	for _, p := range players {
		if p != nil {
			raceByPID[p.PlayerID] = p.Race
		}
	}
	for pid, pe := range ev.Players {
		pl.tierAWorkerOneAtATime(pid, pe, raceByPID[pid])
		pl.tierBNeverProduced(pid, pe)
	}
	return pl
}

// tierAWorkerOneAtATime: same worker tag re-ordered before its prior build could
// finish ⇒ the prior build was abandoned. Provable; applied whole game.
func (pl *Plan) tierAWorkerOneAtATime(pid byte, pe *unittags.PlayerEvidence, race string) {
	if race != "Terran" && race != "Zerg" {
		return
	}
	byTag := map[uint16][]unittags.WorkerBuild{}
	for _, w := range pe.WorkerBuilds {
		byTag[w.Worker] = append(byTag[w.Worker], w)
	}
	for _, list := range byTag {
		sort.Slice(list, func(i, j int) bool { return list[i].Sec < list[j].Sec })
		for i := 0; i+1 < len(list); i++ {
			bt, ok := models.BuildTimeOf(list[i].Building)
			if !ok {
				continue
			}
			// Only a redirect to a DIFFERENT tile proves the earlier build was
			// abandoned. A re-click at the same tile is the same building (the
			// worker keeps building it); command construction already collapses
			// those, and dropping one here would delete the real building.
			sameTile := list[i].X == list[i+1].X && list[i].Y == list[i+1].Y
			if !sameTile && list[i+1].Sec < list[i].Sec+int(bt) {
				pl.markDrop(pid, list[i].Frame, "worker_one_at_a_time")
				pl.TierADrops++
			}
		}
	}
}

// tierBNeverProduced: production buildings (issued within the BO window) that
// never produced a unit are dropped. Considers only builds Tier A did not
// already drop, keeps max(producers, addon-proven, 1) instances of each type,
// and drops the in-window builds left unmatched by a time-respecting assignment
// of builds to producing tags.
func (pl *Plan) tierBNeverProduced(pid byte, pe *unittags.PlayerEvidence) {
	for bldg := range productionBuildings {
		// Collapse same-tile Build commands into one distinct building: a spammed
		// / re-queued placement at one spot is a single building, and counting
		// each command separately makes it consume several producer matches —
		// which then strands real buildings at other tiles as "never produced"
		// and drops them (e.g. a 4-command Gateway placement hid three later
		// gateways). Each distinct building carries all its frames so a dropped
		// one removes every command at that tile. Builds Tier A already dropped
		// are excluded so the two tiers don't both remove the same type.
		type distinctBuilding struct {
			sec    int
			frames []int32
		}
		byTile := map[[2]int]*distinctBuilding{}
		var tileOrder [][2]int
		for _, b := range pe.Builds[bldg] {
			if _, dropped := pl.drops[buildKey{pid, b.Frame}]; dropped {
				continue
			}
			tile := [2]int{b.X, b.Y}
			db := byTile[tile]
			if db == nil {
				db = &distinctBuilding{sec: b.Sec}
				byTile[tile] = db
				tileOrder = append(tileOrder, tile)
			}
			if b.Sec < db.sec {
				db.sec = b.Sec
			}
			db.frames = append(db.frames, b.Frame)
		}
		if len(byTile) == 0 {
			continue
		}
		distinct := make([]*distinctBuilding, 0, len(byTile))
		for _, t := range tileOrder {
			distinct = append(distinct, byTile[t])
		}
		sort.Slice(distinct, func(i, j int) bool { return distinct[i].sec < distinct[j].sec })
		// builds is the per-distinct-building view the matching logic reasons over.
		builds := make([]unittags.Build, len(distinct))
		for i, db := range distinct {
			builds[i] = unittags.Build{Sec: db.sec}
		}

		nProducers := len(pe.Producers[bldg])
		addonOnly := 0
		for tag := range pe.Addons[bldg] {
			if _, produced := pe.Producers[bldg][tag]; !produced {
				addonOnly++
			}
		}
		// Keep: every producing instance, every add-on-proven instance, and at
		// least one instance of any built type (so we never zero out a building
		// that downstream tech-tree / pattern logic may depend on).
		keep := nProducers + addonOnly
		if keep < 1 {
			keep = 1
		}
		if keep >= len(builds) {
			continue
		}
		dropBudget := len(builds) - keep

		// Candidate drops: in-window builds not matched to a producing tag,
		// latest first (a late never-used building is the most droppable).
		unmatched := unmatchedBuilds(builds, producerSecs(pe.Producers[bldg]), startInstances[bldg])
		var cand []int
		for _, i := range unmatched {
			if builds[i].Sec <= boWindowSec {
				cand = append(cand, i)
			}
		}
		sort.Slice(cand, func(a, b int) bool { return builds[cand[a]].Sec > builds[cand[b]].Sec })

		for k := 0; k < len(cand) && k < dropBudget; k++ {
			// Drop every Build command at this distinct building's tile.
			for _, f := range distinct[cand[k]].frames {
				pl.markDrop(pid, f, "never_produced")
			}
			pl.TierBDrops++
		}
	}
}

func (pl *Plan) markDrop(pid byte, frame int32, reason string) {
	k := buildKey{pid, frame}
	if _, exists := pl.drops[k]; !exists {
		pl.drops[k] = reason
	}
}

func producerSecs(producers map[uint16]*unittags.Production) []int {
	secs := make([]int, 0, len(producers))
	for _, p := range producers {
		secs = append(secs, p.FirstSec)
	}
	return secs
}

// unmatchedBuilds matches each build (earliest first) to a distinct producing
// tag whose first production is at or after the build (a building cannot produce
// before it is commanded). startCount producers are pre-claimed for starting
// instances that have no Build command. Returns the indices of builds left
// unmatched — those that never produced. builds must be sorted by Sec.
func unmatchedBuilds(builds []unittags.Build, prodSecs []int, startCount int) []int {
	sort.Ints(prodSecs)
	claimed := make([]bool, len(prodSecs))
	for k := 0; k < startCount && k < len(prodSecs); k++ {
		claimed[k] = true
	}
	var out []int
	for i := range builds {
		matched := false
		for j := range prodSecs {
			if !claimed[j] && prodSecs[j] >= builds[i].Sec {
				claimed[j] = true
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, i)
		}
	}
	return out
}

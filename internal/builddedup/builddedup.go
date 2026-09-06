// Package builddedup removes redundant Build commands using selection-derived
// evidence (see internal/unittags), so building counts reflect what actually
// happened. Two strategies of different confidence:
//
//   - Tier A — worker one-at-a-time (provable, whole game): a worker redirected
//     to another build before its previous one could finish proves the previous
//     never completed. BW does not allow queuing construction, so a second build
//     by the same worker inside the first's build time is conclusive. Terran SCV
//     and Zerg Drone only — Protoss probes warp-in and are freed.
//
//   - Tier B — never-produced production buildings (inferential, BO window): a
//     production-capable building that never produced is dropped, keeping at
//     least one instance of a built type for tech-tree safety and never dropping
//     one proven to exist by an add-on.
//
// Compute returns a Plan whose ShouldDrop predicate earlyfilter consumes. Dedup
// runs FIRST because it is higher-confidence than the resource simulation.
package builddedup

import (
	"sort"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/unittags"
)

// Tier B is inferential, so it is bounded to the build-order phase; Tier A is
// provable and never windowed.
const boWindowSec = 10 * 60

// Zerg production buildings are absent: unit-morphs select larva, not the
// building, so their tags are not building identities.
var productionBuildings = map[string]bool{
	"Nexus": true, "Gateway": true, "Robotics Facility": true, "Stargate": true,
	"Command Center": true, "Barracks": true, "Factory": true, "Starport": true,
}

// Buildings every melee player starts with one of. The starting one produces
// but has no Build command, so it is seeded.
var startInstances = map[string]int{"Nexus": 1, "Command Center": 1}

type buildKey struct {
	pid   byte
	frame int32
}

type Plan struct {
	drops      map[buildKey]string
	TierADrops int
	TierBDrops int
}

func (p *Plan) ShouldDrop(cmd *models.Command) bool {
	if p == nil || cmd == nil || cmd.Player == nil || cmd.ActionType != models.ActionTypeBuild {
		return false
	}
	_, ok := p.drops[buildKey{cmd.Player.PlayerID, cmd.Frame}]
	return ok
}

// Reason returns "" if the command is not dropped.
func (p *Plan) Reason(pid byte, frame int32) string {
	if p == nil {
		return ""
	}
	return p.drops[buildKey{pid, frame}]
}

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
		// A building a producing tag was matched to demonstrably stood and made a unit,
		// so Tier A must not drop it as a worker redirect however the (fragile)
		// worker-tag trail reads (issue #244).
		pl.tierAWorkerOneAtATime(pid, pe, raceByPID[pid], pe.ProducedPlacements())
		pl.tierBNeverProduced(pid, pe)
	}
	return pl
}

// isProduced means that building demonstrably stood and made a unit.
func isProduced(produced map[string]map[[2]int]bool, bldg string, x, y int) bool {
	return produced[bldg][[2]int{x, y}]
}

// tierAWorkerOneAtATime: the same worker tag re-ordered before its prior build
// could finish proves the prior build was abandoned. Applied whole game.
func (pl *Plan) tierAWorkerOneAtATime(pid byte, pe *unittags.PlayerEvidence, race string, produced map[string]map[[2]int]bool) {
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
			// Only a redirect to a DIFFERENT tile proves abandonment: a re-click at the
			// same tile is the same building, command construction already collapses those,
			// and dropping one here would delete the real building.
			sameTile := list[i].X == list[i+1].X && list[i].Y == list[i+1].Y
			if sameTile || list[i+1].Sec >= list[i].Sec+int(bt) {
				continue
			}
			if race == "Terran" && list[i].Building == list[i+1].Building {
				// Same type, same worker, within build time, different tile: one intended
				// Terran building re-placed, since an SCV builds one at a time. The earlyfilter
				// resource sim keeps the EARLIEST affordable placement, so drop the LATER
				// duplicate to match — otherwise each pass drops the other's survivor and the
				// real building vanishes (a Factory re-placed one tile over read as 0).
				//
				// Zerg is excluded: a Drone is freed once a building starts, so its next build is
				// genuinely new, and the supply rungs need the later committed placement.
				//
				// The produced guard is deliberately NOT applied here: a re-placed single
				// building can leave BOTH placements attributed a producing tag (recycled tags,
				// greedy matching), and protecting the later one resurrects the phantom
				// duplicate — inflating the count and starving the sim into dropping a real
				// building elsewhere (a 2-Starport Valkyrie mis-read as mech).
				pl.markDrop(pid, list[i+1].Frame, "worker_one_at_a_time")
			} else {
				// Redirect to a DIFFERENT building before the first could finish (or a Zerg
				// re-placement) means the earlier never completed — UNLESS it went on to
				// produce, which proves it stood. Issue #244: the real expansion Command Center
				// was dropped here despite producing SCVs, so the marker counted factories
				// against a later CC and over-counted "N Fact Expa".
				if isProduced(produced, list[i].Building, list[i].X, list[i].Y) {
					continue
				}
				pl.markDrop(pid, list[i].Frame, "worker_one_at_a_time")
			}
			pl.TierADrops++
		}
	}
}

// tierBNeverProduced drops in-window production buildings that never produced.
// Considers only builds Tier A left, keeps max(producers, addon-proven, 1) of
// each type, and drops those left unmatched by a time-respecting assignment of
// builds to producing tags.
func (pl *Plan) tierBNeverProduced(pid byte, pe *unittags.PlayerEvidence) {
	for bldg := range productionBuildings {
		// Collapse same-tile Builds into one distinct building: a spammed placement at
		// one spot is a single building, and counting each command separately makes it
		// consume several producer matches — which strands real buildings at other tiles
		// as "never produced" (a 4-command Gateway placement hid three later gateways).
		// Each distinct building carries all its frames so dropping it removes every
		// command at that tile. Builds Tier A already dropped are excluded.
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
		// Keep every producing instance, every add-on-proven one, and at least one of
		// any built type, so downstream tech-tree logic is never zeroed out.
		keep := nProducers + addonOnly
		if keep < 1 {
			keep = 1
		}
		if keep >= len(builds) {
			continue
		}
		dropBudget := len(builds) - keep

		// Latest first: a late never-used building is the most droppable.
		unmatched := unmatchedBuilds(builds, producerSecs(pe.Producers[bldg]), startInstances[bldg])
		var cand []int
		for _, i := range unmatched {
			if builds[i].Sec <= boWindowSec {
				cand = append(cand, i)
			}
		}
		sort.Slice(cand, func(a, b int) bool { return builds[cand[a]].Sec > builds[cand[b]].Sec })

		for k := 0; k < len(cand) && k < dropBudget; k++ {
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

// unmatchedBuilds matches each build (earliest first) to a distinct producing tag
// whose first production is at or after it, since a building cannot produce
// before it is commanded. startCount producers are pre-claimed for starting
// instances that have no Build command. Returns indices of builds left unmatched
// — those that never produced. builds must be sorted by Sec.
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

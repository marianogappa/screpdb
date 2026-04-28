package earlyfilter

import (
	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// fastestFrameMs is the per-frame duration in milliseconds at "Fastest" game
// speed (~23.81 frames per second). Brood War replays universally record
// frames; we convert to seconds to drive a continuous gather simulation.
const fastestFrameMs = 42

// Gas-gather model: when a player Right-Clicks (or Targeted Order) on a
// geyser building they own, we assume up to 3 workers leave the mineral
// line. They return after gathering ~100 gas — at 46 gas/min/worker (BW
// saturation rate) that's ~43 seconds with 3 workers. Capping at 3 and
// pre-emptively returning matches the "100 gas / lair / pull off" play
// pattern the user flagged for Zerg.
const (
	gasGatherDurationS    = 43.0
	gasWorkersPerGather   = 3
	gasGeyserProximityPx2 = 96 * 96 // squared euclidean threshold; ~3 tiles
)

// Larva model: each Zerg Hatchery (and its Lair / Hive upgrades) caps at
// 3 larva and produces 1 every 14.4 in-game seconds (Fastest). Drone,
// Zergling, and Overlord morphs each consume one larva. Morph commands
// without an available larva are engine-rejected — the kept replay still
// records the order, but the engine never executed it. Filtering by
// larva availability is the primary lever against drone/morph spam.
const (
	larvaSpawnIntervalS = 14.4
	larvaPerHatchery    = 3
	// larvaPreorderSlackS lets a morph fire if the next larva will spawn
	// within this many seconds. Pro Zergs routinely issue morph commands
	// a beat before the larva is ready — the BW engine accepts and waits.
	// Without slack the filter rejects these as "no_larva" and we land
	// 1 morph short of the player's actual count.
	larvaPreorderSlackS = 5.0
)

// hatcheryLarva tracks a single Hatchery's larva slot. Lair and Hive
// upgrade in-place so they reuse the same struct (no replacement).
type hatcheryLarva struct {
	// completionFrame is the frame the hatchery comes online. The starting
	// hatchery uses 0; built hatcheries use their order frame plus build
	// time.
	completionFrame int32
	// nextSpawnFrame is the frame at which the next larva attempts to
	// spawn. Spawning ticks every 14.4s regardless of cap; if at cap when
	// the timer fires, the spawn is wasted.
	nextSpawnFrame int32
	// available is the current larva count, 0..3.
	available int
}

func newStartingHatchery() hatcheryLarva {
	return hatcheryLarva{
		completionFrame: 0,
		nextSpawnFrame:  secondsToFrame(larvaSpawnIntervalS),
		available:       larvaPerHatchery,
	}
}

func newBuiltHatchery(completionFrame int32) hatcheryLarva {
	return hatcheryLarva{
		completionFrame: completionFrame,
		nextSpawnFrame:  completionFrame + secondsToFrame(larvaSpawnIntervalS),
		available:       larvaPerHatchery,
	}
}

// advance ticks larva spawning up to (and including) `now`. Spawns past
// the cap are wasted.
func (h *hatcheryLarva) advance(now int32) {
	if now < h.completionFrame {
		return
	}
	step := secondsToFrame(larvaSpawnIntervalS)
	for h.nextSpawnFrame <= now {
		if h.available < larvaPerHatchery {
			h.available++
		}
		h.nextSpawnFrame += step
	}
}

// isLarvaConsumingMorph reports whether the subject is a Zerg unit morph
// that consumes a larva (Drone, Zergling, Overlord — the early-window
// set; later units like Hydra/Mutalisk also do but are out of scope).
func isLarvaConsumingMorph(subject string) bool {
	switch subject {
	case models.GeneralUnitDrone, models.GeneralUnitZergling, models.GeneralUnitOverlord:
		return true
	}
	return false
}

func frameToSeconds(frame int32) float64 {
	return float64(frame) * float64(fastestFrameMs) / 1000.0
}

func secondsToFrame(s float64) int32 {
	return int32(s * 1000.0 / float64(fastestFrameMs))
}

// pendingEvent is a scheduled effect on a player's sim state. Buildings
// completing add to `completed`; supply structures add to `supplyMax`;
// worker trains/morphs add to `workers`; gas-gather windows expiring
// adjust `gasWorkers`; Zerg hatch completions append a new larva
// producer. One event captures all of these.
type pendingEvent struct {
	completionFrame   int32
	workersDelta      int
	supplyMaxDelta    int
	gasWorkersDelta   int
	completedBuilding string
	addHatchery       bool
}

// playerSim is the per-player state machine used by the forward pass.
//
// Income model: every alive worker gathers at its race's per-minute rate
// continuously from frame 0. SCV "busy building" downtime is intentionally
// ignored — over-estimating Terran income biases toward admitting more
// commands, matching the user's "err on filtering less" preference.
type playerSim struct {
	race string

	minerals   float64
	supplyUsed int
	supplyMax  int
	workers    int
	// gasWorkers is the number of workers currently mining gas. They are
	// counted in `workers` (the population) but excluded from mineral
	// income.
	gasWorkers int

	completed map[string]int

	pending []pendingEvent

	lastFrame int32

	workerSubject string
	gatherRate    float64 // per worker per minute

	// Geysers known on the map (pixel coords, shared across players).
	geysers []models.MapResourcePosition

	// Per-geyser-index, the completion frame of an own gas building
	// (Refinery / Extractor / Assimilator) placed on it. 0 = none.
	gasBuildingCompletionAtGeyser map[int]int32

	// Per-geyser-index, the frame at which the current gas-mining window
	// ends. Subsequent Harvest1 orders inside the window are no-ops.
	gasActiveUntilFrame map[int]int32

	// hatcheries tracks per-hatchery larva state for Zerg. Empty for
	// other races. The starting hatchery is added at sim init; built
	// hatcheries are appended on completion via the pending-event hook.
	hatcheries []hatcheryLarva
}

func newPlayerSim(race string, geysers []models.MapResourcePosition) *playerSim {
	p := &playerSim{
		race:                          race,
		minerals:                      50,
		supplyUsed:                    4,
		workers:                       4,
		completed:                     map[string]int{},
		geysers:                       geysers,
		gasBuildingCompletionAtGeyser: map[int]int32{},
		gasActiveUntilFrame:           map[int]int32{},
	}
	switch race {
	case "Protoss":
		p.supplyMax = 9
		p.workerSubject = models.GeneralUnitProbe
		p.completed[models.GeneralUnitNexus] = 1
	case "Zerg":
		p.supplyMax = 9
		p.workerSubject = models.GeneralUnitDrone
		p.completed[models.GeneralUnitHatchery] = 1
		p.hatcheries = append(p.hatcheries, newStartingHatchery())
	case "Terran":
		p.supplyMax = 10
		p.workerSubject = models.GeneralUnitSCV
		p.completed[models.GeneralUnitCommandCenter] = 1
	default:
		p.supplyMax = 9
	}
	p.gatherRate = cmdenrich.GatherRatePerMinute(p.workerSubject)
	return p
}

// advanceTo advances the sim's clock to targetFrame, applying mineral income
// and any pending completions in chronological order along the way.
func (p *playerSim) advanceTo(targetFrame int32) {
	if targetFrame <= p.lastFrame {
		return
	}
	cursor := p.lastFrame
	for len(p.pending) > 0 && p.pending[0].completionFrame <= targetFrame {
		ev := p.pending[0]
		p.pending = p.pending[1:]
		p.accumulateIncome(cursor, ev.completionFrame)
		p.workers += ev.workersDelta
		p.supplyMax += ev.supplyMaxDelta
		p.gasWorkers += ev.gasWorkersDelta
		if p.gasWorkers < 0 {
			p.gasWorkers = 0
		}
		if ev.completedBuilding != "" {
			p.completed[ev.completedBuilding]++
		}
		if ev.addHatchery {
			p.hatcheries = append(p.hatcheries, newBuiltHatchery(ev.completionFrame))
		}
		cursor = ev.completionFrame
	}
	// Advance larva spawn timers across all hatcheries.
	for i := range p.hatcheries {
		p.hatcheries[i].advance(targetFrame)
	}
	p.accumulateIncome(cursor, targetFrame)
	p.lastFrame = targetFrame
}

func (p *playerSim) accumulateIncome(fromFrame, toFrame int32) {
	if toFrame <= fromFrame || p.gatherRate == 0 {
		return
	}
	mineralWorkers := p.workers - p.gasWorkers
	if mineralWorkers <= 0 {
		return
	}
	dtSec := frameToSeconds(toFrame - fromFrame)
	p.minerals += float64(mineralWorkers) * (p.gatherRate / 60.0) * dtSec
}

// schedulePending inserts an event in time order. Insertion sort: the list
// stays small (rarely above ~10 simultaneously inflight in early game).
func (p *playerSim) schedulePending(ev pendingEvent) {
	p.pending = append(p.pending, ev)
	for i := len(p.pending) - 1; i > 0 && p.pending[i].completionFrame < p.pending[i-1].completionFrame; i-- {
		p.pending[i], p.pending[i-1] = p.pending[i-1], p.pending[i]
	}
}

// dropDecision describes why the forward pass refused a command. It is the
// payload attached to a Verdict in the trace.
type dropDecision struct {
	verdict Verdict
	reason  string
}

// processBuild applies a Build command's effects to the sim, assuming
// resources have already been checked. Race-specific worker behaviour:
// Probe is unaffected, SCV is occupied for the building's BuildTime
// (ignored — see income-model note), Drone is consumed.
//
// If the Build is a gas building, posBuildTilesXY is the placement tile
// (X, Y) used to associate the gas building with a specific geyser. Pass
// nil if no position is available.
func (p *playerSim) acceptBuild(subject string, econ cmdenrich.UnitEcon, orderFrame int32, posBuildTilesXY *[2]int) {
	p.minerals -= float64(econ.Minerals)
	if p.race == "Zerg" {
		p.workers--
		p.supplyUsed--
	}
	completionFrame := orderFrame + secondsToFrame(econ.BuildTimeS)
	p.schedulePending(pendingEvent{
		completionFrame:   completionFrame,
		supplyMaxDelta:    econ.SupplyDelta,
		completedBuilding: subject,
		// Zerg Hatcheries (and only Hatcheries; Lair / Hive upgrade in
		// place and are out of the 4-min window anyway) become a new
		// larva producer at completion.
		addHatchery: p.race == "Zerg" && subject == models.GeneralUnitHatchery,
	})

	if posBuildTilesXY != nil && isGasBuildingSubject(subject) {
		// Build positions are stored in TILE units; the geyser building
		// footprint is 4×2 tiles, centred on the geyser. Convert to the
		// building's centre pixel and find the nearest known geyser.
		cx := posBuildTilesXY[0]*32 + 64
		cy := posBuildTilesXY[1]*32 + 32
		if idx := p.nearestGeyser(cx, cy, gasGeyserProximityPx2); idx >= 0 {
			if existing, ok := p.gasBuildingCompletionAtGeyser[idx]; !ok || completionFrame < existing {
				p.gasBuildingCompletionAtGeyser[idx] = completionFrame
			}
		}
	}
}

func isGasBuildingSubject(subject string) bool {
	switch subject {
	case models.GeneralUnitRefinery, models.GeneralUnitExtractor, models.GeneralUnitAssimilator:
		return true
	}
	return false
}

// nearestGeyser returns the index of the closest geyser to (px, py) within
// maxDist2 pixels squared. Returns -1 if none in range.
func (p *playerSim) nearestGeyser(px, py, maxDist2 int) int {
	best := -1
	bestD2 := maxDist2 + 1
	for i, g := range p.geysers {
		dx := g.X - px
		dy := g.Y - py
		d2 := dx*dx + dy*dy
		if d2 < bestD2 {
			bestD2 = d2
			best = i
		}
	}
	return best
}

// maybeStartGasGather inspects a player's command for a gas-gather order
// (OrderName="Harvest1") whose target lies near a geyser the player owns
// a completed gas building on. If so, it starts a gas-mining window: 3
// workers leave the mineral line for ~43 seconds (≈ 100 gas at full
// 3-worker saturation). Subsequent Harvest1 orders to the same geyser
// inside the active window are ignored — matches the pro-Zerg "100 gas
// pull-off" pattern conservatively.
func (p *playerSim) maybeStartGasGather(cmd *models.Command) {
	if cmd.OrderName == nil || *cmd.OrderName != models.UnitOrderHarvest1 {
		return
	}
	if cmd.X == nil || cmd.Y == nil {
		return
	}
	idx := p.nearestGeyser(*cmd.X, *cmd.Y, gasGeyserProximityPx2)
	if idx < 0 {
		return
	}
	completion, owned := p.gasBuildingCompletionAtGeyser[idx]
	if !owned || completion > cmd.Frame {
		return
	}
	if p.gasActiveUntilFrame[idx] > cmd.Frame {
		return
	}
	endFrame := cmd.Frame + secondsToFrame(gasGatherDurationS)
	p.gasActiveUntilFrame[idx] = endFrame
	p.gasWorkers += gasWorkersPerGather
	if p.gasWorkers > p.workers {
		p.gasWorkers = p.workers
	}
	p.schedulePending(pendingEvent{
		completionFrame: endFrame,
		gasWorkersDelta: -gasWorkersPerGather,
	})
}

// acceptUnit applies a Train/Morph command's effects (worker or combat unit
// or supply unit like Overlord). Supply *cost* commits at order time per the
// engine; supply *cap* (Overlord +8) commits at completion. Zerg larva-
// consuming morphs (Drone / Zergling / Overlord) decrement one larva from
// any available hatchery.
func (p *playerSim) acceptUnit(subject string, econ cmdenrich.UnitEcon, orderFrame int32) {
	p.minerals -= float64(econ.Minerals)
	p.supplyUsed += econ.SupplyCost
	if isLarvaConsumingMorph(subject) {
		p.consumeLarva()
	}
	completion := orderFrame + secondsToFrame(econ.BuildTimeS)
	ev := pendingEvent{
		completionFrame: completion,
		supplyMaxDelta:  econ.SupplyDelta,
	}
	if cmdenrich.IsWorker(subject) {
		ev.workersDelta = 1
	}
	p.schedulePending(ev)
}

// decide runs the keep/drop logic for one command at the player's current
// frame. The sim must have already been advanced to orderFrame. Returns the
// verdict and the reason string (empty for kept).
//
// Tech-tree prerequisites are NOT used to drop commands: the StarCraft
// engine refuses to execute orders without their prerequisites, so a
// command landing in the replay is itself proof those prerequisites
// existed. Backtrack uses kept commands to drive prereq re-admission
// (see backtrack.go).
func (p *playerSim) decide(enriched cmdenrich.EnrichedCommand, econ cmdenrich.UnitEcon) dropDecision {
	if enriched.Subject == models.GeneralUnitEvolutionChamber {
		return dropDecision{VerdictDropped, "evolution_chamber_heuristic"}
	}
	if enriched.Kind == cmdenrich.KindMakeUnit && isLarvaConsumingMorph(enriched.Subject) {
		if !p.canConsumeLarva(p.lastFrame) {
			return dropDecision{VerdictDropped, "no_larva"}
		}
	}
	if p.minerals < float64(econ.Minerals) {
		return dropDecision{VerdictDropped, "not_enough_minerals"}
	}
	if econ.SupplyCost > 0 && p.supplyUsed+econ.SupplyCost > p.supplyMax {
		return dropDecision{VerdictDropped, "supply_blocked"}
	}
	return dropDecision{VerdictKept, ""}
}

// canConsumeLarva returns true if any hatchery has an available larva at
// `now`, or will produce one within larvaPreorderSlackS seconds. The
// "future larva" leniency models how BW players issue morph commands a
// beat before the larva is actually ready — the engine queues the order
// until the larva spawns. advanceTo must have been called already.
func (p *playerSim) canConsumeLarva(now int32) bool {
	slackFrames := secondsToFrame(larvaPreorderSlackS)
	for i := range p.hatcheries {
		if p.hatcheries[i].available > 0 {
			return true
		}
		if p.hatcheries[i].nextSpawnFrame <= now+slackFrames {
			return true
		}
	}
	return false
}

// consumeLarva takes one larva from the first hatchery that has any
// available, otherwise borrows from the soonest-to-spawn hatchery
// (advancing its nextSpawnFrame by one cycle). Returns true on success.
func (p *playerSim) consumeLarva() bool {
	for i := range p.hatcheries {
		if p.hatcheries[i].available > 0 {
			p.hatcheries[i].available--
			return true
		}
	}
	bestIdx := -1
	var bestFrame int32
	for i := range p.hatcheries {
		if bestIdx < 0 || p.hatcheries[i].nextSpawnFrame < bestFrame {
			bestIdx = i
			bestFrame = p.hatcheries[i].nextSpawnFrame
		}
	}
	if bestIdx < 0 {
		return false
	}
	p.hatcheries[bestIdx].nextSpawnFrame += secondsToFrame(larvaSpawnIntervalS)
	return true
}

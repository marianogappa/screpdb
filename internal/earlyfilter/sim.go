package earlyfilter

import (
	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// Per-frame duration at "Fastest" speed (~23.81 fps). Replays universally
// record frames, so this converts them to seconds for the gather simulation.
const fastestFrameMs = 42

// Gas-gather model: a Right-Click on an owned geyser building pulls up to 3
// workers off minerals; they return after ~100 gas, which at BW's 46
// gas/min/worker saturation rate is ~43 seconds. Capping at 3 and returning
// pre-emptively matches the "100 gas / lair / pull off" Zerg play pattern.
const (
	gasGatherDurationS    = 43.0
	gasWorkersPerGather   = 3
	gasGeyserProximityPx2 = 96 * 96 // squared euclidean threshold; ~3 tiles
)

// Larva model: each Hatchery (and its Lair / Hive upgrades) caps at 3 larva and
// produces one every 14.4 in-game seconds. Morph commands issued with no larva
// available are engine-rejected — the replay still records the order, but the
// engine never executed it — which makes larva availability the primary lever
// against morph spam.
const (
	larvaSpawnIntervalS = 14.4
	larvaPerHatchery    = 3
	// Slack for a morph whose larva is about to spawn. Pro Zergs routinely issue
	// morph commands a beat early and the engine accepts and waits; without slack
	// the filter rejects them as "no_larva" and lands one morph short.
	larvaPreorderSlackS = 5.0
)

// Lair and Hive upgrade in-place, so they reuse this struct.
type hatcheryLarva struct {
	// The starting hatchery uses frame 0; built ones use order frame + build time.
	completionFrame int32
	// Spawning ticks every 14.4s regardless of cap; a tick at cap is wasted.
	nextSpawnFrame int32
	// 0..3.
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

// Spawns past the cap are wasted.
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

// Drone, Zergling and Overlord — the early-window set. Later units (Hydra,
// Mutalisk) consume larva too but are out of scope here.
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

// pendingEvent is a scheduled effect on a player's sim state, covering all of:
// building completions, supply-structure caps, worker trains/morphs, expiring
// gas-gather windows, and a Zerg hatch's new larva producer.
type pendingEvent struct {
	completionFrame   int32
	workersDelta      int
	supplyMaxDelta    int
	gasWorkersDelta   int
	completedBuilding string
	addHatchery       bool
	// buildID ties this completion to a still-cancellable Build in openBuilds, and
	// is non-zero only for Build completions. Once the completion fires, the build
	// is no longer cancellable.
	buildID int
}

// reversibleBuild records what a Build charged to the sim so a later Cancel
// Build can undo it (the extractor / gas trick). Cleared on completion — a
// finished structure cannot be cancelled.
type reversibleBuild struct {
	id              int
	minerals        int
	workersDelta    int // what acceptBuild applied (Zerg: -1; else 0)
	supplyUsedDelta int // what acceptBuild applied (Zerg: -1; else 0)
}

// playerSim is the per-player state machine used by the forward pass. Every
// alive worker gathers at its race's rate continuously from frame 0; SCV "busy
// building" downtime is deliberately ignored, since over-estimating Terran
// income biases toward admitting more commands (err on filtering less).
type playerSim struct {
	race string

	minerals   float64
	supplyUsed int
	supplyMax  int
	workers    int
	// gasWorkers are counted in `workers` (the population) but excluded from
	// mineral income.
	gasWorkers int

	completed map[string]int

	pending []pendingEvent

	// LIFO of in-progress, still-cancellable Builds; buildSeq hands out their ids.
	openBuilds []reversibleBuild
	buildSeq   int

	lastFrame int32

	workerSubject string
	gatherRate    float64 // per worker per minute

	// Pixel coords, shared across players.
	geysers []models.MapResourcePosition

	// Per geyser index, the completion frame of an own gas building on it. 0 = none.
	gasBuildingCompletionAtGeyser map[int]int32

	// Per geyser index, when the current gas-mining window ends. Harvest1 orders
	// inside the window are no-ops.
	gasActiveUntilFrame map[int]int32

	// Empty for non-Zerg. The starting hatchery is added at sim init; built ones
	// are appended on completion via the pending-event hook.
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

// advanceTo applies mineral income and pending completions in chronological
// order along the way.
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
		if ev.buildID != 0 {
			p.removeOpenBuild(ev.buildID)
		}
		if ev.addHatchery {
			p.hatcheries = append(p.hatcheries, newBuiltHatchery(ev.completionFrame))
		}
		cursor = ev.completionFrame
	}
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

// removeOpenBuild drops a completed build, which is no longer cancellable.
func (p *playerSim) removeOpenBuild(id int) {
	for i := range p.openBuilds {
		if p.openBuilds[i].id == id {
			p.openBuilds = append(p.openBuilds[:i], p.openBuilds[i+1:]...)
			return
		}
	}
}

// cancelLastBuild refunds minerals, returns the consumed Drone and its supply
// for Zerg, and unschedules the completion. This models the Zerg extractor /
// gas trick, where an extractor briefly frees a supply so an extra Drone can
// morph past the cap and is then cancelled to pop the Drone back out. Without
// it the sim charges a phantom cost that starves the real early Drone stream.
func (p *playerSim) cancelLastBuild() {
	if len(p.openBuilds) == 0 {
		return
	}
	b := p.openBuilds[len(p.openBuilds)-1]
	p.openBuilds = p.openBuilds[:len(p.openBuilds)-1]
	p.minerals += float64(b.minerals)
	p.workers -= b.workersDelta
	p.supplyUsed -= b.supplyUsedDelta
	for i := range p.pending {
		if p.pending[i].buildID == b.id {
			p.pending = append(p.pending[:i], p.pending[i+1:]...)
			break
		}
	}
}

// Insertion sort: the list stays small, rarely above ~10 inflight early on.
func (p *playerSim) schedulePending(ev pendingEvent) {
	p.pending = append(p.pending, ev)
	for i := len(p.pending) - 1; i > 0 && p.pending[i].completionFrame < p.pending[i-1].completionFrame; i-- {
		p.pending[i], p.pending[i-1] = p.pending[i-1], p.pending[i]
	}
}

// dropDecision is the payload attached to a Verdict in the trace.
type dropDecision struct {
	verdict Verdict
	reason  string
}

// acceptBuild assumes resources were already checked. Race-specific worker
// behaviour: Probe unaffected, SCV occupied for BuildTime (ignored — see the
// income-model note), Drone consumed.
//
// posBuildTilesXY is the placement tile, used to associate a gas building with
// a specific geyser. Pass nil when no position is available.
func (p *playerSim) acceptBuild(subject string, econ cmdenrich.UnitEcon, orderFrame int32, posBuildTilesXY *[2]int) {
	p.minerals -= float64(econ.Minerals)
	workersDelta, supplyUsedDelta := 0, 0
	if p.race == "Zerg" {
		p.workers--
		p.supplyUsed--
		workersDelta, supplyUsedDelta = -1, -1
	}
	p.buildSeq++
	buildID := p.buildSeq
	p.openBuilds = append(p.openBuilds, reversibleBuild{
		id:              buildID,
		minerals:        econ.Minerals,
		workersDelta:    workersDelta,
		supplyUsedDelta: supplyUsedDelta,
	})
	completionFrame := orderFrame + secondsToFrame(econ.BuildTimeS)
	p.schedulePending(pendingEvent{
		completionFrame:   completionFrame,
		supplyMaxDelta:    econ.SupplyDelta,
		completedBuilding: subject,
		// Only Hatcheries: Lair / Hive upgrade in place and are outside the 4-minute
		// window anyway.
		addHatchery: p.race == "Zerg" && subject == models.GeneralUnitHatchery,
		buildID:     buildID,
	})

	if posBuildTilesXY != nil && isGasBuildingSubject(subject) {
		// Build positions are TILE units and the gas-building footprint is 4×2 tiles
		// centred on the geyser, so convert to the building's centre pixel first.
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

// Returns -1 if none within maxDist2 (pixels squared).
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

// maybeStartGasGather starts a gas-mining window when a Harvest1 order targets
// a geyser the player has a completed gas building on: 3 workers leave the
// mineral line for ~43s (≈100 gas at full saturation). Repeat orders to the
// same geyser inside the window are ignored, matching the pro-Zerg "100 gas
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

// acceptUnit applies a Train/Morph that decide() already approved and returns
// how many units it produced. Supply COST commits at order time per the engine;
// supply CAP (Overlord +8) commits at completion.
//
// intended is the issuing selection size (1 when unknown): one Zerg larva-morph
// command morphs every selected larva at once, so the produced count is capped
// by the larva and minerals available BEFORE the command — what the engine
// could actually have built.
//
// Resource accounting deliberately commits only one unit's cost, matching the
// original single-command model, so keep/drop verdicts are unchanged for every
// other command and detector. The multi-unit count is an annotation consumed
// only by build-order supply counting.
func (p *playerSim) acceptUnit(subject string, econ cmdenrich.UnitEcon, orderFrame int32, intended int) int {
	produced := p.producedCount(subject, econ, intended)

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
	return produced
}

// producedCount is >1 only for a multi-larva Zerg morph the player could
// afford. Pure read — does not mutate sim state.
func (p *playerSim) producedCount(subject string, econ cmdenrich.UnitEcon, intended int) int {
	if intended <= 1 {
		return 1
	}
	count := intended
	if econ.Minerals > 0 {
		if affordable := int(p.minerals) / econ.Minerals; affordable < count {
			count = affordable
		}
	}
	if isLarvaConsumingMorph(subject) {
		if larva := p.availableLarvaCount(); larva < count {
			count = larva
		}
	}
	if count < 1 {
		count = 1
	}
	return count
}

// No pre-order slack.
func (p *playerSim) availableLarvaCount() int {
	n := 0
	for i := range p.hatcheries {
		n += p.hatcheries[i].available
	}
	return n
}

// decide runs the keep/drop logic for one command; the sim must already be
// advanced to orderFrame.
//
// Tech-tree prerequisites are deliberately NOT used to drop commands: the
// engine refuses orders without their prerequisites, so a command landing in the
// replay is itself proof they existed. Backtrack uses kept commands to drive
// prereq re-admission (see backtrack.go).
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

// canConsumeLarva also accepts a larva arriving within larvaPreorderSlackS,
// modelling how players issue morphs a beat early and the engine queues the
// order until it spawns. advanceTo must have been called already.
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

// consumeLarva otherwise borrows from the soonest-to-spawn hatchery, advancing
// its nextSpawnFrame by one cycle.
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

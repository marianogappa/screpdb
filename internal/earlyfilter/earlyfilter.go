package earlyfilter

import (
	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

const maxBacktrackIterations = 5

// Apply runs the early-game spam filter on commands and returns a filtered
// list plus optional debug data. See package doc for the algorithm overview.
//
// Apply does not mutate input slices; the Result.Commands slice header is
// fresh, and individual *models.Command pointers alias the input.
//
// mapCtx provides the per-replay map metadata needed by the filter. The
// gas-worker tracker reads mapCtx.Geysers; pass nil to disable gas-aware
// income adjustment (mineral over-counting will resume).
func Apply(replay *models.Replay, players []*models.Player, mapCtx *models.ReplayMapContext, commands []*models.Command, opts Options) Result {
	if opts.MaxSecond == 0 {
		opts.MaxSecond = defaultMaxSecond
	}

	raceByPlayer := map[int64]string{}
	for _, p := range players {
		if p == nil {
			continue
		}
		raceByPlayer[int64(p.PlayerID)] = p.Race
	}

	var geysers []models.MapResourcePosition
	if mapCtx != nil {
		geysers = mapCtx.Geysers
	}

	mustKeep := map[int]bool{}
	forceDrop := map[int]bool{}

	var verdicts map[int]Verdict
	var reasons map[int]string
	var mineralsAfter map[int]int
	iterations := 0

	for it := 0; it < maxBacktrackIterations; it++ {
		iterations = it + 1
		sims := initSims(raceByPlayer, geysers)
		verdicts, reasons, mineralsAfter = runForward(commands, sims, mustKeep, forceDrop, opts)

		violations := findViolations(commands, verdicts)
		if len(violations) == 0 {
			break
		}
		if !resolveViolations(violations, commands, verdicts, mustKeep, forceDrop) {
			break
		}
	}

	filtered := buildFilteredCommands(commands, verdicts)
	stats := buildStats(commands, verdicts, forceDrop)

	var trace *Trace
	if opts.DebugDir != "" {
		trace = buildTrace(replay, players, commands, verdicts, reasons, mineralsAfter, opts.MaxSecond, iterations, raceByPlayer, geysers)
		// Debug output failure must never break ingestion — swallow.
		_ = writeTrace(opts.DebugDir, replay.FileChecksum, trace)
	}

	return Result{
		Commands: filtered,
		Trace:    trace,
		Stats:    stats,
	}
}

func initSims(raceByPlayer map[int64]string, geysers []models.MapResourcePosition) map[int64]*playerSim {
	sims := make(map[int64]*playerSim, len(raceByPlayer))
	for pid, race := range raceByPlayer {
		sims[pid] = newPlayerSim(race, geysers)
	}
	return sims
}

// runForward performs one full pass over commands, dispatching each to its
// player's sim and recording a Verdict + Reason + minerals snapshot per
// command index. mustKeep and forceDrop override the sim's normal decision.
func runForward(
	commands []*models.Command,
	sims map[int64]*playerSim,
	mustKeep, forceDrop map[int]bool,
	opts Options,
) (verdicts map[int]Verdict, reasons map[int]string, mineralsAfter map[int]int) {
	verdicts = make(map[int]Verdict, len(commands))
	reasons = make(map[int]string)
	mineralsAfter = make(map[int]int)

	maxSec := opts.MaxSecond
	if maxSec == 0 {
		maxSec = defaultMaxSecond
	}

	for i, cmd := range commands {
		if cmd == nil || cmd.Player == nil {
			verdicts[i] = VerdictKept
			continue
		}
		pid := int64(cmd.Player.PlayerID)
		sim := sims[pid]
		if sim == nil {
			verdicts[i] = VerdictKept
			continue
		}

		sim.advanceTo(cmd.Frame)
		// Gas-mining detection runs for every command in the window:
		// Right-click / Targeted-Order with OrderName=Harvest1 is the
		// signal that workers leave minerals for gas. Outside the window
		// we still pass through but skip the gas hook (steady state).
		if cmd.SecondsFromGameStart < maxSec {
			sim.maybeStartGasGather(cmd)
		}

		if cmd.SecondsFromGameStart >= maxSec {
			verdicts[i] = VerdictKept
			mineralsAfter[i] = int(sim.minerals)
			continue
		}

		enriched, ok := cmdenrich.Classify(cmd)
		if !ok || !kindFiltered(enriched.Kind) {
			verdicts[i] = VerdictKept
			mineralsAfter[i] = int(sim.minerals)
			continue
		}
		econ, hasEcon := cmdenrich.EconOf(enriched.Subject)
		if !hasEcon {
			verdicts[i] = VerdictKept
			mineralsAfter[i] = int(sim.minerals)
			continue
		}

		switch {
		case forceDrop[i]:
			verdicts[i] = VerdictDroppedByBacktrack
			reasons[i] = "freed_minerals_for_backtrack"
		case mustKeep[i]:
			verdicts[i] = VerdictReadmitted
			reasons[i] = "tech_tree_readmit"
			applyEffects(sim, enriched, econ, cmd)
		default:
			d := sim.decide(enriched, econ)
			verdicts[i] = d.verdict
			reasons[i] = d.reason
			if d.verdict == VerdictKept {
				applyEffects(sim, enriched, econ, cmd)
			}
		}
		mineralsAfter[i] = int(sim.minerals)
	}
	return verdicts, reasons, mineralsAfter
}

func applyEffects(sim *playerSim, enriched cmdenrich.EnrichedCommand, econ cmdenrich.UnitEcon, cmd *models.Command) {
	if enriched.Kind == cmdenrich.KindMakeBuilding {
		var pos *[2]int
		if cmd.X != nil && cmd.Y != nil {
			pos = &[2]int{*cmd.X, *cmd.Y}
		}
		sim.acceptBuild(enriched.Subject, econ, cmd.Frame, pos)
		return
	}
	sim.acceptUnit(enriched.Subject, econ, cmd.Frame)
}

// buildFilteredCommands materialises the kept-or-readmitted command list in
// the original time order. Dropped and forcibly-dropped commands are
// excluded.
func buildFilteredCommands(commands []*models.Command, verdicts map[int]Verdict) []*models.Command {
	out := make([]*models.Command, 0, len(commands))
	for i, cmd := range commands {
		switch verdicts[i] {
		case VerdictKept, VerdictReadmitted:
			out = append(out, cmd)
		case "":
			// Default for unclassified commands (no entry written): keep.
			out = append(out, cmd)
		}
	}
	return out
}

func buildStats(commands []*models.Command, verdicts map[int]Verdict, forceDrop map[int]bool) Stats {
	per := map[int64]PlayerStats{}
	for i, cmd := range commands {
		if cmd == nil || cmd.Player == nil {
			continue
		}
		pid := int64(cmd.Player.PlayerID)
		s := per[pid]
		s.Total++
		switch verdicts[i] {
		case VerdictKept, "":
			s.Kept++
		case VerdictReadmitted:
			s.Kept++
			s.Readmitted++
		case VerdictDropped:
			s.Dropped++
		case VerdictDroppedByBacktrack:
			s.Dropped++
			if forceDrop[i] {
				s.WorkerDropsForBacktrack++
			}
		}
		per[pid] = s
	}
	return Stats{PerPlayer: per}
}

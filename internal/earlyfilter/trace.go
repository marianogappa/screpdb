package earlyfilter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// Verdict labels what the filter decided about a single command.
type Verdict string

const (
	// VerdictKept: the command stayed in the output stream.
	VerdictKept Verdict = "kept"
	// VerdictDropped: the forward pass dropped the command for resource or
	// tech-tree reasons.
	VerdictDropped Verdict = "dropped"
	// VerdictReadmitted: the backtrack pass restored a previously-dropped
	// Build because a kept consequent (Zealot/Marine/Zergling) proved its
	// existence.
	VerdictReadmitted Verdict = "readmitted"
	// VerdictDroppedByBacktrack: the backtrack pass forcibly dropped a
	// previously-kept worker train to free minerals for a re-admitted
	// prerequisite.
	VerdictDroppedByBacktrack Verdict = "dropped_by_backtrack"
)

// Decision is one entry in the per-player decision log for the trace.
type Decision struct {
	Frame         int32   `json:"frame"`
	Second        int     `json:"second"`
	Action        string  `json:"action"`            // ActionType (Build / Train / Unit Morph)
	Subject       string  `json:"subject"`
	Verdict       Verdict `json:"verdict"`
	Reason        string  `json:"reason,omitempty"`
	MineralsAfter int     `json:"minerals_after"`
}

// TickSnapshot is one row in the player's per-second state timeline.
type TickSnapshot struct {
	Second     int    `json:"second"`
	Minerals   int    `json:"minerals"`
	Supply     string `json:"supply"` // formatted "used/max" for human reading
	Workers    int    `json:"workers"`
	Iteration  int    `json:"iteration,omitempty"` // backtrack iteration that produced this snapshot
}

// PlayerTrace gathers everything the trace records for one player.
type PlayerTrace struct {
	PlayerID  int64          `json:"player_id"`
	Race      string         `json:"race"`
	Ticks     []TickSnapshot `json:"ticks"`
	Decisions []Decision     `json:"decisions"`
	Summary   PlayerStats    `json:"summary"`
}

// Trace is the top-level debug payload written to DebugDir.
type Trace struct {
	Replay     string        `json:"replay"`
	MaxSecond  int           `json:"max_second"`
	Iterations int           `json:"iterations"`
	Players    []PlayerTrace `json:"players"`
}

// buildTrace assembles the per-replay debug payload from the final iteration's
// verdicts/reasons plus a fresh snapshot-capturing forward replay so the
// caller can see how mineral / supply / worker counts evolved.
func buildTrace(
	replay *models.Replay,
	players []*models.Player,
	commands []*models.Command,
	verdicts map[int]Verdict,
	reasons map[int]string,
	mineralsAfter map[int]int,
	maxSecond int,
	iterations int,
	raceByPlayer map[int64]string,
	geysers []models.MapResourcePosition,
) *Trace {
	t := &Trace{
		Replay:     replay.FileName,
		MaxSecond:  maxSecond,
		Iterations: iterations,
	}

	decisionsByPlayer := map[int64][]Decision{}
	for i, cmd := range commands {
		if cmd == nil || cmd.Player == nil {
			continue
		}
		if cmd.SecondsFromGameStart >= maxSecond {
			continue
		}
		en, ok := cmdenrich.Classify(cmd)
		if !ok || !kindFiltered(en.Kind) {
			continue
		}
		// Only record decisions for commands the filter could actually
		// reason about (Build/Train/Morph with known econ). Trains of
		// units we don't know costs for would record as "kept" with no
		// useful detail — skip those.
		if _, hasEcon := cmdenrich.EconOf(en.Subject); !hasEcon {
			continue
		}
		pid := int64(cmd.Player.PlayerID)
		decisionsByPlayer[pid] = append(decisionsByPlayer[pid], Decision{
			Frame:         cmd.Frame,
			Second:        cmd.SecondsFromGameStart,
			Action:        cmd.ActionType,
			Subject:       en.Subject,
			Verdict:       verdicts[i],
			Reason:        reasons[i],
			MineralsAfter: mineralsAfter[i],
		})
	}

	ticks := captureTicks(commands, raceByPlayer, verdicts, maxSecond, geysers)

	for _, p := range players {
		if p == nil {
			continue
		}
		pid := int64(p.PlayerID)
		t.Players = append(t.Players, PlayerTrace{
			PlayerID:  pid,
			Race:      p.Race,
			Ticks:     ticks[pid],
			Decisions: decisionsByPlayer[pid],
			Summary:   summaryFor(commands, verdicts, pid),
		})
	}
	return t
}

// captureTicks re-runs the final forward simulation with snapshotting enabled,
// recording each player's mineral / supply / worker state at every 30 seconds
// from 0 to maxSecond. Re-running is cheaper than carrying snapshot state
// through the main loop.
func captureTicks(
	commands []*models.Command,
	raceByPlayer map[int64]string,
	verdicts map[int]Verdict,
	maxSecond int,
	geysers []models.MapResourcePosition,
) map[int64][]TickSnapshot {
	const tickStepSeconds = 30

	sims := map[int64]*playerSim{}
	for pid, race := range raceByPlayer {
		sims[pid] = newPlayerSim(race, geysers)
	}
	out := map[int64][]TickSnapshot{}
	for pid := range sims {
		out[pid] = nil
	}

	emit := func(atSecond int) {
		for pid, sim := range sims {
			sim.advanceTo(secondsToFrame(float64(atSecond)))
			out[pid] = append(out[pid], TickSnapshot{
				Second:   atSecond,
				Minerals: int(sim.minerals),
				Supply:   fmt.Sprintf("%d/%d", sim.supplyUsed, sim.supplyMax),
				Workers:  sim.workers,
			})
		}
	}

	emit(0)
	nextEmit := tickStepSeconds

	for i, cmd := range commands {
		if cmd == nil || cmd.Player == nil {
			continue
		}
		if cmd.SecondsFromGameStart >= maxSecond {
			break
		}
		// Emit tick snapshots at fixed cadence whenever we cross a step.
		for cmd.SecondsFromGameStart >= nextEmit && nextEmit <= maxSecond {
			emit(nextEmit)
			nextEmit += tickStepSeconds
		}
		sim := sims[int64(cmd.Player.PlayerID)]
		if sim == nil {
			continue
		}
		sim.advanceTo(cmd.Frame)
		sim.maybeStartGasGather(cmd)

		en, ok := cmdenrich.Classify(cmd)
		if !ok || !kindFiltered(en.Kind) {
			continue
		}
		econ, hasEcon := cmdenrich.EconOf(en.Subject)
		if !hasEcon {
			continue
		}
		v := verdicts[i]
		if v == VerdictKept || v == VerdictReadmitted {
			if en.Kind == cmdenrich.KindMakeBuilding {
				var pos *[2]int
				if cmd.X != nil && cmd.Y != nil {
					pos = &[2]int{*cmd.X, *cmd.Y}
				}
				sim.acceptBuild(en.Subject, econ, cmd.Frame, pos)
			} else {
				sim.acceptUnit(en.Subject, econ, cmd.Frame)
			}
		}
	}
	for nextEmit <= maxSecond {
		emit(nextEmit)
		nextEmit += tickStepSeconds
	}
	return out
}

func summaryFor(commands []*models.Command, verdicts map[int]Verdict, pid int64) PlayerStats {
	var s PlayerStats
	for i, cmd := range commands {
		if cmd == nil || cmd.Player == nil {
			continue
		}
		if int64(cmd.Player.PlayerID) != pid {
			continue
		}
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
			s.WorkerDropsForBacktrack++
		}
	}
	return s
}

// writeTrace writes t as JSON to <dir>/<checksum>.json. Any I/O error is
// returned to the caller, which logs and ignores — debug output must never
// break ingestion.
func writeTrace(dir string, checksum string, t *Trace) error {
	if dir == "" || t == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	name := checksum
	if name == "" {
		name = "unknown"
	}
	path := filepath.Join(dir, name+".json")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(t); err != nil {
		return fmt.Errorf("encode trace: %w", err)
	}
	return nil
}

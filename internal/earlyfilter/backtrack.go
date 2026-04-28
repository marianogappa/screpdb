package earlyfilter

import (
	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// violation is a kept Train/Morph whose producer building was not present
// (i.e. neither kept nor re-admitted) at its frame in the latest forward
// pass. The backtrack pass uses violations as input to decide which dropped
// Build commands to re-admit.
type violation struct {
	cmdIdx   int
	missing  string // producer subject (e.g. "Gateway")
	playerID int64
	frame    int32
}

// findViolations scans the latest forward-pass verdicts for kept Train/Morph
// commands whose tech-tree producer is not present in the kept set at that
// frame. Each violation flags a missing prerequisite that the backtrack pass
// can attempt to re-admit.
func findViolations(commands []*models.Command, verdicts map[int]Verdict) []violation {
	// Build per-player {producer subject -> sorted completion frames} from
	// kept/readmitted Build commands.
	completions := map[int64]map[string][]int32{}
	for i, cmd := range commands {
		v := verdicts[i]
		if v != VerdictKept && v != VerdictReadmitted {
			continue
		}
		if cmd == nil || cmd.Player == nil {
			continue
		}
		en, ok := cmdenrich.Classify(cmd)
		if !ok || en.Kind != cmdenrich.KindMakeBuilding {
			continue
		}
		econ, ok := cmdenrich.EconOf(en.Subject)
		if !ok {
			continue
		}
		pid := int64(cmd.Player.PlayerID)
		if completions[pid] == nil {
			completions[pid] = map[string][]int32{}
		}
		completions[pid][en.Subject] = append(completions[pid][en.Subject], cmd.Frame+secondsToFrame(econ.BuildTimeS))
	}

	var out []violation
	for i, cmd := range commands {
		v := verdicts[i]
		if v != VerdictKept && v != VerdictReadmitted {
			continue
		}
		if cmd == nil || cmd.Player == nil {
			continue
		}
		en, ok := cmdenrich.Classify(cmd)
		if !ok {
			continue
		}
		pid := int64(cmd.Player.PlayerID)

		switch en.Kind {
		case cmdenrich.KindMakeUnit:
			// Train/Morph: a kept consequent proves its producer existed.
			producer, has := cmdenrich.ProducerOf(en.Subject)
			if !has {
				continue
			}
			// Workers' producers are starting buildings (Nexus/CC/Hatchery) —
			// always present from frame 0, never missing.
			if cmdenrich.IsWorker(en.Subject) {
				continue
			}
			if !anyFrameAtOrBefore(completions[pid][producer], cmd.Frame) {
				out = append(out, violation{
					cmdIdx:   i,
					missing:  producer,
					playerID: pid,
					frame:    cmd.Frame,
				})
			}
		case cmdenrich.KindMakeBuilding:
			// Build: a kept Build proves its tech-tree prerequisites
			// existed. The engine wouldn't have placed Photon Cannon
			// without Forge+Pylon already on the field, so missing
			// prereqs in the simulated state mean the prereq was
			// dropped by the forward pass and must be re-admitted.
			prereqs, has := cmdenrich.PrereqsOf(en.Subject)
			if !has {
				continue
			}
			for _, pq := range prereqs {
				if !anyFrameAtOrBefore(completions[pid][pq], cmd.Frame) {
					out = append(out, violation{
						cmdIdx:   i,
						missing:  pq,
						playerID: pid,
						frame:    cmd.Frame,
					})
				}
			}
		}
	}
	return out
}

func anyFrameAtOrBefore(frames []int32, f int32) bool {
	for _, ff := range frames {
		if ff <= f {
			return true
		}
	}
	return false
}

// resolveViolations attempts to fix each violation by adding the missing
// prerequisite chain to mustKeep and the latest kept worker train before the
// re-admitted prereq's frame to forceDrop. Returns true if any new mustKeep
// or forceDrop entry was added (i.e. progress was made for the next iteration).
//
// Each Build re-admission also pulls in its own prerequisites recursively
// (Gateway → Pylon, etc.) via cmdenrich.PrereqsOf.
func resolveViolations(
	violations []violation,
	commands []*models.Command,
	verdicts map[int]Verdict,
	mustKeep map[int]bool,
	forceDrop map[int]bool,
) bool {
	progress := false
	for _, v := range violations {
		chain := buildPrereqChain(v.missing)
		for _, subj := range chain {
			idx := findLatestBuildBefore(commands, verdicts, v.playerID, subj, v.frame, mustKeep)
			if idx < 0 {
				continue
			}
			if !mustKeep[idx] {
				mustKeep[idx] = true
				progress = true
			}
			// Drop one worker train before the re-admitted Build to free
			// minerals. Skip if we already dropped enough — the next
			// iteration will run forward and check whether the budget
			// balances; if not, this loop fires again.
			wIdx := findLatestKeptWorkerBefore(commands, verdicts, v.playerID, commands[idx].Frame, forceDrop)
			if wIdx >= 0 && !forceDrop[wIdx] {
				forceDrop[wIdx] = true
				progress = true
			}
		}
	}
	return progress
}

// buildPrereqChain returns subj plus its transitive PrereqsOf, deepest-first
// for stable ordering. The set is bounded (Pylon's chain is just [Pylon];
// Gateway's chain is [Pylon, Gateway]).
func buildPrereqChain(subj string) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(s string)
	walk = func(s string) {
		if seen[s] {
			return
		}
		seen[s] = true
		if prereqs, ok := cmdenrich.PrereqsOf(s); ok {
			for _, p := range prereqs {
				walk(p)
			}
		}
		out = append(out, s)
	}
	walk(subj)
	return out
}

// findLatestBuildBefore returns the index of the most recent Build of subject
// for the given player whose frame is strictly before `frame`. Prefers a
// previously-dropped Build (the natural target of re-admission) but falls
// back to any Build of that subject if none was dropped.
func findLatestBuildBefore(
	commands []*models.Command,
	verdicts map[int]Verdict,
	playerID int64,
	subject string,
	frame int32,
	mustKeep map[int]bool,
) int {
	bestDropped := -1
	bestAny := -1
	for i, cmd := range commands {
		if cmd == nil || cmd.Player == nil {
			continue
		}
		if int64(cmd.Player.PlayerID) != playerID {
			continue
		}
		if cmd.Frame >= frame {
			continue
		}
		en, ok := cmdenrich.Classify(cmd)
		if !ok || en.Kind != cmdenrich.KindMakeBuilding {
			continue
		}
		if en.Subject != subject {
			continue
		}
		bestAny = i
		if verdicts[i] == VerdictDropped && !mustKeep[i] {
			bestDropped = i
		}
	}
	if bestDropped >= 0 {
		return bestDropped
	}
	return bestAny
}

// findLatestKeptWorkerBefore returns the index of the most recent kept worker
// Train/Morph for the given player whose frame is strictly before `frame` and
// has not already been forceDrop'd. Returns -1 if no candidate exists.
func findLatestKeptWorkerBefore(
	commands []*models.Command,
	verdicts map[int]Verdict,
	playerID int64,
	frame int32,
	forceDrop map[int]bool,
) int {
	best := -1
	for i, cmd := range commands {
		if cmd == nil || cmd.Player == nil {
			continue
		}
		if int64(cmd.Player.PlayerID) != playerID {
			continue
		}
		if cmd.Frame >= frame {
			continue
		}
		if forceDrop[i] {
			continue
		}
		v := verdicts[i]
		if v != VerdictKept && v != VerdictReadmitted {
			continue
		}
		en, ok := cmdenrich.Classify(cmd)
		if !ok || en.Kind != cmdenrich.KindMakeUnit {
			continue
		}
		if !cmdenrich.IsWorker(en.Subject) {
			continue
		}
		best = i
	}
	return best
}

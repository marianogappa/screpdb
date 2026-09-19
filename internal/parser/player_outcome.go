package parser

import (
	"fmt"
	"strings"

	"github.com/marianogappa/screpdb/internal/models"
)

// ConcessionDoubtProductionSec is how long an opponent must have gone without
// producing before "they were still in the game" stops being credible. Measured
// over 713 concessions: 99.3% of them sit below it.
const ConcessionDoubtProductionSec = 45

// DerivePlayerOutcomes fills in each player's own result, which is a different
// question from their team's and answerable far more often. The team question
// needs every coalition resolved; this one needs only the player's own exit.
// Measured on a 3,364-replay corpus: the team result is knowable in 64% of
// games, a given player's own result in 84%.
//
// Runs after team attribution, whose answer it prefers wherever there is one.
// The saver is handled last because StarCraft records no Leave Game for them —
// the recording simply stops — so their exit has to be read from the fact that
// the file ends while opponents were still playing.
func DerivePlayerOutcomes(players []*models.Player, commands []*models.Command, durationSec int, repSaverPID *byte) string {
	// A game against the computer is not a contest screpdb keeps a result for.
	// Filing it under "unknown" would inflate that bucket with games the default
	// filter already hides, implying a question we failed to answer rather than
	// one we never asked.
	for _, p := range players {
		if p != nil && !p.IsObserver && p.Type == "Computer" {
			for _, q := range players {
				if q == nil || q.IsObserver {
					continue
				}
				q.PlayerOutcome, q.TeamOutcome = models.OutcomeNotScored, models.OutcomeNotScored
				q.PlayerOutcomeReason = "not scored: a game against the computer"
			}
			return "not scored: a game against the computer"
		}
	}

	leftAt := map[byte]int{}
	droppedOut := map[byte]bool{}
	for _, c := range commands {
		if c == nil || c.Player == nil || c.ActionType != "Leave Game" {
			continue
		}
		if _, seen := leftAt[c.Player.PlayerID]; !seen {
			leftAt[c.Player.PlayerID] = c.SecondsFromGameStart
			// The engine records why: a dropped connection is not a concession,
			// even though both write the same command.
			if c.LeaveReason != nil && strings.EqualFold(strings.TrimSpace(*c.LeaveReason), "Dropped") {
				droppedOut[c.Player.PlayerID] = true
			}
		}
	}

	teamOf := map[byte]byte{}
	for _, p := range players {
		if p == nil || p.IsObserver || p.Type == "Computer" {
			continue
		}
		teamOf[p.PlayerID] = p.Team
	}

	// A rival counts as still playing at second t only if they ACTED after it.
	// "No Leave Game recorded" is not the same thing: a destroyed player records
	// no leave either, so reading absence as presence turns the victor of a
	// last-second kill into a conceder.
	lastAction := map[byte]int{}
	for _, c := range commands {
		if c == nil || c.Player == nil || !provesPresence(c.ActionType) {
			continue
		}
		lastAction[c.Player.PlayerID] = max(lastAction[c.Player.PlayerID], c.SecondsFromGameStart)
	}
	rivalActedAfter := func(pid byte, sec int) bool {
		for other, team := range teamOf {
			if other == pid || team == teamOf[pid] {
				continue
			}
			if lastAction[other] > sec {
				return true
			}
		}
		return false
	}

	// For the saver there is no "after": the recording stops with them, so a
	// rival counts as still in the game if they had not left by the end. That
	// reading is right 96% of the time but silently wrong when the rival was a
	// corpse — killed moments before, with no leave to record. Measured over 713
	// such games: the liveliest remaining rival had produced within 10s in 89% of
	// them and within 45s in 99.3%. Past that the "opponent" is almost certainly
	// dead, and asserting a loss is worse than admitting ignorance.
	lastProduction := map[byte]int{}
	for _, c := range commands {
		if c == nil || c.Player == nil {
			continue
		}
		if _, isProduction := ProductionActionTypes[c.ActionType]; isProduction {
			lastProduction[c.Player.PlayerID] = max(lastProduction[c.Player.PlayerID], c.SecondsFromGameStart)
		}
	}
	// rivalStillIn also reports how fresh the liveliest such rival was.
	rivalStillIn := func(pid byte) (bool, int) {
		found, freshest := false, 1<<30
		for other, team := range teamOf {
			if other == pid || team == teamOf[pid] {
				continue
			}
			if _, gone := leftAt[other]; gone {
				continue
			}
			found = true
			freshest = min(freshest, durationSec-lastProduction[other])
		}
		return found, freshest
	}

	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		switch {
		// Their own leave says the connection went, not that they gave up.
		case droppedOut[p.PlayerID]:
			p.PlayerOutcome = models.OutcomeDisconnected
			p.PlayerOutcomeReason = fmt.Sprintf("their own Leave Game at %d:%02d carries the Dropped reason",
				leftAt[p.PlayerID]/60, leftAt[p.PlayerID]%60)

		// They walked out of a game that was demonstrably still being played.
		// This comes before the coalition result on purpose: conceding is
		// conceding, and their side winning afterwards does not undo it.
		case hasLeft(leftAt, p.PlayerID) && rivalActedAfter(p.PlayerID, leftAt[p.PlayerID]):
			p.PlayerOutcome = models.OutcomeLost
			p.PlayerOutcomeReason = fmt.Sprintf("conceded: left at %d:%02d while an opponent was still playing",
				leftAt[p.PlayerID]/60, leftAt[p.PlayerID]%60)

		// The coalition question was settled, which settles this one too.
		case p.TeamOutcome.Known():
			p.PlayerOutcome = p.TeamOutcome
			p.PlayerOutcomeReason = "their side's result, which they share"

		default:
			p.PlayerOutcome = models.OutcomeUnknown
			p.PlayerOutcomeReason = "no evidence either way: the coalition result is unknown and they did not concede"
		}
	}

	// The saver never records a leave, so the two cases above cannot see them.
	// The recording ending while a rival was still playing means they stopped
	// first, which is the same concession as quitting.
	if repSaverPID == nil {
		return ""
	}
	saver := *repSaverPID
	if _, ok := teamOf[saver]; !ok {
		return ""
	}
	for _, p := range players {
		if p == nil || p.PlayerID != saver || p.TeamOutcome.Known() {
			continue
		}
		alive, freshest := rivalStillIn(saver)
		switch {
		case !alive:
			// Nobody left to concede to.
		case freshest >= ConcessionDoubtProductionSec:
			p.PlayerOutcome = models.OutcomeUnknown
			p.PlayerOutcomeReason = fmt.Sprintf(
				"unknown: the recording ends at their exit, but the last opponent still in it had produced nothing for %ds, so they may already have been destroyed",
				freshest)
		default:
			p.PlayerOutcome = models.OutcomeLost
			p.PlayerOutcomeReason = "conceded: the recording ends at their exit with an opponent still in the game"
		}
	}
	return ""
}

func hasLeft(leftAt map[byte]int, pid byte) bool {
	_, ok := leftAt[pid]
	return ok
}

// ApplySaverDisconnectOutcomes overrides every result in a game the saver
// dropped out of. Their leave cluster is a phantom — the other players did not
// quit, the recording merely stopped seeing them — so no winner can be read off
// it. The saver is the one player whose result is known: they disconnected.
func ApplySaverDisconnectOutcomes(players []*models.Player, saverPID byte) {
	for _, p := range players {
		if p == nil {
			continue
		}
		p.TeamOutcome = models.OutcomeUnknown
		if p.PlayerID == saverPID && !p.IsObserver {
			p.PlayerOutcome = models.OutcomeDisconnected
			p.PlayerOutcomeReason = "lost the connection; the game carried on without this recording"
			continue
		}
		p.PlayerOutcome = models.OutcomeUnknown
		p.PlayerOutcomeReason = "unknown: the saver dropped, so the phantom leaves say nothing about anyone"
	}
}

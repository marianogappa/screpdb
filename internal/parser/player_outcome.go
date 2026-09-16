package parser

import "github.com/marianogappa/screpdb/internal/models"

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
func DerivePlayerOutcomes(players []*models.Player, commands []*models.Command, repSaverPID *byte) {
	leftAt := map[byte]int{}
	for _, c := range commands {
		if c == nil || c.Player == nil || c.ActionType != "Leave Game" {
			continue
		}
		if _, seen := leftAt[c.Player.PlayerID]; !seen {
			leftAt[c.Player.PlayerID] = c.SecondsFromGameStart
		}
	}

	teamOf := map[byte]byte{}
	for _, p := range players {
		if p == nil || p.IsObserver || p.Type == "Computer" {
			continue
		}
		teamOf[p.PlayerID] = p.Team
	}

	// A rival was still in the game at second t if some player on another team
	// had not left by then. Quitting while that holds is a concession.
	rivalStillIn := func(pid byte, sec int) bool {
		for other, team := range teamOf {
			if other == pid || team == teamOf[pid] {
				continue
			}
			if left, ok := leftAt[other]; !ok || left > sec {
				return true
			}
		}
		return false
	}

	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		switch {
		// The team question was settled, which settles this one too: a player
		// belongs to their coalition's result whether or not they saw the end.
		case p.TeamOutcome.Known():
			p.Outcome = p.TeamOutcome

		// They quit a game that was still being contested.
		case hasLeft(leftAt, p.PlayerID) && rivalStillIn(p.PlayerID, leftAt[p.PlayerID]):
			p.Outcome = models.OutcomeLost

		default:
			p.Outcome = models.OutcomeUnknown
		}
	}

	// The saver never records a leave, so the two cases above cannot see them.
	// The recording ending while a rival was still playing means they stopped
	// first, which is the same concession as quitting.
	if repSaverPID == nil {
		return
	}
	saver := *repSaverPID
	if _, ok := teamOf[saver]; !ok {
		return
	}
	for _, p := range players {
		if p == nil || p.PlayerID != saver || p.TeamOutcome.Known() {
			continue
		}
		if rivalStillIn(saver, maxInt) {
			p.Outcome = models.OutcomeLost
		}
	}
}

const maxInt = int(^uint(0) >> 1)

func hasLeft(leftAt map[byte]int, pid byte) bool {
	_, ok := leftAt[pid]
	return ok
}

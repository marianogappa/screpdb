package parser

import "github.com/marianogappa/screpdb/internal/models"

// The leave-based winner rule ("largest remaining coalition wins") treats a
// player with no Leave Game command as alive. Elimination emits no command, so
// a coalition wiped out in the final seconds — before the replay saver exits and
// stops the recording — looks like the sole survivor and is credited the win.
//
// CorrectEliminatedWinners overturns that. A coalition that was destroyed cannot
// act after its last unit dies, and cannot Build, Train or Upgrade once its
// production buildings are gone, whereas the coalition that beat it keeps doing
// both until the recording ends.
//
// Calibrated against 59 hand-labelled games: of the 45 whose label discriminates
// it agrees on 42 — every one of the 32 the credited team really won, and 10 of
// the 13 it lost. The three misses are short games where production barely moves;
// they leave the uncorrected answer in place rather than replacing it with a
// wrong one, which is the trade this rule is tuned for. A sweep of a 3,364-replay
// corpus changes 24 games, all but one of them labelled.
const (
	// The credited coalition must have fallen silent this many seconds before the
	// last surviving rival did. Two seconds is indistinguishable from an ordinary
	// gap between commands.
	EliminatedActionLeadSec = 3

	// ...and must have stopped producing this much earlier than that rival. Timing
	// alone is not enough: a winner mopping up often out-idles the loser who is
	// spamming units to survive.
	EliminatedProductionLeadSec = 15

	// Absolute floor on the credited coalition's production drought, so a game
	// where nobody produced late cannot trip the comparison on noise.
	EliminatedProductionMinSec = 20

	// Below this the endgame is too compressed for either signal to separate.
	EliminatedMinDurationSec = 120
)

// ProductionActionTypes: issuing one of these proves the player still owns a production building, which
// is what an eliminated player cannot fake.
var ProductionActionTypes = map[string]struct{}{
	"Build":          {},
	"Train":          {},
	"Train Fighter":  {},
	"Upgrade":        {},
	"Tech":           {},
	"Unit Morph":     {},
	"Building Morph": {},
	"Land":           {},
}

// provesPresence reports whether issuing this command required still being in
// the game with something to command. Chat and Leave Game do not: a player with
// nothing left on the map can still type, and can still quit.
func provesPresence(actionType string) bool {
	switch actionType {
	case "Chat", "Leave Game":
		return false
	}
	return true
}

// sideActivity is one side's best-case liveness: the latest moment any of its
// members acted, and the latest any of them produced.
type sideActivity struct {
	lastAction     int
	lastProduction int
	any            bool
}

func (s *sideActivity) observe(sec int, production bool) {
	s.any = true
	s.lastAction = max(s.lastAction, sec)
	if production {
		s.lastProduction = max(s.lastProduction, sec)
	}
}

// CorrectEliminatedWinners reassigns the win when the credited coalition was
// eliminated rather than victorious. It reports whether it changed anything.
//
// ar carries the end-of-game alliance topology and may be nil, in which case the
// static teams are used — the winner path makes the same choice.
func CorrectEliminatedWinners(players []*models.Player, commands []*models.Command, durationSec int, ar *AllianceResult, repSaverPID *byte) bool {
	if durationSec < EliminatedMinDurationSec {
		return false
	}

	credited := map[byte]bool{}
	for _, p := range players {
		if p == nil || p.IsObserver || p.TeamOutcome != models.OutcomeWon {
			continue
		}
		credited[p.PlayerID] = true
	}

	// With nobody credited the procedure found two or more surviving coalitions
	// and declined. One of them may nonetheless have been wiped out — a corpse
	// issues no Leave Game — which the same evidence can still prove.
	if len(credited) == 0 {
		return creditSurvivorOverEliminated(players, commands, durationSec, ar, repSaverPID)
	}

	// The recording stops when the saver exits, so a credited saver's silence
	// measures his own death or departure, not his coalition's: teammates can win
	// the game after he is gone. Never overturn on that evidence.
	if repSaverPID != nil && credited[*repSaverPID] {
		return false
	}

	// Computers never leave and never concede, so their activity says nothing
	// about whether a human coalition survived.
	measured := map[byte]bool{}
	for _, p := range players {
		if p == nil || p.IsObserver || p.Type == "Computer" {
			continue
		}
		measured[p.PlayerID] = true
	}

	var credSide, rivalSide sideActivity
	lastAction := map[byte]int{}
	for _, c := range commands {
		if c == nil || c.Player == nil || !provesPresence(c.ActionType) {
			continue
		}
		pid := c.Player.PlayerID
		if !measured[pid] {
			continue
		}
		sec := max(c.SecondsFromGameStart, 0)
		lastAction[pid] = max(lastAction[pid], sec)
		_, production := ProductionActionTypes[c.ActionType]
		if credited[pid] {
			credSide.observe(sec, production)
		} else {
			rivalSide.observe(sec, production)
		}
	}
	if !credSide.any || !rivalSide.any {
		return false
	}

	credSilence, rivalSilence := durationSec-credSide.lastAction, durationSec-rivalSide.lastAction
	credDrought, rivalDrought := durationSec-credSide.lastProduction, durationSec-rivalSide.lastProduction

	if credSilence-rivalSilence < EliminatedActionLeadSec ||
		credDrought-rivalDrought < EliminatedProductionLeadSec ||
		credDrought < EliminatedProductionMinSec {
		return false
	}

	// The survivors are whoever the last-acting rival was allied with at the end.
	coalitionOf := winnerCoalitionKeys(players, ar)
	var lastRival *models.Player
	for _, p := range players {
		if p == nil || p.IsObserver || credited[p.PlayerID] || !measured[p.PlayerID] {
			continue
		}
		if lastRival == nil || lastAction[p.PlayerID] > lastAction[lastRival.PlayerID] {
			lastRival = p
		}
	}
	if lastRival == nil {
		return false
	}
	winning, ok := coalitionOf[lastRival.PlayerID]
	if !ok {
		return false
	}

	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		p.TeamOutcome = teamOutcome(coalitionOf[p.PlayerID] == winning)
	}
	return true
}

// winnerCoalitionKeys groups players the same way winner attribution does: by
// end-of-game alliance clique when one was analysed, by static team otherwise.
func winnerCoalitionKeys(players []*models.Player, ar *AllianceResult) map[byte]byte {
	if ar != nil {
		var finalTeams [][]byte
		if n := len(ar.Snapshots); n > 0 {
			finalTeams = ar.Snapshots[n-1].Teams
		}
		return assignCoalitions(players, finalTeams)
	}
	teams := map[byte]byte{}
	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		teams[p.PlayerID] = p.Team
	}
	return teams
}

// creditSurvivorOverEliminated resolves an undecided game in which every
// surviving coalition but one was actually destroyed. The evidence has to come
// from the losers being provably dead, never from the winner looking alive: a
// coalition holding the replay saver always looks alive, because the recording
// stops the moment they stop playing.
func creditSurvivorOverEliminated(players []*models.Player, commands []*models.Command, durationSec int, ar *AllianceResult, repSaverPID *byte) bool {
	coalitionOf := winnerCoalitionKeys(players, ar)

	left := map[byte]bool{}
	for _, c := range commands {
		if c != nil && c.Player != nil && c.ActionType == "Leave Game" {
			left[c.Player.PlayerID] = true
		}
	}

	// Candidates are the coalitions the procedure could not separate: those
	// still holding someone who never left.
	candidates := map[byte]*sideActivity{}
	member := map[byte]bool{}
	for _, p := range players {
		if p == nil || p.IsObserver || p.Type == "Computer" || left[p.PlayerID] {
			continue
		}
		key, ok := coalitionOf[p.PlayerID]
		if !ok {
			continue
		}
		candidates[key] = &sideActivity{}
		member[p.PlayerID] = true
	}
	if len(candidates) < 2 {
		return false
	}

	for _, c := range commands {
		if c == nil || c.Player == nil || !provesPresence(c.ActionType) || !member[c.Player.PlayerID] {
			continue
		}
		side := candidates[coalitionOf[c.Player.PlayerID]]
		if side == nil {
			continue
		}
		_, production := ProductionActionTypes[c.ActionType]
		side.observe(max(c.SecondsFromGameStart, 0), production)
	}

	var liveliest byte
	best := -1
	for key, side := range candidates {
		if !side.any {
			return false
		}
		if side.lastAction > best {
			liveliest, best = key, side.lastAction
		}
	}
	winner := candidates[liveliest]

	for key, side := range candidates {
		if key == liveliest {
			continue
		}
		silence := (durationSec - side.lastAction) - (durationSec - winner.lastAction)
		drought := (durationSec - side.lastProduction) - (durationSec - winner.lastProduction)
		if silence < EliminatedActionLeadSec ||
			drought < EliminatedProductionLeadSec ||
			durationSec-side.lastProduction < EliminatedProductionMinSec {
			return false
		}
	}

	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		p.TeamOutcome = teamOutcome(coalitionOf[p.PlayerID] == liveliest)
	}
	return true
}

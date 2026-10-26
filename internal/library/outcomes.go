package library

import (
	"fmt"
	"slices"
	"strings"

	"github.com/marianogappa/screpdb/internal/models"
)

// A replay's results come from up to three sources, most trusted first: the engine, which watched the
// game itself; the command stream, which can only infer a winner from who left and who kept acting; and
// Battle.net, which knows how the game ended even when this recording stopped before it did. The command
// stream's answer is what the parser stored; the other two are layered over it here, so switching the
// engine off or forgetting Battle.net's answer gives back exactly what the parser said.

type EngineResultKind uint8

const (
	EngineLeft EngineResultKind = iota
	EngineDropped
	EngineDefeated
	EngineWon
)

// EngineResult is something the engine saw happen to one player.
type EngineResult struct {
	ReplayPlayerID uint8
	Kind           EngineResultKind
	Frame          uint32
}

// BnetResult is one player's result as Battle.net records it on their own profile.
type BnetResult struct {
	Key     string
	Outcome models.Outcome
}

// parsedOutcomes is what the parser said, kept while a layer overrides it.
type parsedOutcomes struct {
	players    []Player
	teamReason string
	bnetSource string
}

func (r *Replay) setBnetResults(results []BnetResult, source string) {
	r.bnetResults, r.bnetSource = results, source
	r.resolveOutcomes()
}

// BnetResults is the Battle.net answer layered over this replay, if any.
func (r *Replay) BnetResults() []BnetResult { return r.bnetResults }

// resolveOutcomes rebuilds Players' results from the parser's, then the engine's, then Battle.net's.
func (r *Replay) resolveOutcomes() {
	engine := r.EngineData != nil && r.EngineData.ResultsKnown
	if r.parsed == nil {
		if !engine && len(r.bnetResults) == 0 {
			return
		}
		r.parsed = &parsedOutcomes{players: r.Players, teamReason: r.TeamOutcomeReason, bnetSource: r.BnetOutcomeSource}
	}
	players := slices.Clone(r.parsed.players)
	teamReason, bnetSource := r.parsed.teamReason, r.parsed.bnetSource
	if engine {
		if why, ok := applyEngineResults(players, r.EngineData.Results, r.Flags, teamReason); ok {
			teamReason = why
		}
	}
	if why, ok := applyBnetResults(players, r.bnetResults, r.Flags); ok {
		teamReason, bnetSource = why, r.bnetSource
	}
	r.Players, r.TeamOutcomeReason, r.BnetOutcomeSource = players, teamReason, bnetSource
	if !engine && len(r.bnetResults) == 0 {
		r.parsed = nil
	}
}

// WinnerUndecided reports whether no side of a scored game is known to have won.
func (r *Replay) WinnerUndecided() bool {
	scored := false
	for i := range r.Players {
		p := &r.Players[i]
		if p.IsObserver() {
			continue
		}
		switch p.TeamOutcome() {
		case models.OutcomeNotScored:
			return false
		case models.OutcomeWon:
			return false
		}
		scored = true
	}
	return scored
}

func setOutcomes(p *Player, player, team models.Outcome, reason string) {
	p.Flags &^= PlayerResultWon | PlayerResultLost | PlayerResultDisconnected | TeamResultWon | TeamResultLost
	p.Flags |= OutcomeFlags(player, PlayerResultWon, PlayerResultLost) | OutcomeFlags(team, TeamResultWon, TeamResultLost)
	p.PlayerOutcomeReason = reason
}

func notScored(players []Player) bool {
	for i := range players {
		if players[i].Flags.Has(PlayerResultNotScored) {
			return true
		}
	}
	return false
}

func clock(frame uint32) string {
	sec := frame * 42 / 1000
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}

// applyEngineResults settles results from what the engine saw. StarCraft itself declares the winners, so
// when it did, nothing needs inferring: winners and their teammates won, everyone else lost. When the
// recording stopped first, players the engine saw leave, drop or be defeated still get that result, a
// side is out once all of them are, and the last side in wins.
func applyEngineResults(players []Player, results []EngineResult, flags Flags, parserReason string) (string, bool) {
	// A saver who lost the connection leaves a burst of phantom departures behind, which the engine
	// replays as real ones; the parser already recognised them and knows better.
	if notScored(players) || flags.Has(FlagSaverDisconnect) || sides(players) < 2 {
		return "", false
	}
	final := map[uint8]EngineResult{}
	for _, res := range results {
		if prev, seen := final[res.ReplayPlayerID]; !seen || res.Kind > prev.Kind {
			final[res.ReplayPlayerID] = res
		}
	}
	teamsKnown := !flags.Has(FlagTeamInfoIncomplete)
	winningTeams := map[uint8]bool{}
	var winners []string
	for i := range players {
		p := &players[i]
		if !p.IsObserver() && final[p.ReplayPlayerID].Kind == EngineWon {
			winningTeams[p.Team] = true
			winners = append(winners, p.Name)
		}
	}

	team := map[int]models.Outcome{}
	why := ""
	if len(winners) > 0 {
		for i := range players {
			p := &players[i]
			switch {
			case p.IsObserver():
			case final[p.ReplayPlayerID].Kind == EngineWon:
				team[i] = models.OutcomeWon
			case teamsKnown && winningTeams[p.Team]:
				team[i] = models.OutcomeWon
			case teamsKnown || hasResult(final, p):
				team[i] = models.OutcomeLost
			}
		}
		why = "the engine saw " + strings.Join(winners, ", ") + " win"
	} else {
		out := map[uint8]bool{}
		for i := range players {
			p := &players[i]
			if p.IsObserver() {
				continue
			}
			if _, ok := out[p.Team]; !ok {
				out[p.Team] = true
			}
			out[p.Team] = out[p.Team] && hasResult(final, p)
		}
		alive := 0
		for _, gone := range out {
			if !gone {
				alive++
			}
		}
		// The saver's own exit is never recorded, so only the parser can tell they conceded; its answer
		// stands for the sides still in, unless it credited a side the engine saw go.
		parserStands := true
		for i := range players {
			if p := &players[i]; !p.IsObserver() && out[p.Team] && p.TeamOutcome() == models.OutcomeWon {
				parserStands = false
			}
		}
		for i := range players {
			p := &players[i]
			switch {
			case p.IsObserver() || alive == 0:
			case teamsKnown && out[p.Team]:
				team[i] = models.OutcomeLost
			case teamsKnown && alive == 1:
				team[i] = models.OutcomeWon
			case parserStands:
				team[i] = p.TeamOutcome()
			}
		}
		switch {
		case alive == 0:
			why = "no winner: the engine saw every side leave"
		case teamsKnown && alive == 1:
			why = "the engine saw every other side leave or be defeated"
		case parserStands:
			why = parserReason
		default:
			why = fmt.Sprintf("no winner: the recording ends with %d sides still in the game", alive)
		}
	}

	for i := range players {
		p := &players[i]
		if p.IsObserver() {
			continue
		}
		res, ok := final[p.ReplayPlayerID]
		t, teamOK := team[i]
		if !teamOK {
			t = models.OutcomeUnknown
		}
		switch {
		case ok && res.Kind == EngineWon:
			setOutcomes(p, models.OutcomeWon, t, "won: the engine saw them win at "+clock(res.Frame))
		case ok && res.Kind == EngineDefeated:
			setOutcomes(p, models.OutcomeLost, t, "defeated at "+clock(res.Frame)+": the engine saw their last building fall")
		case ok && res.Kind == EngineDropped:
			setOutcomes(p, models.OutcomeDisconnected, t, "dropped at "+clock(res.Frame))
		case ok:
			setOutcomes(p, models.OutcomeLost, t, "conceded: left at "+clock(res.Frame)+" before the game was decided")
		case t.Known():
			setOutcomes(p, t, t, "their side's result, which they share")
		default:
			setOutcomes(p, p.PlayerOutcome(), t, p.PlayerOutcomeReason)
		}
	}
	return why, true
}

// sides counts the teams with someone playing; a game of one side has no result, though the engine
// declares it won at once.
func sides(players []Player) int {
	teams := map[uint8]bool{}
	for i := range players {
		if !players[i].IsObserver() {
			teams[players[i].Team] = true
		}
	}
	return len(teams)
}

func hasResult(final map[uint8]EngineResult, p *Player) bool {
	_, ok := final[p.ReplayPlayerID]
	return ok
}

// applyBnetResults settles a game nothing else could from the players' own Battle.net records. A recorded
// win names the winning side; in a 1v1, a recorded loss names it too.
func applyBnetResults(players []Player, results []BnetResult, flags Flags) (string, bool) {
	if len(results) == 0 || notScored(players) || !(&Replay{Players: players}).WinnerUndecided() {
		return "", false
	}
	byKey := map[string]models.Outcome{}
	for _, r := range results {
		byKey[r.Key] = r.Outcome
	}
	var active []int
	for i := range players {
		if !players[i].IsObserver() {
			active = append(active, i)
		}
	}
	winningTeams := map[uint8]bool{}
	var winners []string
	for _, i := range active {
		if byKey[players[i].Key] == models.OutcomeWon {
			winningTeams[players[i].Team] = true
			winners = append(winners, players[i].Name)
		}
	}
	if len(winners) == 0 && len(active) == 2 {
		for n, i := range active {
			if byKey[players[i].Key] == models.OutcomeLost {
				other := players[active[1-n]]
				winningTeams[other.Team] = true
				winners = append(winners, other.Name)
			}
		}
	}
	if len(winners) == 0 || (len(winningTeams) > 1 && len(active) == 2) {
		return "", false
	}
	teamsKnown := !flags.Has(FlagTeamInfoIncomplete)
	for _, i := range active {
		p := &players[i]
		team := models.OutcomeUnknown
		switch {
		case winningTeams[p.Team] && (teamsKnown || byKey[p.Key] == models.OutcomeWon):
			team = models.OutcomeWon
		case teamsKnown:
			team = models.OutcomeLost
		}
		switch own, recorded := byKey[p.Key]; {
		case recorded && own == models.OutcomeWon:
			setOutcomes(p, own, team, "Battle.net records a win on their own profile")
		case recorded && own == models.OutcomeLost:
			setOutcomes(p, own, team, "Battle.net records a loss on their own profile")
		case recorded && own == models.OutcomeDisconnected:
			setOutcomes(p, own, team, "Battle.net records a disconnect on their own profile")
		case p.PlayerOutcome().Known() || p.PlayerOutcome() == models.OutcomeDisconnected:
			setOutcomes(p, p.PlayerOutcome(), team, p.PlayerOutcomeReason)
		default:
			setOutcomes(p, team, team, "their side's result, which they share")
		}
	}
	return "Battle.net names " + strings.Join(winners, ", ") + " as winning", true
}

package parser

import (
	"sort"

	"github.com/marianogappa/screpdb/internal/models"
)

// StackingThresholdSec is the minimum contiguous duration of an "uneven non-solo
// team sizes" topology that flips a melee game's team_stacking flag and earns
// the 😈 marker. Five minutes is long enough to filter transient mid-game
// re-alliances (someone's teammate dies and they re-ally) but short enough to
// catch deliberate ganging-up.
const StackingThresholdSec = 300

// AllianceSnapshot is one observed team topology, valid from Sec until the
// next snapshot's Sec (or game end for the last one).
type AllianceSnapshot struct {
	Sec      int      `json:"sec"`
	Teams    [][]byte `json:"teams"` // each entry is a sorted []player_id; teams ordered by min pid
	Stacking bool     `json:"stacking"`
}

// AllianceResult is what the parser/replay pipeline and the dashboard endpoint
// both consume.
type AllianceResult struct {
	Snapshots         []AllianceSnapshot
	ResolvedTeams     map[byte]byte // player_id → 1-indexed team_id, derived from the longest-held topology
	AnyMutualResolved bool          // at least one mutual alliance pair was observed
	TeamStackingFlag  bool          // any single contiguous stacking band lasted > StackingThresholdSec
}

// AnalyzeAlliances replays the alliance command stream chronologically,
// tracking each player's allies set and emitting a topology snapshot every
// time the mutual-alliance graph changes.
//
// Caller is expected to have filtered to a melee game with >2 active players;
// this function is happy to run on any input, but the output is only
// meaningful for that gate.
//
// The "active player set" is everyone who is not Observer and not type
// "Computer" — mirroring screp's own computeMeleeTeams filter.
func AnalyzeAlliances(players []*models.Player, commands []*models.Command, durationSec int) AllianceResult {
	// Build slot/pid → player lookups for the active set.
	slotToPlayer := map[byte]*models.Player{}
	pidToPlayer := map[byte]*models.Player{}
	activePIDs := []byte{}
	for _, p := range players {
		if p == nil || p.IsObserver || p.Type == "Computer" {
			continue
		}
		slotToPlayer[byte(p.SlotID)] = p
		pidToPlayer[p.PlayerID] = p
		activePIDs = append(activePIDs, p.PlayerID)
	}
	sort.Slice(activePIDs, func(i, j int) bool { return activePIDs[i] < activePIDs[j] })

	if len(activePIDs) == 0 {
		return AllianceResult{ResolvedTeams: map[byte]byte{}}
	}

	// allies[pid] = set of pids the issuer currently considers allied. Initial
	// state: each player is allied with self only (mirrors screp).
	allies := make(map[byte]map[byte]bool, len(activePIDs))
	for _, pid := range activePIDs {
		allies[pid] = map[byte]bool{pid: true}
	}

	// Initial snapshot at sec=0 (everyone solo).
	snapshots := []AllianceSnapshot{computeSnapshot(0, allies, activePIDs)}
	anyMutual := snapshots[0].hasMutual()

	for _, cmd := range commands {
		if cmd == nil || cmd.ActionType != "Alliance" {
			continue
		}
		issuer := cmd.Player
		if issuer == nil || issuer.IsObserver || issuer.Type == "Computer" {
			continue
		}
		issuerPID := issuer.PlayerID

		// Map slot-IDs from the command to active player_ids. Observers and
		// computers are filtered (the computer slot ID can appear on certain
		// random-team maps; mirrors screp's filterOutObserverSlotIDs).
		newSet := map[byte]bool{issuerPID: true}
		if cmd.AlliancePlayerIDs != nil {
			for _, slotID := range *cmd.AlliancePlayerIDs {
				p, ok := slotToPlayer[byte(slotID)]
				if !ok {
					continue
				}
				newSet[p.PlayerID] = true
			}
		}

		if setsEqual(allies[issuerPID], newSet) {
			continue
		}
		allies[issuerPID] = newSet

		snap := computeSnapshot(cmd.SecondsFromGameStart, allies, activePIDs)
		// Dedupe identical topology (e.g., one-way change without altering
		// any mutual edge).
		if teamsEqual(snap.Teams, snapshots[len(snapshots)-1].Teams) {
			continue
		}
		snapshots = append(snapshots, snap)
		if snap.hasMutual() {
			anyMutual = true
		}
	}

	// Stacking-band scan: contiguous runs where Stacking == true.
	stackingFlag := false
	for i, snap := range snapshots {
		if !snap.Stacking {
			continue
		}
		end := durationSec
		// Advance end to the first non-stacking snapshot or game end.
		j := i + 1
		for j < len(snapshots) && snapshots[j].Stacking {
			j++
		}
		if j < len(snapshots) {
			end = snapshots[j].Sec
		}
		if end-snap.Sec > StackingThresholdSec {
			stackingFlag = true
			break
		}
	}

	// ResolvedTeams: pick the snapshot with the longest held duration. That's
	// the "dominant" partition — robust to brief re-alliances and the simplest
	// honest answer.
	resolved := dominantResolvedTeams(snapshots, durationSec, activePIDs)

	return AllianceResult{
		Snapshots:         snapshots,
		ResolvedTeams:     resolved,
		AnyMutualResolved: anyMutual,
		TeamStackingFlag:  stackingFlag,
	}
}

func (s AllianceSnapshot) hasMutual() bool {
	for _, t := range s.Teams {
		if len(t) >= 2 {
			return true
		}
	}
	return false
}

func computeSnapshot(sec int, allies map[byte]map[byte]bool, activePIDs []byte) AllianceSnapshot {
	teams := mutualAllianceTeams(allies, activePIDs)
	return AllianceSnapshot{
		Sec:      sec,
		Teams:    teams,
		Stacking: isStacking(teams),
	}
}

// mutualAllianceTeams computes connected components of the mutual-alliance
// graph (edge a↔b iff a∈allies[b] AND b∈allies[a], excluding self-loops).
// Returns teams sorted by min(pid) within team, and teams ordered by their
// min(pid) for stable rendering across snapshots.
func mutualAllianceTeams(allies map[byte]map[byte]bool, activePIDs []byte) [][]byte {
	parent := map[byte]byte{}
	for _, pid := range activePIDs {
		parent[pid] = pid
	}
	var find func(byte) byte
	find = func(x byte) byte {
		if parent[x] == x {
			return x
		}
		parent[x] = find(parent[x])
		return parent[x]
	}
	union := func(a, b byte) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		// Union by smaller-pid-as-root for determinism.
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}

	for a, aSet := range allies {
		for b := range aSet {
			if a == b {
				continue
			}
			if _, ok := parent[a]; !ok {
				continue
			}
			if _, ok := parent[b]; !ok {
				continue
			}
			if bSet, ok := allies[b]; ok && bSet[a] {
				union(a, b)
			}
		}
	}

	groups := map[byte][]byte{}
	for _, pid := range activePIDs {
		root := find(pid)
		groups[root] = append(groups[root], pid)
	}

	teams := make([][]byte, 0, len(groups))
	for _, g := range groups {
		sort.Slice(g, func(i, j int) bool { return g[i] < g[j] })
		teams = append(teams, g)
	}
	sort.Slice(teams, func(i, j int) bool { return teams[i][0] < teams[j][0] })
	return teams
}

// isStacking: among teams of size ≥2, sizes must all match. A 2v2v1 is fine
// (one solo is valid); a 3v2 is not. Need at least 2 non-solo teams to compare.
func isStacking(teams [][]byte) bool {
	var sizes []int
	for _, t := range teams {
		if len(t) >= 2 {
			sizes = append(sizes, len(t))
		}
	}
	if len(sizes) < 2 {
		return false
	}
	first := sizes[0]
	for _, s := range sizes[1:] {
		if s != first {
			return true
		}
	}
	return false
}

// dominantResolvedTeams returns player_id → 1-indexed team_id from the snapshot
// with the longest held duration. If no mutual alliance exists, every player
// gets their own team_id (caller can detect this via AnyMutualResolved=false).
func dominantResolvedTeams(snapshots []AllianceSnapshot, durationSec int, activePIDs []byte) map[byte]byte {
	if len(snapshots) == 0 {
		out := map[byte]byte{}
		for i, pid := range activePIDs {
			out[pid] = byte(i + 1)
		}
		return out
	}

	bestIdx := 0
	bestDur := 0
	for i, s := range snapshots {
		end := durationSec
		if i+1 < len(snapshots) {
			end = snapshots[i+1].Sec
		}
		dur := end - s.Sec
		// Tie-break: prefer snapshots with mutual alliance over all-solo.
		if dur > bestDur || (dur == bestDur && s.hasMutual() && !snapshots[bestIdx].hasMutual()) {
			bestDur = dur
			bestIdx = i
		}
	}

	out := map[byte]byte{}
	for i, team := range snapshots[bestIdx].Teams {
		teamID := byte(i + 1)
		for _, pid := range team {
			out[pid] = teamID
		}
	}
	return out
}

// DeriveWinnersFromLeaves applies the "largest remaining team wins" algorithm
// (mirrors screp's `computeWinners` at replay.go:701-805) to the post-fallback
// team assignments. Mutates the players slice: sets IsWinner=true on every
// member of the detected winning team (and clears it on every other player —
// this function is the authoritative winner setter for the fallback path).
//
// repSaverPID is optional. screp doesn't record a Leave Game command for the
// replay saver, so when known we append a virtual leave for them as the last
// leaver — that's what lets the "all non-obs players left" tie-break fire on
// games where one team simply quit and the saver from the other team is the
// final non-leaver.
//
// Bails out (no winners assigned) when the algorithm can't determine a single
// winner. That's the same behavior screp has when WinnerTeam == 0.
func DeriveWinnersFromLeaves(players []*models.Player, commands []*models.Command, repSaverPID *byte) {
	for _, p := range players {
		if p == nil {
			continue
		}
		p.IsWinner = false
	}

	teamSizes := map[byte]int{}      // non-observer non-computer counts
	teamCompsCount := map[byte]int{} // non-observer computer counts
	nonObsCount := 0
	pidToPlayer := map[byte]*models.Player{}
	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		if p.Type == "Computer" {
			teamCompsCount[p.Team]++
		} else {
			teamSizes[p.Team]++
		}
		nonObsCount++
		pidToPlayer[p.PlayerID] = p
	}

	// If a team consists only of computers, screp bails — we can't detect
	// winners from leave commands because computers never leave.
	for team := range teamCompsCount {
		if teamSizes[team] == 0 {
			return
		}
	}

	// Walk leave commands in chronological order (the parser already feeds
	// them that way). Filter to non-observers. Track player_ids for the
	// last-leaver tie-break.
	leaverPIDs := make([]byte, 0)
	for _, cmd := range commands {
		if cmd == nil || cmd.ActionType != "Leave Game" {
			continue
		}
		if cmd.Player == nil || cmd.Player.IsObserver {
			continue
		}
		leaverPIDs = append(leaverPIDs, cmd.Player.PlayerID)
	}
	if repSaverPID != nil {
		if saver, ok := pidToPlayer[*repSaverPID]; ok && !saver.IsObserver {
			leaverPIDs = append(leaverPIDs, *repSaverPID)
		}
	}

	for _, pid := range leaverPIDs {
		if p, ok := pidToPlayer[pid]; ok {
			teamSizes[p.Team]--
		}
	}

	if len(teamSizes) < 2 || len(leaverPIDs) == 0 {
		return
	}

	// Largest remaining team wins (when uniquely largest).
	var maxTeam byte
	maxSize := -1
	for team, size := range teamSizes {
		if size > maxSize {
			maxTeam, maxSize = team, size
		}
	}
	if maxSize > 0 {
		count := 0
		for _, size := range teamSizes {
			if size == maxSize {
				count++
			}
		}
		if count == 1 {
			markWinnersOnTeam(players, maxTeam)
			return
		}
	}

	// Tie / zero-max fallback: when every non-obs player has left, the last
	// leaver's team wins. Frequently triggers when an observer saved the
	// replay and was the actual final non-leaver.
	if len(leaverPIDs) == nonObsCount {
		lastPID := leaverPIDs[len(leaverPIDs)-1]
		if last, ok := pidToPlayer[lastPID]; ok {
			markWinnersOnTeam(players, last.Team)
		}
	}
}

func markWinnersOnTeam(players []*models.Player, team byte) {
	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		if p.Team == team {
			p.IsWinner = true
		}
	}
}

func setsEqual(a, b map[byte]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func teamsEqual(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

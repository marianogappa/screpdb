package parser

import (
	"sort"

	"github.com/marianogappa/screpdb/internal/models"
)

// Minimum contiguous duration of an "uneven non-solo team sizes" topology that
// flips a melee game's team_stacking flag. Five minutes filters transient
// mid-game re-alliances (a teammate dies and they re-ally) while still catching
// deliberate ganging-up.
const StackingThresholdSec = 300

// The shorter floor for a stacking band that never dissolves. A stack intact
// until the game is over is a decisive gang-up, not the transient re-alliance
// the 5-minute threshold exists to filter, so it earns the flag sooner.
const StackingEndOfGameThresholdSec = 120

// The "effectively dead" threshold that filters ghost players out of the
// stacking check. A player whose recent action rate falls below this and never
// recovers is treated as gone for the rest of the game (monotonic).
const (
	InactivityWindowSec   = 60
	InactivityMinActions  = 20
	InactivityEndGraceSec = 60 // skip emitting stop events if T is in the last minute of the game
)

// Alliance transitions after this point are surfaced as game events for
// storyline value.
const LateAllianceThresholdSec = 600

// AllianceSnapshot is valid from Sec until the next snapshot's Sec (or game end).
type AllianceSnapshot struct {
	Sec      int      `json:"sec"`
	Teams    [][]byte `json:"teams"` // each entry is a sorted []player_id; teams ordered by min pid
	Stacking bool     `json:"stacking"`
}

// Activity captures per-player presence. Both maps are monotonic: once a player
// is "gone" at second T they are treated as gone for every second ≥ T.
type Activity struct {
	StoppedSecByPID map[byte]int // pid → sec the player became permanently inactive (only set when applicable)
	LeaveSecByPID   map[byte]int // pid → sec of the player's first Leave Game command
}

type AllianceResult struct {
	Snapshots         []AllianceSnapshot
	ResolvedTeams     map[byte]byte // player_id → 1-indexed team_id, derived from the longest-held topology
	AnyMutualResolved bool          // at least one mutual alliance pair was observed
	TeamStackingFlag  bool          // any single contiguous stacking band lasted > StackingThresholdSec

	// Inputs surfaced for event emission. The analyzer owns these because it
	// already iterates the command stream and the activity maps in lockstep.
	StoppedSecByPID         map[byte]int       // copy of activity.StoppedSecByPID for emitters
	LateAllianceTransitions []AllianceSnapshot // topology-changing snapshots after LateAllianceThresholdSec
	StackingBandStartSec    int                // start sec of the qualifying band (zero when TeamStackingFlag is false)
	StackingBandTeams       [][]byte           // alliance topology at the start of the qualifying band (for the event description)
}

// Used at ingest time; the dashboard reconstructs the same maps from stored
// replay_events instead of rescanning commands.
func ComputeActivity(players []*models.Player, commands []*models.Command, durationSec int) Activity {
	activePIDs := map[byte]bool{}
	for _, p := range players {
		if p == nil || p.IsObserver || p.Type == "Computer" {
			continue
		}
		activePIDs[p.PlayerID] = true
	}

	leaveSec := map[byte]int{}
	timesByPID := map[byte][]int{}

	for _, cmd := range commands {
		if cmd == nil || cmd.Player == nil {
			continue
		}
		pid := cmd.Player.PlayerID
		if !activePIDs[pid] {
			continue
		}
		sec := cmd.SecondsFromGameStart
		if sec < 0 {
			sec = 0
		}
		timesByPID[pid] = append(timesByPID[pid], sec)
		if cmd.ActionType == "Leave Game" {
			if _, exists := leaveSec[pid]; !exists {
				leaveSec[pid] = sec
			}
		}
	}

	stoppedSec := map[byte]int{}
	for pid := range activePIDs {
		times := timesByPID[pid]
		sort.Ints(times)
		stop, ok := computeStoppedSec(times, durationSec)
		if !ok {
			continue
		}
		// A Leave Game already covers the "gone" semantics, so don't double-emit a
		// stop event — but still record the earlier of the two so stacking sees them
		// as gone from the earliest moment.
		if leaveAt, hasLeave := leaveSec[pid]; hasLeave && leaveAt <= stop {
			continue
		}
		stoppedSec[pid] = stop
	}

	return Activity{
		StoppedSecByPID: stoppedSec,
		LeaveSecByPID:   leaveSec,
	}
}

// computeStoppedSec finds the latest second at which the player's recent
// 60-second action count hit InactivityMinActions; after that they never
// recovered, so they count as gone for stacking.
//
// Returns (0, true) when the player never reached the threshold at all (AFK
// from the start), and (_, false) when they were active until the very end —
// including within InactivityEndGraceSec of it, which is not a stop.
func computeStoppedSec(actionTimes []int, durationSec int) (int, bool) {
	if len(actionTimes) == 0 {
		return 0, true
	}
	// Timestamps are sorted, so the window count is an index difference.
	lastAlive := -1
	for i := range actionTimes {
		t := actionTimes[i]
		lo := sort.SearchInts(actionTimes, t-InactivityWindowSec)
		count := i - lo + 1
		if count >= InactivityMinActions {
			lastAlive = t
		}
	}
	if lastAlive < 0 {
		// Never hit the threshold, so stopped from second 0.
		return 0, true
	}
	if durationSec-lastAlive < InactivityEndGraceSec {
		return 0, false
	}
	return lastAlive, true
}

// AnalyzeAlliances replays the alliance command stream chronologically and
// emits a topology snapshot whenever the mutual-alliance graph changes.
//
// Stacking is computed from the EFFECTIVE topology: players who left or went
// permanently inactive are dropped from the team-size comparison. Callers are
// expected to have filtered to a melee game with >2 active players. The active
// set is everyone not Observer and not "Computer", mirroring screp's own
// computeMeleeTeams filter.
func AnalyzeAlliances(players []*models.Player, commands []*models.Command, durationSec int, activity Activity) AllianceResult {
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
		return AllianceResult{
			ResolvedTeams:   map[byte]byte{},
			StoppedSecByPID: copyByteIntMap(activity.StoppedSecByPID),
		}
	}

	// Each player starts allied with self only, mirroring screp.
	allies := make(map[byte]map[byte]bool, len(activePIDs))
	for _, pid := range activePIDs {
		allies[pid] = map[byte]bool{pid: true}
	}

	// Initial snapshot at sec=0: everyone solo.
	snapshots := []AllianceSnapshot{computeSnapshot(0, allies, activePIDs, activity)}
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

		// Observers and computers are filtered out: the computer slot ID can appear on
		// certain random-team maps (mirrors screp's filterOutObserverSlotIDs).
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

		snap := computeSnapshot(cmd.SecondsFromGameStart, allies, activePIDs, activity)
		// Dedupe identical topology, e.g. a one-way change altering no mutual edge.
		if teamsEqual(snap.Teams, snapshots[len(snapshots)-1].Teams) {
			continue
		}
		snapshots = append(snapshots, snap)
		if snap.hasMutual() {
			anyMutual = true
		}
	}

	// Splice in virtual snapshots at activity transitions: the topology doesn't
	// change there, but the effective view does, so Stacking can flip with no
	// alliance command. Inserted only when Stacking actually changes.
	snapshots = injectActivitySnapshots(snapshots, activity)

	// Scan contiguous runs where Stacking == true, computed against the effective
	// view inside computeSnapshot.
	stackingFlag := false
	bandStartSec := 0
	var bandTeams [][]byte
	for i, snap := range snapshots {
		if !snap.Stacking {
			continue
		}
		end := durationSec
		j := i + 1
		for j < len(snapshots) && snapshots[j].Stacking {
			j++
		}
		threshold := StackingThresholdSec
		if j < len(snapshots) {
			end = snapshots[j].Sec
		} else {
			// The band runs to game end — never dissolved — so apply the shorter floor.
			threshold = StackingEndOfGameThresholdSec
		}
		if end-snap.Sec > threshold {
			stackingFlag = true
			bandStartSec = snap.Sec
			bandTeams = cloneTeams(snap.Teams)
			break
		}
	}

	// Snapshots are already deduped by topology, so each late one is a real change.
	lateTransitions := make([]AllianceSnapshot, 0)
	for _, snap := range snapshots {
		if snap.Sec > LateAllianceThresholdSec {
			lateTransitions = append(lateTransitions, AllianceSnapshot{
				Sec:      snap.Sec,
				Teams:    cloneTeams(snap.Teams),
				Stacking: snap.Stacking,
			})
		}
	}

	resolved := dominantResolvedTeams(snapshots, durationSec, activePIDs)

	return AllianceResult{
		Snapshots:               snapshots,
		ResolvedTeams:           resolved,
		AnyMutualResolved:       anyMutual,
		TeamStackingFlag:        stackingFlag,
		StoppedSecByPID:         copyByteIntMap(activity.StoppedSecByPID),
		LateAllianceTransitions: lateTransitions,
		StackingBandStartSec:    bandStartSec,
		StackingBandTeams:       bandTeams,
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

func computeSnapshot(sec int, allies map[byte]map[byte]bool, activePIDs []byte, activity Activity) AllianceSnapshot {
	teams := mutualAllianceTeams(allies, activePIDs)
	effective := effectiveTeamsAt(teams, sec, activity)
	return AllianceSnapshot{
		Sec:      sec,
		Teams:    teams,
		Stacking: isStacking(effective),
	}
}

// injectActivitySnapshots inserts a virtual snapshot at each activity
// transition where the effective stacking flips. Topology is copied from the
// prior snapshot; only Stacking is recomputed at the new second.
func injectActivitySnapshots(snapshots []AllianceSnapshot, activity Activity) []AllianceSnapshot {
	if len(snapshots) == 0 {
		return snapshots
	}
	timeSet := map[int]bool{}
	for _, sec := range activity.LeaveSecByPID {
		if sec > 0 {
			timeSet[sec] = true
		}
	}
	for _, sec := range activity.StoppedSecByPID {
		if sec > 0 {
			timeSet[sec] = true
		}
	}
	if len(timeSet) == 0 {
		return snapshots
	}
	times := make([]int, 0, len(timeSet))
	for t := range timeSet {
		times = append(times, t)
	}
	sort.Ints(times)

	out := make([]AllianceSnapshot, 0, len(snapshots)+len(times))
	si := 0
	for _, t := range times {
		for si < len(snapshots) && snapshots[si].Sec <= t {
			out = append(out, snapshots[si])
			si++
		}
		if len(out) == 0 {
			continue
		}
		last := out[len(out)-1]
		// A real snapshot at sec=t already used the activity at sec=t.
		if last.Sec == t {
			continue
		}
		effective := effectiveTeamsAt(last.Teams, t, activity)
		newStacking := isStacking(effective)
		if newStacking == last.Stacking {
			continue
		}
		out = append(out, AllianceSnapshot{
			Sec:      t,
			Teams:    cloneTeams(last.Teams),
			Stacking: newStacking,
		})
	}
	for si < len(snapshots) {
		out = append(out, snapshots[si])
		si++
	}
	return out
}

// effectiveTeamsAt drops players who have left or stopped by sec, and any
// team that empties. The original slice is not mutated.
//
// Cliques can overlap, so plain departure filtering would emit duplicate
// singletons — every clique a departed player's lone partner survived in
// shrinks to {p}. So when a player is still in a surviving size-≥2 clique, any
// singleton entry for them is dropped, and only one shrunk singleton survives.
func effectiveTeamsAt(teams [][]byte, sec int, activity Activity) [][]byte {
	departed := func(pid byte) bool {
		if leaveAt, ok := activity.LeaveSecByPID[pid]; ok && leaveAt <= sec {
			return true
		}
		if stoppedAt, ok := activity.StoppedSecByPID[pid]; ok && stoppedAt <= sec {
			return true
		}
		return false
	}
	survived := make([][]byte, 0, len(teams))
	for _, team := range teams {
		filtered := make([]byte, 0, len(team))
		for _, pid := range team {
			if departed(pid) {
				continue
			}
			filtered = append(filtered, pid)
		}
		if len(filtered) > 0 {
			survived = append(survived, filtered)
		}
	}
	inLarger := map[byte]bool{}
	for _, team := range survived {
		if len(team) >= 2 {
			for _, pid := range team {
				inLarger[pid] = true
			}
		}
	}
	out := make([][]byte, 0, len(survived))
	seenSolo := map[byte]bool{}
	for _, team := range survived {
		if len(team) == 1 {
			pid := team[0]
			if inLarger[pid] || seenSolo[pid] {
				continue
			}
			seenSolo[pid] = true
		}
		out = append(out, team)
	}
	return out
}

// mutualAllianceTeams enumerates the maximal cliques of the mutual-alliance
// graph (edge a↔b iff each is in the other's allies, self-loops excluded).
//
// A Brood War "team" is a clique, not a connected component: a chain A↔B↔C↔D
// is NOT a 4-stack but three overlapping pair-stacks, and the component model
// made the detector report it as "4v2 stacked". A player legitimately appearing
// in more than one clique is the honest answer for a non-transitive graph.
//
// Singletons are appended as solo teams so every player has an entry to render
// against. Cliques sort by size (larger first) then min(pid) for stable output.
func mutualAllianceTeams(allies map[byte]map[byte]bool, activePIDs []byte) [][]byte {
	sortedPIDs := append([]byte(nil), activePIDs...)
	sort.Slice(sortedPIDs, func(i, j int) bool { return sortedPIDs[i] < sortedPIDs[j] })

	isActive := map[byte]bool{}
	for _, pid := range sortedPIDs {
		isActive[pid] = true
	}

	mutual := func(a, b byte) bool {
		if a == b {
			return false
		}
		aSet, ok := allies[a]
		if !ok || !aSet[b] {
			return false
		}
		bSet, ok := allies[b]
		if !ok || !bSet[a] {
			return false
		}
		return true
	}

	n := len(sortedPIDs)
	// Brute-force subset enumeration is fine for melee (n≤8 → ≤256 subsets). Past
	// 16 we'd want Bron-Kerbosch; guard so a broken assumption can't OOM.
	if n > 16 {
		// Fall back to non-overlapping component grouping so the analyzer still
		// produces *some* answer rather than spinning over 65k subsets.
		return mutualAllianceTeamsComponents(allies, sortedPIDs, mutual)
	}

	type subset struct {
		members []byte
		mask    uint32
	}
	cliques := make([]subset, 0)
	for mask := uint32(1); mask < (uint32(1) << uint(n)); mask++ {
		members := make([]byte, 0, n)
		for i := 0; i < n; i++ {
			if mask&(uint32(1)<<uint(i)) != 0 {
				members = append(members, sortedPIDs[i])
			}
		}
		if len(members) < 2 {
			continue
		}
		isClique := true
		for i := 0; i < len(members) && isClique; i++ {
			for j := i + 1; j < len(members); j++ {
				if !mutual(members[i], members[j]) {
					isClique = false
					break
				}
			}
		}
		if !isClique {
			continue
		}
		cliques = append(cliques, subset{members: members, mask: mask})
	}

	maximal := make([]subset, 0, len(cliques))
	for i, c := range cliques {
		dominated := false
		for j, d := range cliques {
			if i == j {
				continue
			}
			if c.mask != d.mask && (c.mask&d.mask) == c.mask {
				dominated = true
				break
			}
		}
		if !dominated {
			maximal = append(maximal, c)
		}
	}

	out := make([][]byte, 0, len(maximal)+n)
	inAClique := map[byte]bool{}
	for _, c := range maximal {
		team := append([]byte(nil), c.members...)
		out = append(out, team)
		for _, pid := range team {
			inAClique[pid] = true
		}
	}
	for _, pid := range sortedPIDs {
		if !inAClique[pid] {
			out = append(out, []byte{pid})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i][0] < out[j][0]
	})
	return out
}

// mutualAllianceTeamsComponents is the fallback past the clique-enumeration
// ceiling (n>16). Melee never hits this path; it exists so the analyzer stays
// safe if a future replay format grows the slot count.
func mutualAllianceTeamsComponents(allies map[byte]map[byte]bool, sortedPIDs []byte, mutual func(a, b byte) bool) [][]byte {
	parent := map[byte]byte{}
	for _, pid := range sortedPIDs {
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
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}
	for i, a := range sortedPIDs {
		for _, b := range sortedPIDs[i+1:] {
			if mutual(a, b) {
				union(a, b)
			}
		}
	}
	groups := map[byte][]byte{}
	for _, pid := range sortedPIDs {
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

// isStacking: among teams of size ≥2, sizes must all match — a 2v2v1 is fine
// (one solo is valid), a 3v2 is not. Needs 2 non-solo teams to compare.
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

// Uses the snapshot with the longest held duration. With no mutual alliance
// every player gets their own team_id (detectable via AnyMutualResolved=false).
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

// winningTeamByLeaves applies screp's "largest remaining team wins" rule (see
// computeWinners in rep/replay.go) over an arbitrary team grouping. Returns
// (0, false) in the same situations where screp leaves WinnerTeam == 0.
//
// screp records no Leave Game for the replay saver, so when repSaverPID is
// known a virtual leave is appended for them as the last leaver. That is what
// lets the "all non-obs players left" tie-break fire on games where one team
// quit and the saver from the other team is the final non-leaver.
func winningTeamByLeaves(players []*models.Player, commands []*models.Command, teamOf map[byte]byte, repSaverPID *byte) (byte, bool) {
	teamSizes := map[byte]int{}
	teamCompsCount := map[byte]int{}
	nonObsCount := 0
	pidToPlayer := map[byte]*models.Player{}
	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		team := teamOf[p.PlayerID]
		if p.Type == "Computer" {
			teamCompsCount[team]++
		} else {
			teamSizes[team]++
		}
		nonObsCount++
		pidToPlayer[p.PlayerID] = p
	}

	for team := range teamCompsCount {
		if teamSizes[team] == 0 {
			return 0, false
		}
	}

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
		if _, ok := pidToPlayer[pid]; ok {
			teamSizes[teamOf[pid]]--
		}
	}

	if len(teamSizes) < 2 || len(leaverPIDs) == 0 {
		return 0, false
	}

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
			return maxTeam, true
		}
	}

	if len(leaverPIDs) == nonObsCount {
		lastPID := leaverPIDs[len(leaverPIDs)-1]
		if _, ok := pidToPlayer[lastPID]; ok {
			return teamOf[lastPID], true
		}
	}
	return 0, false
}

// DeriveWinnersFromLeaves applies the rule to the static p.Team assignments,
// mutating the slice. It assigns no winners when a single winner can't be
// determined — the same behaviour screp has when WinnerTeam == 0.
func DeriveWinnersFromLeaves(players []*models.Player, commands []*models.Command, repSaverPID *byte) {
	for _, p := range players {
		if p == nil {
			continue
		}
		p.IsWinner = false
	}

	teamOf := map[byte]byte{}
	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		teamOf[p.PlayerID] = p.Team
	}

	if team, ok := winningTeamByLeaves(players, commands, teamOf, repSaverPID); ok {
		markWinnersByTeam(players, teamOf, team)
	}
}

// DeriveWinnersFromFinalTopology credits the winning coalition using the
// END-OF-GAME alliance topology rather than the longest-held display teams.
// That credits coalitions formed mid-game, and — crucially — stable teams whose
// alliance screp missed, since computeMeleeTeams only inspects the first ~90
// seconds and otherwise makes everyone a singleton, which then ties. Credited
// winners can therefore span two display teams; the Alliances tab explains why.
//
// Non-destructive: IsWinner is only cleared and re-set when a single coalition
// is determined (allied-victory semantics — a teammate who left still won), so
// it never erases a winner it cannot reproduce.
func DeriveWinnersFromFinalTopology(players []*models.Player, commands []*models.Command, ar AllianceResult, repSaverPID *byte) {
	var finalTeams [][]byte
	if n := len(ar.Snapshots); n > 0 {
		finalTeams = ar.Snapshots[n-1].Teams
	}
	coalitionOf := assignCoalitions(players, finalTeams)

	team, ok := winningTeamByLeaves(players, commands, coalitionOf, repSaverPID)
	if !ok {
		return
	}
	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		p.IsWinner = coalitionOf[p.PlayerID] == team
	}
}

// assignCoalitions keys each non-observer player to their LARGEST mutual
// clique's min pid — cliques arrive pre-sorted size-desc then min-pid, so
// largest-first processing lands each player in theirs. Players in no clique,
// and computers (never in the alliance graph), get a singleton keyed by pid.
func assignCoalitions(players []*models.Player, finalTeams [][]byte) map[byte]byte {
	coalitionOf := map[byte]byte{}
	for _, team := range finalTeams {
		if len(team) == 0 {
			continue
		}
		key := team[0]
		for _, pid := range team {
			if pid < key {
				key = pid
			}
		}
		for _, pid := range team {
			if _, done := coalitionOf[pid]; !done {
				coalitionOf[pid] = key
			}
		}
	}
	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		if _, done := coalitionOf[p.PlayerID]; !done {
			coalitionOf[p.PlayerID] = p.PlayerID
		}
	}
	return coalitionOf
}

func markWinnersByTeam(players []*models.Player, teamOf map[byte]byte, team byte) {
	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		if teamOf[p.PlayerID] == team {
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

func cloneTeams(in [][]byte) [][]byte {
	if in == nil {
		return nil
	}
	out := make([][]byte, len(in))
	for i, t := range in {
		out[i] = append([]byte(nil), t...)
	}
	return out
}

func copyByteIntMap(in map[byte]int) map[byte]int {
	if in == nil {
		return nil
	}
	out := make(map[byte]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

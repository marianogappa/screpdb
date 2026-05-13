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

// InactivityWindowSec / InactivityMinActions define the "effectively dead"
// threshold used to filter ghost players out of the stacking topology check.
// A player whose recent action rate falls below 20 commands/min and never
// recovers is treated as gone for the rest of the game (monotonic).
const (
	InactivityWindowSec   = 60
	InactivityMinActions  = 20
	InactivityEndGraceSec = 60 // skip emitting stop events if T is in the last minute of the game
)

// LateAllianceThresholdSec defines what counts as a "late" alliance —
// alliance-topology-changing transitions after this point are surfaced as
// game events for storyline value.
const LateAllianceThresholdSec = 600

// AllianceSnapshot is one observed team topology, valid from Sec until the
// next snapshot's Sec (or game end for the last one).
type AllianceSnapshot struct {
	Sec      int      `json:"sec"`
	Teams    [][]byte `json:"teams"` // each entry is a sorted []player_id; teams ordered by min pid
	Stacking bool     `json:"stacking"`
}

// Activity captures per-player presence info: when each player left (Leave
// Game command) and when each player became permanently inactive (their
// last 60-second action rate that hit ≥20). Both are monotonic — once a
// player is "gone" at sec T, they're treated as gone for all sec ≥ T.
type Activity struct {
	StoppedSecByPID map[byte]int // pid → sec the player became permanently inactive (only set when applicable)
	LeaveSecByPID   map[byte]int // pid → sec of the player's first Leave Game command
}

// AllianceResult is what the parser/replay pipeline and the dashboard endpoint
// both consume.
type AllianceResult struct {
	Snapshots         []AllianceSnapshot
	ResolvedTeams     map[byte]byte // player_id → 1-indexed team_id, derived from the longest-held topology
	AnyMutualResolved bool          // at least one mutual alliance pair was observed
	TeamStackingFlag  bool          // any single contiguous stacking band lasted > StackingThresholdSec

	// Inputs surfaced for event emission. The analyzer is the natural owner
	// of these — it already iterates the command stream and the activity
	// maps in lockstep.
	StoppedSecByPID         map[byte]int       // copy of activity.StoppedSecByPID for emitters
	LateAllianceTransitions []AllianceSnapshot // topology-changing snapshots after LateAllianceThresholdSec
	StackingBandStartSec    int                // start sec of the qualifying band (zero when TeamStackingFlag is false)
	StackingBandTeams       [][]byte           // alliance topology at the start of the qualifying band (for the event description)
}

// ComputeActivity builds the Activity maps from the full command stream.
// Used at ingest time; the dashboard reconstructs the same maps from
// stored replay_events instead of rescanning commands.
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
		// If the player also has a Leave Game, the leave event already
		// covers the "gone" semantics — don't double-emit a stop event,
		// but still record the earlier of the two so stacking sees them
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
// 60-second action count hit InactivityMinActions. After that second they
// never recovered, so they can be treated as gone for stacking purposes.
//
// Returns (stoppedSec, true) when:
//   - The player has at least one "alive" moment in their command history
//     (a window with ≥InactivityMinActions actions), AND
//   - The last alive moment is at least InactivityEndGraceSec before game end
//     (otherwise they were active right up to the end — no stop event).
//
// Returns (0, true) when the player never reached the threshold at all
// (e.g. AFK from the start) — they are inactive for the whole game.
//
// Returns (_, false) when the player was active until the very end.
func computeStoppedSec(actionTimes []int, durationSec int) (int, bool) {
	if len(actionTimes) == 0 {
		return 0, true
	}
	// Walk forward; for each action time t, count actions in [t-W, t]
	// using the index difference (timestamps are sorted).
	lastAlive := -1
	for i := range actionTimes {
		t := actionTimes[i]
		// Find first index j where actionTimes[j] >= t - InactivityWindowSec.
		lo := sort.SearchInts(actionTimes, t-InactivityWindowSec)
		count := i - lo + 1
		if count >= InactivityMinActions {
			lastAlive = t
		}
	}
	if lastAlive < 0 {
		// Never hit the threshold — count as stopped from sec 0.
		return 0, true
	}
	if durationSec-lastAlive < InactivityEndGraceSec {
		return 0, false
	}
	return lastAlive, true
}

// AnalyzeAlliances replays the alliance command stream chronologically,
// tracking each player's allies set and emitting a topology snapshot every
// time the mutual-alliance graph changes.
//
// The Stacking boolean on each snapshot is computed from the **effective**
// view of the topology — players who have left or gone permanently inactive
// (per `activity`) are dropped from team-size comparison. Caller is expected
// to have filtered to a melee game with >2 active players.
//
// The "active player set" is everyone who is not Observer and not type
// "Computer" — mirroring screp's own computeMeleeTeams filter.
func AnalyzeAlliances(players []*models.Player, commands []*models.Command, durationSec int, activity Activity) AllianceResult {
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
		return AllianceResult{
			ResolvedTeams:   map[byte]byte{},
			StoppedSecByPID: copyByteIntMap(activity.StoppedSecByPID),
		}
	}

	// allies[pid] = set of pids the issuer currently considers allied. Initial
	// state: each player is allied with self only (mirrors screp).
	allies := make(map[byte]map[byte]bool, len(activePIDs))
	for _, pid := range activePIDs {
		allies[pid] = map[byte]bool{pid: true}
	}

	// Initial snapshot at sec=0 (everyone solo).
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

		snap := computeSnapshot(cmd.SecondsFromGameStart, allies, activePIDs, activity)
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

	// Splice in virtual snapshots at activity-transition times. The alliance
	// topology (Teams) doesn't change at these moments, but the effective
	// view does — so Stacking may flip even when no alliance command fired.
	// Virtual snapshots are only inserted when they actually change Stacking
	// from the prior snapshot's flag.
	snapshots = injectActivitySnapshots(snapshots, activity)

	// Stacking-band scan: contiguous runs where Stacking == true. The flag
	// is computed against the effective view inside computeSnapshot.
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
		if j < len(snapshots) {
			end = snapshots[j].Sec
		}
		if end-snap.Sec > StackingThresholdSec {
			stackingFlag = true
			bandStartSec = snap.Sec
			bandTeams = cloneTeams(snap.Teams)
			break
		}
	}

	// Late-alliance transitions: snapshots with Sec > LateAllianceThresholdSec.
	// Snapshots are already deduped by topology, so each is a real change.
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

// injectActivitySnapshots inserts virtual snapshots at every activity-
// transition time (Leave Game / stopped playing) where the effective
// stacking flips relative to the most recent snapshot. The alliance
// topology at the inserted snapshot is copied from the prior snapshot —
// only Stacking is recomputed against the effective view at the new sec.
func injectActivitySnapshots(snapshots []AllianceSnapshot, activity Activity) []AllianceSnapshot {
	if len(snapshots) == 0 {
		return snapshots
	}
	// Collect unique transition times > 0.
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
		// Drain alliance snapshots whose Sec <= t.
		for si < len(snapshots) && snapshots[si].Sec <= t {
			out = append(out, snapshots[si])
			si++
		}
		if len(out) == 0 {
			continue
		}
		last := out[len(out)-1]
		// If a real alliance snapshot already lives at sec=t, no need for a
		// virtual one — the real snapshot already used the activity at sec=t.
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

// effectiveTeamsAt returns the teams with players who have left or stopped
// playing by sec dropped out. Empty teams are dropped from the slice. The
// original teams slice is not mutated.
//
// Cliques can overlap (a player may belong to several maximal cliques), so a
// pass-through of departure filtering would emit duplicate singletons —
// every clique a departed player's lone partner survived in shrinks to {p}.
// We dedupe: when a player is still part of a surviving size-≥2 clique, we
// drop any singleton entry for them. When they're a singleton in multiple
// shrunk cliques, only one survives.
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
// graph (edge a↔b iff a∈allies[b] AND b∈allies[a], excluding self-loops).
//
// A "team" in Brood War semantics is a clique: every member must mutually
// ally every other member. Connected components — the previous model — over-
// reports teams: a chain A↔B↔C↔D (with no other edges) is NOT a 4-stack, it
// is three overlapping pair-stacks {A,B}, {B,C}, {C,D}. Without this fix the
// stacking detector flags a chain of pair-alliances as e.g. "4v2 stacked"
// even though none of the four chain members all-mutually ally each other.
// A player can legitimately appear in more than one returned clique — that
// is the honest answer when the alliance graph is not transitive.
//
// Singletons (active players not in any mutual edge) are appended as solo
// teams so every player has at least one entry to render against. Cliques
// are sorted by size (larger first) then by min(pid) for stable output.
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
	// Brute-force subset enumeration is fine for melee (n≤8 → ≤256 subsets).
	// If n grows past 16 we'd want Bron-Kerbosch; guard so we don't OOM if
	// the assumption ever breaks.
	if n > 16 {
		// Fall back to a non-overlapping component grouping — same shape as
		// the old algorithm — so the analyzer still produces *some* answer
		// rather than spinning over 65k subsets.
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

	// Keep only maximal cliques.
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

// mutualAllianceTeamsComponents is the connected-component fallback used
// only when player count exceeds the brute-force clique enumeration ceiling
// (n>16). Melee never hits this path; included so the analyzer stays safe
// if future replay formats grow the slot count.
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

	teamSizes := map[byte]int{}
	teamCompsCount := map[byte]int{}
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

	for team := range teamCompsCount {
		if teamSizes[team] == 0 {
			return
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
		if p, ok := pidToPlayer[pid]; ok {
			teamSizes[p.Team]--
		}
	}

	if len(teamSizes) < 2 || len(leaverPIDs) == 0 {
		return
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
			markWinnersOnTeam(players, maxTeam)
			return
		}
	}

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

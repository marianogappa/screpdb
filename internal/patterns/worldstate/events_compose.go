package worldstate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/utils"
)

// recallClusterGapSec collapses consecutive recalls from the same source base
// by the same player into one event. The cluster keeps first-second as its
// timestamp and carries last-second + count in the payload. Clustering cuts
// noise in sustained-attack replays and widens the range attack-coincidence
// inference can match against.
const recallClusterGapSec = 20

// How far the cluster's range is extended when looking for a coincident
// attack/drop/nuke candidate.
const (
	recallAttackPreSec  = 15
	recallAttackPostSec = 30
)

// The "post-recall activity" target proxy: once units arrive at the Arbiter
// they move / right-click / attack from there, and those commands carry X/Y near
// the destination polygon. Counting per-poly hits after the cluster and taking
// the dominant polygon catches aggressive recalls that never reach the pressure
// threshold — a brief skirmish, or a player relying on auto-attack.
const (
	recallActivityWindowPreSec  = 5
	recallActivityWindowPostSec = 60
	recallActivityMinCommands   = 3
)

// recallTargetPayload is persisted in ReplayEvent.Payload. Keys are short to
// keep on-disk cost low: recall is the only event type with target-side
// metadata, and one game can produce dozens of casts.
type recallTargetPayload struct {
	N  int                  `json:"n,omitempty"`  // cluster size, omit when 1
	LE int                  `json:"le,omitempty"` // last second, omit when equal to Second
	S  []int                `json:"s,omitempty"`  // source point [x, y]
	T  []int                `json:"t,omitempty"`  // target point [x, y], omit when target unknown
	TB *recallTargetBaseRef `json:"tb,omitempty"` // target base, omit when target unknown
	TP byte                 `json:"tp,omitempty"` // target owner replay_player_id, omit when no owner
	TV string               `json:"tv,omitempty"` // target via: "a" attack-coincidence (units harvested), "p" post-recall activity, "t" unit-tag-backtrack
}

type recallTargetBaseRef struct {
	K  string `json:"k,omitempty"`  // base.kind ("starting"|"natural"|"expansion"|"")
	O  int    `json:"o,omitempty"`  // base.clock
	NO *int   `json:"no,omitempty"` // natural_of_oclock when applicable
	MO *bool  `json:"mo,omitempty"` // mineral_only when true
}

// recallCluster collapses a run of recalls by one player from one source base.
// sourceBaseIdx == -1 (source outside any base) makes the cluster per-cast,
// size 1, since there is no stable cluster key.
type recallCluster struct {
	pid           byte
	sourceBaseIdx int
	firstSec      int
	lastSec       int
	sourceX       int
	sourceY       int
	count         int
}

// indexOwnershipByPoly returns polyID → timeline sorted by frame.
func indexOwnershipByPoly(ownership []PolyOwnership) map[int][]OwnEvent {
	out := make(map[int][]OwnEvent, len(ownership))
	for _, t := range ownership {
		out[t.PolyID] = t.Events
	}
	return out
}

// Cheap linear walk per polygon: timelines are small, typically ≤ 5 events.
func (e *Engine) syncOwnersAtSec(timelineByPoly map[int][]OwnEvent, sec int) {
	for pi := range e.ownerByBase {
		evs := timelineByPoly[pi]
		owner := neutralPID
		for _, ev := range evs {
			if ev.Sec > sec {
				break
			}
			owner = ev.Owner
		}
		e.ownerByBase[pi] = owner
	}
}

// runRushPass walks the buffered stream, syncing ownership at each frame, and
// invokes the rush/proxy/race-change/zergling-rush helpers per command.
func (e *Engine) runRushPass(ownership []PolyOwnership) {
	timelineByPoly := indexOwnershipByPoly(ownership)

	for i, ec := range e.stream {
		cmd := e.streamCommands[i]
		e.syncOwnersAtSec(timelineByPoly, ec.Second)

		pid, ok := e.playerIDFromCommand(cmd)
		if !ok {
			continue
		}
		if leaveSec, left := e.leaveSec[pid]; left && ec.Second > leaveSec {
			continue
		}

		sec := ec.Second
		e.recordRecentAttackUnit(pid, sec, cmd)
		e.recordRecentCast(pid, sec, ec)
		e.recordMarineTraining(pid, sec, cmd)
		e.processRaceSwitchEvent(cmd, pid, sec)
		e.processZerglingRushEvent(cmd, pid, sec)

		// Build/Land coords are tile-space; the rush helpers expect pixels.
		if isBuildLike(cmd.ActionType) && cmd.X != nil && cmd.Y != nil {
			x := tileToPixel(float64(*cmd.X))
			y := tileToPixel(float64(*cmd.Y))
			biEvent := pointToEventBase(x, y, e.bases)
			e.tryEmitRushBuildEvents(cmd, pid, sec, x, y)
			e.tryEmitProxyBuildEvents(cmd, pid, sec, x, y, biEvent)
			e.tryEmitMannerPylonEvent(cmd, pid, sec, x, y)
		}

		// Uses the legacy actionType-based pressure classifier so the existing
		// rush-detection threshold stays unchanged.
		if cmd.X != nil && cmd.Y != nil {
			orderName := ""
			if cmd.OrderName != nil {
				orderName = *cmd.OrderName
			}
			if enemyBasePressureForZergRush(cmd.ActionType, cmd.OrderID, orderName) {
				x := float64(*cmd.X)
				y := float64(*cmd.Y)
				biEvent := pointToEventBase(x, y, e.bases)
				if biEvent >= 0 {
					e.recordZergRushAttack(pid, sec, biEvent)
				}
			}
		}

		e.finalizeZergRushCandidates(sec, false)
	}
}

// emitOwnershipTransitions emits expansion / takeover / location_inactive in
// chronological order.
func (e *Engine) emitOwnershipTransitions(ownership []PolyOwnership) {
	for _, t := range ownership {
		baseIdx := t.PolyID
		if baseIdx < 0 || baseIdx >= len(e.bases) {
			continue
		}
		var prevOwner byte = neutralPID
		first := true
		for _, ev := range t.Events {
			sec := ev.Sec
			if e.replay != nil && sec > e.replay.DurationSeconds {
				sec = e.replay.DurationSeconds
			}
			switch ev.Reason {
			case "init":
				prevOwner = ev.Owner
				first = false
				continue
			case "start":
				prevOwner = ev.Owner
				first = false
				continue
			case "claim":
				prevOwner = ev.Owner
				first = false
				continue
			case "expansion":
				if ev.Owner != neutralPID {
					where := e.decorateBaseDescriptionForPlayer(ev.Owner, baseIdx, e.bases[baseIdx].DisplayName)
					e.emitEvent("expansion", sec,
						fmt.Sprintf("%s expands to %s", e.playerName(ev.Owner), where),
						e.playerRef(ev.Owner), nil, baseIdx, nil)
				}
			case "takeover":
				var target *NarrativePlayerRef
				if !first && prevOwner != neutralPID {
					target = e.playerRef(prevOwner)
				}
				if ev.Owner != neutralPID {
					where := e.decorateBaseDescriptionForPlayer(ev.Owner, baseIdx, e.bases[baseIdx].DisplayName)
					e.emitEvent("takeover", sec,
						fmt.Sprintf("%s takes over %s", e.playerName(ev.Owner), where),
						e.playerRef(ev.Owner), target, baseIdx, nil)
				}
			case "timeout":
				if !first && prevOwner != neutralPID {
					e.emitEvent("location_inactive", sec,
						fmt.Sprintf("%s loses %s", e.playerName(prevOwner), e.bases[baseIdx].DisplayName),
						e.playerRef(prevOwner), nil, baseIdx, nil)
				}
			}
			prevOwner = ev.Owner
			first = false
		}
	}
}

// emitRecallEvents emits one "recall" event per cluster of Arbiter Recall
// casts. Each cluster runs through inferRecallTarget against the attack/drop/
// nuke candidates to estimate the Arbiter's position, because the recall's
// destination is the Arbiter's location and the cast command doesn't carry it.
//
// The "Made recalls" marker still surfaces only the first cast per player; this
// is the cluster-level counterpart for the game-events list.
func (e *Engine) emitRecallEvents(ownership []PolyOwnership, candidates []CandidateAttack) {
	timelineByPoly := make(map[int][]OwnEvent, len(ownership))
	for _, t := range ownership {
		timelineByPoly[t.PolyID] = t.Events
	}
	ownerAtSec := func(polyID int, sec int) byte {
		evs := timelineByPoly[polyID]
		owner := neutralPID
		for _, ev := range evs {
			if ev.Sec > sec {
				break
			}
			owner = ev.Owner
		}
		return owner
	}

	clusters := e.buildRecallClusters()
	for _, cl := range clusters {
		// Attack-coincidence first: it gives location AND units to harvest.
		targetIdx, attackCand, foundByAttack := e.inferRecallTargetByAttack(cl, candidates, ownerAtSec)
		// Post-recall activity proxy gives location only.
		var foundByActivity bool
		if !foundByAttack {
			if t, ok := e.inferRecallTargetByPostActivity(cl); ok {
				targetIdx = t
				foundByActivity = true
			}
		}

		desc, payload, attackUnits, target := e.composeRecallEvent(cl, ownerAtSec, targetIdx, attackCand, foundByAttack, foundByActivity)
		e.emitRecallEvent(cl, desc, target, attackUnits, payload)
	}
}

// Mirrors the out-of-polygon → nearest-base fallback BuildAttacks uses, so
// commands fired just past a polygon edge aren't lost.
func (e *Engine) inferRecallTargetByPostActivity(cl recallCluster) (int, bool) {
	if len(e.bases) == 0 {
		return -1, false
	}
	lo := cl.lastSec - recallActivityWindowPreSec
	hi := cl.lastSec + recallActivityWindowPostSec
	counts := map[int]int{}
	for _, ec := range e.stream {
		if ec.Second < lo {
			continue
		}
		if ec.Second > hi {
			break // stream is in chronological order
		}
		if byte(ec.PlayerID) != cl.pid {
			continue
		}
		if ec.X == nil || ec.Y == nil {
			continue
		}
		// Recall casts are counted by their click (source) position, which we already
		// know.
		if ec.Kind == cmdenrich.KindCast && ec.Subject == "Recall" {
			continue
		}
		// Production / research commands carry their producing building's inferred
		// location (issue #175), but a recall destination is where the recalled army
		// went — macro at a Barracks must not sway the activity clustering.
		switch ec.Kind {
		case cmdenrich.KindMakeUnit, cmdenrich.KindTech, cmdenrich.KindUpgrade:
			continue
		}
		x, y := *ec.X, *ec.Y
		bi := pointToEventBase(float64(x), float64(y), e.bases)
		if bi < 0 {
			continue
		}
		if bi == cl.sourceBaseIdx {
			continue
		}
		counts[bi]++
	}
	bestIdx := -1
	bestCount := 0
	for idx, c := range counts {
		if c < recallActivityMinCommands {
			continue
		}
		if c > bestCount {
			bestCount = c
			bestIdx = idx
		}
	}
	if bestIdx == -1 {
		return -1, false
	}
	return bestIdx, true
}

// Casts whose source falls outside any base form per-cast clusters of size 1:
// there is no stable cluster key for them.
func (e *Engine) buildRecallClusters() []recallCluster {
	type clusterKey struct {
		pid           byte
		sourceBaseIdx int
	}
	openByKey := map[clusterKey]int{} // key → index into out
	out := []recallCluster{}
	for _, ec := range e.stream {
		if ec.Kind != cmdenrich.KindCast || ec.Subject != "Recall" {
			continue
		}
		pid := byte(ec.PlayerID)
		if leaveSec, left := e.leaveSec[pid]; left && ec.Second > leaveSec {
			continue
		}
		sourceBaseIdx := -1
		x, y := 0, 0
		if ec.X != nil && ec.Y != nil {
			x, y = *ec.X, *ec.Y
			sourceBaseIdx = pointToEventBase(float64(x), float64(y), e.bases)
		}
		if sourceBaseIdx < 0 {
			out = append(out, recallCluster{
				pid:           pid,
				sourceBaseIdx: -1,
				firstSec:      ec.Second,
				lastSec:       ec.Second,
				sourceX:       x,
				sourceY:       y,
				count:         1,
			})
			continue
		}
		key := clusterKey{pid: pid, sourceBaseIdx: sourceBaseIdx}
		if idx, ok := openByKey[key]; ok && ec.Second-out[idx].lastSec <= recallClusterGapSec {
			out[idx].lastSec = ec.Second
			out[idx].count++
			continue
		}
		openByKey[key] = len(out)
		out = append(out, recallCluster{
			pid:           pid,
			sourceBaseIdx: sourceBaseIdx,
			firstSec:      ec.Second,
			lastSec:       ec.Second,
			sourceX:       x,
			sourceY:       y,
			count:         1,
		})
	}
	return out
}

// inferRecallTargetByAttack picks the candidate that best explains the
// cluster's destination, or (-1, _, false) when none qualifies. Filter:
//
//   - same attacker; type ∈ {attack, drop, nuke} (scout is too weak)
//   - defender != neutral, so the target is a real base
//   - excludes candidates targeting the recall's own source base: a recall to
//     its own source is meaningless, likely a mid-map fight at the same place,
//     and unknown is safer than mis-labelled
//   - "attack": any overlap of [firstSec-15, lastSec+30] with [OpenSec, CloseSec]
//   - "drop"/"nuke": Second within [firstSec-15, lastSec+30]
//
// Ties break on the smallest |Second - clusterMid|.
func (e *Engine) inferRecallTargetByAttack(cl recallCluster, candidates []CandidateAttack, _ func(int, int) byte) (int, CandidateAttack, bool) {
	if len(candidates) == 0 {
		return -1, CandidateAttack{}, false
	}
	lo := cl.firstSec - recallAttackPreSec
	hi := cl.lastSec + recallAttackPostSec
	mid := (cl.firstSec + cl.lastSec) / 2
	bestIdx := -1
	bestDist := -1
	for i := range candidates {
		c := candidates[i]
		if c.Attacker != cl.pid {
			continue
		}
		if c.Defender == neutralPID {
			continue
		}
		if c.Type != "attack" && c.Type != "drop" && c.Type != "nuke" {
			continue
		}
		if cl.sourceBaseIdx >= 0 && c.PolyID == cl.sourceBaseIdx {
			continue
		}
		switch c.Type {
		case "attack":
			cLo, cHi := c.OpenSec, c.CloseSec
			if cHi < cLo {
				cHi = cLo
			}
			if cHi < lo || cLo > hi {
				continue
			}
		default: // drop, nuke
			if c.Second < lo || c.Second > hi {
				continue
			}
		}
		d := c.Second - mid
		if d < 0 {
			d = -d
		}
		if bestIdx == -1 || d < bestDist {
			bestIdx = i
			bestDist = d
		}
	}
	if bestIdx == -1 {
		return -1, CandidateAttack{}, false
	}
	return candidates[bestIdx].PolyID, candidates[bestIdx], true
}

// composeRecallEvent builds the description, payload, attack-units list and
// target ref, covering the four cases of (target known/unknown × count 1/N).
func (e *Engine) composeRecallEvent(cl recallCluster, ownerAtSec func(int, int) byte, targetBaseIdx int, attackCand CandidateAttack, foundByAttack bool, foundByActivity bool) (string, *string, []string, *NarrativePlayerRef) {
	actorName := e.playerName(cl.pid)
	sourceLabel := ""
	if cl.sourceBaseIdx >= 0 && cl.sourceBaseIdx < len(e.bases) {
		sourceLabel = e.bases[cl.sourceBaseIdx].DisplayName
	}
	countSuffix := ""
	if cl.count > 1 {
		countSuffix = fmt.Sprintf(" (×%d)", cl.count)
	}

	pl := recallTargetPayload{
		S: []int{cl.sourceX, cl.sourceY},
	}
	if cl.count > 1 {
		pl.N = cl.count
	}
	if cl.lastSec != cl.firstSec {
		pl.LE = cl.lastSec
	}

	var target *NarrativePlayerRef
	attackUnits := []string{"Arbiter"}

	if targetBaseIdx >= 0 && targetBaseIdx < len(e.bases) {
		tBase := e.bases[targetBaseIdx]
		targetLabel := tBase.DisplayName
		owner := ownerAtSec(targetBaseIdx, cl.firstSec)
		ownerLabel := ""
		if owner != neutralPID && owner != cl.pid && !e.sameTeam(cl.pid, owner) {
			target = e.playerRef(owner)
			ownerLabel = e.playerName(owner)
		}
		// Apply natural-of-X decoration so the description carries the same context
		// attack and scout events do.
		decoratedTarget := e.decorateBaseDescriptionForPlayer(cl.pid, targetBaseIdx, targetLabel)

		baseType, baseOclock, naturalOf, mineralOnly := e.locationForBase(targetBaseIdx)
		tb := recallTargetBaseRef{}
		if baseType != nil {
			tb.K = *baseType
		}
		if baseOclock != nil {
			tb.O = *baseOclock
		}
		if naturalOf != nil {
			no := *naturalOf
			tb.NO = &no
		}
		if mineralOnly != nil && *mineralOnly {
			mo := true
			tb.MO = &mo
		}
		pl.TB = &tb
		pl.T = []int{int(tBase.CenterX), int(tBase.CenterY)}
		if target != nil {
			pl.TP = byte(target.PlayerID)
		}
		switch {
		case foundByAttack:
			pl.TV = "a" // attack-coincidence (source for harvested units)
		case foundByActivity:
			pl.TV = "p" // post-recall activity heuristic
		default:
			pl.TV = "t" // unit-tag-backtrack (Phase 2)
		}

		var sourceClause string
		if sourceLabel != "" {
			sourceClause = " from " + sourceLabel
		}
		var targetClause string
		if ownerLabel != "" {
			targetClause = fmt.Sprintf(" to %s %s", ownerLabel, decoratedTarget)
		} else {
			targetClause = " to " + decoratedTarget
		}
		desc := fmt.Sprintf("%s recalls%s%s%s", actorName, countSuffix, sourceClause, targetClause)

		// Harvest the player's army composition so the overlay can render the recalled
		// units at the source. With a matched candidate, pull casts + builds from its
		// pressure window (richer — includes spell evidence). Otherwise fall back to the
		// epicenter window at the cluster's first cast, which gives the army the player
		// has been training in the run-up: close enough for "what got recalled".
		var harvested []string
		switch {
		case foundByAttack && attackCand.Type == "attack":
			harvested = e.attackUnitsCombined(attackCand)
		case foundByAttack: // drop, nuke
			harvested = e.buildUnitsInEpicenterWindow(attackCand.Attacker, attackCand.Second)
		default:
			harvested = e.buildUnitsInEpicenterWindow(cl.pid, cl.firstSec)
		}
		if len(harvested) > 0 {
			seen := map[string]struct{}{"Arbiter": {}}
			for _, u := range harvested {
				if _, ok := seen[u]; ok {
					continue
				}
				seen[u] = struct{}{}
				attackUnits = append(attackUnits, u)
			}
		}

		raw, _ := json.Marshal(pl)
		s := string(raw)
		return desc, &s, attackUnits, target
	}

	var sourceClause string
	if sourceLabel != "" {
		sourceClause = " from " + sourceLabel
	}
	desc := fmt.Sprintf("%s recalls%s%s (destination unknown)", actorName, countSuffix, sourceClause)
	raw, _ := json.Marshal(pl)
	s := string(raw)
	return desc, &s, attackUnits, nil
}

// emitRecallEvent bypasses emitEvent because it must carry a Payload string,
// and because the entry layer's same-second/same-description dedup would
// collapse identical adjacent recalls — clustering is explicit here, so every
// emit is a distinct cluster and must be kept.
func (e *Engine) emitRecallEvent(cl recallCluster, description string, target *NarrativePlayerRef, attackUnits []string, payload *string) {
	if description == "" {
		return
	}
	actor := e.playerRef(cl.pid)
	base := e.baseRef(cl.sourceBaseIdx)
	e.entries = append(e.entries, NarrativeEntry{
		Type:        "recall",
		Second:      cl.firstSec,
		Description: description,
		Actor:       actor,
		Target:      target,
		Base:        base,
		ActorOrigin: e.actorOrigin(actor, base),
		Ownership:   e.ownershipSnapshot(),
	})
	rev := e.toReplayEvent("recall", cl.firstSec, actor, target, cl.sourceBaseIdx, attackUnits)
	rev.Payload = payload
	e.replayEvents = append(e.replayEvents, rev)
}

// emitLeaveGameEvents emits leave_game for a deliberate leave and
// player_dropped for a connection drop. On a saver disconnect (issue #358) the
// end-of-replay cluster is an artifact of the SAVER's connection dying — those
// players never left — so it condenses into one mass_disconnect attributed to
// the saver rather than a phantom leave each.
func (e *Engine) emitLeaveGameEvents() {
	pids := make([]byte, 0, len(e.leaveSec))
	for pid := range e.leaveSec {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool { return e.leaveSec[pids[i]] < e.leaveSec[pids[j]] })
	for _, pid := range pids {
		dropped := e.leaveReason[pid] == "Dropped"
		if e.massDisconnect != nil && dropped && e.leaveSec[pid] == e.massDisconnect.clusterSec {
			continue
		}
		if dropped {
			e.emitEvent("player_dropped", e.leaveSec[pid],
				fmt.Sprintf("%s drops from the game (connection lost)", e.playerName(pid)),
				e.playerRef(pid), nil, -1, nil)
			continue
		}
		e.emitEvent("leave_game", e.leaveSec[pid],
			fmt.Sprintf("%s leaves the game", e.playerName(pid)),
			e.playerRef(pid), nil, -1, nil)
	}
	if e.massDisconnect != nil {
		e.emitEvent("mass_disconnect", e.massDisconnect.clusterSec,
			fmt.Sprintf("%s lost connection; the game ended without a result", e.playerName(e.massDisconnect.saverPID)),
			e.playerRef(e.massDisconnect.saverPID), nil, -1, nil)
	}
}

// Drop subtype routing (cliff_drop only) happens here, using the source
// command's UnitTypes payload.
func (e *Engine) emitAttackCandidates(candidates []CandidateAttack) {
	// Indexed by frame for unit-types lookup at drop time.
	cmdByFrame := make(map[int32]*models.Command, len(e.streamCommands))
	for i, ec := range e.stream {
		cmdByFrame[ec.Frame] = e.streamCommands[i]
	}

	// BuildAttacks already returns stream order; stable-sort defensively.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Second < candidates[j].Second
	})

	// Per attacker, sorted (Second, SubjectName) of every aggressive cast, for the
	// "attack involves a cast new for this attacker" check.
	spellsByAttacker := buildSpellHistoryByAttacker(e.stream)

	attackedAlready := map[byte]bool{}
	knownUnitsByAttacker := map[byte]map[string]bool{}
	knownSpellsByAttacker := map[byte]map[string]bool{}
	scoutEmitted := map[byte]bool{}

	// Mid-map attack fallback: hold the earliest neutral-defender candidate so it
	// can be promoted post-loop when the replay would otherwise have zero attack
	// events. Picks up 1v1 mid-map fights where pressure opens on an unowned
	// polygon and every candidate is rejected as defender=neutral.
	var earliestNeutralAttack *CandidateAttack

	for _, c := range candidates {
		if e.sameTeam(c.Attacker, c.Defender) {
			continue
		}
		switch c.Type {
		case "scout":
			if scoutEmitted[c.Attacker] {
				continue
			}
			scoutEmitted[c.Attacker] = true
			e.emitScoutCandidate(c, cmdByFrame[c.Frame])
		case "nuke":
			e.emitNukeCandidate(c, cmdByFrame[c.Frame])
		case "drop":
			// Drops are emitted by the dedicated pass (emitDropEvents in drops_events.go).
			// They flow through this loop only so inferRecallTargetByAttack can match them.
			continue
		case "attack":
			if e.use1v1Attacks {
				continue // 1v1 attacks are emitted by emit1v1Attacks
			}
			if c.Defender == neutralPID {
				if earliestNeutralAttack == nil || c.Second < earliestNeutralAttack.Second {
					cc := c
					earliestNeutralAttack = &cc
				}
				continue
			}
			e.emitAttackIfImportant(c, cmdByFrame[c.Frame], spellsByAttacker,
				attackedAlready, knownUnitsByAttacker, knownSpellsByAttacker)
		}
	}

	if len(attackedAlready) == 0 && earliestNeutralAttack != nil {
		if opp, ok := e.singleOpponentHuman(earliestNeutralAttack.Attacker); ok {
			c := *earliestNeutralAttack
			c.Defender = opp
			e.emitAttackIfImportant(c, cmdByFrame[c.Frame], spellsByAttacker,
				attackedAlready, knownUnitsByAttacker, knownSpellsByAttacker)
		}
	}
}

// Used by the mid-map attack fallback to attribute a defender for a
// neutral-polygon candidate.
func (e *Engine) singleOpponentHuman(attacker byte) (byte, bool) {
	var opp byte = neutralPID
	count := 0
	for _, pid := range e.humanPlayerIDs {
		if pid == attacker {
			continue
		}
		if e.sameTeam(pid, attacker) {
			continue
		}
		opp = pid
		count++
	}
	if count == 1 {
		return opp, true
	}
	return neutralPID, false
}

func (e *Engine) emitScoutCandidate(c CandidateAttack, cmd *models.Command) {
	if c.Defender == neutralPID {
		return
	}
	scoutUnits := scoutUnitsForCandidate(e, c, cmd)
	e.emitEvent("scout", c.Second,
		fmt.Sprintf("%s scouts %s %s",
			e.playerName(c.Attacker), e.playerName(c.Defender),
			e.bases[c.PolyID].DisplayName),
		e.playerRef(c.Attacker), e.playerRef(c.Defender), c.PolyID, scoutUnits)
}

func (e *Engine) emitNukeCandidate(c CandidateAttack, cmd *models.Command) {
	if c.Defender == neutralPID {
		return
	}
	e.emitEvent("nuke", c.Second,
		fmt.Sprintf("%s nukes %s %s",
			e.playerName(c.Attacker), e.playerName(c.Defender),
			e.bases[c.PolyID].DisplayName),
		e.playerRef(c.Attacker), e.playerRef(c.Defender), c.PolyID,
		unitTypesFromCommand(cmd))
}

// isCliffDrop mirrors the marker-side gate in markers/cliff_drop.go: Terran on
// a Big Game Hunters map, Siege Tank produced by drop time, dropping into the
// top-left or bottom-right corner.
func (e *Engine) isCliffDrop(c CandidateAttack, firstTankSecByPlayer, firstDropshipSecByPlayer map[byte]int) bool {
	if e.replay == nil {
		return false
	}
	if !utils.IsBigGameHuntersMap(e.replay.MapName) {
		return false
	}
	player, ok := e.players[c.Attacker]
	if !ok || player == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(player.Race), "terran") {
		return false
	}
	tankSec, hasTank := firstTankSecByPlayer[c.Attacker]
	if !hasTank || c.Second < tankSec {
		return false
	}
	// A cliff drop needs a transport; without this a Bunker's UnloadAll near a
	// corner would qualify.
	dropshipSec, hasDropship := firstDropshipSecByPlayer[c.Attacker]
	if !hasDropship || c.Second < dropshipSec {
		return false
	}
	mapWidthPx := int(e.replay.MapWidth) * 32
	mapHeightPx := int(e.replay.MapHeight) * 32
	return utils.IsCliffDropPosition(c.X, c.Y, mapWidthPx, mapHeightPx)
}

// Empty when no player ever produced one.
func buildFirstTankSecByPlayer(stream []cmdenrich.EnrichedCommand) map[byte]int {
	return buildFirstUnitSecByPlayer(stream, models.GeneralUnitSiegeTankTankMode)
}

// Empty when no player ever produced one.
func buildFirstDropshipSecByPlayer(stream []cmdenrich.EnrichedCommand) map[byte]int {
	return buildFirstUnitSecByPlayer(stream, models.GeneralUnitDropship)
}

func buildFirstUnitSecByPlayer(stream []cmdenrich.EnrichedCommand, subject string) map[byte]int {
	out := map[byte]int{}
	for _, ec := range stream {
		if ec.Kind != cmdenrich.KindMakeUnit {
			continue
		}
		if ec.Subject != subject {
			continue
		}
		pid := byte(ec.PlayerID)
		if _, seen := out[pid]; seen {
			continue
		}
		out[pid] = ec.Second
	}
	return out
}

// emitAttackIfImportant keeps a candidate when it is the player's first attack,
// when the defender later leaves the game (so the attack mattered), when it
// lands in a rush window and the attacker already has a rush event, or when it
// introduces a unit type or spell cast the attacker hasn't shown in any prior
// emitted attack. Everything else is dropped.
func (e *Engine) emitAttackIfImportant(c CandidateAttack, cmd *models.Command,
	spellsByAttacker map[byte][]spellEvent,
	attackedAlready map[byte]bool,
	knownUnitsByAttacker map[byte]map[string]bool,
	knownSpellsByAttacker map[byte]map[string]bool,
) {
	if c.Defender == neutralPID {
		return
	}
	attackUnits := e.attackUnitsCombined(c)
	keep := false

	if !attackedAlready[c.Attacker] {
		keep = true
	}
	if !keep {
		if leaveSec, defLeft := e.leaveSec[c.Defender]; defLeft && leaveSec >= c.Second {
			keep = true
		}
	}
	if !keep && c.Second <= rushBuildWindowSec && e.attackerHasRushEvent(c.Attacker) {
		keep = true
	}

	if knownUnitsByAttacker[c.Attacker] == nil {
		knownUnitsByAttacker[c.Attacker] = map[string]bool{}
	}
	novelUnit := false
	for _, u := range attackUnits {
		if !knownUnitsByAttacker[c.Attacker][u] {
			novelUnit = true
		}
	}
	if novelUnit {
		keep = true
	}

	// Any cast within ±60s that isn't in the attacker's known-spells set.
	novelSpell := false
	if knownSpellsByAttacker[c.Attacker] == nil {
		knownSpellsByAttacker[c.Attacker] = map[string]bool{}
	}
	for _, s := range spellsByAttacker[c.Attacker] {
		if s.Second < c.Second-60 || s.Second > c.Second+60 {
			continue
		}
		if !knownSpellsByAttacker[c.Attacker][s.Subject] {
			novelSpell = true
		}
	}
	if novelSpell {
		keep = true
	}

	if !keep {
		return
	}

	for _, u := range attackUnits {
		knownUnitsByAttacker[c.Attacker][u] = true
	}
	for _, s := range spellsByAttacker[c.Attacker] {
		if s.Second < c.Second-60 || s.Second > c.Second+60 {
			continue
		}
		knownSpellsByAttacker[c.Attacker][s.Subject] = true
	}
	attackedAlready[c.Attacker] = true

	prevLen := len(e.replayEvents)
	e.emitEvent("attack", c.Second,
		fmt.Sprintf("%s attacks %s %s",
			e.playerName(c.Attacker), e.playerName(c.Defender),
			e.bases[c.PolyID].DisplayName),
		e.playerRef(c.Attacker), e.playerRef(c.Defender), c.PolyID, attackUnits)
	// emitEvent may suppress via dedup, so only attach cast counts when a row was
	// actually appended.
	if len(e.replayEvents) > prevLen {
		if counts := e.attackCastCounts(c); len(counts) > 0 {
			e.replayEvents[len(e.replayEvents)-1].AttackCastCounts = counts
		}
	}
}

// Whether a rush_proxy-pass event for this attacker is already in entries.
func (e *Engine) attackerHasRushEvent(attacker byte) bool {
	for _, ev := range e.replayEvents {
		if ev.SourceReplayPlayerID == nil || *ev.SourceReplayPlayerID != attacker {
			continue
		}
		switch ev.EventType {
		case "zergling_rush", "cannon_rush", "bunker_rush",
			"proxy_gate", "proxy_rax", "proxy_factory", "proxy_starport":
			return true
		}
	}
	return false
}

// Falls back to the player's race worker when the command's UnitType is empty.
func scoutUnitsForCandidate(e *Engine, c CandidateAttack, cmd *models.Command) []string {
	if cmd != nil && cmd.UnitType != nil {
		u := strings.TrimSpace(*cmd.UnitType)
		if u != "" {
			n := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(u, " ", ""), "_", ""))
			if strings.Contains(n, "overlord") {
				return []string{models.GeneralUnitOverlord}
			}
		}
	}
	if w := e.workerUnitForPlayer(c.Attacker); w != "" {
		return []string{w}
	}
	return nil
}

type spellEvent struct {
	Second  int
	Subject string
}

// For the importance filter's "novel cast" check.
func buildSpellHistoryByAttacker(stream []cmdenrich.EnrichedCommand) map[byte][]spellEvent {
	out := map[byte][]spellEvent{}
	for _, ec := range stream {
		if ec.Kind != cmdenrich.KindCast {
			continue
		}
		if !castIsAggressive(ec.OrderName) {
			continue
		}
		pid := byte(ec.PlayerID)
		out[pid] = append(out[pid], spellEvent{Second: ec.Second, Subject: ec.Subject})
	}
	return out
}

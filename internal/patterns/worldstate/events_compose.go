package worldstate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/utils"
)

// indexOwnershipByPoly returns a map of polyID → timeline (sorted by frame).
// Used by the rush-pass owner-sync helper.
func indexOwnershipByPoly(ownership []PolyOwnership) map[int][]OwnEvent {
	out := make(map[int][]OwnEvent, len(ownership))
	for _, t := range ownership {
		out[t.PolyID] = t.Events
	}
	return out
}

// syncOwnersAtSec sets e.ownerByBase to the owner-at-second derived from
// the ownership timelines. Cheap linear walk per polygon — timelines are
// small (typically ≤ 5 events per poly).
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

// runRushPass walks the buffered enriched stream, syncing ownership state
// at each frame from the ownership timeline, and invokes the existing
// rush/proxy/race-change/zergling-rush helpers per command. Their
// emissions go through emitEvent → e.entries / e.replayEvents.
func (e *Engine) runRushPass(ownership []PolyOwnership) {
	timelineByPoly := indexOwnershipByPoly(ownership)

	for i, ec := range e.stream {
		cmd := e.streamCommands[i]
		e.syncOwnersAtSec(timelineByPoly, ec.Second)

		pid, ok := e.playerIDFromCommand(cmd)
		if !ok {
			continue
		}
		// Skip commands issued after the player has left.
		if leaveSec, left := e.leaveSec[pid]; left && ec.Second > leaveSec {
			continue
		}

		sec := ec.Second
		e.recordRecentAttackUnit(pid, sec, cmd)
		e.recordRecentCast(pid, sec, ec)
		e.recordMarineTraining(pid, sec, cmd)
		e.processRaceSwitchEvent(cmd, pid, sec)
		e.processZerglingRushEvent(cmd, pid, sec)

		// Build/Land coords are tile-space — convert to pixels for the
		// rush helpers (they expect pixel-space x/y).
		if isBuildLike(cmd.ActionType) && cmd.X != nil && cmd.Y != nil {
			x := tileToPixel(float64(*cmd.X))
			y := tileToPixel(float64(*cmd.Y))
			biEvent := pointToEventBase(x, y, e.bases)
			e.tryEmitRushBuildEvents(cmd, pid, sec, x, y)
			e.tryEmitProxyBuildEvents(cmd, pid, sec, x, y, biEvent)
		}

		// Zerg-rush attack tracking — uses the legacy actionType-based
		// pressure classifier so the existing rush-detection threshold
		// stays unchanged.
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

		// Periodic finalize for zerg-rush observation windows.
		e.finalizeZergRushCandidates(sec, false)
	}
}

// emitOwnershipTransitions walks the ownership timelines and emits
// expansion / takeover / location_inactive narrative events in
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

// emitRecallEvents emits one "recall" game_event per Arbiter Recall cast.
// The "Made recalls" marker (firstCastEvaluator) still surfaces only the
// first cast per player; this layer is the per-cast counterpart for the
// game-events list, so observers can see every recall location/time.
func (e *Engine) emitRecallEvents(ownership []PolyOwnership) {
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

	for _, ec := range e.stream {
		if ec.Kind != cmdenrich.KindCast || ec.Subject != "Recall" {
			continue
		}
		pid := byte(ec.PlayerID)
		if leaveSec, left := e.leaveSec[pid]; left && ec.Second > leaveSec {
			continue
		}
		baseIdx := -1
		var target *NarrativePlayerRef
		if ec.X != nil && ec.Y != nil {
			baseIdx = pointToEventBase(float64(*ec.X), float64(*ec.Y), e.bases)
		}
		if baseIdx >= 0 && baseIdx < len(e.bases) {
			owner := ownerAtSec(baseIdx, ec.Second)
			if owner != neutralPID && owner != pid && !e.sameTeam(pid, owner) {
				target = e.playerRef(owner)
			}
		}
		desc := fmt.Sprintf("%s recalls", e.playerName(pid))
		if baseIdx >= 0 && baseIdx < len(e.bases) {
			where := e.bases[baseIdx].DisplayName
			if target != nil {
				desc = fmt.Sprintf("%s recalls onto %s %s", e.playerName(pid), e.playerName(byte(target.PlayerID)), where)
			} else {
				desc = fmt.Sprintf("%s recalls to %s", e.playerName(pid), where)
			}
		}
		e.emitEvent("recall", ec.Second, desc, e.playerRef(pid), target, baseIdx, []string{"Arbiter"})
	}
}

func (e *Engine) emitLeaveGameEvents() {
	pids := make([]byte, 0, len(e.leaveSec))
	for pid := range e.leaveSec {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool { return e.leaveSec[pids[i]] < e.leaveSec[pids[j]] })
	for _, pid := range pids {
		e.emitEvent("leave_game", e.leaveSec[pid],
			fmt.Sprintf("%s leaves the game", e.playerName(pid)),
			e.playerRef(pid), nil, -1, nil)
	}
}

// emitAttackCandidates applies the importance filter to attack candidates
// and emits the survivors. Drop subtype routing (reaver_drop / dt_drop)
// happens here using the source command's UnitTypes payload.
func (e *Engine) emitAttackCandidates(candidates []CandidateAttack) {
	// Index source commands by frame for unit-types lookup at drop time.
	cmdByFrame := make(map[int32]*models.Command, len(e.streamCommands))
	for i, ec := range e.stream {
		cmdByFrame[ec.Frame] = e.streamCommands[i]
	}

	// Build chronologically ordered candidate stream — BuildAttacks
	// already returns them in stream order, but stable-sort defensively.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Second < candidates[j].Second
	})

	// Pre-compute attacker spell-cast history from the stream: for each
	// attacker, sorted (Second, SubjectName) of every aggressive cast.
	// Used to detect "attack involves a spell cast new for this attacker."
	spellsByAttacker := buildSpellHistoryByAttacker(e.stream)

	// Pre-compute the second each player produced their first Siege Tank.
	// Used by the cliff-drop classifier in emitDropCandidate so we can
	// re-route a generic drop to "cliff_drop" when (BGH map + Terran +
	// tank-by-now + corner position) lines up.
	firstTankSecByPlayer := buildFirstTankSecByPlayer(e.stream)

	// Per-attacker filter state.
	attackedAlready := map[byte]bool{}
	knownUnitsByAttacker := map[byte]map[string]bool{}
	knownSpellsByAttacker := map[byte]map[string]bool{}
	reaverDropEmitted := map[byte]bool{}
	scoutEmitted := map[byte]bool{}

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
			e.emitDropCandidate(c, cmdByFrame[c.Frame], reaverDropEmitted, firstTankSecByPlayer)
		case "attack":
			e.emitAttackIfImportant(c, cmdByFrame[c.Frame], spellsByAttacker,
				attackedAlready, knownUnitsByAttacker, knownSpellsByAttacker)
		}
	}
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

func (e *Engine) emitDropCandidate(c CandidateAttack, cmd *models.Command, reaverDropEmitted map[byte]bool, firstTankSecByPlayer map[byte]int) {
	if c.Defender == neutralPID {
		return
	}
	dropUnitTypes := unitTypesFromCommand(cmd)
	dropType := "drop"
	hasReaver := hasUnitType(dropUnitTypes, models.GeneralUnitReaver)
	hasDT := hasUnitType(dropUnitTypes, models.GeneralUnitDarkTemplar)
	if hasReaver {
		dropType = "reaver_drop"
	} else if hasDT {
		dropType = "dt_drop"
	} else if e.isCliffDrop(c, firstTankSecByPlayer) {
		dropType = "cliff_drop"
	}
	// Importance: DT drops always emit; first reaver drop per attacker
	// emits, subsequent reavers are dropped; generic drops always emit
	// (rare events, useful for storyline).
	if dropType == "reaver_drop" {
		if reaverDropEmitted[c.Attacker] {
			return
		}
		reaverDropEmitted[c.Attacker] = true
	}
	dropPhrase := "drops on"
	if dropType == "cliff_drop" {
		dropPhrase = "cliff drops"
	}
	e.emitEvent(dropType, c.Second,
		fmt.Sprintf("%s %s %s %s",
			e.playerName(c.Attacker), dropPhrase, e.playerName(c.Defender),
			e.bases[c.PolyID].DisplayName),
		e.playerRef(c.Attacker), e.playerRef(c.Defender), c.PolyID, dropUnitTypes)
}

// isCliffDrop reports whether a generic drop candidate qualifies as a
// "cliff drop" — Terran attacker on a Big Game Hunters map who has
// produced a Siege Tank by drop time, dropping into the top-left or
// bottom-right corner of the map. Mirrors the marker-side gate in
// internal/patterns/markers/cliff_drop.go.
func (e *Engine) isCliffDrop(c CandidateAttack, firstTankSecByPlayer map[byte]int) bool {
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
	mapWidthPx := int(e.replay.MapWidth) * 32
	mapHeightPx := int(e.replay.MapHeight) * 32
	return utils.IsCliffDropPosition(c.X, c.Y, mapWidthPx, mapHeightPx)
}

// buildFirstTankSecByPlayer scans the enriched stream once and returns,
// per player, the second their first Siege Tank was produced. Empty
// when no player ever produced one.
func buildFirstTankSecByPlayer(stream []cmdenrich.EnrichedCommand) map[byte]int {
	out := map[byte]int{}
	for _, ec := range stream {
		if ec.Kind != cmdenrich.KindMakeUnit {
			continue
		}
		if ec.Subject != models.GeneralUnitSiegeTankTankMode {
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

// emitAttackIfImportant applies the user-defined importance filter:
//
//   - First attack of each player → keep.
//   - Defender leaves the game later → keep (the attack mattered).
//   - Attack happens during a rush window (≤ rushBuildWindowSec) AND
//     attacker has a rush event already emitted → keep.
//   - Attack contains a unit type the attacker hasn't shown in any
//     prior emitted attack → keep.
//   - Attack involves a spell cast the attacker hasn't featured before
//     in any emitted attack → keep.
//   - Otherwise drop the candidate.
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

	// Novel cast: any spell cast within ±60s by this attacker that's not
	// in the attacker's known-spells set.
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

	// Register this attack's units / spells in the per-attacker history.
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
	// emitEvent may suppress via dedup; only attach cast counts if a new
	// row was actually appended.
	if len(e.replayEvents) > prevLen {
		if counts := e.attackCastCounts(c); len(counts) > 0 {
			e.replayEvents[len(e.replayEvents)-1].AttackCastCounts = counts
		}
	}
}

// attackerHasRushEvent reports whether a rush_proxy-pass event for this
// attacker is already in the entries list (zergling/cannon/bunker/proxy).
func (e *Engine) attackerHasRushEvent(attacker byte) bool {
	for _, ev := range e.replayEvents {
		if ev.SourceReplayPlayerID == nil || *ev.SourceReplayPlayerID != attacker {
			continue
		}
		switch ev.EventType {
		case "zergling_rush", "cannon_rush", "bunker_rush",
			"proxy_gate", "proxy_rax", "proxy_factory":
			return true
		}
	}
	return false
}

// scoutUnitsForCandidate produces the attack-unit-types payload for a
// scout event by reading the source command. Falls back to the player's
// race worker when the command's UnitType is empty.
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

// buildSpellHistoryByAttacker collects all aggressive spell casts per
// attacker for the importance filter's "novel cast" check.
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


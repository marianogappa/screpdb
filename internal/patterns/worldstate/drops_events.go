package worldstate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// Drop target-inference tuning. Mirrors the Recall-side constants in
// events_compose.go but with tighter pre-windows since drop coordinates are
// usually well-defined (the unload lands somewhere specific).
const (
	dropAttackPreSec           = 10
	dropAttackPostSec          = 30
	dropActivityWindowPreSec   = 5
	dropActivityWindowPostSec  = 60
	dropActivityMinCommands    = 3
)

// dropTargetPayload mirrors recallTargetPayload — short JSON keys so the
// on-disk cost stays low across the full drop history of a replay. Adds
// `sb` (source base) since for drops the event.base column carries the
// destination polygon, not the source.
type dropTargetPayload struct {
	N  int                  `json:"n,omitempty"`  // cluster size, omit when 1
	LE int                  `json:"le,omitempty"` // last unload second, omit when equal to first
	S  []int                `json:"s,omitempty"`  // source point [x, y]
	SB *recallTargetBaseRef `json:"sb,omitempty"` // source base ref (where transports loaded)
	T  []int                `json:"t,omitempty"`  // target point [x, y]
	TB *recallTargetBaseRef `json:"tb,omitempty"` // target base
	TP byte                 `json:"tp,omitempty"` // target owner replay_player_id
	TV string               `json:"tv,omitempty"` // target via: "a" attack-coincidence, "p" post-drop activity
}

// emitDropEvents is the drops counterpart to emitRecallEvents. For every
// DropCluster it picks the best hostile target (attack-coincidence first,
// post-drop activity second) and emits a `drop` / `reaver_drop` / `dt_drop` /
// `cliff_drop` event with the s/t/tb/tp/tv/n/le payload the dashboard maps
// to source/target arrow endpoints.
//
// Clusters whose neither inference path finds a hostile target are not
// emitted — mirrors Recall's homecoming-suppression behavior.
func (e *Engine) emitDropEvents(ownership []PolyOwnership, clusters []DropCluster, candidates []CandidateAttack) {
	if len(clusters) == 0 {
		return
	}

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

	firstTankSecByPlayer := buildFirstTankSecByPlayer(e.stream)
	firstDropshipSecByPlayer := buildFirstDropshipSecByPlayer(e.stream)

	reaverDropEmitted := map[byte]bool{}

	for _, cl := range clusters {
		// Skip clusters whose destination is the player's own base or a
		// teammate's — these are home returns even if owner happens to be
		// non-neutral due to early-game ownership snapshots.
		dstOwner := ownerAtSec(cl.DstPolyID, cl.FirstSec)
		if dstOwner == cl.PID || (dstOwner != neutralPID && e.sameTeam(cl.PID, dstOwner)) {
			continue
		}

		// Tier 1: attack-coincidence. Drops that line up with an attack
		// pressure / nuke event near the drop time are real harassment.
		targetVia := ""
		var matchedCand CandidateAttack
		if cand, ok := e.inferDropTargetByAttack(cl, candidates); ok {
			matchedCand = cand
			targetVia = "a"
		}

		// Tier 2: post-drop activity proxy. Even without a pressure
		// threshold being met, sustained spatial activity at a non-source
		// polygon following the drop signals real engagement.
		if targetVia == "" {
			if _, ok := e.inferDropTargetByPostActivity(cl); ok {
				targetVia = "p"
			}
		}

		if targetVia == "" {
			// No hostile signal — drop is a home return (or unobserved).
			continue
		}

		_ = matchedCand // reserved for future use (kept for symmetry with Recall)

		// Build participating-units list. Epicenter window around the
		// cluster's mid-second. Workers (SCV/Probe/Drone) are excluded
		// because workers trained inside the drop window are typically
		// unrelated to the unload — keeping them in dilutes the dt_drop /
		// reaver_drop subtype routing (a DT drop that coincides with a
		// worker train would otherwise route to plain "drop"). Flyers are
		// filtered for the original reason: they can't be loaded into a
		// transport.
		mid := (cl.FirstSec + cl.LastSec) / 2
		rawUnits := e.buildOrTrainUnitsInWindow(cl.PID, mid)
		dropUnits := make([]string, 0, len(rawUnits))
		for _, u := range rawUnits {
			if models.IsFlyingUnit(u) || models.IsWorker(u) {
				continue
			}
			dropUnits = append(dropUnits, u)
		}

		// Subtype routing. Mirrors the historical emitDropCandidate but
		// keyed off the drop-pass's own coordinate/cluster info.
		dropType := "drop"
		hasReaver := hasUnitType(dropUnits, models.GeneralUnitReaver)
		hasDT := hasUnitType(dropUnits, models.GeneralUnitDarkTemplar)
		switch {
		case hasReaver:
			dropType = "reaver_drop"
		case hasDT:
			dropType = "dt_drop"
		case e.isCliffDropForCluster(cl, firstTankSecByPlayer, firstDropshipSecByPlayer):
			dropType = "cliff_drop"
		}

		if dropType == "reaver_drop" {
			if reaverDropEmitted[cl.PID] {
				continue
			}
			reaverDropEmitted[cl.PID] = true
		}

		e.emitDropEvent(cl, ownerAtSec, dropType, dropUnits, targetVia)
	}
}

// inferDropTargetByAttack picks the attack/nuke candidate near the drop's
// time range that confirms the drop is offensive. We deliberately exclude
// "drop" candidates (which the drop pass itself emits) to avoid self-match,
// and "scout" candidates which are too weak a signal.
func (e *Engine) inferDropTargetByAttack(cl DropCluster, candidates []CandidateAttack) (CandidateAttack, bool) {
	if len(candidates) == 0 {
		return CandidateAttack{}, false
	}
	lo := cl.FirstSec - dropAttackPreSec
	hi := cl.LastSec + dropAttackPostSec
	mid := (cl.FirstSec + cl.LastSec) / 2
	bestIdx := -1
	bestDist := -1
	for i := range candidates {
		c := candidates[i]
		if c.Attacker != cl.PID {
			continue
		}
		if c.Defender == neutralPID {
			continue
		}
		if c.Type != "attack" && c.Type != "nuke" {
			continue
		}
		// An attack candidate at the source base of this drop doesn't
		// validate offensive intent — the player can attack from home.
		if cl.SourceBaseIdx >= 0 && c.PolyID == cl.SourceBaseIdx {
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
		default: // nuke
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
		return CandidateAttack{}, false
	}
	return candidates[bestIdx], true
}

// inferDropTargetByPostActivity counts the player's spatial commands per
// polygon in [lastSec - 5, lastSec + 60], excludes the drop's source polygon,
// and returns true if any polygon (including the destination) has at least
// dropActivityMinCommands hits. Mirrors recall's activity heuristic.
func (e *Engine) inferDropTargetByPostActivity(cl DropCluster) (int, bool) {
	if len(e.bases) == 0 {
		return -1, false
	}
	lo := cl.LastSec - dropActivityWindowPreSec
	hi := cl.LastSec + dropActivityWindowPostSec
	counts := map[int]int{}
	for _, ec := range e.stream {
		if ec.Second < lo {
			continue
		}
		if ec.Second > hi {
			break
		}
		if byte(ec.PlayerID) != cl.PID {
			continue
		}
		if ec.X == nil || ec.Y == nil {
			continue
		}
		x, y := *ec.X, *ec.Y
		if ec.Kind == cmdenrich.KindMakeBuilding {
			x = x*32 + 16
			y = y*32 + 16
		}
		bi := pointToEventBase(float64(x), float64(y), e.bases)
		if bi < 0 {
			continue
		}
		if cl.SourceBaseIdx >= 0 && bi == cl.SourceBaseIdx {
			continue
		}
		counts[bi]++
	}
	bestIdx := -1
	bestCount := 0
	for idx, c := range counts {
		if c < dropActivityMinCommands {
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

// buildOrTrainUnitsInWindow returns distinct unit-build names produced by the
// player in [mid - past, mid + future]. Unlike buildUnitsInEpicenterWindow it
// walks the enriched stream directly rather than the attacker-only
// attackUnitsByPID buffer (drops aren't necessarily flagged as attacks). The
// caller is expected to filter out workers/flyers as needed.
func (e *Engine) buildOrTrainUnitsInWindow(pid byte, mid int) []string {
	epicenter := mid - attackUnitsEpicenterOffsetSec
	lo := epicenter - attackUnitsPastSec
	hi := epicenter + attackUnitsFutureSec
	seen := map[string]struct{}{}
	out := []string{}
	for _, ec := range e.stream {
		if ec.Second < lo {
			continue
		}
		if ec.Second > hi {
			break
		}
		if byte(ec.PlayerID) != pid {
			continue
		}
		if ec.Kind != cmdenrich.KindMakeUnit {
			continue
		}
		u := ec.Subject
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out
}

// isCliffDropForCluster gates a generic drop to the "cliff_drop" subtype:
// Big Game Hunters map, Terran attacker with a Dropship + Siege Tank by drop
// time, and at least one unload landing in a corner cliff box. It tests the
// individual unload points rather than the cluster centroid — a cluster that
// merges a corner cliff drop with a nearby edge unload has a centroid pulled
// off the cliff, but the corner unloads themselves still qualify.
func (e *Engine) isCliffDropForCluster(cl DropCluster, firstTankSecByPlayer, firstDropshipSecByPlayer map[byte]int) bool {
	for _, u := range cl.Unloads {
		c := CandidateAttack{
			Attacker: cl.PID,
			Second:   cl.FirstSec,
			X:        u[0],
			Y:        u[1],
		}
		if e.isCliffDrop(c, firstTankSecByPlayer, firstDropshipSecByPlayer) {
			return true
		}
	}
	return false
}

func (e *Engine) emitDropEvent(cl DropCluster, ownerAtSec func(int, int) byte, dropType string, dropUnits []string, targetVia string) {
	actor := e.playerRef(cl.PID)
	owner := ownerAtSec(cl.DstPolyID, cl.FirstSec)
	var target *NarrativePlayerRef
	if owner != neutralPID && owner != cl.PID && !e.sameTeam(cl.PID, owner) {
		target = e.playerRef(owner)
	}
	if target == nil {
		return
	}

	// Source fallback: when no Load was paired (the common case — icza/screp
	// doesn't expose transport-targeting right-clicks reliably), use the
	// player's start base as a stand-in source so the arrow always renders.
	// Production buildings (Starport / Stargate / Lair) for transports
	// typically sit at the main, so start is the best default.
	if !cl.HasSource {
		if startIdx, ok := e.startBaseByPID[cl.PID]; ok && startIdx >= 0 && startIdx < len(e.bases) {
			cl.SourceBaseIdx = startIdx
			cl.SourceX = int(e.bases[startIdx].CenterX)
			cl.SourceY = int(e.bases[startIdx].CenterY)
			cl.HasSource = true
		}
	}

	// Build the description. Mirrors the historical "drops on" / "cliff
	// drops" phrasing but adds the (xN) suffix for multi-unload clusters
	// and the optional "from <source>" clause.
	dropPhrase := "drops on"
	if dropType == "cliff_drop" {
		dropPhrase = "cliff drops"
	}
	countSuffix := ""
	if cl.Count > 1 {
		countSuffix = fmt.Sprintf(" (×%d)", cl.Count)
	}
	sourceClause := ""
	if cl.HasSource && cl.SourceBaseIdx >= 0 && cl.SourceBaseIdx < len(e.bases) {
		sourceClause = " from " + e.bases[cl.SourceBaseIdx].DisplayName
	}
	targetLabel := e.bases[cl.DstPolyID].DisplayName
	decoratedTarget := e.decorateBaseDescriptionForPlayer(cl.PID, cl.DstPolyID, targetLabel)
	desc := fmt.Sprintf("%s %s%s%s %s %s", e.playerName(cl.PID), dropPhrase, countSuffix, sourceClause, e.playerName(byte(target.PlayerID)), decoratedTarget)

	// Payload.
	pl := dropTargetPayload{}
	if cl.Count > 1 {
		pl.N = cl.Count
	}
	if cl.LastSec != cl.FirstSec {
		pl.LE = cl.LastSec
	}
	if cl.HasSource {
		pl.S = []int{cl.SourceX, cl.SourceY}
		if cl.SourceBaseIdx >= 0 && cl.SourceBaseIdx < len(e.bases) {
			sBaseType, sBaseOclock, sNaturalOf, sMineralOnly := e.locationForBase(cl.SourceBaseIdx)
			sb := recallTargetBaseRef{}
			if sBaseType != nil {
				sb.K = *sBaseType
			}
			if sBaseOclock != nil {
				sb.O = *sBaseOclock
			}
			if sNaturalOf != nil {
				no := *sNaturalOf
				sb.NO = &no
			}
			if sMineralOnly != nil && *sMineralOnly {
				mo := true
				sb.MO = &mo
			}
			pl.SB = &sb
		}
	}
	pl.T = []int{cl.DstX, cl.DstY}
	baseType, baseOclock, naturalOf, mineralOnly := e.locationForBase(cl.DstPolyID)
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
	pl.TP = byte(target.PlayerID)
	pl.TV = targetVia
	raw, _ := json.Marshal(pl)
	payloadStr := string(raw)

	// Append narrative entry + replay event. We bypass emitEvent to attach
	// the Payload field (same pattern as Recall).
	base := e.baseRef(cl.DstPolyID)
	e.entries = append(e.entries, NarrativeEntry{
		Type:        dropType,
		Second:      cl.FirstSec,
		Description: desc,
		Actor:       actor,
		Target:      target,
		Base:        base,
		ActorOrigin: e.actorOrigin(actor, base),
		Ownership:   e.ownershipSnapshot(),
	})
	rev := e.toReplayEvent(dropType, cl.FirstSec, actor, target, cl.DstPolyID, dropUnits)
	rev.Payload = &payloadStr
	e.replayEvents = append(e.replayEvents, rev)
	_ = strings.TrimSpace // keep strings import in case future formatting tweaks
}

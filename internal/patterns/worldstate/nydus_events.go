package worldstate

import (
	"encoding/json"
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// nydusTargetPayload mirrors dropTargetPayload — short JSON keys keep the
// on-disk cost low. Source is the attacker's home (the canal entrance, stood
// in for by the start base); target is the forward exit where the army
// emerges, which is also the event's location column.
type nydusTargetPayload struct {
	N  int                  `json:"n,omitempty"`  // teleport-wave count when observed
	S  []int                `json:"s,omitempty"`  // source point [x, y]
	SB *recallTargetBaseRef `json:"sb,omitempty"` // source base ref (home)
	T  []int                `json:"t,omitempty"`  // target point [x, y] (the exit)
	TB *recallTargetBaseRef `json:"tb,omitempty"` // target base (where the exit landed)
	TP byte                 `json:"tp,omitempty"` // target owner replay_player_id
	TV string               `json:"tv,omitempty"` // confirmation: "t" teleport, "a" attack, "p" activity
}

// emitNydusEvents synthesizes one `nydus_attack` game event per offensive
// nydus cluster: a Nydus Canal exit placed forward in enemy territory. The
// exit position is the attack epicenter, much like a drop's unload point.
func (e *Engine) emitNydusEvents(clusters []NydusCluster) {
	for _, cl := range clusters {
		e.emitNydusEvent(cl)
	}
}

func (e *Engine) emitNydusEvent(cl NydusCluster) {
	actor := e.playerRef(cl.PID)
	if cl.Defender == neutralPID || cl.Defender == cl.PID || e.sameTeam(cl.PID, cl.Defender) {
		return
	}
	target := e.playerRef(cl.Defender)
	if actor == nil || target == nil {
		return
	}

	// Participating units: ground army produced around the insertion. Workers
	// and flyers are excluded — flyers can't travel through a canal and workers
	// are usually unrelated production.
	rawUnits := e.buildOrTrainUnitsInWindow(cl.PID, cl.Sec)
	nydusUnits := make([]string, 0, len(rawUnits))
	for _, u := range rawUnits {
		if models.IsFlyingUnit(u) || models.IsWorker(u) {
			continue
		}
		nydusUnits = append(nydusUnits, u)
	}

	// Source = the attacker's home (start base), a stand-in for the canal
	// entrance the army funnels in from, so the arrow always renders.
	var sourceX, sourceY, sourceBaseIdx int
	hasSource := false
	if startIdx, ok := e.startBaseByPID[cl.PID]; ok && startIdx >= 0 && startIdx < len(e.bases) {
		sourceBaseIdx = startIdx
		sourceX = int(e.bases[startIdx].CenterX)
		sourceY = int(e.bases[startIdx].CenterY)
		hasSource = true
	}

	targetLabel := e.bases[cl.PolyID].DisplayName
	decoratedTarget := e.decorateBaseDescriptionForPlayer(cl.PID, cl.PolyID, targetLabel)
	// Conservative phrasing: the exit being placed forward is what we observe;
	// the army may never cross (the canal can be killed first), so we don't
	// assert units arrived.
	desc := fmt.Sprintf("%s makes an offensive nydus onto %s %s", e.playerName(cl.PID), e.playerName(byte(target.PlayerID)), decoratedTarget)

	pl := nydusTargetPayload{}
	if cl.Count > 0 {
		pl.N = cl.Count
	}
	pl.TV = cl.Via
	if hasSource {
		pl.S = []int{sourceX, sourceY}
		sBaseType, sBaseOclock, sNaturalOf, sMineralOnly := e.locationForBase(sourceBaseIdx)
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
	pl.T = []int{cl.X, cl.Y}
	baseType, baseOclock, naturalOf, mineralOnly := e.locationForBase(cl.PolyID)
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
	raw, _ := json.Marshal(pl)
	payloadStr := string(raw)

	base := e.baseRef(cl.PolyID)
	e.entries = append(e.entries, NarrativeEntry{
		Type:        "nydus_attack",
		Second:      cl.Sec,
		Description: desc,
		Actor:       actor,
		Target:      target,
		Base:        base,
		ActorOrigin: e.actorOrigin(actor, base),
		Ownership:   e.ownershipSnapshot(),
	})
	rev := e.toReplayEvent("nydus_attack", cl.Sec, actor, target, cl.PolyID, nydusUnits)
	rev.Payload = &payloadStr
	e.replayEvents = append(e.replayEvents, rev)
}

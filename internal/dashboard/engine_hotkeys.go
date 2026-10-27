//go:build !js

package dashboard

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/library"
)

// engineAssigns pairs a player's stream assigns with the engine's. The stream and the engine see the same
// commands, so per group they line up in order; a stream assign takes the first unused engine assign of its
// group within a second of it.
type engineAssigns struct {
	byGroup map[uint8][]engine.HotkeyAssign
	next    map[uint8]int
	last    map[uint8]engine.HotkeyAssign
}

func newEngineAssigns(dg *engine.Digest, slot int8) *engineAssigns {
	a := &engineAssigns{byGroup: map[uint8][]engine.HotkeyAssign{}, next: map[uint8]int{}, last: map[uint8]engine.HotkeyAssign{}}
	for _, h := range dg.Hotkeys {
		if h.Owner == slot {
			a.byGroup[h.Group] = append(a.byGroup[h.Group], h)
		}
	}
	return a
}

// match finds the engine assign behind a stream assign of group at sec; it must be called in stream order.
func (a *engineAssigns) match(group uint8, sec int64) (engine.HotkeyAssign, bool) {
	list := a.byGroup[group]
	k := a.next[group]
	for k < len(list) && frameSecond(list[k].Frame) < sec-1 {
		k++
	}
	m, ok := engine.HotkeyAssign{}, false
	if k < len(list) && frameSecond(list[k].Frame) <= sec+1 {
		m, ok = list[k], true
		k++
	} else if prev, seen := a.last[group]; seen && sec-frameSecond(prev.Frame) <= 1 {
		// The engine reports a repeated ctrl+number in the same frame once.
		m, ok = prev, true
	}
	a.next[group] = k
	if ok {
		a.last[group] = m
	}
	return m, ok
}

// applyEngineHotkeys puts the engine's main unit and exact count on each unit assign of the timeline.
func (d *Dashboard) applyEngineHotkeys(replayID int64, payload *hotkeyTimelinePayload) {
	dg, rec := d.engineDigest(replayID)
	if dg == nil {
		return
	}
	slotByPlayer := map[int64]int8{}
	for ordinal, p := range rec.Players {
		slotByPlayer[library.PlayerID(rec.ID, uint8(ordinal))] = int8(p.Slot)
	}
	exact := true
	for i := range payload.Players {
		p := &payload.Players[i]
		slot, ok := slotByPlayer[p.PlayerID]
		if !ok {
			exact = false
			continue
		}
		assigns := newEngineAssigns(dg, slot)
		for j := range p.Events {
			ev := &p.Events[j]
			if byte(ev[evType]) != hotkeystream.TypeAssignUnits {
				continue
			}
			match, ok := assigns.match(uint8(ev[evGroup]), int64(ev[evSec]))
			unit := repcmd.UnitByID(match.MainType)
			if !ok || unit == nil || unit.Name == "" {
				exact = false
				continue
			}
			ev[evUnitType] = int(match.MainType)
			ev[evUnitCount] = int(match.Count)
			payload.Units[match.MainType] = unit.Name
		}
	}
	payload.UnitTypesExact = exact
}

// engineHotkeyUnits puts the engine's main unit type and exact count on a game's unit assigns, for the
// player's hotkey signature; a game without trusted engine data keeps its stream as recorded.
func (d *Dashboard) engineHotkeyUnits(replayID int64, slot int8, events []hotkeystream.Event) {
	dg, _ := d.engineDigest(replayID)
	if dg == nil {
		return
	}
	assigns := newEngineAssigns(dg, slot)
	for i := range events {
		e := &events[i]
		if e.Type != hotkeystream.TypeAssignUnits {
			continue
		}
		if m, ok := assigns.match(e.Group, int64(e.Sec)); ok {
			e.Unit, e.Count = m.MainType, m.Count
		}
	}
}

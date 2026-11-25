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
	for i := range payload.Players {
		p := &payload.Players[i]
		slot, ok := slotByPlayer[p.PlayerID]
		if !ok {
			continue
		}
		assigns := newEngineAssigns(dg, slot)
		for j := range p.Events {
			if byte(p.Events[j][evType]) != hotkeystream.TypeAssignUnits {
				continue
			}
			if match, ok := assigns.match(uint8(p.Events[j][evGroup]), int64(p.Events[j][evSec])); ok {
				applyEngineAssign(p, j, match, payload)
			}
		}
	}
	payload.UnitTypesExact = true
}

// applyEngineAssign puts what the engine saw on one unit assign of the timeline. The stream only recognizes a
// building from what it later did, so a building that was keyed and never used reaches here as one unit; the
// engine knows it was a building, and the assign is shown as one (its placement stays unknown).
func applyEngineAssign(p *hotkeyTimelinePlayer, j int, match engine.HotkeyAssign, payload *hotkeyTimelinePayload) {
	unit := repcmd.UnitByID(match.MainType)
	if unit == nil || unit.Name == "" {
		return
	}
	ev := &p.Events[j]
	if building := hotkeystream.BuildingID(unit.Name); building != 0 && match.Count == 1 {
		ev[evType] = int(hotkeystream.TypeAssignBuilding)
		ev[evCount] = 0
		ev[evBuilding] = int(building)
		ev[evTileX], ev[evTileY] = int(hotkeystream.TileUnknown), int(hotkeystream.TileUnknown)
		payload.Buildings[building] = unit.Name
		return
	}
	ev[evUnitType] = int(match.MainType)
	ev[evUnitCount] = int(match.Count)
	payload.Units[match.MainType] = unit.Name
	if composition := unitComposition(match.Units, payload.Units); len(composition) > 0 {
		if p.Compositions == nil {
			p.Compositions = map[int][][2]int{}
		}
		p.Compositions[j] = composition
	}
}

// unitComposition lists an assign's unit types as [unitType, count] pairs and names each of them.
func unitComposition(units []engine.UnitCount, names map[uint16]string) [][2]int {
	out := make([][2]int, 0, len(units))
	for _, u := range units {
		names[u.Type] = repcmd.UnitByID(u.Type).Name
		out = append(out, [2]int{int(u.Type), int(u.Count)})
	}
	return out
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

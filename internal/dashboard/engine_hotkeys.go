//go:build !js

package dashboard

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/library"
)

// applyEngineHotkeys puts the engine's main unit and exact count on each unit assign of the timeline.
// The stream and the engine see the same commands, so per group they line up in order; a stream assign
// takes the first unused engine assign of its group within a second of it.
func (d *Dashboard) applyEngineHotkeys(replayID int64, payload *hotkeyTimelinePayload) {
	dg, rec := d.engineDigest(replayID)
	if dg == nil {
		return
	}
	slotByPlayer := map[int64]int8{}
	for ordinal, p := range rec.Players {
		slotByPlayer[library.PlayerID(rec.ID, uint8(ordinal))] = int8(p.Slot)
	}
	type key struct {
		slot  int8
		group uint8
	}
	byGroup := map[key][]engine.HotkeyAssign{}
	for _, h := range dg.Hotkeys {
		k := key{h.Owner, h.Group}
		byGroup[k] = append(byGroup[k], h)
	}
	exact := true
	for i := range payload.Players {
		p := &payload.Players[i]
		slot, ok := slotByPlayer[p.PlayerID]
		if !ok {
			exact = false
			continue
		}
		next := map[uint8]int{}
		last := map[uint8]engine.HotkeyAssign{}
		for j := range p.Events {
			ev := &p.Events[j]
			if byte(ev[evType]) != hotkeystream.TypeAssignUnits {
				continue
			}
			group := uint8(ev[evGroup])
			sec := int64(ev[evSec])
			list := byGroup[key{slot, group}]
			k := next[group]
			for k < len(list) && frameSecond(list[k].Frame) < sec-1 {
				k++
			}
			match, ok := engine.HotkeyAssign{}, false
			if k < len(list) && frameSecond(list[k].Frame) <= sec+1 {
				match, ok = list[k], true
				k++
			} else if prev, seen := last[group]; seen && sec-frameSecond(prev.Frame) <= 1 {
				// The engine reports a repeated ctrl+number in the same frame once.
				match, ok = prev, true
			}
			next[group] = k
			unit := repcmd.UnitByID(match.MainType)
			if !ok || unit == nil || unit.Name == "" {
				exact = false
				continue
			}
			last[group] = match
			ev[evUnitType] = int(match.MainType)
			ev[evUnitCount] = int(match.Count)
			payload.Units[match.MainType] = unit.Name
		}
	}
	payload.UnitTypesExact = exact
}

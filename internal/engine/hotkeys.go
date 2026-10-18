package engine

import (
	"github.com/icza/screp/rep/repcmd"
	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/screpsim"
)

// HotkeyAssign is what a hotkey group held right after a ctrl or shift + number: its most valuable unit
// type and how many units it held. Replays only record unit tags, so this needs the engine.
type HotkeyAssign struct {
	Frame    int32
	Owner    int8
	Group    uint8
	MainType uint16
	Count    uint8
}

type hotkeyGroupKey struct {
	owner int8
	group uint8
}

func (p *passState) recordHotkey(e *screpsim.CommandResult) {
	hc, ok := e.Cmd.(*repcmd.HotkeyCmd)
	if !ok || hc.HotkeyType == nil || hc.Group > 9 || e.Owner > maxPlayerSlot {
		return
	}
	key := hotkeyGroupKey{owner: e.Owner, group: hc.Group}
	switch hc.HotkeyType.Name {
	case "Select":
		p.hotkeyGroups[key] = append([]openbw.UnitID(nil), e.Selection...)
		return
	case "Assign":
		p.hotkeyGroups[key] = append([]openbw.UnitID(nil), e.Selection...)
	case "Add":
		p.hotkeyGroups[key] = unionUnits(p.hotkeyGroups[key], e.Selection)
	default:
		return
	}
	if a, ok := p.summarizeGroup(e.Frame, key); ok {
		p.d.Hotkeys = append(p.d.Hotkeys, a)
	}
}

// summarizeGroup picks the group's main unit by total value, so three Lurkers outrank eight Zerglings.
func (p *passState) summarizeGroup(frame int32, key hotkeyGroupKey) (HotkeyAssign, bool) {
	counts := map[openbw.UnitType]int{}
	total := 0
	for _, id := range p.hotkeyGroups[key] {
		u, ok := p.g.Engine().Unit(id)
		if !ok || u.Owner != key.owner || u.Dying {
			continue
		}
		counts[u.Type]++
		total++
	}
	if total == 0 {
		return HotkeyAssign{}, false
	}
	var main openbw.UnitType
	best := -1
	for t, n := range counts {
		score := n * int(max(CostOf(uint16(t)).Value(), 1))
		if score > best || score == best && t < main {
			main, best = t, score
		}
	}
	return HotkeyAssign{Frame: frame, Owner: key.owner, Group: key.group, MainType: uint16(main), Count: uint8(min(total, 255))}, true
}

func unionUnits(a, b []openbw.UnitID) []openbw.UnitID {
	out := append([]openbw.UnitID(nil), a...)
	for _, id := range b {
		found := false
		for _, have := range out {
			if have == id {
				found = true
				break
			}
		}
		if !found {
			out = append(out, id)
		}
	}
	return out
}

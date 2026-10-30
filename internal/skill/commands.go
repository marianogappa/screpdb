package skill

import (
	"github.com/icza/screp/rep/repcmd"
)

// Commands are the skill metrics read from one player's command stream, from MeasuredFromSecond to the end.
type Commands struct {
	// UsedRecalls are hotkey recalls per minute followed by a command before the selection changes again:
	// selecting a group in order to do something with it.
	UsedRecalls float64
	// CheckRecalls are the other recalls per minute: cycling through groups to look at them.
	CheckRecalls float64
	// ArmyCadence is the share of 10-second windows in which the player started army production.
	ArmyCadence float64
	// MicroActions are unit abilities used per minute: Hold Position, unloads, sieging, Stim Packs, cloaking,
	// Archon merges and spells.
	MicroActions float64
}

const (
	// MeasuredFromSecond skips the opening, when players cycle hotkeys to warm up their hands.
	MeasuredFromSecond = 120
	millisPerFrame     = 42
	fromFrame          = MeasuredFromSecond * 1000 / millisPerFrame
	cadenceBinFrames   = 10 * 1000 / millisPerFrame
	// doubleTapFrames joins presses of one group into one recall: a double tap only moves the camera there.
	doubleTapFrames = 8
	// repeatFrames joins presses of one ability within a second into one use: spamming Stim or Hold Position
	// is one decision.
	repeatFrames = 1000 / millisPerFrame
)

// nonArmy are the SCV, Drone, Overlord and Probe unit IDs.
var nonArmy = map[uint16]bool{0x07: true, 0x29: true, 0x2A: true, 0x40: true}

// CommandsOf measures every player in cmds over a game of endFrame frames; players are keyed by replay
// player ID. A game shorter than MeasuredFromSecond plus a minute measures nobody.
func CommandsOf(cmds []repcmd.Cmd, endFrame int32) map[byte]Commands {
	if endFrame-fromFrame < 60*1000/millisPerFrame {
		return nil
	}
	byPlayer := map[byte][]repcmd.Cmd{}
	for _, c := range cmds {
		id := c.BaseCmd().PlayerID
		byPlayer[id] = append(byPlayer[id], c)
	}
	minutes := float64(endFrame-fromFrame) * millisPerFrame / 60000
	bins := int(endFrame-fromFrame)/cadenceBinFrames + 1
	out := map[byte]Commands{}
	for id, own := range byPlayer {
		used, checks := recalls(own)
		out[id] = Commands{UsedRecalls: used / minutes, CheckRecalls: checks / minutes, ArmyCadence: armyCadence(own, bins), MicroActions: microActions(own) / minutes}
	}
	return out
}

func recalls(cmds []repcmd.Cmd) (used, checks float64) {
	for i := 0; i < len(cmds); i++ {
		h, ok := hotkeySelect(cmds[i])
		if !ok || int32(cmds[i].BaseCmd().Frame) < fromFrame {
			continue
		}
		last := cmds[i].BaseCmd().Frame
		j := i + 1
		for ; j < len(cmds); j++ {
			n, ok := hotkeySelect(cmds[j])
			if !ok || n.Group != h.Group || int32(cmds[j].BaseCmd().Frame-last) > doubleTapFrames {
				break
			}
			last = cmds[j].BaseCmd().Frame
		}
		if actsBeforeReselecting(cmds[j:]) {
			used++
		} else {
			checks++
		}
		i = j - 1
	}
	return used, checks
}

func actsBeforeReselecting(cmds []repcmd.Cmd) bool {
	for _, c := range cmds {
		if selectionChanger(c) {
			return false
		}
		if isAction(c) {
			return true
		}
	}
	return false
}

func armyCadence(cmds []repcmd.Cmd, bins int) float64 {
	producing := make([]bool, bins)
	for _, c := range cmds {
		f := int32(c.BaseCmd().Frame)
		if f < fromFrame {
			continue
		}
		if t, ok := c.(*repcmd.TrainCmd); ok && t.Unit != nil && !nonArmy[t.Unit.ID] {
			producing[min(bins-1, int(f-fromFrame)/cadenceBinFrames)] = true
		}
	}
	n := 0
	for _, p := range producing {
		if p {
			n++
		}
	}
	return float64(n) / float64(bins)
}

// Burrowing and lifting off are left out of micro actions: on ladder, Zerg who burrowed more were rated lower,
// and a floating building is not micro.
func microAction(c repcmd.Cmd) (kind byte, ok bool) {
	switch id := c.BaseCmd().Type.ID; id {
	case repcmd.TypeIDHoldPosition, repcmd.TypeIDStim:
		return id, true
	case repcmd.TypeIDSiege, repcmd.TypeIDUnsiege:
		return repcmd.TypeIDSiege, true
	case repcmd.TypeIDCloack, repcmd.TypeIDDecloack:
		return repcmd.TypeIDCloack, true
	case repcmd.TypeIDMergeArchon, repcmd.TypeIDMergeDarkArchon:
		return repcmd.TypeIDMergeArchon, true
	case repcmd.TypeIDUnloadAll, repcmd.TypeIDUnload, repcmd.TypeIDUnload121:
		return repcmd.TypeIDUnload, true
	}
	t, ok := c.(*repcmd.TargetedOrderCmd)
	switch {
	case !ok || t.Order == nil:
		return 0, false
	case t.Order.ID == repcmd.OrderIDUnload || t.Order.ID == repcmd.OrderIDMoveUnload:
		return repcmd.TypeIDUnload, true
	case spellOrders[t.Order.ID]:
		return t.Order.ID, true
	}
	return 0, false
}

// spellOrders are every spell a unit can be ordered to cast, plus painting a Nuke.
var spellOrders = map[byte]bool{
	0x7e: true, 0x73: true, 0x77: true, 0x78: true, 0x79: true, 0x7a: true, 0x89: true, 0x8d: true, 0x8e: true,
	0x8f: true, 0x90: true, 0x91: true, 0x92: true, 0x93: true, 0x94: true, 0xb4: true, 0xb5: true, 0xb6: true,
	0xb8: true, 0xb9: true, 0xba: true,
}

func microActions(cmds []repcmd.Cmd) float64 {
	n := 0.0
	last := map[byte]int32{}
	for _, c := range cmds {
		f := int32(c.BaseCmd().Frame)
		kind, ok := microAction(c)
		if !ok || f < fromFrame {
			continue
		}
		if prev, seen := last[kind]; seen && f-prev < repeatFrames {
			continue
		}
		last[kind] = f
		n++
	}
	return n
}

func hotkeySelect(c repcmd.Cmd) (*repcmd.HotkeyCmd, bool) {
	h, ok := c.(*repcmd.HotkeyCmd)
	return h, ok && h.HotkeyType != nil && h.HotkeyType.ID == repcmd.HotkeyTypeIDSelect
}

func selectionChanger(c repcmd.Cmd) bool {
	switch c.BaseCmd().Type.ID {
	case repcmd.TypeIDSelect, repcmd.TypeIDSelectAdd, repcmd.TypeIDSelectRemove,
		repcmd.TypeIDSelect121, repcmd.TypeIDSelectAdd121, repcmd.TypeIDSelectRemove121:
		return true
	}
	_, ok := hotkeySelect(c)
	return ok
}

func isAction(c repcmd.Cmd) bool {
	if selectionChanger(c) {
		return false
	}
	switch c.BaseCmd().Type.ID {
	case repcmd.TypeIDHotkey, repcmd.TypeIDSync, repcmd.TypeIDKeepAlive, repcmd.TypeIDChat, repcmd.TypeIDSaveGame,
		repcmd.TypeIDMinimapPing:
		return false
	}
	return true
}

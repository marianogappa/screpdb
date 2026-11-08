package skill

import (
	"math"

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
	// MicroActions are the 15-second windows per minute in which the player used a unit ability: Hold
	// Position, Stim Packs, cloaking, Archon merges or transport drops.
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
	// microWindowFrames is the stretch in which any number of micro actions count as one: Hold Position pressed
	// 20 times in 20 seconds is one burst, not 20 decisions. On ladder 15 seconds kept as many pros on Pro as
	// counting presses did, while 20 or more seconds started to cost them.
	microWindowFrames = 15 * 1000 / millisPerFrame
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

// Only Hold Position, Stim Packs, cloaking, Archon merges and transport drops count as micro: on ladder they
// followed MMR, while sieging, Bunker unloads and spells did not. A beginner sieging and unsieging one tank
// all game is not microing.
func microAction(c repcmd.Cmd) (kind byte, ok bool) {
	switch id := c.BaseCmd().Type.ID; id {
	case repcmd.TypeIDHoldPosition, repcmd.TypeIDStim:
		return id, true
	case repcmd.TypeIDCloack, repcmd.TypeIDDecloack:
		return repcmd.TypeIDCloack, true
	case repcmd.TypeIDMergeArchon, repcmd.TypeIDMergeDarkArchon:
		return repcmd.TypeIDMergeArchon, true
	}
	if t, ok := c.(*repcmd.TargetedOrderCmd); ok && t.Order != nil && (t.Order.ID == repcmd.OrderIDUnload || t.Order.ID == repcmd.OrderIDMoveUnload) {
		return repcmd.TypeIDUnload, true
	}
	return 0, false
}

func microActions(cmds []repcmd.Cmd) float64 {
	windows := map[int32]bool{}
	for _, c := range cmds {
		f := int32(c.BaseCmd().Frame)
		if _, ok := microAction(c); ok && f >= fromFrame {
			windows[(f-fromFrame)/microWindowFrames] = true
		}
	}
	return float64(len(windows))
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

// microArmyValue is the army worth, from 8:00, at which every micro action counts in full. Below it they
// count in proportion: Hold Position spammed on two Zerglings is not microing an army.
const microArmyValue = 3000

// ScaledMicro is micro actions per minute weighed by the army they were used on; armyValue is NaN when the
// game never measured an army, and then the actions count in full.
func ScaledMicro(micro, armyValue float64) float64 {
	if math.IsNaN(armyValue) {
		return micro
	}
	return micro * min(1, armyValue/microArmyValue)
}

// startedCadenceSlack is how much more often than units start a player may click to train them: on ladder,
// players of every tier start units in nearly every window they click in (1.03 to 1.15 times as many).
const startedCadenceSlack = 1.25

// StartedCadence is army cadence held to the windows in which army units really started, so clicking Train on
// full queues all game is not steady production; started is NaN without an engine pass, and then the clicks
// count.
func StartedCadence(cadence, started float64) float64 {
	if math.IsNaN(started) {
		return cadence
	}
	return min(cadence, startedCadenceSlack*started)
}

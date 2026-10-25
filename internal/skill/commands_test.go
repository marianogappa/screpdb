package skill

import (
	"math"
	"testing"

	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/rep/repcore"
)

const sec = 1000 / millisPerFrame

func base(frame int, typeID byte) *repcmd.Base {
	return &repcmd.Base{Frame: repcore.Frame(frame), PlayerID: 1, Type: repcmd.TypeByID(typeID)}
}

func recall(frame int, group byte) repcmd.Cmd {
	return &repcmd.HotkeyCmd{Base: base(frame, repcmd.TypeIDHotkey), HotkeyType: repcmd.HotkeyTypeByID(repcmd.HotkeyTypeIDSelect), Group: group}
}

func order(frame int) repcmd.Cmd {
	return &repcmd.TargetedOrderCmd{Base: base(frame, repcmd.TypeIDTargetedOrder), Order: repcmd.OrderByID(repcmd.OrderIDMove)}
}

func train(frame int, unit uint16) repcmd.Cmd {
	return &repcmd.TrainCmd{Base: base(frame, repcmd.TypeIDTrain), Unit: repcmd.UnitByID(unit)}
}

func TestCommandsSplitRecallsAndCountArmyWindows(t *testing.T) {
	const marine, scv = 0x00, 0x07
	at := func(s int) int { return fromFrame + s*sec }
	cmds := []repcmd.Cmd{
		recall(at(0)-sec, 1), order(at(0) - sec + 5),
		recall(at(1), 1), recall(at(1)+3, 1), order(at(1) + 10),
		recall(at(2), 2), recall(at(3), 3), order(at(3) + 5),
		train(at(5), marine), train(at(7), marine), train(at(25), scv),
		recall(at(30), 4),
	}
	end := int32(at(120))
	got := CommandsOf(cmds, end)[1]
	minutes := float64(end-fromFrame) * millisPerFrame / 60000
	if math.Abs(got.UsedRecalls-2/minutes) > 1e-9 || math.Abs(got.CheckRecalls-2/minutes) > 1e-9 {
		t.Fatalf("used %v checks %v per minute, want 2 of each over %.2f minutes: a double tap is one recall and the opening is skipped", got.UsedRecalls, got.CheckRecalls, minutes)
	}
	bins := float64((int(end)-fromFrame)/cadenceBinFrames + 1)
	if math.Abs(got.ArmyCadence-1/bins) > 1e-9 {
		t.Fatalf("army cadence %v, want one window of %v (SCVs are not army)", got.ArmyCadence, bins)
	}
	if CommandsOf(cmds, int32(at(30))) != nil {
		t.Fatal("under a minute past 2:00 measures nothing")
	}
}

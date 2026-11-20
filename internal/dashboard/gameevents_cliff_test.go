package dashboard

import (
	"testing"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine/moments"
	"github.com/marianogappa/screpdb/internal/library"
)

func TestMarkCliffDropsRenamesTheNearestDropOfThatPlayer(t *testing.T) {
	detail := &workflowGameDetail{
		Players: []workflowGamePlayer{{PlayerID: 1, DetectedPatterns: []workflowPatternValue{{EventType: "cliff_drop", DetectedSecond: 381}}}},
		GameEvents: []workflowGameEvent{
			{Type: "drop", Second: 379, Actor: &workflowGameEventPlayer{PlayerID: 1}},
			{Type: "drop", Second: 407, Actor: &workflowGameEventPlayer{PlayerID: 1}},
			{Type: "drop", Second: 380, Actor: &workflowGameEventPlayer{PlayerID: 2}},
		},
	}
	markCliffDrops(detail)
	got := []string{detail.GameEvents[0].Type, detail.GameEvents[1].Type, detail.GameEvents[2].Type}
	want := []string{"cliff_drop", "drop", "drop"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event types %v, want %v", got, want)
		}
	}
}

func TestADropshipDropOnABigGameHuntersCornerIsACliffDrop(t *testing.T) {
	rec := &library.Replay{Map: library.Strings.Intern("Big Game Hunters"), MapWidth: 128, MapHeight: 128}
	dropship := uint16(bwgame.UnitTypesTerranDropship)
	if got := momentEventType(moments.Event{Type: "drop", X: 88, Y: 16, Units: []uint16{dropship}}, rec); got != "cliff_drop" {
		t.Fatalf("corner drop is %s", got)
	}
	if got := momentEventType(moments.Event{Type: "drop", X: 164, Y: 690, Units: []uint16{dropship}}, rec); got != "drop" {
		t.Fatalf("a drop off the cliff is %s", got)
	}
	if got := momentEventType(moments.Event{Type: "drop", X: 88, Y: 16, Units: []uint16{uint16(bwgame.UnitTypesProtossShuttle)}}, rec); got != "drop" {
		t.Fatalf("a Shuttle drop is %s", got)
	}
}

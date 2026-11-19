package moments

import (
	"testing"

	"github.com/marianogappa/openbw-go/bwgame"
)

func lurkerDrop() Event {
	return Event{Type: "drop", Second: 677, Actor: 0, Target: 1, Members: []uint32{11, 12, 13}, Outcome: Outcome{Workers: 8, KilledUnits: 11}, Region: Region{X: 0, Y: 0, W: 600, H: 600}}
}

func undetectedKills(killers ...uint32) Event {
	return Event{Type: InsightUndetectedCloak, Insight: true, Second: 682, Seconds: 20, Actor: 0, Target: 1, Members: killers, Result: CloakNotThere, Outcome: Outcome{Workers: 5, KilledUnits: 6}, Region: Region{X: 500, Y: 0, W: 600, H: 600}}
}

func TestKillsByDroppedLurkersFoldIntoTheDrop(t *testing.T) {
	events, why := Combine([]Event{lurkerDrop(), undetectedKills(12, 13)}, 1)
	if len(events) != 1 || why != 0 {
		t.Fatalf("events %+v why %d", events, why)
	}
	e := events[0]
	if e.Type != "drop" || len(e.Parts) != 1 || e.Parts[0].Type != InsightUndetectedCloak || e.Parts[0].Result != CloakNotThere {
		t.Fatalf("combined %+v", e)
	}
	if !e.Insight || e.Seconds != 25 || e.Outcome.Workers != 8 || e.Region.W != 1100 {
		t.Fatalf("the drop takes in the kills' span, frame and outcome: %+v", e)
	}
}

func TestKillsByOtherUnitsStayApart(t *testing.T) {
	if events, why := Combine([]Event{lurkerDrop(), undetectedKills(40, 41, 12)}, 1); len(events) != 2 || why != 1 {
		t.Fatalf("Lurkers that walked in are not the drop's: %+v", events)
	}
	late := undetectedKills(12, 13)
	late.Second = 677 + fightFollowUp + 1
	if events, _ := Combine([]Event{lurkerDrop(), late}, -1); len(events) != 2 {
		t.Fatal("kills long after the drop are a story of their own")
	}
	noIDs := undetectedKills()
	if events, _ := Combine([]Event{lurkerDrop(), noIDs}, -1); len(events) != 2 {
		t.Fatal("without unit IDs nothing is proven")
	}
}

func TestOnlyListedPairsCombine(t *testing.T) {
	eraser := Event{Type: InsightEraser, Insight: true, Second: 680, Actor: 0, Target: 1, Members: []uint32{12}}
	if events, _ := Combine([]Event{lurkerDrop(), eraser}, -1); len(events) != 2 {
		t.Fatal("an Eraser does not fold into a drop")
	}
}

func TestContainLurkersKillingUndetectedFoldIntoTheContain(t *testing.T) {
	contain := Event{Type: InsightLurkerContain, Insight: true, Second: 500, Seconds: 90, Actor: 0, Target: 1, Members: []uint32{12, 13, 14}}
	kills := undetectedKills(13, 14)
	kills.Second = 560
	events, why := Combine([]Event{contain, kills}, 1)
	if len(events) != 1 || why != 0 || len(events[0].Parts) != 1 {
		t.Fatalf("events %+v why %d", events, why)
	}
}

func TestADropFoldsIntoItsElevatorBeforeItsKills(t *testing.T) {
	elevator := Event{Type: InsightElevator, Insight: true, Second: 670, Seconds: 30, Actor: 0, Target: 1, Members: []uint32{11, 12, 13, 14}}
	events, why := Combine([]Event{elevator, lurkerDrop(), undetectedKills(12, 13)}, 2)
	if len(events) != 1 || why != 0 || len(events[0].Parts) != 2 {
		t.Fatalf("events %+v why %d", events, why)
	}
}

func TestHitsOnOneBaseChainIntoOnePushButAnotherBaseStaysApart(t *testing.T) {
	hydras := []uint16{ut(bwgame.UnitTypesZergHydralisk)}
	events := []Event{
		{Type: "attack", Second: 461, Actor: 1, Target: 0, Base: 2, Units: hydras, Outcome: Outcome{Workers: 13, TownHall: true}},
		{Type: "worker_kills", Second: 528, Seconds: 20, Actor: 1, Target: 0, Base: 1, Units: hydras, Outcome: Outcome{Workers: 12, Killed: 1125}},
		{Type: "worker_kills", Second: 607, Seconds: 30, Actor: 1, Target: 0, Base: 1, Units: hydras, Outcome: Outcome{Workers: 22, Killed: 1925}},
		{Type: "worker_kills", Second: 900, Actor: 1, Target: 0, Base: 1, Units: hydras, Outcome: Outcome{Workers: 6}},
	}
	got := ChainPushes(events)
	if len(got) != 3 || got[1].Second != 528 || got[1].Outcome.Workers != 34 || got[1].Seconds != 109 || len(got[1].Parts) != 1 {
		t.Fatalf("the two hits on the main are one push, the natural and the much later hit their own: %+v", got)
	}
}

package moments

import (
	"slices"
	"testing"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

func lingsAtTheNexus(leaves int) *engine.Digest {
	d := tvzDigest()
	d.Initial[1].Type = ut(bwgame.UnitTypesProtossNexus)
	d.Army = []engine.ArmySnapshot{army(300, 0, bwgame.UnitTypesZergZergling, 12, 3650, 3650)}
	d.Outcomes = []engine.Outcome{{Frame: f(leaves), Owner: 1, Kind: engine.OutcomeLeft}, {Frame: f(leaves), Owner: 0, Kind: engine.OutcomeWon}}
	return d
}

func overrun(in Insights) (Event, bool) {
	i := slices.IndexFunc(in.Events, func(e Event) bool { return e.Type == InsightOverrun })
	if i < 0 {
		return Event{}, false
	}
	return in.Events[i], true
}

func TestZerglingsInAnUndefendedBaseAreAnOverrunThatEndsTheGame(t *testing.T) {
	in := DetectInsights(lingsAtTheNexus(330), testBases, tvz)
	e, ok := overrun(in)
	if !ok || e.Actor != 0 || e.Target != 1 || e.Count != 12 || e.Defenders != 0 {
		t.Fatalf("overrun %+v in %+v", e, in.Events)
	}
	if in.Why == nil || in.Events[in.Why.Event].Type != InsightOverrun || in.Why.Loser != 1 {
		t.Fatalf("the overrun is why the Protoss left: %+v", in.Why)
	}
}

func TestAnOverrunNeedsTheVictimToLeaveAndNothingToAnswerIt(t *testing.T) {
	if _, ok := overrun(DetectInsights(lingsAtTheNexus(600), testBases, tvz)); ok {
		t.Fatal("a Protoss who stays five more minutes was not overrun")
	}
	d := lingsAtTheNexus(330)
	d.Army[0].Units = append(d.Army[0].Units, army(300, 1, bwgame.UnitTypesProtossZealot, 3, 3700, 3600).Units...)
	if _, ok := overrun(DetectInsights(d, testBases, tvz)); ok {
		t.Fatal("three Zealots answer twelve Zerglings")
	}
	d = lingsAtTheNexus(330)
	d.Army[0].Units = append(d.Army[0].Units, army(300, 1, bwgame.UnitTypesProtossCorsair, 6, 3700, 3600).Units...)
	if _, ok := overrun(DetectInsights(d, testBases, tvz)); !ok {
		t.Fatal("Corsairs do not answer Zerglings")
	}
	d = lingsAtTheNexus(330)
	d.Outcomes = nil
	d.LastFrame = f(330)
	if _, ok := overrun(DetectInsights(d, testBases, tvz)); !ok {
		t.Fatal("the recorder's own replay, cut when they quit, still shows the overrun")
	}
}

func TestEachLosingTeamGetsItsOwnTurningPoint(t *testing.T) {
	d := digest()
	d.Players = []engine.Player{{Slot: 0}, {Slot: 1}, {Slot: 2}, {Slot: 3}}
	d.Outcomes = []engine.Outcome{
		{Frame: f(60), Owner: 1, Kind: engine.OutcomeLeft},
		{Frame: f(700), Owner: 3, Kind: engine.OutcomeLeft},
		{Frame: f(900), Owner: 2, Kind: engine.OutcomeLeft},
	}
	players := []Player{{Slot: 0, Human: true, Team: 1}, {Slot: 1, Human: true, Team: 1}, {Slot: 2, Human: true, Team: 2}, {Slot: 3, Human: true, Team: 2}}
	events := []Event{
		{Type: "attack", Second: 500, Actor: 0, Target: 1, Outcome: Outcome{Killed: 3000, TownHall: true}},
		{Type: "attack", Second: 600, Actor: 0, Target: 3, Outcome: Outcome{Killed: 2000, TownHall: true}},
		{Type: "attack", Second: 850, Actor: 0, Target: 2, Outcome: Outcome{Killed: 1600, Lost: 400}},
		{Type: "attack", Second: 860, Actor: 2, Target: 0, Outcome: Outcome{Killed: 4000, TownHall: true}},
	}
	whys := Whys(d, testBases, players, events)
	if len(whys) != 1 || whys[0].Event != 2 || whys[0].Loser != 2 {
		t.Fatalf("only the team that left lost, explained by the attack in its last four minutes: %+v", whys)
	}
}

func TestARecallThatThrewTheArmyAwayIsWhyTheRecallerLeft(t *testing.T) {
	d := digest()
	d.LastFrame = f(950)
	players := []Player{{Slot: 0, Human: true}, {Slot: 1, Human: true}}
	events := []Event{
		{Type: "dt_drop", Second: 506, Actor: 1, Target: 0, Outcome: Outcome{Killed: 150, Lost: 225}},
		{Type: "recall", Second: 846, Actor: 1, Target: 0, Result: RecallTradedPoorly, Outcome: Outcome{Killed: 250, Lost: 2100}},
	}
	whys := Whys(d, testBases, players, events)
	if len(whys) != 1 || whys[0].Event != 1 || whys[0].Loser != 1 {
		t.Fatalf("in the recalling player's own replay, the failed recall is why they left: %+v", whys)
	}
}

func TestTheLibrarysResultNamesTheLosingSide(t *testing.T) {
	d := digest()
	d.LastFrame = f(950)
	events := []Event{
		{Type: "attack", Second: 800, Actor: 1, Target: 0, Outcome: Outcome{Killed: 1800, Lost: 300, TownHall: true}},
		{Type: "recall", Second: 846, Actor: 1, Target: 0, Result: RecallTradedPoorly, Outcome: Outcome{Killed: 250, Lost: 2100}},
	}
	players := []Player{{Slot: 0, Human: true, Result: ResultWon}, {Slot: 1, Human: true, Result: ResultLost}}
	whys := Whys(d, testBases, players, events)
	if len(whys) != 1 || whys[0].Event != 1 || whys[0].Loser != 1 {
		t.Fatalf("the side the library says lost is the one explained, even when the other took a hit: %+v", whys)
	}
}

func TestALoneHarasserOverrunsOnlyAPlayerWithNoArmyAnywhere(t *testing.T) {
	d := tvzDigest()
	d.Initial[1].Type = ut(bwgame.UnitTypesProtossNexus)
	d.Army = []engine.ArmySnapshot{army(300, 0, bwgame.UnitTypesZergHydralisk, 1, 3650, 3650)}
	d.Outcomes = []engine.Outcome{{Frame: f(330), Owner: 1, Kind: engine.OutcomeLeft}, {Frame: f(330), Owner: 0, Kind: engine.OutcomeWon}}
	if _, ok := overrun(DetectInsights(d, testBases, tvz)); !ok {
		t.Fatal("one Hydralisk in the base of a player with no army is an overrun")
	}
	d.Army[0].Units = append(d.Army[0].Units, army(300, 1, bwgame.UnitTypesProtossDragoon, 4, 1500, 1500).Units...)
	if _, ok := overrun(DetectInsights(d, testBases, tvz)); ok {
		t.Fatal("a player with four Dragoons across the map was not unprepared")
	}
}

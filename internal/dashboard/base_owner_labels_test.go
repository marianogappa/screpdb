package dashboard

import "testing"

func TestBasesAreNamedByTheLayoutClockEachPlayerStartedOn(t *testing.T) {
	you := &workflowGameEventPlayer{PlayerID: 1, Name: "MarineTax"}
	them := &workflowGameEventPlayer{PlayerID: 2, Name: "43ewrgf"}
	clock := func(v int64) *int64 { return &v }
	events := []workflowGameEvent{
		{Type: "player_start", Actor: them, Base: &workflowGameEventBase{Name: "start 1", Kind: "start", Clock: 1}},
		{Type: "player_start", Actor: you, Base: &workflowGameEventBase{Name: "start 3", Kind: "start", Clock: 3}},
		{Type: "attack", Actor: you, Target: them, ActorStartClock: clock(1), TargetStartClock: clock(12), Base: &workflowGameEventBase{Name: "start 1", Kind: "start", Clock: 1}},
	}
	players := []workflowGamePlayer{{PlayerID: 1, Name: "MarineTax"}, {PlayerID: 2, Name: "43ewrgf"}}
	got := resolveNaturalOwnerLabels(events, players, map[int64]int{1: 1, 2: 12})
	attack := got[2]
	if attack.Base.Name != "43ewrgf's main base" || *attack.TargetStartClock != 1 || *attack.ActorStartClock != 3 {
		t.Fatalf("the attack is at the target's main, as the layout names it: %q actor %d target %d", attack.Base.Name, *attack.ActorStartClock, *attack.TargetStartClock)
	}
}

func TestABaseIsNamedAfterItsStarterOnlyWhileTheyPlay(t *testing.T) {
	gone := &workflowGameEventPlayer{PlayerID: 1, Name: "chobo86"}
	taker := &workflowGameEventPlayer{PlayerID: 2, Name: "TZPNEWB"}
	three := int64(3)
	natural := func() *workflowGameEventBase {
		return &workflowGameEventBase{Name: "natural of 3", Kind: "natural", Clock: 3, NaturalOfClock: &three}
	}
	left := int64(500)
	events := []workflowGameEvent{
		{Type: "player_start", Second: 0, Actor: gone, Base: &workflowGameEventBase{Name: "start 3", Kind: "start", Clock: 3}},
		{Type: "player_start", Second: 0, Actor: taker, Base: &workflowGameEventBase{Name: "start 1", Kind: "start", Clock: 1}},
		{Type: "attack", Second: 400, Actor: taker, Target: gone, Base: natural()},
		{Type: "attack", Second: 600, Actor: taker, Target: gone, Base: &workflowGameEventBase{Name: "start 3", Kind: "start", Clock: 3}},
		{Type: "expansion", Second: 700, Actor: taker, Base: natural()},
		{Type: "leave_game", Second: left, Actor: gone},
		{Type: "recall", Second: 800, Actor: gone, Target: taker, TargetBase: natural()},
	}
	players := []workflowGamePlayer{{PlayerID: 1, Name: "chobo86"}, {PlayerID: 2, Name: "TZPNEWB"}}
	got := resolveNaturalOwnerLabels(events, players, map[int64]int{1: 3, 2: 1})
	want := []string{"chobo86's natural", "3's main base", "3's natural", "TZPNEWB's expa at 3's natural"}
	for i, name := range []string{got[2].Base.Name, got[3].Base.Name, got[4].Base.Name, got[6].TargetBase.Name} {
		if name != want[i] {
			t.Errorf("name %d: got %q, want %q", i, name, want[i])
		}
	}
}

func TestABaseTakenFromAStarterWhoStillPlaysIsNamedAfterItsTaker(t *testing.T) {
	starter := &workflowGameEventPlayer{PlayerID: 1, Name: "chobo86"}
	taker := &workflowGameEventPlayer{PlayerID: 2, Name: "TZPNEWB"}
	main := func() *workflowGameEventBase { return &workflowGameEventBase{Name: "start 3", Kind: "start", Clock: 3} }
	events := []workflowGameEvent{
		{Type: "player_start", Actor: starter, Base: main()},
		{Type: "player_start", Actor: taker, Base: &workflowGameEventBase{Name: "start 1", Kind: "start", Clock: 1}},
		{Type: "expansion", Second: 373, Actor: taker, Base: main()},
		{Type: "recall", Second: 837, Actor: starter, Target: taker, TargetBase: main()},
	}
	players := []workflowGamePlayer{{PlayerID: 1, Name: "chobo86"}, {PlayerID: 2, Name: "TZPNEWB"}}
	got := resolveNaturalOwnerLabels(events, players, map[int64]int{1: 3, 2: 1})
	if got[2].Base.Name != "chobo86's main base" || got[3].TargetBase.Name != "TZPNEWB's expa at 3's main base" {
		t.Fatalf("taking the main: %q, then held: %q", got[2].Base.Name, got[3].TargetBase.Name)
	}
}

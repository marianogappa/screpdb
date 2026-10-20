package dashboard

import "testing"

func TestCurateWeighsOutcomeAndSteadiesRegions(t *testing.T) {
	actor := &workflowGameEventPlayer{PlayerID: 1}
	detail := workflowGameDetail{DurationSeconds: 1800, MapWidthPixels: 4096, MapHeightPixels: 4096, Players: []workflowGamePlayer{{PlayerID: 1}, {PlayerID: 2}}}
	for i := range 40 {
		detail.GameEvents = append(detail.GameEvents, workflowGameEvent{Type: "attack", Second: int64(40 * i), Actor: actor, Outcome: &workflowGameEventOutcome{Killed: 50}})
	}
	heavy := len(detail.GameEvents) - 1
	detail.GameEvents[heavy].Outcome = &workflowGameEventOutcome{Killed: 2500, Lost: 1000, Workers: 12, TownHall: true, First: true}
	detail.GameEvents[heavy].Engine = &workflowEngineEvent{Region: &workflowEngineRegion{X: 1000, Y: 1000, W: 600, H: 900}}

	curateGameEvents(&detail)

	if !detail.GameEvents[heavy].Key {
		t.Fatal("a town-hall kill with a big trade is not key")
	}
	if got := *detail.GameEvents[heavy].Engine.Region; got != (workflowEngineRegion{X: 700, Y: 1000, W: 1200, H: 900}) {
		t.Fatalf("region = %+v", got)
	}
	key := 0
	for _, e := range detail.GameEvents {
		if e.Key {
			key++
		}
	}
	if key == len(detail.GameEvents) {
		t.Fatal("every small attack is key")
	}
}

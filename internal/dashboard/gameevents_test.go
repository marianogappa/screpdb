package dashboard

import "testing"

func TestSteadyRegionKeepsOneShapeInsideTheMap(t *testing.T) {
	cases := []struct {
		name string
		in   workflowEngineRegion
		want workflowEngineRegion
	}{
		{"tall widens", workflowEngineRegion{X: 1000, Y: 1000, W: 2400, H: 3000}, workflowEngineRegion{X: 96, Y: 1000, W: 4000, H: 3000}},
		{"wide grows taller", workflowEngineRegion{X: 1000, Y: 1000, W: 3000, H: 750}, workflowEngineRegion{X: 1000, Y: 250, W: 3000, H: 2250}},
		{"small grows to the minimum around its centre", workflowEngineRegion{X: 2000, Y: 2000, W: 600, H: 900}, workflowEngineRegion{X: 1276, Y: 1682, W: 2048, H: 1536}},
		{"slides off the corner", workflowEngineRegion{X: -200, Y: -100, W: 800, H: 600}, workflowEngineRegion{X: 0, Y: 0, W: 2048, H: 1536}},
		{"shrinks to the map", workflowEngineRegion{X: 0, Y: 0, W: 8000, H: 6000}, workflowEngineRegion{X: 0, Y: 1024, W: 4096, H: 3072}},
	}
	for _, c := range cases {
		got := *steadyRegion(c.in, 4096, 4096)
		if got != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}

func TestCurateMarksKeyEventsAndKeepsMandatoryOnes(t *testing.T) {
	actor := &workflowGameEventPlayer{PlayerID: 1}
	detail := workflowGameDetail{DurationSeconds: 600, Players: []workflowGamePlayer{{PlayerID: 1}, {PlayerID: 2}}}
	detail.GameEvents = append(detail.GameEvents, workflowGameEvent{Type: "bo_openers"})
	for i := range 30 {
		detail.GameEvents = append(detail.GameEvents, workflowGameEvent{Type: "expansion", Second: int64(60 + 20*i), Actor: actor})
	}
	detail.GameEvents = append(detail.GameEvents, workflowGameEvent{Type: "leave_game", Second: 590, Actor: actor})
	curateGameEvents(&detail)
	key := 0
	for _, e := range detail.GameEvents {
		if e.Key {
			key++
		}
	}
	if !detail.GameEvents[0].Key || !detail.GameEvents[len(detail.GameEvents)-1].Key {
		t.Fatal("openers and leaves are always key")
	}
	if key >= len(detail.GameEvents)/2 {
		t.Fatalf("%d of %d events are key", key, len(detail.GameEvents))
	}
}

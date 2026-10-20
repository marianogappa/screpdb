package dashboard

import (
	"reflect"
	"testing"
)

func TestApplyAlliancePayload(t *testing.T) {
	var e workflowGameEvent
	applyAlliancePayload(&e, `not json`)
	if e.AllianceTeams != nil {
		t.Fatalf("bad json set teams %+v", e.AllianceTeams)
	}
	applyAlliancePayload(&e, `{"teams":[["solo"],["a"," "]]}`)
	if e.AllianceTeams != nil {
		t.Fatalf("teams under two players kept: %+v", e.AllianceTeams)
	}
	applyAlliancePayload(&e, `{"teams":[[" alice ","bob",""],["solo"],["carol","dave"]]}`)
	want := [][]workflowGameEventPlayer{{{Name: "alice"}, {Name: "bob"}}, {{Name: "carol"}, {Name: "dave"}}}
	if !reflect.DeepEqual(e.AllianceTeams, want) {
		t.Fatalf("teams = %+v", e.AllianceTeams)
	}
}

func TestResolveAllianceTeamPlayersByRawAndDisplayName(t *testing.T) {
	if got := resolveAllianceTeamPlayers(nil, nil, nil); got != nil {
		t.Fatalf("nil events = %+v", got)
	}
	players := []workflowGamePlayer{{PlayerID: 1, Name: "chobo86 (you)", Color: "red"}, {PlayerID: 2, Name: "bob", Color: "blue"}}
	events := []workflowGameEvent{
		{Type: "late_alliance", AllianceTeams: [][]workflowGameEventPlayer{{{Name: "chobo86"}, {Name: "bob"}, {Name: "ghost"}}}},
		{Type: "expansion", AllianceTeams: [][]workflowGameEventPlayer{{{Name: "bob"}}}},
		{Type: "late_alliance"},
	}
	got := resolveAllianceTeamPlayers(events, players, map[string]string{"chobo86": "chobo86 (you)", "nobody": "missing"})
	team := got[0].AllianceTeams[0]
	if team[0] != (workflowGameEventPlayer{PlayerID: 1, Name: "chobo86", Color: "red"}) {
		t.Errorf("raw name not resolved: %+v", team[0])
	}
	if team[1] != (workflowGameEventPlayer{PlayerID: 2, Name: "bob", Color: "blue"}) {
		t.Errorf("display name not resolved: %+v", team[1])
	}
	if team[2] != (workflowGameEventPlayer{Name: "ghost"}) {
		t.Errorf("unknown name touched: %+v", team[2])
	}
	if got[1].AllianceTeams[0][0].PlayerID != 0 {
		t.Error("non-alliance event was resolved")
	}
}

func TestLookupOverlayBaseByClockPrefersStartingBase(t *testing.T) {
	metas := []overlayBaseMeta{
		{Base: workflowGameEventBase{Name: "12 expa", Clock: 12}},
		{Base: workflowGameEventBase{Name: "12 main", Clock: 12}, IsStarting: true},
		{Base: workflowGameEventBase{Name: "3 expa", Clock: 3}},
	}
	if b, ok := lookupOverlayBaseByClock(metas, 12); !ok || b.Name != "12 main" {
		t.Errorf("clock 12 = %q %v", b.Name, ok)
	}
	if b, ok := lookupOverlayBaseByClock(metas, 3); !ok || b.Name != "3 expa" {
		t.Errorf("clock 3 = %q %v", b.Name, ok)
	}
	if b, ok := lookupOverlayBaseByClock(metas, 6); ok || b.Name != "" {
		t.Errorf("clock 6 = %+v %v", b, ok)
	}
}

func TestFallbackActorOriginFromOwnership(t *testing.T) {
	bases := map[string]workflowGameEventBase{
		"expansion|3": {Name: "3 expa", Kind: "expansion"},
		"start|12":    {Name: "12 main", Kind: "Start"},
		"start|6":     {Name: "6 main", Kind: "starting"},
	}
	owner := func(id int64) *workflowGameEventPlayer { return &workflowGameEventPlayer{PlayerID: id} }

	ownership := map[string]*workflowGameEventPlayer{"expansion|3": owner(1), "start|12": owner(1), "start|6": owner(2), "gone|1": owner(1), "nil": nil}
	if b, ok := fallbackActorOriginFromOwnership(1, ownership, bases); !ok || b.Name != "12 main" {
		t.Errorf("player 1 = %q %v, want the main", b.Name, ok)
	}
	if b, ok := fallbackActorOriginFromOwnership(2, ownership, bases); !ok || b.Name != "6 main" {
		t.Errorf("player 2 = %q %v", b.Name, ok)
	}

	onlyExpa := map[string]*workflowGameEventPlayer{"expansion|3": owner(1), "gone|1": owner(1), "nil": nil}
	if b, ok := fallbackActorOriginFromOwnership(1, onlyExpa, bases); !ok || b.Name != "3 expa" {
		t.Errorf("expansion fallback = %q %v", b.Name, ok)
	}
	if _, ok := fallbackActorOriginFromOwnership(3, ownership, bases); ok {
		t.Error("player owning nothing got an origin")
	}
}

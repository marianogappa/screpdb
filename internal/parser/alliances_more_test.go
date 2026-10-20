package parser

import (
	"reflect"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func observer(pid byte, slot uint16) *models.Player {
	return &models.Player{PlayerID: pid, SlotID: slot, Type: "Human", IsObserver: true}
}

func aiPlayer(pid byte, slot uint16) *models.Player {
	return &models.Player{PlayerID: pid, SlotID: slot, Type: "Computer"}
}

func teamPlayer(pid, team byte) *models.Player {
	pl := p(pid, uint16(pid))
	pl.Team = team
	return pl
}

func TestComputeActivitySkipsIrrelevantInputs(t *testing.T) {
	p1, p2 := p(1, 0), p(2, 1)
	obs, ai := observer(9, 5), aiPlayer(8, 6)
	players := []*models.Player{nil, obs, ai, p1, p2}
	commands := []*models.Command{nil, {ActionType: "Right Click"}, genericCmd(10, obs), genericCmd(10, ai), leaveCmd(-3, p1)}
	got := ComputeActivity(players, commands, 1000)
	want := Activity{StoppedSecByPID: map[byte]int{2: 0}, LeaveSecByPID: map[byte]int{1: 0}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAnalyzeAlliancesWithNoActivePlayers(t *testing.T) {
	activity := Activity{StoppedSecByPID: map[byte]int{5: 10}}
	got := AnalyzeAlliances([]*models.Player{nil, observer(1, 0), aiPlayer(2, 1)}, nil, 600, activity)
	if got.Snapshots != nil || len(got.ResolvedTeams) != 0 || got.ResolvedTeams == nil {
		t.Fatalf("got %+v", got)
	}
	if !reflect.DeepEqual(got.StoppedSecByPID, activity.StoppedSecByPID) {
		t.Fatalf("stopped %v", got.StoppedSecByPID)
	}
	got.StoppedSecByPID[5] = 99
	if activity.StoppedSecByPID[5] != 10 {
		t.Fatal("result aliases the activity map")
	}
}

func TestAnalyzeAlliancesIgnoresUnqualifiedIssuersAndSlots(t *testing.T) {
	p1, p2 := p(1, 0), p(2, 1)
	obs, ai := observer(9, 5), aiPlayer(8, 6)
	commands := []*models.Command{
		nil,
		{ActionType: "Alliance", SecondsFromGameStart: 5, Player: p1},
		allianceCmd(6, nil, 0, 1),
		allianceCmd(7, obs, 0, 1, 5),
		allianceCmd(8, ai, 0, 1, 6),
		allianceCmd(10, p1, 0, 1, 7, 5, 6),
		allianceCmd(12, p2, 0, 1),
	}
	got := AnalyzeAlliances([]*models.Player{p1, p2, obs, ai}, commands, 600, emptyActivity())
	if len(got.Snapshots) != 2 || got.Snapshots[1].Sec != 12 {
		t.Fatalf("snapshots %+v", got.Snapshots)
	}
	expectTeams(t, got.Snapshots[1].Teams, [][]byte{{1, 2}})
	if !got.AnyMutualResolved || got.ResolvedTeams[1] != 1 || got.ResolvedTeams[2] != 1 {
		t.Fatalf("resolved %v mutual %v", got.ResolvedTeams, got.AnyMutualResolved)
	}
}

func TestInjectActivitySnapshotsEdges(t *testing.T) {
	if got := injectActivitySnapshots(nil, emptyActivity()); got != nil {
		t.Fatalf("empty input grew: %v", got)
	}
	teams := [][]byte{{1, 2, 3}, {4, 5}}
	snaps := []AllianceSnapshot{
		{Sec: 100, Teams: teams, Stacking: true},
		{Sec: 200, Teams: teams, Stacking: true},
		{Sec: 500, Teams: teams, Stacking: true},
	}
	activity := Activity{
		LeaveSecByPID:   map[byte]int{6: 50, 3: 300, 4: 0},
		StoppedSecByPID: map[byte]int{7: 200},
	}
	got := injectActivitySnapshots(snaps, activity)
	want := []AllianceSnapshot{snaps[0], snaps[1], {Sec: 300, Teams: teams, Stacking: false}, snaps[2]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	got[2].Teams[0][0] = 42
	if teams[0][0] != 1 {
		t.Fatal("injected snapshot aliases the prior topology")
	}
}

func TestMutualAllianceTeamsFallsBackToComponentsPastSixteen(t *testing.T) {
	var pids []byte
	allies := map[byte]map[byte]bool{}
	for pid := byte(17); pid >= 1; pid-- {
		pids = append(pids, pid)
		allies[pid] = map[byte]bool{pid: true}
	}
	link := func(a, b byte) { allies[a][b], allies[b][a] = true, true }
	link(1, 2)
	link(2, 3)
	link(1, 3)
	link(10, 11)
	link(12, 13)
	link(13, 14)
	allies[16][17] = true
	delete(allies, 15)

	got := mutualAllianceTeams(allies, pids)
	want := [][]byte{{1, 2, 3}, {4}, {5}, {6}, {7}, {8}, {9}, {10, 11}, {12, 13, 14}, {15}, {16}, {17}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestDominantResolvedTeamsWithoutSnapshots(t *testing.T) {
	got := dominantResolvedTeams(nil, 600, []byte{4, 7, 9})
	if !reflect.DeepEqual(got, map[byte]byte{4: 1, 7: 2, 9: 3}) {
		t.Fatalf("got %v", got)
	}
}

func TestSoleSurvivingCoalitionEdges(t *testing.T) {
	p1, p2, p3 := p(1, 0), p(2, 1), p(3, 2)
	obs, ai := observer(9, 5), aiPlayer(8, 6)

	team, ok, reason := soleSurvivingCoalition([]*models.Player{p1, p2}, nil, map[byte]byte{1: 1, 2: 1}, nil, true)
	if !ok || team != 1 || !strings.Contains(reason, "allied") {
		t.Fatalf("allied coalition: %d %v %q", team, ok, reason)
	}

	noisy := []*models.Command{nil, {ActionType: "Leave Game"}, leaveCmd(50, obs), genericCmd(60, p1)}
	players := []*models.Player{nil, obs, ai, p1, p2, p3}
	team, ok, _ = soleSurvivingCoalition(players, noisy, map[byte]byte{1: 1, 2: 2, 9: 3, 8: 4}, nil, false)
	if ok || team != 0 {
		t.Fatalf("two survivors must decline: %d %v", team, ok)
	}

	leaves := []*models.Command{nil, leaveCmd(100, p1), leaveCmd(200, p3), {ActionType: "Leave Game"}, leaveCmd(150, p2)}
	team, ok, reason = soleSurvivingCoalition([]*models.Player{p1, p2, p3}, leaves, map[byte]byte{1: 1, 2: 2}, nil, false)
	if !ok || team != 2 || !strings.Contains(reason, "2:30") {
		t.Fatalf("last attributable leave: %d %v %q", team, ok, reason)
	}
}

func TestDeriveWinnersFromLeavesSkipsNilAndObservers(t *testing.T) {
	p1, p2 := teamPlayer(1, 1), teamPlayer(2, 2)
	obs := observer(9, 5)
	obs.TeamOutcome = models.OutcomeWon
	players := []*models.Player{nil, obs, p1, p2}
	DeriveWinnersFromLeaves(players, []*models.Command{leaveCmd(100, p2)}, nil)
	if p1.TeamOutcome != models.OutcomeWon || p2.TeamOutcome != models.OutcomeLost || obs.TeamOutcome != models.OutcomeUnknown {
		t.Fatalf("outcomes p1 %v p2 %v obs %v", p1.TeamOutcome, p2.TeamOutcome, obs.TeamOutcome)
	}
}

func TestDeriveWinnersFromFinalTopologyCoalitions(t *testing.T) {
	p1, p2, p3, p4 := p(1, 0), p(2, 1), p(3, 2), p(4, 3)
	obs := observer(9, 5)
	obs.TeamOutcome = models.OutcomeDisconnected
	ar := AllianceResult{Snapshots: []AllianceSnapshot{{Sec: 0}, {Sec: 300, Teams: [][]byte{{}, {2, 1}, {3}}}}}
	players := []*models.Player{nil, obs, p1, p2, p3, p4}
	DeriveWinnersFromFinalTopology(players, []*models.Command{leaveCmd(100, p3), leaveCmd(110, p4)}, ar, nil)
	want := map[*models.Player]models.Outcome{p1: models.OutcomeWon, p2: models.OutcomeWon, p3: models.OutcomeLost, p4: models.OutcomeLost, obs: models.OutcomeDisconnected}
	for pl, o := range want {
		if pl.TeamOutcome != o {
			t.Errorf("pid %d: %v, want %v", pl.PlayerID, pl.TeamOutcome, o)
		}
	}

	q1, q2 := p(1, 0), p(2, 1)
	q1.TeamOutcome = models.OutcomeLost
	reason := DeriveWinnersFromFinalTopology([]*models.Player{q1, q2}, nil, AllianceResult{}, nil)
	if q1.TeamOutcome != models.OutcomeLost || q2.TeamOutcome != models.OutcomeUnknown || !strings.HasPrefix(reason, "no winner") {
		t.Fatalf("undecided game rewrote outcomes: %v %v %q", q1.TeamOutcome, q2.TeamOutcome, reason)
	}
}

func TestAssignCoalitionsKeysByMinPID(t *testing.T) {
	got := assignCoalitions([]*models.Player{nil, observer(9, 5), p(5, 4), p(3, 2)}, [][]byte{{4, 3, 2}, {}, {3, 6}})
	want := map[byte]byte{2: 2, 3: 2, 4: 2, 6: 3, 5: 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCloneHelpersPreserveNil(t *testing.T) {
	if cloneTeams(nil) != nil || copyByteIntMap(nil) != nil {
		t.Fatal("nil inputs cloned to non-nil")
	}
}

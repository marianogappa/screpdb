package parser

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func TestPreviousSnapshotTeams(t *testing.T) {
	snaps := []AllianceSnapshot{
		{Sec: 0, Teams: [][]byte{{1}, {2}, {3}}},
		{Sec: 100, Teams: [][]byte{{1, 2}, {3}}},
		{Sec: 400, Teams: [][]byte{{1, 2, 3}}},
	}
	cases := []struct {
		name string
		sec  int
		want [][]byte
	}{
		{"before_first_snapshot", 0, nil},
		{"between_first_and_second", 100, [][]byte{{1}, {2}, {3}}},
		{"exactly_at_later_snapshot", 400, [][]byte{{1, 2}, {3}}},
		{"after_all_snapshots", 9999, [][]byte{{1, 2, 3}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := previousSnapshotTeams(snaps, c.sec)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestPairsOf(t *testing.T) {
	cases := []struct {
		name  string
		teams [][]byte
		want  [][2]byte
	}{
		{"solos_have_no_pairs", [][]byte{{1}, {2}}, nil},
		{"single_pair", [][]byte{{2, 1}}, [][2]byte{{1, 2}}},
		{"triangle_yields_three_pairs", [][]byte{{1, 2, 3}}, [][2]byte{{1, 2}, {1, 3}, {2, 3}}},
		{"two_teams", [][]byte{{3, 4}, {1, 2}}, [][2]byte{{1, 2}, {3, 4}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set := pairsOf(c.teams)
			got := make([][2]byte, 0, len(set))
			for p := range set {
				got = append(got, p)
			}
			sort.Slice(got, func(i, j int) bool {
				if got[i][0] != got[j][0] {
					return got[i][0] < got[j][0]
				}
				return got[i][1] < got[j][1]
			})
			if len(c.want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestPairsAdded(t *testing.T) {
	cases := []struct {
		name string
		prev [][]byte
		curr [][]byte
		want [][2]byte
	}{
		{
			name: "new_pair_from_all_solos",
			prev: [][]byte{{1}, {2}, {3}},
			curr: [][]byte{{1, 2}, {3}},
			want: [][2]byte{{1, 2}},
		},
		{
			name: "nil_prev_treated_as_solos",
			prev: nil,
			curr: [][]byte{{1, 2}},
			want: [][2]byte{{1, 2}},
		},
		{
			name: "reaffirmed_pairs_not_reported",
			prev: [][]byte{{1, 2}},
			curr: [][]byte{{1, 2}, {3, 4}},
			want: [][2]byte{{3, 4}},
		},
		{
			name: "no_new_pairs",
			prev: [][]byte{{1, 2}, {3, 4}},
			curr: [][]byte{{1, 2}, {3, 4}},
			want: nil,
		},
		{
			name: "sorted_by_min_pid",
			prev: [][]byte{{1}, {2}, {3}, {4}},
			curr: [][]byte{{3, 4}, {1, 2}},
			want: [][2]byte{{1, 2}, {3, 4}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pairsAdded(c.prev, c.curr)
			if len(c.want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestPairsToTeams(t *testing.T) {
	pairs := [][2]byte{{1, 2}, {3, 4}}
	got := pairsToTeams(pairs)
	want := [][]byte{{1, 2}, {3, 4}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if got := pairsToTeams(nil); len(got) != 0 {
		t.Fatalf("expected empty teams for nil pairs, got %v", got)
	}
}

func TestPickAllianceIssuerAndAlly(t *testing.T) {
	cases := []struct {
		name       string
		teams      [][]byte
		wantIssuer byte
		wantAlly   byte
	}{
		{"all_solos_no_pick", [][]byte{{1}, {2}}, 0, 0},
		{"single_pair", [][]byte{{1, 2}}, 1, 2},
		{"largest_team_wins", [][]byte{{5, 6}, {1, 2, 3}}, 1, 2},
		{"tie_broken_by_smallest_min_pid_first_seen", [][]byte{{2, 3}, {4, 5}}, 2, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issuer, ally := pickAllianceIssuerAndAlly(c.teams)
			if issuer != c.wantIssuer || ally != c.wantAlly {
				t.Fatalf("got (%d,%d) want (%d,%d)", issuer, ally, c.wantIssuer, c.wantAlly)
			}
		})
	}
}

func TestMarshalAllianceTopology(t *testing.T) {
	pidToName := map[byte]string{1: "Alice", 2: "Bob"}

	t.Run("empty_teams_returns_not_ok", func(t *testing.T) {
		if _, ok := marshalAllianceTopology(nil, pidToName); ok {
			t.Fatalf("expected ok=false for empty teams")
		}
	})

	t.Run("named_topology", func(t *testing.T) {
		payload, ok := marshalAllianceTopology([][]byte{{1, 2}}, pidToName)
		if !ok {
			t.Fatalf("expected ok=true")
		}
		var decoded struct {
			Teams [][]string `json:"teams"`
		}
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		want := [][]string{{"Alice", "Bob"}}
		if !reflect.DeepEqual(decoded.Teams, want) {
			t.Fatalf("got %v want %v", decoded.Teams, want)
		}
	})

	t.Run("unknown_pid_falls_back_to_player_label", func(t *testing.T) {
		payload, ok := marshalAllianceTopology([][]byte{{1, 9}}, pidToName)
		if !ok {
			t.Fatalf("expected ok=true")
		}
		var decoded struct {
			Teams [][]string `json:"teams"`
		}
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		want := [][]string{{"Alice", "Player 9"}}
		if !reflect.DeepEqual(decoded.Teams, want) {
			t.Fatalf("got %v want %v", decoded.Teams, want)
		}
	})
}

func TestMarshalStackingPayload(t *testing.T) {
	pidToName := map[byte]string{1: "A", 2: "B", 3: "C", 4: "D", 5: "E"}

	t.Run("empty_returns_not_ok", func(t *testing.T) {
		if _, ok := marshalStackingPayload(nil, pidToName); ok {
			t.Fatalf("expected ok=false for empty teams")
		}
	})

	t.Run("sizes_sorted_desc_and_joined", func(t *testing.T) {
		payload, ok := marshalStackingPayload([][]byte{{1, 2}, {3, 4, 5}}, pidToName)
		if !ok {
			t.Fatalf("expected ok=true")
		}
		var decoded struct {
			TeamSizes    string     `json:"team_sizes"`
			Teams        [][]string `json:"teams"`
			ThresholdSec int        `json:"threshold_sec"`
		}
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if decoded.TeamSizes != "3v2" {
			t.Fatalf("team_sizes got %q want %q", decoded.TeamSizes, "3v2")
		}
		if decoded.ThresholdSec != StackingThresholdSec {
			t.Fatalf("threshold_sec got %d want %d", decoded.ThresholdSec, StackingThresholdSec)
		}
		want := [][]string{{"A", "B"}, {"C", "D", "E"}}
		if !reflect.DeepEqual(decoded.Teams, want) {
			t.Fatalf("teams got %v want %v", decoded.Teams, want)
		}
	})
}

func TestBuildAllianceDerivedEvents_StopAndStacking(t *testing.T) {
	players := []*models.Player{p(1, 1), p(2, 2), p(3, 3), p(4, 4), p(5, 5)}
	ar := AllianceResult{
		StoppedSecByPID:      map[byte]int{3: 500, 1: 100},
		TeamStackingFlag:     true,
		StackingBandStartSec: 10,
		StackingBandTeams:    [][]byte{{1, 2}, {3, 4, 5}},
	}
	events := BuildAllianceDerivedEvents(players, ar)

	var stops []worldstateEventShape
	var stacking *worldstateEventShape
	for i := range events {
		e := events[i]
		switch e.EventType {
		case "player_stopped_playing":
			stops = append(stops, worldstateEventShape{sec: e.Second, pid: *e.SourceReplayPlayerID})
		case "team_stacking_detected":
			s := worldstateEventShape{sec: e.Second}
			stacking = &s
		}
	}
	if len(stops) != 2 {
		t.Fatalf("expected 2 stop events, got %d", len(stops))
	}
	if stops[0].sec != 100 || stops[0].pid != 1 {
		t.Fatalf("expected first stop (sorted by sec) pid=1 sec=100, got %+v", stops[0])
	}
	if stops[1].sec != 500 || stops[1].pid != 3 {
		t.Fatalf("expected second stop pid=3 sec=500, got %+v", stops[1])
	}
	if stacking == nil {
		t.Fatalf("expected a team_stacking_detected event")
	}
	if stacking.sec != 10 {
		t.Fatalf("stacking event sec got %d want 10", stacking.sec)
	}
}

func TestBuildAllianceDerivedEvents_LateAllianceOnlyNewPairs(t *testing.T) {
	players := []*models.Player{p(1, 1), p(2, 2), p(3, 3), p(4, 4)}
	ar := AllianceResult{
		StoppedSecByPID: map[byte]int{},
		Snapshots: []AllianceSnapshot{
			{Sec: 0, Teams: [][]byte{{1}, {2}, {3}, {4}}},
			{Sec: 700, Teams: [][]byte{{1, 2}, {3}, {4}}},
			{Sec: 800, Teams: [][]byte{{1, 2}, {3, 4}}},
		},
		LateAllianceTransitions: []AllianceSnapshot{
			{Sec: 700, Teams: [][]byte{{1, 2}, {3}, {4}}},
			{Sec: 800, Teams: [][]byte{{1, 2}, {3, 4}}},
		},
	}
	events := BuildAllianceDerivedEvents(players, ar)

	var late []worldstate2 // sec, source, target
	for _, e := range events {
		if e.EventType != "late_alliance" {
			continue
		}
		late = append(late, worldstate2{sec: e.Second, src: *e.SourceReplayPlayerID, tgt: *e.TargetReplayPlayerID})
	}
	if len(late) != 2 {
		t.Fatalf("expected 2 late_alliance events, got %d", len(late))
	}
	if late[0].sec != 700 || late[0].src != 1 || late[0].tgt != 2 {
		t.Fatalf("first late event got %+v want sec=700 src=1 tgt=2", late[0])
	}
	if late[1].sec != 800 || late[1].src != 3 || late[1].tgt != 4 {
		t.Fatalf("second late event should carry ONLY the new {3,4} pair, got %+v", late[1])
	}
}

func TestBuildAllianceDerivedEvents_LateAllianceNoNewPairsSkipped(t *testing.T) {
	players := []*models.Player{p(1, 1), p(2, 2)}
	ar := AllianceResult{
		StoppedSecByPID: map[byte]int{},
		Snapshots: []AllianceSnapshot{
			{Sec: 100, Teams: [][]byte{{1, 2}}},
			{Sec: 700, Teams: [][]byte{{1, 2}}},
		},
		LateAllianceTransitions: []AllianceSnapshot{
			{Sec: 700, Teams: [][]byte{{1, 2}}},
		},
	}
	events := BuildAllianceDerivedEvents(players, ar)
	for _, e := range events {
		if e.EventType == "late_alliance" {
			t.Fatalf("expected no late_alliance event when the pair already existed in the same snapshot, got %+v", e)
		}
	}
}

type worldstateEventShape struct {
	sec int
	pid byte
}

type worldstate2 struct {
	sec int
	src byte
	tgt byte
}

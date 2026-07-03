package parser

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func TestRaceInitial(t *testing.T) {
	cases := []struct {
		race string
		want byte
	}{
		{"Protoss", 'P'},
		{"Terran", 'T'},
		{"Zerg", 'Z'},
		{"Random", 'R'},
		{"", '?'},
	}
	for _, c := range cases {
		if got := raceInitial(c.race); got != c.want {
			t.Fatalf("raceInitial(%q) = %q want %q", c.race, got, c.want)
		}
	}
}

func TestCountActiveMeleePlayers(t *testing.T) {
	players := []*models.Player{
		{PlayerID: 1, Type: "Human"},
		{PlayerID: 2, Type: "Human", IsObserver: true},
		{PlayerID: 3, Type: "Computer"},
		{PlayerID: 4, Type: "Human"},
		nil,
	}
	if got := countActiveMeleePlayers(players); got != 2 {
		t.Fatalf("countActiveMeleePlayers = %d want 2 (observer, computer and nil excluded)", got)
	}
	if got := countActiveMeleePlayers(nil); got != 0 {
		t.Fatalf("countActiveMeleePlayers(nil) = %d want 0", got)
	}
}

func TestAllActivePlayersHaveTeam(t *testing.T) {
	t.Run("all_have_team", func(t *testing.T) {
		players := []*models.Player{
			{PlayerID: 1, Type: "Human", Team: 1},
			{PlayerID: 2, Type: "Human", Team: 2},
		}
		if !allActivePlayersHaveTeam(players) {
			t.Fatalf("expected true when every active player has a non-zero team")
		}
	})

	t.Run("one_missing_team", func(t *testing.T) {
		players := []*models.Player{
			{PlayerID: 1, Type: "Human", Team: 1},
			{PlayerID: 2, Type: "Human", Team: 0},
		}
		if allActivePlayersHaveTeam(players) {
			t.Fatalf("expected false when an active player has team 0")
		}
	})

	t.Run("inactive_players_ignored", func(t *testing.T) {
		players := []*models.Player{
			{PlayerID: 1, Type: "Human", Team: 1},
			{PlayerID: 2, Type: "Human", Team: 0, IsObserver: true},
			{PlayerID: 3, Type: "Computer", Team: 0},
			nil,
		}
		if !allActivePlayersHaveTeam(players) {
			t.Fatalf("expected true — observers, computers and nil should be ignored")
		}
	})
}

func TestComputeTeamFormatAndMatchup(t *testing.T) {
	t.Run("no_players", func(t *testing.T) {
		format, matchup := computeTeamFormatAndMatchup(nil)
		if format != "" || matchup != "" {
			t.Fatalf("expected empty format/matchup, got %q/%q", format, matchup)
		}
	})

	t.Run("2v2_matchup", func(t *testing.T) {
		players := []*models.Player{
			{PlayerID: 1, Type: "Human", Team: 1, Race: "Terran"},
			{PlayerID: 2, Type: "Human", Team: 1, Race: "Protoss"},
			{PlayerID: 3, Type: "Human", Team: 2, Race: "Zerg"},
			{PlayerID: 4, Type: "Human", Team: 2, Race: "Zerg"},
		}
		format, matchup := computeTeamFormatAndMatchup(players)
		if format != "2v2" {
			t.Fatalf("format got %q want 2v2", format)
		}
		if matchup != "PTvZZ" {
			t.Fatalf("matchup got %q want PTvZZ", matchup)
		}
	})

	t.Run("observers_excluded", func(t *testing.T) {
		players := []*models.Player{
			{PlayerID: 1, Type: "Human", Team: 1, Race: "Terran"},
			{PlayerID: 2, Type: "Human", Team: 2, Race: "Zerg"},
			{PlayerID: 3, Type: "Human", Team: 3, Race: "Protoss", IsObserver: true},
		}
		format, matchup := computeTeamFormatAndMatchup(players)
		if format != "1v1" {
			t.Fatalf("format got %q want 1v1", format)
		}
		if matchup != "TvZ" {
			t.Fatalf("matchup got %q want TvZ", matchup)
		}
	})
}

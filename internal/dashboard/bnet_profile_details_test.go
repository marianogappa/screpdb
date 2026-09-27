package dashboard

import (
	"reflect"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library/persist"
)

func TestBnetRecentGamesSidesAndMatchup(t *testing.T) {
	profile := persist.BnetProfile{Toons: []persist.BnetToon{{Toon: "Me"}}}
	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	game := func(players ...persist.BnetGamePlayer) persist.BnetGame {
		return persist.BnetGame{GameID: "g", CreateTime: at, Players: players}
	}
	cases := []struct {
		name      string
		game      persist.BnetGame
		matchup   string
		opponents []string
	}{
		{
			name:      "1v1 without teams reads our race first",
			game:      game(persist.BnetGamePlayer{Toon: "Foe", Race: "zerg"}, persist.BnetGamePlayer{Toon: "Me", Race: "terran"}),
			matchup:   "TvZ",
			opponents: []string{"Foe"},
		},
		{
			name: "2v2 leaves the teammate out",
			game: game(
				persist.BnetGamePlayer{Toon: "Me", Race: "protoss", Team: 1},
				persist.BnetGamePlayer{Toon: "Ally", Race: "zerg", Team: 1},
				persist.BnetGamePlayer{Toon: "Foe1", Race: "terran", Team: 2},
				persist.BnetGamePlayer{Toon: "Foe2", Race: "terran", Team: 2},
			),
			matchup:   "2v2",
			opponents: []string{"Foe1", "Foe2"},
		},
		{
			name: "melee with more than two players has unknown sides",
			game: game(
				persist.BnetGamePlayer{Toon: "Me", Race: "protoss"},
				persist.BnetGamePlayer{Toon: "A", Race: "zerg"},
				persist.BnetGamePlayer{Toon: "B", Race: "terran"},
			),
			matchup: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := bnetRecentGamesFromArchive(profile, []persist.BnetGame{c.game})[0]
			if got.Matchup != c.matchup {
				t.Fatalf("matchup %q, want %q", got.Matchup, c.matchup)
			}
			var names []string
			for _, o := range got.Opponents {
				names = append(names, o.Toon)
			}
			if !reflect.DeepEqual(names, c.opponents) {
				t.Fatalf("opponents %v, want %v", names, c.opponents)
			}
			if wantCount := map[bool]int{true: len(c.game.Players)}[c.matchup == ""]; got.PlayerCount != wantCount {
				t.Fatalf("player count %d", got.PlayerCount)
			}
		})
	}
}

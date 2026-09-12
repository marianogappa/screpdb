package dashboard

import (
	"testing"
	"time"
)

func countsAt(base time.Time, rows ...coPlayerCount) []coPlayerCount {
	for i := range rows {
		if rows[i].LastPlayed.IsZero() {
			rows[i].LastPlayed = base
		}
	}
	return rows
}

func TestMergeCoPlayersByAccount(t *testing.T) {
	base := time.Date(2026, 9, 6, 21, 0, 0, 0, time.UTC)

	t.Run("two toons of one account are one person", func(t *testing.T) {
		rows := countsAt(base,
			coPlayerCount{PlayerKey: "lemner", PlayerName: "Lemner", Games: 9, LastPlayed: base},
			coPlayerCount{PlayerKey: "tanchik", PlayerName: "Tanchik", Games: 5, LastPlayed: base.Add(-time.Hour)},
		)
		index := regularsIdentityIndex{
			auroraByKey:  map[string]int64{"lemner": 42, "tanchik": 42},
			gatewayByKey: map[string]int64{"tanchik": 20},
		}

		merged := mergeCoPlayersByAccount(rows, index)

		if len(merged) != 1 {
			t.Fatalf("got %d identities, want 1", len(merged))
		}
		person := merged[0]
		if person.games != 14 {
			t.Errorf("games = %d, want the 9 and 5 summed", person.games)
		}
		if person.displayName() != "Lemner" {
			t.Errorf("display name = %q, want the name used for the most games", person.displayName())
		}
		if names := person.otherNames(); len(names) != 1 || names[0] != "Tanchik" {
			t.Errorf("other names = %v, want just Tanchik", names)
		}
		if !person.lastPlayed.Equal(base) {
			t.Errorf("lastPlayed = %v, want the newer of the two", person.lastPlayed)
		}
		// The refresh sweep has to address the toon we can actually reach,
		// which here is the alt, not the name they play most.
		if person.refreshToon != "tanchik" || person.gateway != 20 {
			t.Errorf("refresh target = %q/%d, want tanchik/20", person.refreshToon, person.gateway)
		}
	})

	t.Run("a toon with no cached account stands alone", func(t *testing.T) {
		// The safe failure: an unmerged regular is merely counted twice, while
		// a wrong merge would credit someone with a stranger's games.
		rows := countsAt(base,
			coPlayerCount{PlayerKey: "someone", PlayerName: "Someone", Games: 7},
			coPlayerCount{PlayerKey: "another", PlayerName: "Another", Games: 6},
		)

		merged := mergeCoPlayersByAccount(rows, regularsIdentityIndex{auroraByKey: map[string]int64{}, gatewayByKey: map[string]int64{}})

		if len(merged) != 2 {
			t.Fatalf("got %d identities, want 2 unmerged", len(merged))
		}
	})

	t.Run("different accounts never merge", func(t *testing.T) {
		rows := countsAt(base,
			coPlayerCount{PlayerKey: "a", PlayerName: "A", Games: 7},
			coPlayerCount{PlayerKey: "b", PlayerName: "B", Games: 6},
		)
		index := regularsIdentityIndex{auroraByKey: map[string]int64{"a": 1, "b": 2}, gatewayByKey: map[string]int64{}}

		if merged := mergeCoPlayersByAccount(rows, index); len(merged) != 2 {
			t.Fatalf("got %d identities, want 2", len(merged))
		}
	})
}

func TestRankRegulars(t *testing.T) {
	base := time.Date(2026, 9, 6, 21, 0, 0, 0, time.UTC)
	identity := func(name string, games int, last time.Time) *regularsIdentity {
		return &regularsIdentity{names: map[string]int{name: games}, games: games, lastPlayed: last}
	}

	t.Run("the floor drops one-off opponents", func(t *testing.T) {
		ranked := rankRegulars([]*regularsIdentity{
			identity("regular", regularsMinGames, base),
			identity("stranger", regularsMinGames-1, base),
		})
		if len(ranked) != 1 || ranked[0].displayName() != "regular" {
			t.Fatalf("ranked = %v, want only the one at the floor", ranked)
		}
	})

	t.Run("most played first, ties broken by who was seen last", func(t *testing.T) {
		ranked := rankRegulars([]*regularsIdentity{
			identity("older", 10, base.Add(-48*time.Hour)),
			identity("newer", 10, base),
			identity("most", 30, base.Add(-72*time.Hour)),
		})
		got := []string{ranked[0].displayName(), ranked[1].displayName(), ranked[2].displayName()}
		want := []string{"most", "newer", "older"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("order = %v, want %v", got, want)
			}
		}
	})

	t.Run("the list is capped", func(t *testing.T) {
		many := make([]*regularsIdentity, 0, regularsMax*2)
		for i := range regularsMax * 2 {
			many = append(many, identity(string(rune('a'+i%26)), regularsMinGames+i, base))
		}
		if ranked := rankRegulars(many); len(ranked) != regularsMax {
			t.Fatalf("got %d, want the cap of %d", len(ranked), regularsMax)
		}
	})
}

func TestRegularFreshness(t *testing.T) {
	now := time.Date(2026, 9, 12, 19, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		age  time.Duration
		want string
	}{
		{"just finished a game", 5 * time.Minute, regularFreshnessNow},
		{"at the hour boundary", regularsPlayingNow, regularFreshnessNow},
		{"just past the hour", regularsPlayingNow + time.Minute, regularFreshnessLately},
		{"yesterday", 24 * time.Hour, regularFreshnessLately},
		{"at the lately boundary", regularsPlayedLately, regularFreshnessLately},
		{"a week ago is not worth mentioning", 7 * 24 * time.Hour, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := regularFreshness(now.Add(-tc.age), now); got != tc.want {
				t.Errorf("freshness = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPlayerTimeInGame(t *testing.T) {
	// An SC:R replay keeps recording after a player drops, and the
	// complete-replays feature can even store a co-player's fuller copy, so
	// the replay's length is the game's, not the user's share of it.
	if got := playerTimeInGame(4334, 900); got != 900 {
		t.Errorf("got %d, want the user's leave stamp of 900", got)
	}
	// Zero means they were still there when it ended.
	if got := playerTimeInGame(1500, 0); got != 1500 {
		t.Errorf("got %d, want the full 1500", got)
	}
	// A leave stamp past the recording is not usable.
	if got := playerTimeInGame(1500, 9000); got != 1500 {
		t.Errorf("got %d, want the recording clamped to 1500", got)
	}
}

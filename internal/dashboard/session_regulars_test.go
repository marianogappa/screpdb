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
	fresh := now.Add(-time.Minute) // we asked Battle.net a minute ago

	t.Run("tiers, with a fresh observation", func(t *testing.T) {
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
				if got := regularFreshness(now.Add(-tc.age), fresh, now); got != tc.want {
					t.Errorf("freshness = %q, want %q", got, tc.want)
				}
			})
		}
	})

	// A claim about *this moment* needs a look at this moment. With a stale
	// observation the person is demoted rather than dropped: a game within the
	// hour is still a game within three days, and that much is still true.
	t.Run("a stale observation cannot support the live tier", func(t *testing.T) {
		// The game must be as old as the look, or the game would itself be the
		// fresher look — see the subtest below.
		stale := now.Add(-regularsObservationWindow - time.Minute)
		if got := regularFreshness(stale, stale, now); got != regularFreshnessLately {
			t.Errorf("freshness = %q, want a demotion to %q", got, regularFreshnessLately)
		}
		// Right at the window it still counts, so a sweep landing on schedule
		// never blinks the row off.
		atWindow := now.Add(-regularsObservationWindow)
		if got := regularFreshness(now.Add(-10*time.Minute), atWindow, now); got != regularFreshnessNow {
			t.Errorf("freshness = %q, want %q at the window boundary", got, regularFreshnessNow)
		}
	})

	// The whole point of the synergy: a game we already hold is a look we
	// already took, whoever paid for it — the watcher ingesting the replay of
	// the game just played, or a fetch made for some other surface that
	// happened to report it. Without this the row waits on a request of its own
	// to say what we already know, and the person the user finished playing two
	// minutes ago reads as absent.
	t.Run("a known game is its own observation", func(t *testing.T) {
		justPlayed := now.Add(-2 * time.Minute)
		for _, tc := range []struct {
			name     string
			observed time.Time
		}{
			{"never fetched at all", time.Time{}},
			{"last fetched long past the max age", now.Add(-20 * time.Hour)},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := regularFreshness(justPlayed, tc.observed, now); got != regularFreshnessNow {
					t.Errorf("freshness = %q, want %q", got, regularFreshnessNow)
				}
			})
		}
		// It cannot flatter the picture either: an old game is still an old
		// game, and dates the look no later than itself.
		if got := regularFreshness(now.Add(-20*time.Hour), time.Time{}, now); got != "" {
			t.Errorf("freshness = %q, want an old game to stay silent", got)
		}
	})

	t.Run("the window outlives the sweep interval", func(t *testing.T) {
		// Otherwise a sweep that lands exactly on time would already be too old
		// to support the tier it just refreshed, and "now" could never appear.
		if regularsObservationWindow <= regularsRefreshEvery {
			t.Fatalf("observation window %v must exceed the sweep interval %v", regularsObservationWindow, regularsRefreshEvery)
		}
	})

	t.Run("an unanswered picture is dropped, not narrated", func(t *testing.T) {
		lastSeen := now.Add(-2 * time.Hour)
		for _, tc := range []struct {
			name     string
			observed time.Time
			want     string
		}{
			{"never asked", time.Time{}, ""},
			{"asked a sweep ago", now.Add(-regularsRefreshEvery), regularFreshnessLately},
			{"asked at the max age", now.Add(-regularsObservationMaxAge), regularFreshnessLately},
			{"asked longer ago than the max age", now.Add(-regularsObservationMaxAge - time.Minute), ""},
			{"asked most of a day ago", now.Add(-20 * time.Hour), ""},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := regularFreshness(lastSeen, tc.observed, now); got != tc.want {
					t.Errorf("freshness = %q, want %q", got, tc.want)
				}
			})
		}
	})

	// The max age answers "who is around", so it is bounded by how often we
	// look, not by how wide the tier's own window is. A stale observation omits
	// everyone who started playing since, which makes a surviving list biased
	// rather than merely old.
	t.Run("the max age is measured in sweeps, not days", func(t *testing.T) {
		if regularsObservationMaxAge > 4*regularsRefreshEvery {
			t.Fatalf("max age %v is too lenient for a %v sweep", regularsObservationMaxAge, regularsRefreshEvery)
		}
		if regularsObservationMaxAge < regularsObservationWindow {
			t.Fatalf("max age %v must not undercut the live window %v", regularsObservationMaxAge, regularsObservationWindow)
		}
	})
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

func TestGatewayOrderForPutsKnownFirst(t *testing.T) {
	fallback := []int64{30, 20, 10}

	if got := gatewayOrderFor(0, fallback); len(got) != 3 || got[0] != 30 {
		t.Fatalf("with nothing known the sweep is unchanged, got %v", got)
	}

	// One request instead of three: the gateway we have evidence for goes
	// first, and is not asked twice.
	got := gatewayOrderFor(10, fallback)
	want := []int64{10, 30, 20}
	if len(got) != len(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestRefreshRegularsSkipsWhatWeAlreadyObserved(t *testing.T) {
	now := time.Date(2026, 9, 12, 19, 0, 0, 0, time.UTC)
	regulars := []sessionRegular{
		// Seen a moment ago by some route that cost us nothing: a request here
		// could only repeat what we hold.
		{PlayerKey: "fresh", refreshToon: "fresh", gateway: 30, observedAt: now.Add(-time.Minute)},
		// Last looked at longer ago than a sweep, so worth the request.
		{PlayerKey: "stale", refreshToon: "stale", gateway: 30, observedAt: now.Add(-2 * regularsRefreshEvery)},
		// Never looked at.
		{PlayerKey: "unknown", refreshToon: "unknown", gateway: 30},
	}

	var targets []string
	for _, regular := range regulars {
		if !regular.observedAt.IsZero() && now.Sub(regular.observedAt) < regularsObservationGoodEnough {
			continue
		}
		targets = append(targets, regular.PlayerKey)
	}

	if len(targets) != 2 || targets[0] != "stale" || targets[1] != "unknown" {
		t.Fatalf("targets = %v, want the stale and never-seen regulars only", targets)
	}
}

// The skip must not stretch the sweep: an observation between the interval and
// the wider tolerance the live tier allows still earns a request, or someone
// who started playing in between would go unnoticed for two sweeps.
func TestRegularsObservationGoodEnoughIsTighterThanTheTier(t *testing.T) {
	if regularsObservationGoodEnough >= regularsRefreshEvery {
		t.Fatalf("good-enough %v must sit under the sweep interval %v", regularsObservationGoodEnough, regularsRefreshEvery)
	}
	if regularsObservationGoodEnough >= regularsObservationWindow {
		t.Fatalf("good-enough %v must be tighter than the tier's tolerance %v", regularsObservationGoodEnough, regularsObservationWindow)
	}
}

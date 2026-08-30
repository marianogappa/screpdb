package dashboard

import (
	"testing"
	"time"
)

func rowsAt(base time.Time, offsets ...time.Duration) []sessionGameRow {
	rows := make([]sessionGameRow, 0, len(offsets))
	for i, offset := range offsets {
		rows = append(rows, sessionGameRow{ReplayID: int64(i + 1), PlayedAt: base.Add(-offset)})
	}
	return rows
}

func TestGamingSessionWindow(t *testing.T) {
	now := time.Date(2026, 8, 30, 20, 0, 0, 0, time.UTC)

	t.Run("no games is no session", func(t *testing.T) {
		if _, _, _, ok := gamingSessionWindow(nil, now); ok {
			t.Fatal("expected no session")
		}
	})

	t.Run("a stale last game ends the session", func(t *testing.T) {
		rows := rowsAt(now, 4*time.Hour, 5*time.Hour)
		if _, _, _, ok := gamingSessionWindow(rows, now); ok {
			t.Fatal("a game older than the recency window must not open a session")
		}
	})

	t.Run("games within the gap are one sitting", func(t *testing.T) {
		rows := rowsAt(now, 10*time.Minute, 40*time.Minute, 80*time.Minute)
		start, end, count, ok := gamingSessionWindow(rows, now)
		if !ok {
			t.Fatal("expected a session")
		}
		if count != 3 {
			t.Fatalf("count = %d, want 3", count)
		}
		if !end.Equal(now.Add(-10 * time.Minute)) {
			t.Errorf("end = %v", end)
		}
		if !start.Equal(now.Add(-80 * time.Minute)) {
			t.Errorf("start = %v", start)
		}
	})

	t.Run("a gap longer than the threshold cuts the session", func(t *testing.T) {
		// Two games close together, then a 5h hole, then an older cluster that
		// belongs to a previous sitting and must be excluded.
		rows := rowsAt(now, 10*time.Minute, 40*time.Minute, 6*time.Hour, 7*time.Hour)
		_, _, count, ok := gamingSessionWindow(rows, now)
		if !ok {
			t.Fatal("expected a session")
		}
		if count != 2 {
			t.Fatalf("count = %d, want 2 (the older cluster is a different sitting)", count)
		}
	})

	t.Run("the gap is measured between consecutive games, not from the latest", func(t *testing.T) {
		// Each step is under the gap, so a long session chains together even
		// though the earliest game is far older than the recency window.
		rows := rowsAt(now, 0, 2*time.Hour, 4*time.Hour, 6*time.Hour, 8*time.Hour)
		_, _, count, ok := gamingSessionWindow(rows, now)
		if !ok {
			t.Fatal("expected a session")
		}
		if count != 5 {
			t.Fatalf("count = %d, want 5", count)
		}
	})
}

func TestAutosaveOnly(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{`/Users/me/StarCraft/Maps/Replays/Autosave/LastReplay.rep`, true},
		{`C:\Users\me\Documents\StarCraft\Maps\Replays\Autosave\LastReplay.rep`, true},
		{`/Users/me/StarCraft/Maps/Replays/AUTOSAVE/x.rep`, true},
		{`/Users/me/Downloads/some_pro_game.rep`, false},
		{`/Users/me/StarCraft/Maps/Replays/000_screpdb_watch_me/x.rep`, false},
		// "autosave" as part of a longer name is not the Autosave folder.
		{`/Users/me/Replays/autosaved/x.rep`, false},
	} {
		if got := autosaveOnly(tc.path); got != tc.want {
			t.Errorf("autosaveOnly(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestGamingSessionPlayers(t *testing.T) {
	youKeys := map[string]struct{}{"me": {}}
	games := []workflowGameListItem{
		{
			ReplayID: 1,
			Players: []workflowGameListPlayer{
				{PlayerKey: "me", Name: "Me", Team: 1, IsWinner: true},
				{PlayerKey: "mate", Name: "Mate", Team: 1, IsWinner: true, Race: "Zerg"},
				{PlayerKey: "foe", Name: "Foe", Team: 2, IsWinner: false, Race: "Terran"},
			},
		},
		{
			ReplayID: 2,
			Players: []workflowGameListPlayer{
				{PlayerKey: "me", Name: "Me", Team: 1, IsWinner: false},
				{PlayerKey: "foe", Name: "Foe", Team: 2, IsWinner: true, Race: "Protoss"},
			},
		},
	}
	apm := map[gamePlayerKey]sessionAPM{
		{ReplayID: 1, PlayerKey: "foe"}: {APM: 100},
		{ReplayID: 2, PlayerKey: "foe"}: {APM: 200},
	}

	opponents, allies := gamingSessionPlayers(games, apm, youKeys)

	if len(opponents) != 1 || opponents[0].PlayerKey != "foe" {
		t.Fatalf("opponents = %+v, want just foe", opponents)
	}
	foe := opponents[0]
	if foe.Games != 2 || foe.Wins != 1 || foe.Losses != 1 {
		t.Errorf("foe = %+v, want 2 games 1-1", foe)
	}
	if foe.APM != 150 {
		t.Errorf("foe APM = %d, want the 100/200 mean of 150", foe.APM)
	}
	if len(foe.Races) != 2 || foe.Races[0] != "Protoss" || foe.Races[1] != "Terran" {
		t.Errorf("foe races = %v, want both sorted", foe.Races)
	}

	if len(allies) != 1 || allies[0].PlayerKey != "mate" {
		t.Fatalf("allies = %+v, want just mate", allies)
	}
	// An ally shares the user's result, so a record against them is meaningless
	// and must stay empty rather than reading as a win over a team-mate.
	mate := allies[0]
	if mate.Games != 1 || mate.Wins != 0 || mate.Losses != 0 {
		t.Errorf("mate = %+v, want 1 game and no record", mate)
	}
	for _, list := range [][]gamingSessionPlayer{opponents, allies} {
		for _, p := range list {
			if p.PlayerKey == "me" {
				t.Error("the user must never appear in their own opponent or ally list")
			}
		}
	}
}

func TestParseBnetProfileDetail(t *testing.T) {
	payload := []byte(`{
		"aurora_id": 12345,
		"battle_tag": "Someone",
		"country_code": "ARG",
		"toons": [
			{"toon": "quiet", "gateway_id": 10, "games_last_week": 0},
			{"toon": "main", "gateway_id": 30, "games_last_week": 40}
		],
		"matchmaked_stats": [
			{"rating": 1400, "highest_rating": 1471, "wins": 3, "losses": 1},
			{"rating": 1200, "highest_rating": 1300, "wins": 2, "losses": 2}
		]
	}`)
	got := parseBnetProfileDetail("Main", payload)
	if got == nil {
		t.Fatal("expected a detail")
	}
	if got.AuroraID != 12345 || got.BattleTag != "Someone" || got.CountryCode != "ARG" {
		t.Errorf("identity fields wrong: %+v", got)
	}
	// Most played toon first, so the account they actually use leads.
	if len(got.Toons) != 2 || got.Toons[0].Toon != "main" {
		t.Errorf("toons = %+v, want the most played first", got.Toons)
	}
	if !got.PlaysLadder {
		t.Error("matchmaked_stats present means they ladder")
	}
	if got.MMR != 1400 || got.HighestMMR != 1471 {
		t.Errorf("mmr = %d/%d, want the best across records", got.MMR, got.HighestMMR)
	}
	if got.LadderWins != 5 || got.LadderLosses != 3 {
		t.Errorf("ladder record = %d-%d, want summed across records", got.LadderWins, got.LadderLosses)
	}
}

func TestParseBnetProfileDetail_Unusable(t *testing.T) {
	// A page that shows profile decoration must still render when the payload
	// is missing or malformed, so these yield no detail rather than an error.
	if parseBnetProfileDetail("x", nil) != nil {
		t.Error("empty payload must yield no detail")
	}
	if parseBnetProfileDetail("x", []byte("not json")) != nil {
		t.Error("malformed payload must yield no detail")
	}
	got := parseBnetProfileDetail("x", []byte(`{"aurora_id": 7}`))
	if got == nil || got.PlaysLadder {
		t.Errorf("a profile with no matchmaked_stats must not read as a ladder player: %+v", got)
	}
}

func TestParseReplayDate(t *testing.T) {
	// The form the ingest path actually writes.
	got, err := parseReplayDate("2026-08-23 23:18:27 +0400 +04")
	if err != nil {
		t.Fatalf("parseReplayDate: %v", err)
	}
	if got.Year() != 2026 || got.Month() != time.August || got.Day() != 23 {
		t.Errorf("parsed = %v", got)
	}
	if _, err := parseReplayDate("not a date"); err == nil {
		t.Error("expected an error for an unparseable date")
	}
}

func TestFormatYouDisplayName(t *testing.T) {
	if got := formatYouDisplayName("chobo86"); got != "chobo86 "+youMarker {
		t.Errorf("got %q", got)
	}
	// Idempotent: marking an already-marked name must not double the marker.
	if got := formatYouDisplayName("chobo86 " + youMarker); got != "chobo86 "+youMarker {
		t.Errorf("got %q", got)
	}
	if got := formatYouDisplayName("  "); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestYouLookupKeys(t *testing.T) {
	// Replays usually carry the bare name while CSettings has the full tag, so
	// both must match.
	got := youLookupKeys("Chobo86#1234")
	if len(got) != 2 || got[0] != "chobo86#1234" || got[1] != "chobo86" {
		t.Fatalf("got %v", got)
	}
	if got := youLookupKeys("  "); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

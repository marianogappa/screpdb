package dashboard

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
	"github.com/marianogappa/screpdb/internal/library/persist"
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
		if _, _, _, ok := gamingSessionWindow(nil); ok {
			t.Fatal("expected no session")
		}
	})

	t.Run("a stale last game is still a session, just not a live one", func(t *testing.T) {
		rows := rowsAt(now, 4*time.Hour, 5*time.Hour)
		_, end, count, ok := gamingSessionWindow(rows)
		if !ok {
			t.Fatal("a finished sitting is still the last session")
		}
		if count != 2 {
			t.Fatalf("count = %d, want 2", count)
		}
		if gamingSessionIsLive(end, now) {
			t.Error("a game older than the recency window must not read as live")
		}
	})

	t.Run("games within the gap are one sitting", func(t *testing.T) {
		rows := rowsAt(now, 10*time.Minute, 40*time.Minute, 80*time.Minute)
		start, end, count, ok := gamingSessionWindow(rows)
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
		if !gamingSessionIsLive(end, now) {
			t.Error("a sitting that ended ten minutes ago is live")
		}
	})

	t.Run("a gap longer than the threshold cuts the session", func(t *testing.T) {
		// Two games close together, then a 5h hole, then an older cluster that
		// belongs to a previous sitting and must be excluded.
		rows := rowsAt(now, 10*time.Minute, 40*time.Minute, 6*time.Hour, 7*time.Hour)
		_, _, count, ok := gamingSessionWindow(rows)
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
		_, _, count, ok := gamingSessionWindow(rows)
		if !ok {
			t.Fatal("expected a session")
		}
		if count != 5 {
			t.Fatalf("count = %d, want 5", count)
		}
	})
}

func TestGamingSessionLiveness(t *testing.T) {
	now := time.Date(2026, 8, 30, 20, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name         string
		age          time.Duration
		live         bool
		recentEnough bool
	}{
		{"just finished", 0, true, true},
		{"inside the recency window", gamingSessionRecency - time.Minute, true, true},
		{"exactly at the recency boundary", gamingSessionRecency, true, true},
		{"just past the recency boundary", gamingSessionRecency + time.Minute, false, true},
		{"weeks ago", 22 * 24 * time.Hour, false, true},
		{"exactly at the max age", gamingSessionMaxAge, false, true},
		{"past the max age", gamingSessionMaxAge + time.Minute, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			end := now.Add(-tc.age)
			if got := gamingSessionIsLive(end, now); got != tc.live {
				t.Errorf("gamingSessionIsLive = %v, want %v", got, tc.live)
			}
			if got := gamingSessionIsRecentEnough(end, now); got != tc.recentEnough {
				t.Errorf("gamingSessionIsRecentEnough = %v, want %v", got, tc.recentEnough)
			}
		})
	}
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
		{
			ReplayID: 3,
			Players: []workflowGameListPlayer{
				{PlayerKey: "me", Name: "Me", Team: 1, IsWinner: false},
				{PlayerKey: "foe", Name: "Foe", Team: 2, IsWinner: false, Race: "Protoss"},
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
	// Game 3 was never resolved, so it counts as played but adds no record:
	// booking the user's missing win as a loss would invent a win for Foe.
	foe := opponents[0]
	if foe.Games != 3 || foe.Wins != 1 || foe.Losses != 1 {
		t.Errorf("foe = %+v, want 3 games 1-1", foe)
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

func TestSummarizeGamingSessionUndecided(t *testing.T) {
	now := time.Date(2026, 8, 30, 20, 0, 0, 0, time.UTC)
	youKeys := map[string]struct{}{"me": {}}
	games := []workflowGameListItem{
		{
			ReplayID: 1,
			Players: []workflowGameListPlayer{
				{PlayerKey: "me", Team: 1, IsWinner: true},
				{PlayerKey: "foe", Team: 2, IsWinner: false},
			},
		},
		{
			ReplayID: 2,
			Players: []workflowGameListPlayer{
				{PlayerKey: "me", Team: 1, IsWinner: false},
				{PlayerKey: "foe", Team: 2, IsWinner: true},
			},
		},
		{
			ReplayID: 3,
			Players: []workflowGameListPlayer{
				{PlayerKey: "me", Team: 1, IsWinner: false},
				{PlayerKey: "foe", Team: 2, IsWinner: false},
			},
		},
	}

	stats := summarizeGamingSession(rowsAt(now, 0, time.Hour, 2*time.Hour), games, nil, youKeys)

	if stats.Wins != 1 || stats.Losses != 1 || stats.Undecided != 1 {
		t.Fatalf("record = %d-%d with %d undecided, want 1-1 with 1", stats.Wins, stats.Losses, stats.Undecided)
	}
	// The unresolved game must not drag the rate down as if it were lost.
	if stats.WinRate != 0.5 {
		t.Fatalf("win rate = %v, want 0.5 over decided games only", stats.WinRate)
	}
}

func TestBnetProfileDetailFromRecord(t *testing.T) {
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
	record, _ := persist.DistillBnetProfile("Main", 30, time.Now(), payload)
	got := bnetProfileDetailFromRecord(record, nil)
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

func TestBnetProfileDetailFromRecord_CollapsesGatewayRepeats(t *testing.T) {
	// The bridge lists a toon once per gateway it exists on. The alias row must
	// show each name once, not once per gateway.
	payload := []byte(`{
		"aurora_id": 7,
		"toons": [
			{"toon": "chobo85", "gateway_id": 10, "games_last_week": 2},
			{"toon": "chobo85", "gateway_id": 11, "games_last_week": 3},
			{"toon": "Chobo85s", "gateway_id": 10, "games_last_week": 1},
			{"toon": "chobo85s", "gateway_id": 11, "games_last_week": 0},
			{"toon": "chobo86", "gateway_id": 10, "games_last_week": 9}
		]
	}`)
	record, _ := persist.DistillBnetProfile("chobo86", 10, time.Now(), payload)
	got := bnetProfileDetailFromRecord(record, nil)
	names := []string{}
	for _, toon := range got.Toons {
		names = append(names, toon.Toon)
	}
	if len(names) != 3 {
		t.Fatalf("toons = %v, want one entry per distinct name", names)
	}
	if names[0] != "chobo86" || names[1] != "chobo85" || names[2] != "Chobo85s" {
		t.Errorf("toons = %v, want most played first with the first-seen casing", names)
	}
	// Weekly counts are per gateway, so the collapsed entry carries their sum.
	if got.Toons[1].GamesLastWeek != 5 {
		t.Errorf("chobo85 games_last_week = %d, want 5 summed across gateways", got.Toons[1].GamesLastWeek)
	}
	// The account total still counts every gateway row.
	if got.GamesLastWeek != 15 {
		t.Errorf("account games_last_week = %d, want 15", got.GamesLastWeek)
	}
}

func TestBnetProfileDetailFromRecord_Unusable(t *testing.T) {
	// A missing or malformed payload distils to a not-found record, so the
	// pages that show profile decoration never see it.
	if record, _ := persist.DistillBnetProfile("x", 30, time.Now(), nil); record.Found {
		t.Error("empty payload must distil to a not-found record")
	}
	if record, _ := persist.DistillBnetProfile("x", 30, time.Now(), []byte("not json")); record.Found {
		t.Error("malformed payload must distil to a not-found record")
	}
	record, _ := persist.DistillBnetProfile("x", 30, time.Now(), []byte(`{"aurora_id": 7}`))
	got := bnetProfileDetailFromRecord(record, nil)
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

// sessionReplay builds one autosaved 1v1 between the user and an opponent,
// played at playedAt.
//
// The store writes the replay date with time.Time.String(), which appends a
// monotonic reading ("m=+0.000") that parseReplayDate then rejects, so any
// time derived from time.Now() has to be stripped with Round(0).
func sessionReplay(id int64, playedAt time.Time, autosave bool) *library.Replay {
	path := fmt.Sprintf("/sc/Maps/Replays/Autosave/LastReplay-%d.rep", id)
	if !autosave {
		path = fmt.Sprintf("/Users/me/Downloads/some_pro_game-%d.rep", id)
	}
	r := librarytest.Replay(
		librarytest.WithID(id),
		librarytest.WithChecksum(fmt.Sprintf("session-%d", id)),
		librarytest.WithDate(playedAt.Round(0)),
		librarytest.WithPath(path, playedAt.Round(0)),
		librarytest.WithPlayer("Chobo86", librarytest.Team(1), librarytest.Winner(), librarytest.APM(120, 90)),
		librarytest.WithPlayer("Foe", librarytest.Team(2), librarytest.APM(200, 150)),
	)
	if autosave {
		r.Flags |= library.FlagIsAutosave
	}
	return r
}

func newSessionDashboard(t *testing.T, replays ...*library.Replay) *Dashboard {
	t.Helper()
	d := newTestDashboardWithReplays(t, replays...)
	d.youKeys.Store(youKeySetFromBattleTags([]string{"Chobo86#1234"}))
	return d
}

func TestGamingSessionNeedsYou(t *testing.T) {
	// Without CSettings there is no "you", and the whole feature stays dark
	// rather than guessing which of the replay's players the user is.
	d := newTestDashboardWithReplays(t, sessionReplay(1, time.Now().Add(-time.Hour), true))
	resp, err := d.gamingSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.HasSession || resp.Active {
		t.Fatalf("no you must mean no session: %+v", resp)
	}
}

func TestGamingSessionIgnoresNonAutosavedReplays(t *testing.T) {
	// A replay the user downloaded or was sent says nothing about them sitting
	// down to play, so it must not invent a session.
	d := newSessionDashboard(t, sessionReplay(1, time.Now().Add(-time.Hour), false))
	resp, err := d.gamingSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.HasSession || resp.Active {
		t.Fatalf("a downloaded replay must not open a session: %+v", resp)
	}
}

func TestGamingSessionFinishedVersusLive(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name       string
		age        time.Duration
		hasSession bool
		active     bool
	}{
		{"still playing", 20 * time.Minute, true, true},
		{"finished this morning", 8 * time.Hour, true, false},
		{"finished weeks ago", 22 * 24 * time.Hour, true, false},
		{"older than the max age", gamingSessionMaxAge + 24*time.Hour, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			end := now.Add(-tc.age)
			d := newSessionDashboard(t,
				sessionReplay(1, end, true),
				sessionReplay(2, end.Add(-30*time.Minute), true),
			)
			resp, err := d.gamingSession(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if resp.HasSession != tc.hasSession || resp.Active != tc.active {
				t.Fatalf("has_session/active = %v/%v, want %v/%v", resp.HasSession, resp.Active, tc.hasSession, tc.active)
			}
			if !tc.hasSession {
				return
			}
			// A finished session still carries the full payload: the view it
			// feeds is the same one a live session renders.
			if len(resp.Games) != 2 || resp.Stats.Games != 2 {
				t.Fatalf("games = %d, stats.games = %d, want both 2", len(resp.Games), resp.Stats.Games)
			}
			if len(resp.Opponents) != 1 || resp.Opponents[0].PlayerName != "Foe" {
				t.Fatalf("opponents = %+v, want Foe", resp.Opponents)
			}
		})
	}
}

func TestSummarizeGamingSessionElapsedIncludesTheLastGame(t *testing.T) {
	// Replays are dated from their start, so a sitting whose last game ran 20
	// minutes ends 20 minutes after that game's stamp. Without it the elapsed
	// total can come out shorter than the time actually spent in game.
	now := time.Date(2026, 8, 30, 20, 0, 0, 0, time.UTC)
	rows := rowsAt(now, 0, 30*time.Minute)
	games := []workflowGameListItem{
		{ReplayID: 1, DurationSeconds: 1200, Players: []workflowGameListPlayer{{PlayerKey: "me", Team: 1}}},
		{ReplayID: 2, DurationSeconds: 1500, Players: []workflowGameListPlayer{{PlayerKey: "me", Team: 1}}},
	}

	stats := summarizeGamingSession(rows, games, nil, map[string]struct{}{"me": {}})

	// 30 minutes between the two starts, plus the 20-minute last game.
	if stats.DurationSeconds != 3000 {
		t.Fatalf("duration = %ds, want 3000 (the span plus the last game)", stats.DurationSeconds)
	}
	if stats.PlayedSeconds != 2700 {
		t.Fatalf("played = %ds, want 2700", stats.PlayedSeconds)
	}
	if stats.PlayedSeconds > stats.DurationSeconds {
		t.Error("time in game can never exceed the elapsed sitting")
	}
	if stats.EndedAt != now.Add(20*time.Minute).Format(time.RFC3339) {
		t.Errorf("ended_at = %s, want the end of the last game", stats.EndedAt)
	}
}

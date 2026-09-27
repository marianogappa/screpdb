package dashboard

import (
	"testing"
	"time"
)

func seoulGames(count int, weekday time.Weekday, hour int) []bnetHabitGame {
	loc, _ := time.LoadLocation("Asia/Seoul")
	base := time.Date(2026, 8, 1, hour, 30, 0, 0, loc) // 2026-08-01 is a Saturday
	for base.Weekday() != weekday {
		base = base.AddDate(0, 0, 1)
	}
	out := make([]bnetHabitGame, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, bnetHabitGame{At: base.AddDate(0, 0, 7*(i%6)).Add(time.Duration(i%3) * 20 * time.Minute).UTC()})
	}
	return out
}

func TestComputeBnetPlayHabitsNeedsAFloor(t *testing.T) {
	if got := computeBnetPlayHabits(seoulGames(10, time.Saturday, 21), "KR"); got != nil {
		t.Fatalf("10 games must not produce habits, got %+v", got)
	}
	oneWeek := make([]bnetHabitGame, 0, 20)
	for i := 0; i < 20; i++ {
		oneWeek = append(oneWeek, bnetHabitGame{At: time.Date(2026, 8, 1, 12, i, 0, 0, time.UTC)})
	}
	if got := computeBnetPlayHabits(oneWeek, "KR"); got != nil {
		t.Fatalf("one long session must not produce habits, got %+v", got)
	}
}

func TestComputeBnetPlayHabitsWeekendEveningsInLocalTime(t *testing.T) {
	// 21:30 Seoul is 12:30 UTC: without the zone this would read as afternoon.
	games := seoulGames(18, time.Saturday, 21)
	got := computeBnetPlayHabits(games, "KR")
	if got == nil {
		t.Fatal("expected habits")
	}
	if got.WeekendShare != 1 {
		t.Fatalf("weekend share %v", got.WeekendShare)
	}
	if got.TimeOfDay["evening"] != 1 {
		t.Fatalf("evening share %v (%+v)", got.TimeOfDay["evening"], got.TimeOfDay)
	}
	if got.Summary != "Plays mostly on weekends, usually in their evenings. Over 35 observed days, about one session a week of 3 games each, from 18 games." {
		t.Fatalf("summary %q", got.Summary)
	}
}

func TestComputeBnetPlayHabitsMultiZoneCountrySkipsTimeOfDay(t *testing.T) {
	got := computeBnetPlayHabits(seoulGames(18, time.Tuesday, 21), "US")
	if got == nil {
		t.Fatal("expected habits")
	}
	if got.TimeOfDay != nil || got.TimeZone != "" {
		t.Fatalf("multi-zone country must not report time of day: %+v", got)
	}
	if got.WeekendShare != 0 || got.Summary != "Plays mostly on weekdays. Over 35 observed days, about one session a week of 3 games each, from 18 games." {
		t.Fatalf("summary %q weekend %v", got.Summary, got.WeekendShare)
	}
}

func TestComputeBnetPlayHabitsAcceptsAlpha3CountryCodes(t *testing.T) {
	got := computeBnetPlayHabits(seoulGames(18, time.Saturday, 21), "KOR")
	if got == nil || got.TimeOfDay["evening"] != 1 {
		t.Fatalf("Battle.net's alpha-3 KOR must resolve to Seoul: %+v", got)
	}
}

func TestComputeBnetPlayHabitsSessions(t *testing.T) {
	// Two sittings a week for three weeks: three 20-minute games back to
	// back on Tuesday, five on Friday, and one Friday all-nighter of twelve.
	base := time.Date(2026, 8, 4, 19, 0, 0, 0, time.UTC)
	var games []bnetHabitGame
	sitting := func(start time.Time, n int) {
		for i := 0; i < n; i++ {
			games = append(games, bnetHabitGame{At: start.Add(time.Duration(i) * 25 * time.Minute), Seconds: 20 * 60})
		}
	}
	for week := 0; week < 3; week++ {
		tuesday := base.AddDate(0, 0, 7*week)
		sitting(tuesday, 3)
		friday := tuesday.AddDate(0, 0, 3)
		if week == 2 {
			sitting(friday, 12)
		} else {
			sitting(friday, 5)
		}
	}
	got := computeBnetPlayHabits(games, "")
	if got == nil {
		t.Fatal("expected habits")
	}
	if got.ObservedDays != 17 || got.GamesPerSession != 5 || got.MinutesPerSession != 120 {
		t.Fatalf("days %d games/session %d minutes/session %d", got.ObservedDays, got.GamesPerSession, got.MinutesPerSession)
	}
	if want := "Plays mostly on weekdays. Over 17 observed days, about 2.5 sessions a week of 5 games and 2h each, from 31 games."; got.Summary != want {
		t.Fatalf("summary %q", got.Summary)
	}
}

func TestSummarizeBnetPlayHabitsNightReadsAsLateAtNight(t *testing.T) {
	h := &bnetPlayHabits{Games: 20, ObservedDays: 30, SessionsPerWeek: 2, GamesPerSession: 3, MinutesPerSession: 60,
		WeekendShare: 0.4, TimeOfDay: map[string]float64{"evening": 0.4, "night": 0.4, "morning": 0.2}}
	if got, want := summarizeBnetPlayHabits(h), "Plays throughout the week, in their evenings and late at night. Over 30 observed days, about 2 sessions a week of 3 games and 1h each, from 20 games."; got != want {
		t.Fatalf("summary %q", got)
	}
}

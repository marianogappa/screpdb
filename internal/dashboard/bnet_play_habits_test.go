package dashboard

import (
	"testing"
	"time"
)

func seoulGames(count int, weekday time.Weekday, hour int) []time.Time {
	loc, _ := time.LoadLocation("Asia/Seoul")
	base := time.Date(2026, 8, 1, hour, 30, 0, 0, loc) // 2026-08-01 is a Saturday
	for base.Weekday() != weekday {
		base = base.AddDate(0, 0, 1)
	}
	out := make([]time.Time, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, base.AddDate(0, 0, 7*(i%6)).Add(time.Duration(i%3)*20*time.Minute).UTC())
	}
	return out
}

func TestComputeBnetPlayHabitsNeedsAFloor(t *testing.T) {
	if got := computeBnetPlayHabits(seoulGames(10, time.Saturday, 21), "KR"); got != nil {
		t.Fatalf("10 games must not produce habits, got %+v", got)
	}
	oneWeek := make([]time.Time, 0, 20)
	for i := 0; i < 20; i++ {
		oneWeek = append(oneWeek, time.Date(2026, 8, 1, 12, i, 0, 0, time.UTC))
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
	if got.Summary != "Plays mostly on weekends, usually in the evening (Seoul time), from 18 games over 6 weeks." {
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
	if got.WeekendShare != 0 || got.Summary != "Plays mostly on weekdays, from 18 games over 6 weeks." {
		t.Fatalf("summary %q weekend %v", got.Summary, got.WeekendShare)
	}
}

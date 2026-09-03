package dashboard

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	_ "time/tzdata" // country time zones must resolve on Windows machines without a zoneinfo database

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
)

// Play habits are read off the rolling bnet_game_results cache. One profile
// fetch only shows ~20 games and a single long evening fills that, so nothing
// is claimed until the cache holds bnetHabitsMinGames games spread over
// bnetHabitsMinWeeks distinct weeks inside bnetHabitsWindow.
const (
	bnetHabitsWindow   = 90 * 24 * time.Hour
	bnetHabitsMinGames = 15
	bnetHabitsMinWeeks = 3
)

// bnetPlayHabits describes when an account plays. Time of day is only
// reported when the account's country has a single time zone; a weekday versus
// weekend split survives a wrong zone (a few games near midnight move), so it
// is always reported once the floor is met.
type bnetPlayHabits struct {
	Games        int                `json:"games"`
	Weeks        int                `json:"weeks"`
	WindowDays   int                `json:"window_days"`
	WeekendShare float64            `json:"weekend_share"`
	TimeOfDay    map[string]float64 `json:"time_of_day,omitempty"`
	TimeZone     string             `json:"time_zone,omitempty"`
	Summary      string             `json:"summary"`
}

// countryTimeZones lists countries that span a single IANA zone (or whose
// StarCraft population overwhelmingly lives in one). Multi-zone countries such
// as the US, Canada, Russia, Brazil and Australia are deliberately absent.
var countryTimeZones = map[string]string{
	"KR": "Asia/Seoul", "JP": "Asia/Tokyo", "CN": "Asia/Shanghai", "TW": "Asia/Taipei", "HK": "Asia/Hong_Kong",
	"SG": "Asia/Singapore", "PH": "Asia/Manila", "MY": "Asia/Kuala_Lumpur", "VN": "Asia/Ho_Chi_Minh", "TH": "Asia/Bangkok",
	"IN": "Asia/Kolkata", "IL": "Asia/Jerusalem", "TR": "Europe/Istanbul",
	"PL": "Europe/Warsaw", "DE": "Europe/Berlin", "FR": "Europe/Paris", "ES": "Europe/Madrid", "IT": "Europe/Rome",
	"NL": "Europe/Amsterdam", "BE": "Europe/Brussels", "AT": "Europe/Vienna", "CH": "Europe/Zurich", "SE": "Europe/Stockholm",
	"NO": "Europe/Oslo", "DK": "Europe/Copenhagen", "CZ": "Europe/Prague", "HU": "Europe/Budapest", "HR": "Europe/Zagreb",
	"RS": "Europe/Belgrade", "SK": "Europe/Bratislava", "SI": "Europe/Ljubljana", "GB": "Europe/London", "IE": "Europe/Dublin",
	"PT": "Europe/Lisbon", "FI": "Europe/Helsinki", "UA": "Europe/Kyiv", "BG": "Europe/Sofia", "RO": "Europe/Bucharest",
	"GR": "Europe/Athens", "LT": "Europe/Vilnius", "LV": "Europe/Riga", "EE": "Europe/Tallinn", "BY": "Europe/Minsk",
	"AR": "America/Argentina/Buenos_Aires", "PE": "America/Lima", "CO": "America/Bogota", "VE": "America/Caracas",
	"UY": "America/Montevideo", "PY": "America/Asuncion", "NZ": "Pacific/Auckland", "ZA": "Africa/Johannesburg",
}

var timeOfDayOrder = []string{"morning", "afternoon", "evening", "night"}

func timeOfDaySlot(hour int) string {
	switch {
	case hour >= 6 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 18:
		return "afternoon"
	case hour >= 18:
		return "evening"
	}
	return "night"
}

// bnetGameResultRows turns a parsed profile into cache rows for its account.
func bnetGameResultRows(auroraID int64, detail *bnetProfileDetail) []dashboarddb.BnetGameResultRow {
	if auroraID == 0 || detail == nil {
		return nil
	}
	rows := make([]dashboarddb.BnetGameResultRow, 0, len(detail.RecentGames))
	for _, g := range detail.RecentGames {
		playedAt, err := time.Parse(time.RFC3339, g.PlayedAt)
		if err != nil || g.GameID == "" {
			continue
		}
		rows = append(rows, dashboarddb.BnetGameResultRow{
			AuroraID:        auroraID,
			GameID:          g.GameID,
			CreateTime:      playedAt,
			Toon:            g.Toon,
			Gateway:         g.Gateway,
			Race:            g.Race,
			Result:          g.Result,
			APM:             g.APM,
			DurationSeconds: g.DurationSeconds,
			MapName:         g.MapName,
			MatchGUID:       g.MatchGUID,
		})
	}
	return rows
}

// rememberBnetGameResults appends a freshly fetched profile's games to the
// rolling cache. Failures are logged by the caller's crash guard, never
// surfaced: the cache is an accumulating convenience.
func (d *Dashboard) rememberBnetGameResults(ctx context.Context, auroraID int64, payload []byte, toon string) error {
	detail := parseBnetProfileDetail(toon, payload)
	return d.dbStore.UpsertBnetGameResults(ctx, bnetGameResultRows(auroraID, detail))
}

// bnetPlayHabitsFor reads the cache for an account and, once the floor is met,
// summarises when they play. countryCode picks the local time zone.
func (d *Dashboard) bnetPlayHabitsFor(ctx context.Context, auroraID int64, countryCode string, now time.Time) *bnetPlayHabits {
	if auroraID == 0 {
		return nil
	}
	times, err := d.dbStore.ListBnetGameTimes(ctx, auroraID, now.Add(-bnetHabitsWindow))
	if err != nil {
		return nil
	}
	return computeBnetPlayHabits(times, countryCode)
}

func computeBnetPlayHabits(times []time.Time, countryCode string) *bnetPlayHabits {
	if len(times) < bnetHabitsMinGames {
		return nil
	}
	var loc *time.Location
	zoneName := ""
	if zone, ok := countryTimeZones[strings.ToUpper(strings.TrimSpace(countryCode))]; ok {
		if l, err := time.LoadLocation(zone); err == nil {
			loc, zoneName = l, zone
		}
	}
	weekLoc := loc
	if weekLoc == nil {
		weekLoc = time.UTC
	}
	weeks := map[string]bool{}
	weekend := 0
	slots := map[string]int{}
	for _, t := range times {
		local := t.In(weekLoc)
		year, week := local.ISOWeek()
		weeks[fmt.Sprintf("%d-%02d", year, week)] = true
		if wd := local.Weekday(); wd == time.Saturday || wd == time.Sunday {
			weekend++
		}
		if loc != nil {
			slots[timeOfDaySlot(local.Hour())]++
		}
	}
	if len(weeks) < bnetHabitsMinWeeks {
		return nil
	}
	habits := &bnetPlayHabits{
		Games:        len(times),
		Weeks:        len(weeks),
		WindowDays:   int(bnetHabitsWindow / (24 * time.Hour)),
		WeekendShare: float64(weekend) / float64(len(times)),
		TimeZone:     zoneName,
	}
	if loc != nil {
		habits.TimeOfDay = map[string]float64{}
		for _, slot := range timeOfDayOrder {
			habits.TimeOfDay[slot] = float64(slots[slot]) / float64(len(times))
		}
	}
	habits.Summary = summarizeBnetPlayHabits(habits)
	return habits
}

func summarizeBnetPlayHabits(h *bnetPlayHabits) string {
	var when string
	switch {
	case h.WeekendShare >= 0.6:
		when = "mostly on weekends"
	case h.WeekendShare <= 0.2:
		when = "mostly on weekdays"
	default:
		when = "throughout the week"
	}
	parts := []string{when}
	if len(h.TimeOfDay) > 0 {
		slots := make([]string, 0, len(h.TimeOfDay))
		for slot := range h.TimeOfDay {
			slots = append(slots, slot)
		}
		sort.Slice(slots, func(i, j int) bool {
			if h.TimeOfDay[slots[i]] != h.TimeOfDay[slots[j]] {
				return h.TimeOfDay[slots[i]] > h.TimeOfDay[slots[j]]
			}
			return slots[i] < slots[j]
		})
		top := slots[0]
		switch {
		case h.TimeOfDay[top] >= 0.5:
			parts = append(parts, fmt.Sprintf("usually in the %s", top))
		case len(slots) > 1 && h.TimeOfDay[top]+h.TimeOfDay[slots[1]] >= 0.7:
			parts = append(parts, fmt.Sprintf("%s and %s", top, slots[1]))
		}
	}
	summary := fmt.Sprintf("Plays %s", strings.Join(parts, ", "))
	if h.TimeZone != "" {
		summary += " (" + strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(h.TimeZone, "Asia/"), "Europe/"), "_", " ") + " time)"
	}
	return summary + fmt.Sprintf(", from %d games over %d weeks.", h.Games, h.Weeks)
}

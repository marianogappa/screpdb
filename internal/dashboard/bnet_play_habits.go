package dashboard

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	_ "time/tzdata" // country time zones must resolve on Windows machines without a zoneinfo database
)

// Play habits are read off the game archive. One profile
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
	Games        int `json:"games"`
	Weeks        int `json:"weeks"`
	ObservedDays int `json:"observed_days"`
	// Sessions use the gaming-session gap. Games and minutes per session are
	// medians, so one all-nighter does not move them.
	SessionsPerWeek   float64            `json:"sessions_per_week"`
	GamesPerSession   int                `json:"games_per_session"`
	MinutesPerSession int                `json:"minutes_per_session"`
	WindowDays        int                `json:"window_days"`
	WeekendShare      float64            `json:"weekend_share"`
	TimeOfDay         map[string]float64 `json:"time_of_day,omitempty"`
	TimeZone          string             `json:"time_zone,omitempty"`
	Summary           string             `json:"summary"`
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

// Battle.net reports ISO alpha-3 country codes while countryTimeZones is keyed
// alpha-2, so without this no ordinary player ever got a time of day.
var countryAlpha3To2 = map[string]string{
	"KOR": "KR", "JPN": "JP", "CHN": "CN", "TWN": "TW", "HKG": "HK", "SGP": "SG", "PHL": "PH", "MYS": "MY",
	"VNM": "VN", "THA": "TH", "IND": "IN", "ISR": "IL", "TUR": "TR", "POL": "PL", "DEU": "DE", "FRA": "FR",
	"ESP": "ES", "ITA": "IT", "NLD": "NL", "BEL": "BE", "AUT": "AT", "CHE": "CH", "SWE": "SE", "NOR": "NO",
	"DNK": "DK", "CZE": "CZ", "HUN": "HU", "HRV": "HR", "SRB": "RS", "SVK": "SK", "SVN": "SI", "GBR": "GB",
	"IRL": "IE", "PRT": "PT", "FIN": "FI", "UKR": "UA", "BGR": "BG", "ROU": "RO", "GRC": "GR", "LTU": "LT",
	"LVA": "LV", "EST": "EE", "BLR": "BY", "ARG": "AR", "PER": "PE", "COL": "CO", "VEN": "VE", "URY": "UY",
	"PRY": "PY", "NZL": "NZ", "ZAF": "ZA",
}

type bnetHabitGame struct {
	At      time.Time
	Seconds int
}

// bnetPlayHabitsFor reads the cache for an account and, once the floor is met,
// summarises when they play. countryCode picks the local time zone.
func (d *Dashboard) bnetPlayHabitsFor(ctx context.Context, auroraID int64, countryCode string, now time.Time) *bnetPlayHabits {
	if auroraID == 0 {
		return nil
	}
	archived, err := d.dbStore.ListBnetGamesByAccount(ctx, auroraID)
	if err != nil {
		return nil
	}
	since := now.Add(-bnetHabitsWindow)
	var games []bnetHabitGame
	for _, g := range archived {
		if g.CreateTime.IsZero() || g.CreateTime.Before(since) {
			continue
		}
		game := bnetHabitGame{At: g.CreateTime.UTC()}
		for _, p := range g.Players {
			if p.Seconds > game.Seconds {
				game.Seconds = p.Seconds
			}
		}
		games = append(games, game)
	}
	return computeBnetPlayHabits(games, countryCode)
}

func computeBnetPlayHabits(games []bnetHabitGame, countryCode string) *bnetPlayHabits {
	if len(games) < bnetHabitsMinGames {
		return nil
	}
	sort.Slice(games, func(i, j int) bool { return games[i].At.Before(games[j].At) })
	var loc *time.Location
	zoneName := ""
	code := strings.ToUpper(strings.TrimSpace(countryCode))
	if a2, ok := countryAlpha3To2[code]; ok {
		code = a2
	}
	if zone, ok := countryTimeZones[code]; ok {
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
	for _, g := range games {
		local := g.At.In(weekLoc)
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
		Games:        len(games),
		Weeks:        len(weeks),
		WindowDays:   int(bnetHabitsWindow / (24 * time.Hour)),
		WeekendShare: float64(weekend) / float64(len(games)),
		TimeZone:     zoneName,
	}
	fillBnetSessionHabits(habits, games)
	if loc != nil {
		habits.TimeOfDay = map[string]float64{}
		for _, slot := range timeOfDayOrder {
			habits.TimeOfDay[slot] = float64(slots[slot]) / float64(len(games))
		}
	}
	habits.Summary = summarizeBnetPlayHabits(habits)
	return habits
}

func fillBnetSessionHabits(h *bnetPlayHabits, games []bnetHabitGame) {
	var gamesPer, minutesPer []int
	for i := 0; i < len(games); {
		start := games[i].At
		end := start.Add(time.Duration(games[i].Seconds) * time.Second)
		j := i + 1
		for ; j < len(games) && games[j].At.Sub(end) <= gamingSessionGap; j++ {
			if e := games[j].At.Add(time.Duration(games[j].Seconds) * time.Second); e.After(end) {
				end = e
			}
		}
		gamesPer = append(gamesPer, j-i)
		minutesPer = append(minutesPer, int(end.Sub(start).Minutes()))
		i = j
	}
	days := games[len(games)-1].At.Sub(games[0].At).Hours() / 24
	h.ObservedDays = max(1, int(math.Round(days)))
	h.SessionsPerWeek = float64(len(gamesPer)) / (float64(h.ObservedDays) / 7)
	h.GamesPerSession = medianOf(gamesPer)
	h.MinutesPerSession = medianOf(minutesPer)
}

func medianOf(xs []int) int {
	sorted := append([]int(nil), xs...)
	sort.Ints(sorted)
	return sorted[len(sorted)/2]
}

func formatSessionLength(minutes int) string {
	minutes = max(5, int(math.Round(float64(minutes)/5))*5)
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes%60 == 0 {
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dh %dm", minutes/60, minutes%60)
}

func formatSessionsPerWeek(rate float64) string {
	switch rounded := math.Round(rate*2) / 2; {
	case rounded < 1:
		return "less than one session a week"
	case rounded == 1:
		return "about one session a week"
	case rounded == math.Trunc(rounded):
		return fmt.Sprintf("about %d sessions a week", int(rounded))
	default:
		return fmt.Sprintf("about %.1f sessions a week", rounded)
	}
}

func timeOfDayPhrase(slot string) string {
	if slot == "night" {
		return "late at night"
	}
	return "in their " + slot + "s"
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
			parts = append(parts, "usually "+timeOfDayPhrase(top))
		case len(slots) > 1 && h.TimeOfDay[top]+h.TimeOfDay[slots[1]] >= 0.7:
			parts = append(parts, timeOfDayPhrase(top)+" and "+timeOfDayPhrase(slots[1]))
		}
	}
	gamesWord := "games"
	if h.GamesPerSession == 1 {
		gamesWord = "game"
	}
	// Games other accounts reported can lack a duration; a session made only
	// of those has no length worth quoting.
	length := ""
	if h.MinutesPerSession > 0 {
		length = " and " + formatSessionLength(h.MinutesPerSession)
	}
	return fmt.Sprintf("Plays %s. Over %d observed days, %s of %d %s%s each, from %d games.",
		strings.Join(parts, ", "), h.ObservedDays, formatSessionsPerWeek(h.SessionsPerWeek),
		h.GamesPerSession, gamesWord, length, h.Games)
}

package db

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
)

// libPlayerScope is which player slots a population aggregate counts. The SQL
// split the same way: the filter-chip and colour queries counted every
// non-observer slot, the players list and its APM/hotkey siblings counted
// human non-observer slots only.
type libPlayerScope string

const (
	libPlayerScopeNonObserver libPlayerScope = "nonobserver"
	libPlayerScopeHuman       libPlayerScope = "human"
)

// libPlayerAggregate is one player key's group in the corpus-wide aggregate
// that every population read is built from, so a page and its total can never
// disagree.
type libPlayerAggregate struct {
	Key         string
	Name        string
	Games       int64
	apmSum      float64
	apmGames    int64
	hotkeyGames int64
	LastPlayed  time.Time
	raceGames   map[library.Race]int64
}

// AverageAPM is AVG(CASE WHEN apm > 0 THEN apm END) coalesced to zero.
func (a *libPlayerAggregate) AverageAPM() float64 {
	if a.apmGames == 0 {
		return 0
	}
	return a.apmSum / float64(a.apmGames)
}

// Race is the dominant race: a share above 0.67 wins, anything else is Random.
func (a *libPlayerAggregate) Race() string {
	if a.Games <= 0 {
		return "Random"
	}
	for _, race := range []library.Race{library.RaceProtoss, library.RaceTerran, library.RaceZerg} {
		if float64(a.raceGames[race])/float64(a.Games) > 0.67 {
			return race.String()
		}
	}
	return "Random"
}

func (s *LibStore) playerAggregates(scope libPlayerScope) []libPlayerAggregate {
	v := s.view()
	return memo(v, "players:aggregate:"+string(scope), func() []libPlayerAggregate {
		byKey := map[string]*libPlayerAggregate{}
		for _, r := range v.Replays() {
			for i := range r.Players {
				p := &r.Players[i]
				if p.IsObserver() {
					continue
				}
				if scope == libPlayerScopeHuman && !p.IsHuman() {
					continue
				}
				aggregate := byKey[p.Key]
				if aggregate == nil {
					aggregate = &libPlayerAggregate{Key: p.Key, Name: p.Name, raceGames: map[library.Race]int64{}}
					byKey[p.Key] = aggregate
				}
				if p.Name < aggregate.Name {
					aggregate.Name = p.Name
				}
				aggregate.Games++
				if p.APM > 0 {
					aggregate.apmSum += float64(p.APM)
					aggregate.apmGames++
				}
				if p.HotkeyStream != nil {
					aggregate.hotkeyGames++
				}
				if r.Date.After(aggregate.LastPlayed) {
					aggregate.LastPlayed = r.Date
				}
				aggregate.raceGames[p.Race]++
			}
		}
		out := make([]libPlayerAggregate, 0, len(byKey))
		for _, aggregate := range byKey {
			out = append(out, *aggregate)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
		return out
	})
}

// playersListDaysAgo is julianday('now') - julianday(substr(last_played,1,19)):
// the stored timestamp's wall clock read as UTC, and the difference truncated
// to whole days.
func playersListDaysAgo(lastPlayed, now time.Time) int64 {
	if lastPlayed.IsZero() {
		return 0
	}
	wall := time.Date(
		lastPlayed.Year(), lastPlayed.Month(), lastPlayed.Day(),
		lastPlayed.Hour(), lastPlayed.Minute(), lastPlayed.Second(), 0, time.UTC,
	)
	return int64(now.UTC().Sub(wall).Hours() / 24)
}

// libPlayersListSortColumns is the whitelist ListWorkflowPlayers interpolated
// into its ORDER BY.
var libPlayersListSortColumns = map[string]func(a, b WorkflowPlayersListRow) int{
	"player_name":          func(a, b WorkflowPlayersListRow) int { return strings.Compare(a.PlayerName, b.PlayerName) },
	"race":                 func(a, b WorkflowPlayersListRow) int { return strings.Compare(a.Race, b.Race) },
	"games_played":         func(a, b WorkflowPlayersListRow) int { return compareInt64(a.GamesPlayed, b.GamesPlayed) },
	"average_apm":          func(a, b WorkflowPlayersListRow) int { return compareFloat64(a.AverageAPM, b.AverageAPM) },
	"last_played_days_ago": func(a, b WorkflowPlayersListRow) int { return compareInt64(a.LastPlayedDaysAgo, b.LastPlayedDaysAgo) },
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func compareFloat64(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// playersListRows is BuildWorkflowPlayersListBaseSQL plus its WHERE: the
// grouped aggregate, the name-substring filter, the five-game floor and the
// last-played buckets. Sorting and paging happen on top of it, so the count,
// the page and the bucket counts are always the same set of rows.
func (s *LibStore) playersListRows(query PlayersQuery) []WorkflowPlayersListRow {
	nameFilter := normalizeKey(query.NameFilter)
	now := time.Now()
	buckets := libLastPlayedBuckets(query.LastPlayed)
	aggregates := s.playerAggregates(libPlayerScopeHuman)
	rows := make([]WorkflowPlayersListRow, 0, len(aggregates))
	for i := range aggregates {
		aggregate := &aggregates[i]
		if nameFilter != "" && !strings.Contains(aggregate.Key, nameFilter) {
			continue
		}
		if query.OnlyFivePlus && aggregate.Games < 5 {
			continue
		}
		daysAgo := playersListDaysAgo(aggregate.LastPlayed, now)
		if len(buckets) > 0 && !anyLastPlayedBucket(buckets, daysAgo) {
			continue
		}
		if !matchesAnyBucket(query.Races, strings.ToLower(strings.TrimSpace(aggregate.Race()))) {
			continue
		}
		if !matchesAnyBucket(query.ApmBuckets, playerApmBucket(aggregate.AverageAPM())) {
			continue
		}
		if !matchesAnyBucket(query.GamesBuckets, playerGamesBucket(aggregate.Games)) {
			continue
		}
		lastPlayed := ""
		if !aggregate.LastPlayed.IsZero() {
			lastPlayed = aggregate.LastPlayed.String()
		}
		rows = append(rows, WorkflowPlayersListRow{
			PlayerKey:         aggregate.Key,
			PlayerName:        aggregate.Name,
			Race:              aggregate.Race(),
			GamesPlayed:       aggregate.Games,
			AverageAPM:        aggregate.AverageAPM(),
			LastPlayed:        lastPlayed,
			LastPlayedDaysAgo: daysAgo,
		})
	}
	return rows
}

// libLastPlayedBuckets mirrors BuildWorkflowPlayersListWhere: recognised
// buckets OR together, unrecognised ones impose nothing.
func libLastPlayedBuckets(lastPlayed []string) []int64 {
	var limits []int64
	for _, bucket := range lastPlayed {
		switch strings.ToLower(strings.TrimSpace(bucket)) {
		case "1m", "30d":
			limits = append(limits, 30)
		case "3m", "90d":
			limits = append(limits, 90)
		}
	}
	return limits
}

func anyLastPlayedBucket(limits []int64, daysAgo int64) bool {
	for _, limit := range limits {
		if daysAgo <= limit {
			return true
		}
	}
	return false
}

func (s *LibStore) CountReplays(ctx context.Context) (int64, error) {
	return int64(s.snapshot().Len()), nil
}

func (s *LibStore) CountPlayers(ctx context.Context, query PlayersQuery) (int64, error) {
	return int64(len(s.playersListRows(query))), nil
}

func (s *LibStore) ListPlayers(ctx context.Context, query PlayersQuery, limit, offset int) ([]WorkflowPlayersListRow, error) {
	rows := s.playersListRows(query)
	compare, ok := libPlayersListSortColumns[strings.ToLower(strings.TrimSpace(query.SortColumn))]
	if !ok {
		compare = libPlayersListSortColumns["games_played"]
	}
	descending := strings.EqualFold(strings.TrimSpace(query.SortDir), "DESC")
	sort.SliceStable(rows, func(i, j int) bool {
		order := compare(rows[i], rows[j])
		if descending {
			order = -order
		}
		if order != 0 {
			return order < 0
		}
		return rows[i].PlayerName < rows[j].PlayerName
	})
	items := []WorkflowPlayersListRow{}
	if limit == 0 || offset >= len(rows) {
		return items, nil
	}
	if offset < 0 {
		offset = 0
	}
	end := len(rows)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return append(items, rows[offset:end]...), nil
}

func (s *LibStore) CountPlayersLastPlayedBuckets(ctx context.Context, query PlayersQuery) (int64, int64, error) {
	var count1m, count3m int64
	for _, row := range s.playersListRows(query) {
		if row.LastPlayedDaysAgo <= 30 {
			count1m++
		}
		if row.LastPlayedDaysAgo <= 90 {
			count3m++
		}
	}
	return count1m, count3m, nil
}

func (s *LibStore) ListPlayerApmAggregates(ctx context.Context, minGames int64) ([]PlayerApmAggregateRow, error) {
	aggregates := s.playerAggregates(libPlayerScopeHuman)
	out := make([]PlayerApmAggregateRow, 0, len(aggregates))
	for i := range aggregates {
		aggregate := &aggregates[i]
		averageAPM := aggregate.AverageAPM()
		if aggregate.Games < minGames || averageAPM <= 0 {
			continue
		}
		out = append(out, PlayerApmAggregateRow{
			PlayerKey:   aggregate.Key,
			PlayerName:  aggregate.Name,
			AverageAPM:  averageAPM,
			GamesPlayed: aggregate.Games,
		})
	}
	return out, nil
}

func (s *LibStore) ListHotkeyGamesRateByPlayer(ctx context.Context) (map[string]float64, error) {
	aggregates := s.playerAggregates(libPlayerScopeHuman)
	out := make(map[string]float64, len(aggregates))
	for i := range aggregates {
		aggregate := &aggregates[i]
		if aggregate.Games == 0 {
			out[aggregate.Key] = 0
			continue
		}
		out[aggregate.Key] = float64(aggregate.hotkeyGames) * 100.0 / float64(aggregate.Games)
	}
	return out, nil
}

func (s *LibStore) ListTopPlayerColorRows(ctx context.Context) ([]PlayerColorRow, error) {
	aggregates := s.playerAggregates(libPlayerScopeNonObserver)
	result := make([]PlayerColorRow, 0, len(aggregates))
	for i := range aggregates {
		result = append(result, PlayerColorRow{PlayerKey: aggregates[i].Key, Games: aggregates[i].Games})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Games != result[j].Games {
			return result[i].Games > result[j].Games
		}
		return result[i].PlayerKey < result[j].PlayerKey
	})
	if len(result) > 15 {
		result = result[:15]
	}
	return result, nil
}

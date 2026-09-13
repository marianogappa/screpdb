package db

import (
	"context"
	"strings"
)

// Player facet bucket keys. They are stable identifiers, not labels: the UI
// translates them, and the API accepts them verbatim.
const (
	PlayerApmUnder60  = "lt60"
	PlayerApm60To120  = "60_120"
	PlayerApm120To200 = "120_200"
	PlayerApm200Plus  = "gte200"
	PlayerGames1To4   = "1_4"
	PlayerGames5To19  = "5_19"
	PlayerGames20Plus = "gte20"
)

// PlayerApmBuckets and PlayerGamesBuckets are the offered options, in the order
// the UI shows them.
var (
	PlayerApmBuckets   = []string{PlayerApmUnder60, PlayerApm60To120, PlayerApm120To200, PlayerApm200Plus}
	PlayerGamesBuckets = []string{PlayerGames1To4, PlayerGames5To19, PlayerGames20Plus}
)

// PlayerFacetCounts is how many players each option would match.
type PlayerFacetCounts struct {
	Races map[string]int64
	Apm   map[string]int64
	Games map[string]int64
}

func playerApmBucket(apm float64) string {
	switch {
	case apm < 60:
		return PlayerApmUnder60
	case apm < 120:
		return PlayerApm60To120
	case apm < 200:
		return PlayerApm120To200
	}
	return PlayerApm200Plus
}

func playerGamesBucket(games int64) string {
	switch {
	case games < 5:
		return PlayerGames1To4
	case games < 20:
		return PlayerGames5To19
	}
	return PlayerGames20Plus
}

// matchesAnyBucket reports whether value is selected. An empty selection
// imposes nothing, which is what makes the facets AND across and OR within.
func matchesAnyBucket(selected []string, value string) bool {
	if len(selected) == 0 {
		return true
	}
	for _, key := range selected {
		if strings.EqualFold(strings.TrimSpace(key), value) {
			return true
		}
	}
	return false
}

// CountPlayerFacets counts each option against the rest of the query with that
// option's own facet cleared.
//
// Counting a facet under its own filter is what makes a filter bar a dead end:
// pick Zerg and every other race reads zero, so the one thing the counts are
// there for — seeing what else you could pick — stops working. Clearing the
// facet first keeps every option's count answering "how many would I get if I
// chose this instead".
func (s *LibStore) CountPlayerFacets(_ context.Context, query PlayersQuery) (PlayerFacetCounts, error) {
	counts := PlayerFacetCounts{
		Races: map[string]int64{},
		Apm:   map[string]int64{},
		Games: map[string]int64{},
	}

	withoutRaces := query
	withoutRaces.Races = nil
	for _, row := range s.playersListRows(withoutRaces) {
		if race := strings.ToLower(strings.TrimSpace(row.Race)); race != "" {
			counts.Races[race]++
		}
	}

	withoutApm := query
	withoutApm.ApmBuckets = nil
	for _, row := range s.playersListRows(withoutApm) {
		counts.Apm[playerApmBucket(row.AverageAPM)]++
	}

	withoutGames := query
	withoutGames.GamesBuckets = nil
	for _, row := range s.playersListRows(withoutGames) {
		counts.Games[playerGamesBucket(row.GamesPlayed)]++
	}

	return counts, nil
}

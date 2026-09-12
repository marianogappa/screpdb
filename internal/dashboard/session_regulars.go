package dashboard

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Regulars are the people who keep turning up in the user's games. The window
// is long enough that a monthly opponent still counts, the floor is high enough
// that someone met once in a public lobby does not, and the cap keeps the list
// scannable.
const (
	regularsWindow   = 180 * 24 * time.Hour
	regularsMinGames = 5
	regularsMax      = 20
)

// regularsRecently is how fresh a person's newest known game must be before we
// say they played recently. It is deliberately not called "online": nothing in
// the data says anyone is logged in. A game is only known once it has been
// played and published, so this always describes the past.
const regularsRecently = time.Hour

// sessionRegular is one person the user plays with often. Identity is merged
// across accounts, so a regular is a human, not a toon.
type sessionRegular struct {
	PlayerKey      string             `json:"player_key"`
	PlayerName     string             `json:"player_name"`
	CountryCode    string             `json:"country_code,omitempty"`
	Games          int                `json:"games"`
	Accounts       []string           `json:"accounts,omitempty"`
	LastPlayedWith string             `json:"last_played_with"`
	LastSeen       string             `json:"last_seen,omitempty"`
	PlayedRecently bool               `json:"played_recently"`
	Profile        *bnetProfileDetail `json:"profile,omitempty"`
}

// regularsIdentity is one merged human, accumulated across their toons.
type regularsIdentity struct {
	auroraID   int64
	names      map[string]int
	games      int
	lastPlayed time.Time
	topKey     string
	topGames   int
}

// mergeCoPlayersByAccount folds the raw per-toon counts into one entry per
// human. Two toons merge when Battle.net says they belong to the same aurora
// account; a toon with no cached profile stands alone, which is the safe
// failure — an unmerged regular is merely counted twice, while a wrong merge
// would attribute someone's games to a stranger.
func mergeCoPlayersByAccount(rows []coPlayerCount, auroraByKey map[string]int64) []*regularsIdentity {
	byGroup := map[string]*regularsIdentity{}
	order := []*regularsIdentity{}
	for _, row := range rows {
		key := normalizePlayerKey(row.PlayerKey)
		if key == "" {
			continue
		}
		auroraID := auroraByKey[key]
		group := key
		if auroraID != 0 {
			group = "aurora:" + strconv.FormatInt(auroraID, 10)
		}
		identity, ok := byGroup[group]
		if !ok {
			identity = &regularsIdentity{auroraID: auroraID, names: map[string]int{}}
			byGroup[group] = identity
			order = append(order, identity)
		}
		identity.names[row.PlayerName] += row.Games
		identity.games += row.Games
		if row.LastPlayed.After(identity.lastPlayed) {
			identity.lastPlayed = row.LastPlayed
		}
		if row.Games > identity.topGames {
			identity.topKey, identity.topGames = key, row.Games
		}
	}
	return order
}

// coPlayerCount is the dashboard-side shape of one co-player tally, so the
// merge logic is testable without a store.
type coPlayerCount struct {
	PlayerKey  string
	PlayerName string
	Games      int
	LastPlayed time.Time
}

// displayName picks the name a regular is best known by: the one they used for
// the most games, so a brief alt-account detour does not rename them.
func (r *regularsIdentity) displayName() string {
	best, bestGames := "", -1
	for name, games := range r.names {
		if games > bestGames || (games == bestGames && strings.ToLower(name) < strings.ToLower(best)) {
			best, bestGames = name, games
		}
	}
	return best
}

// otherNames lists the regular's other account names, most used first, so the
// UI can show that "Tanchik" and "Lemner" are one person.
func (r *regularsIdentity) otherNames() []string {
	primary := strings.ToLower(r.displayName())
	names := make([]string, 0, len(r.names))
	for name := range r.names {
		if strings.ToLower(name) != primary {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		if r.names[names[i]] != r.names[names[j]] {
			return r.names[names[i]] > r.names[names[j]]
		}
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names
}

// rankRegulars applies the floor and the cap: the people seen often enough to
// be worth listing, most played first, ties broken by who was seen last.
func rankRegulars(identities []*regularsIdentity) []*regularsIdentity {
	kept := make([]*regularsIdentity, 0, len(identities))
	for _, identity := range identities {
		if identity.games >= regularsMinGames {
			kept = append(kept, identity)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool {
		if kept[i].games != kept[j].games {
			return kept[i].games > kept[j].games
		}
		if !kept[i].lastPlayed.Equal(kept[j].lastPlayed) {
			return kept[i].lastPlayed.After(kept[j].lastPlayed)
		}
		return strings.ToLower(kept[i].displayName()) < strings.ToLower(kept[j].displayName())
	})
	if len(kept) > regularsMax {
		kept = kept[:regularsMax]
	}
	return kept
}

// sessionRegulars builds the ranked, identity-merged list of people the user
// plays with, annotated with how recently each was last seen in a game.
func (d *Dashboard) sessionRegulars(ctx context.Context, youKeys map[string]struct{}, now time.Time) ([]sessionRegular, error) {
	if len(youKeys) == 0 {
		return []sessionRegular{}, nil
	}
	rows, err := d.dbStore.ListCoPlayerCounts(ctx, sortedKeys(youKeys), now.Add(-regularsWindow))
	if err != nil {
		return nil, err
	}
	counts := make([]coPlayerCount, 0, len(rows))
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		counts = append(counts, coPlayerCount{
			PlayerKey:  row.PlayerKey,
			PlayerName: row.PlayerName,
			Games:      row.Games,
			LastPlayed: row.LastPlayed,
		})
		keys = append(keys, row.PlayerKey)
	}

	identities := rankRegulars(mergeCoPlayersByAccount(counts, d.auroraIDsByPlayerKey(ctx, keys)))

	out := make([]sessionRegular, 0, len(identities))
	for _, identity := range identities {
		regular := sessionRegular{
			PlayerKey:      identity.topKey,
			PlayerName:     identity.displayName(),
			Games:          identity.games,
			Accounts:       identity.otherNames(),
			LastPlayedWith: identity.lastPlayed.Format(time.RFC3339),
		}
		lastSeen := identity.lastPlayed
		if fresh, ok := d.bnetLastGameAt(ctx, identity.auroraID); ok && fresh.After(lastSeen) {
			lastSeen = fresh
		}
		regular.LastSeen = lastSeen.Format(time.RFC3339)
		regular.PlayedRecently = now.Sub(lastSeen) <= regularsRecently
		out = append(out, regular)
	}
	return d.withRegularProfiles(ctx, out), nil
}

// auroraIDsByPlayerKey maps each toon name we know to the Battle.net account it
// belongs to. Both the fetched toon and every alternate account listed on that
// profile are registered, which is what lets an alt merge into its owner even
// when only one of the two was ever fetched.
func (d *Dashboard) auroraIDsByPlayerKey(ctx context.Context, playerKeys []string) map[string]int64 {
	out := map[string]int64{}
	if len(playerKeys) == 0 {
		return out
	}
	profiles, err := d.dbStore.ListBnetProfilesByPlayerKeys(ctx, playerKeys)
	if err != nil {
		return out
	}
	for _, profile := range profiles {
		if profile.AuroraID == 0 {
			continue
		}
		if key := normalizePlayerKey(profile.Toon); key != "" {
			out[key] = profile.AuroraID
		}
		for _, toon := range profile.Toons {
			if key := normalizePlayerKey(toon.Toon); key != "" {
				out[key] = profile.AuroraID
			}
		}
	}
	return out
}

// bnetLastGameAt reports the newest game Battle.net has published for an
// account. It reads the local archive only, so it costs nothing; the archive is
// refreshed by the poll in session_regulars_refresh.go.
func (d *Dashboard) bnetLastGameAt(ctx context.Context, auroraID int64) (time.Time, bool) {
	if auroraID == 0 {
		return time.Time{}, false
	}
	times, err := d.dbStore.ListBnetGameTimes(ctx, auroraID, time.Time{})
	if err != nil || len(times) == 0 {
		return time.Time{}, false
	}
	newest := times[0]
	for _, t := range times[1:] {
		if t.After(newest) {
			newest = t
		}
	}
	return newest, true
}

func (d *Dashboard) withRegularProfiles(ctx context.Context, regulars []sessionRegular) []sessionRegular {
	if len(regulars) == 0 {
		return regulars
	}
	keys := make([]string, 0, len(regulars))
	for _, regular := range regulars {
		keys = append(keys, regular.PlayerKey)
	}
	details := d.bnetProfileDetailsByPlayerKeys(ctx, keys)
	for i := range regulars {
		detail, ok := details[regulars[i].PlayerKey]
		if !ok {
			continue
		}
		d.fillLocalPlayerKeys(ctx, detail)
		regulars[i].Profile = detail
		if regulars[i].CountryCode == "" {
			regulars[i].CountryCode = detail.CountryCode
		}
	}
	return regulars
}

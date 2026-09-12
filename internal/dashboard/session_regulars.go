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

// Freshness has two tiers, and both are stated as the past tense of a game,
// never as presence: nothing in this data says anyone is logged in. A game is
// only known once it has been played and published.
//
//	regularsPlayingNow  someone who finished a game this recently is very likely
//	                    still at the keyboard, which is the whole point of the
//	                    surface: it is worth starting a session for.
//	regularsPlayedLately the fallback when nobody is around right now, so the
//	                    page still says who is alive at all rather than going
//	                    blank.
const (
	regularsPlayingNow   = time.Hour
	regularsPlayedLately = 3 * 24 * time.Hour
)

// Freshness tiers, as carried to the frontend. The empty string means the
// person is not fresh enough to mention at all.
const (
	regularFreshnessNow    = "now"
	regularFreshnessLately = "lately"
)

func regularFreshness(lastSeen, now time.Time) string {
	switch age := now.Sub(lastSeen); {
	case age <= regularsPlayingNow:
		return regularFreshnessNow
	case age <= regularsPlayedLately:
		return regularFreshnessLately
	}
	return ""
}

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
	Freshness      string             `json:"freshness,omitempty"`
	Profile        *bnetProfileDetail `json:"profile,omitempty"`

	// refreshToon and gateway address the one account of this person we know
	// answers. Kept out of the payload: they only serve the refresh sweep.
	refreshToon string
	gateway     int64
}

// regularsIdentity is one merged human, accumulated across their toons.
type regularsIdentity struct {
	auroraID   int64
	names      map[string]int
	games      int
	lastPlayed time.Time
	topKey     string
	topGames   int
	// refreshToon and gateway are any one of this person's toons we have
	// actually reached, which is not necessarily the one they play most: an
	// identity is often merged from a fetched account onto an unfetched alt.
	refreshToon string
	gateway     int64
}

// mergeCoPlayersByAccount folds the raw per-toon counts into one entry per
// human. Two toons merge when Battle.net says they belong to the same aurora
// account; a toon with no cached profile stands alone, which is the safe
// failure — an unmerged regular is merely counted twice, while a wrong merge
// would attribute someone's games to a stranger.
func mergeCoPlayersByAccount(rows []coPlayerCount, index regularsIdentityIndex) []*regularsIdentity {
	auroraByKey := index.auroraByKey
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
		if identity.gateway == 0 {
			if gateway, ok := index.gatewayByKey[key]; ok && gateway != 0 {
				identity.refreshToon, identity.gateway = key, gateway
			}
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

	index := d.identityIndex(ctx, keys)
	identities := rankRegulars(mergeCoPlayersByAccount(counts, index))

	out := make([]sessionRegular, 0, len(identities))
	for _, identity := range identities {
		regular := sessionRegular{
			PlayerKey:      identity.topKey,
			PlayerName:     identity.displayName(),
			Games:          identity.games,
			Accounts:       identity.otherNames(),
			LastPlayedWith: identity.lastPlayed.Format(time.RFC3339),
			refreshToon:    identity.refreshToon,
			gateway:        identity.gateway,
		}
		lastSeen := identity.lastPlayed
		if fresh, ok := d.bnetLastGameAt(ctx, identity.auroraID); ok && fresh.After(lastSeen) {
			lastSeen = fresh
		}
		regular.LastSeen = lastSeen.Format(time.RFC3339)
		regular.Freshness = regularFreshness(lastSeen, now)
		out = append(out, regular)
	}
	return d.withRegularProfiles(ctx, out), nil
}

// regularsIdentityIndex is what the cached Battle.net profiles tell us about a
// set of toons: which account each belongs to, and which gateway we last
// reached it on.
type regularsIdentityIndex struct {
	auroraByKey  map[string]int64
	gatewayByKey map[string]int64
}

// identityIndex maps each toon name we know to the Battle.net account it
// belongs to. Both the fetched toon and every alternate account listed on that
// profile are registered, which is what lets an alt merge into its owner even
// when only one of the two was ever fetched; the gateway is recorded only for
// toons actually fetched, since that is the only one we know answers.
func (d *Dashboard) identityIndex(ctx context.Context, playerKeys []string) regularsIdentityIndex {
	index := regularsIdentityIndex{auroraByKey: map[string]int64{}, gatewayByKey: map[string]int64{}}
	if len(playerKeys) == 0 {
		return index
	}
	profiles, err := d.dbStore.ListBnetProfilesByPlayerKeys(ctx, playerKeys)
	if err != nil {
		return index
	}
	for _, profile := range profiles {
		if profile.AuroraID == 0 {
			continue
		}
		if key := normalizePlayerKey(profile.Toon); key != "" {
			index.auroraByKey[key] = profile.AuroraID
			index.gatewayByKey[key] = profile.Gateway
		}
		for _, toon := range profile.Toons {
			if key := normalizePlayerKey(toon.Toon); key != "" {
				index.auroraByKey[key] = profile.AuroraID
			}
		}
	}
	return index
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

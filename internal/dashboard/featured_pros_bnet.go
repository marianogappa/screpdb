package dashboard

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/library/persist"
	"github.com/marianogappa/screpdb/internal/propack"
)

// featuredProVisitMaxAge is how old a pro's newest profile answer may be before
// opening their page asks again. A profile carries the account's last 25 games,
// so this bounds how far behind "Recent games" can be while someone is looking,
// matching the session view's regulars sweep.
const featuredProVisitMaxAge = 30 * time.Minute

type featuredProRefreshState struct {
	mu        sync.Mutex
	inflight  map[string]bool
	attempted map[bnetToonGateway]time.Time
	// learned holds, per pro, the toons that answered for an account after a
	// pack toon stopped doing so. A renamed toon drops out of its account's
	// list, so nothing cached links the pro to the account any more; this
	// does until the pack is rebaked with the new name.
	learned map[string][]string
	// fetching runs the refreshes one request at a time: the bridge answers
	// serially, so pros refreshing side by side queue past its timeout and
	// every one of them fails.
	fetching sync.Mutex
}

type bnetToonGateway struct {
	toon    string
	gateway int64
}

// bnetRefreshTarget is one request plus the toons to try when it answers
// "not found": a toon can be renamed while its account lives on, and asking an
// alternate is how the account's games keep arriving.
type bnetRefreshTarget struct {
	bnetToonGateway
	alternates []bnetToonGateway
}

const featuredProMaxAlternates = 3

// featuredProBnetProfiles returns the cached rows that speak for the pro: the
// rows of the pack's toons, misses included, plus every found profile that
// lists one of them among its account's toons. The latter keeps an account in
// view after one of its toons is renamed, and counts a profile fetched for an
// alias (by the session sweep, say) as the fresh answer about the account it is.
func (d *Dashboard) featuredProBnetProfiles(ctx context.Context, pro *propack.Pro) []persist.BnetProfile {
	names := make([]string, 0, len(pro.Toons))
	for _, toon := range pro.Toons {
		names = append(names, toon.Toon)
	}
	d.featuredRefresh.mu.Lock()
	names = append(names, d.featuredRefresh.learned[pro.ID]...)
	d.featuredRefresh.mu.Unlock()
	keys := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		if key := normalizePlayerKey(name); key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	direct, err := d.dbStore.ListBnetProfilesByPlayerKeys(ctx, keys)
	if err != nil {
		return nil
	}
	listing, err := d.dbStore.ListBnetProfilesListingPlayerKeys(ctx, keys)
	if err != nil {
		return direct
	}
	profiles := direct
	have := map[bnetToonGateway]bool{}
	for _, p := range direct {
		have[bnetToonGateway{p.Toon, p.Gateway}] = true
	}
	for _, p := range listing {
		if key := (bnetToonGateway{p.Toon, p.Gateway}); !have[key] {
			have[key] = true
			profiles = append(profiles, p)
		}
	}
	return profiles
}

// featuredProRefreshTargets picks the requests that would bring the pro's
// picture up to maxAge. Battle.net reports games per account, so one request
// per account is enough however many toons it holds; a pack toon that no cached
// account owns is asked on its own, since it may be an account we have never
// seen. A toon Battle.net does not have keeps that answer for the usual 24h.
// Toons are compared exactly: Battle.net tells I from l, so lowercasing would
// let a miss for one spelling stand in for a found row of another.
func featuredProRefreshTargets(toons []propack.Toon, profiles []persist.BnetProfile, now time.Time, maxAge time.Duration) []bnetRefreshTarget {
	type account struct {
		freshest time.Time
		row      bnetToonGateway
		toons    []persist.BnetToon
		pack     bnetToonGateway
		packAt   time.Time
	}
	accounts := map[int64]*account{}
	owned := map[bnetToonGateway]bool{}
	rows := map[bnetToonGateway]persist.BnetProfile{}
	for _, p := range profiles {
		rows[bnetToonGateway{strings.TrimSpace(p.Toon), p.Gateway}] = p
		if !p.Found || p.AuroraID == 0 {
			continue
		}
		a := accounts[p.AuroraID]
		if a == nil {
			a = &account{}
			accounts[p.AuroraID] = a
		}
		if a.row.toon == "" || p.FetchedAt.After(a.freshest) {
			a.freshest = p.FetchedAt
			a.row = bnetToonGateway{strings.TrimSpace(p.Toon), p.Gateway}
			a.toons = p.Toons
		}
		for _, t := range p.Toons {
			owned[bnetToonGateway{strings.TrimSpace(t.Toon), int64(t.Gateway)}] = true
		}
	}
	var targets []bnetRefreshTarget
	for _, toon := range toons {
		if _, ok := bnetfacade.GatewayNames[toon.Gateway]; !ok || strings.TrimSpace(toon.Toon) == "" {
			continue
		}
		key := bnetToonGateway{strings.TrimSpace(toon.Toon), int64(toon.Gateway)}
		row, cached := rows[key]
		if cached && row.Found && row.AuroraID != 0 {
			a := accounts[row.AuroraID]
			if a.pack.toon == "" || row.FetchedAt.After(a.packAt) {
				a.pack, a.packAt = key, row.FetchedAt
			}
			continue
		}
		if owned[key] {
			continue
		}
		if cached && now.Sub(row.FetchedAt) < bnetProfileTTL {
			continue
		}
		targets = append(targets, bnetRefreshTarget{bnetToonGateway: key})
	}
	ids := make([]int64, 0, len(accounts))
	for id := range accounts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		a := accounts[id]
		if now.Sub(a.freshest) < maxAge {
			continue
		}
		target := bnetRefreshTarget{bnetToonGateway: a.pack}
		if target.toon == "" {
			target.bnetToonGateway = a.row
		}
		alternates := append([]persist.BnetToon(nil), a.toons...)
		sort.SliceStable(alternates, func(i, j int) bool { return alternates[i].GamesLastWeek > alternates[j].GamesLastWeek })
		for _, t := range alternates {
			alt := bnetToonGateway{strings.TrimSpace(t.Toon), int64(t.Gateway)}
			if _, ok := bnetfacade.GatewayNames[t.Gateway]; !ok || alt.toon == "" || alt == target.bnetToonGateway {
				continue
			}
			if row, cached := rows[alt]; cached && !row.Found && now.Sub(row.FetchedAt) < bnetProfileTTL {
				continue
			}
			if len(target.alternates) == featuredProMaxAlternates {
				break
			}
			target.alternates = append(target.alternates, alt)
		}
		targets = append(targets, target)
	}
	return targets
}

// refreshFeaturedProBnet starts, in the background, the requests that bring a
// pro's accounts up to maxAge, and reports whether a refresh for this pro is in
// flight. Each request is tried at most once per maxAge whatever its outcome,
// so a page that reloads itself when the refresh ends cannot turn a refused or
// failing request into a loop. The end is announced as an observation either
// way, which is what tells an open page to stop showing the refresh.
func (d *Dashboard) refreshFeaturedProBnet(pro *propack.Pro, profiles []persist.BnetProfile, prio bnetfacade.Priority, maxAge time.Duration) bool {
	if d.bnetDisabled.Load() {
		return false
	}
	if addr, _ := d.bnetAddr.Load().(string); addr == "" {
		return false
	}
	now := time.Now()
	s := &d.featuredRefresh
	s.mu.Lock()
	if s.inflight[pro.ID] {
		s.mu.Unlock()
		return true
	}
	var pending []bnetRefreshTarget
	for _, target := range featuredProRefreshTargets(pro.Toons, profiles, now, maxAge) {
		if at, ok := s.attempted[target.bnetToonGateway]; ok && now.Sub(at) < maxAge {
			continue
		}
		pending = append(pending, target)
	}
	if len(pending) == 0 {
		s.mu.Unlock()
		return false
	}
	if s.inflight == nil {
		s.inflight = map[string]bool{}
		s.attempted = map[bnetToonGateway]time.Time{}
		s.learned = map[string][]string{}
	}
	for key, at := range s.attempted {
		if now.Sub(at) >= bnetProfileTTL {
			delete(s.attempted, key)
		}
	}
	for _, target := range pending {
		s.attempted[target.bnetToonGateway] = now
	}
	s.inflight[pro.ID] = true
	s.mu.Unlock()

	d.bnetBackfillActive.Add(1)
	go func() {
		defer crashreport.GuardNonFatal(nil)
		defer func() {
			s.mu.Lock()
			delete(s.inflight, pro.ID)
			s.mu.Unlock()
			d.bnetBackfillActive.Add(-1)
			if d.libraryHub != nil {
				d.libraryHub.publishObservation()
			}
		}()
		for _, target := range pending {
			for i, ask := range append([]bnetToonGateway{target.bnetToonGateway}, target.alternates...) {
				s.fetching.Lock()
				res, err := d.getOrFetchBnetProfile(d.ctx, ask.toon, ask.gateway, prio, maxAge)
				s.fetching.Unlock()
				if err == nil && res.Found && i > 0 {
					s.mu.Lock()
					s.learned[pro.ID] = append(s.learned[pro.ID], ask.toon)
					s.mu.Unlock()
				}
				if err != nil || res.Found || res.Stale {
					break
				}
			}
		}
	}()
	return true
}

// bnetProfileGamesListed is how many games the page lists: a profile answer
// holds each toon's last 25, so a toon that went quiet months ago would
// otherwise trail the account's current games with its old ones.
const bnetProfileGamesListed = 25

// latestListedGames keeps the newest games of the profile's latest answer,
// from the account's archived games sorted newest first.
func latestListedGames(p persist.BnetProfile, archived []persist.BnetGame) []persist.BnetGame {
	n := 0
	for n < len(archived) && n < bnetProfileGamesListed && !archived[n].CreateTime.Before(p.ListedFrom) {
		n++
	}
	return archived[:n]
}

// mergeFeaturedBnetDetail builds one Battle.net section out of all of a pro's
// accounts. The account that played most recently leads: its ratings, totals
// and recent games are the page's, and the games are the newest of its latest
// answer, so an older account's games or ones that answer has rolled off never
// pad the list with months-old play. Every account's toons still show as
// aliases, most recently played first.
func mergeFeaturedBnetDetail(profiles []persist.BnetProfile, gamesByAccount map[int64][]persist.BnetGame) *bnetProfileDetail {
	byAccount := map[int64]persist.BnetProfile{}
	for _, p := range profiles {
		if !p.Found || p.AuroraID == 0 {
			continue
		}
		if existing, ok := byAccount[p.AuroraID]; ok && !p.FetchedAt.After(existing.FetchedAt) {
			continue
		}
		byAccount[p.AuroraID] = p
	}
	if len(byAccount) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(byAccount))
	for id := range byAccount {
		ids = append(ids, id)
	}
	latestGame := func(id int64) time.Time {
		var latest time.Time
		for _, g := range gamesByAccount[id] {
			if g.CreateTime.After(latest) {
				latest = g.CreateTime
			}
		}
		return latest
	}
	sort.Slice(ids, func(i, j int) bool {
		li, lj := latestGame(ids[i]), latestGame(ids[j])
		if !li.Equal(lj) {
			return li.After(lj)
		}
		fi, fj := byAccount[ids[i]].FetchedAt, byAccount[ids[j]].FetchedAt
		if !fi.Equal(fj) {
			return fi.After(fj)
		}
		return ids[i] < ids[j]
	})
	lead := byAccount[ids[0]]
	lead.Toons = append([]persist.BnetToon(nil), lead.Toons...)
	for _, id := range ids[1:] {
		lead.Toons = append(lead.Toons, byAccount[id].Toons...)
	}
	archived := append([]persist.BnetGame(nil), gamesByAccount[ids[0]]...)
	sort.SliceStable(archived, func(i, j int) bool { return archived[i].CreateTime.After(archived[j].CreateTime) })
	games := latestListedGames(lead, archived)
	detail := bnetProfileDetailFromRecord(lead, games)
	if len(archived) > 0 {
		detail.LastPlayedAt = archived[0].CreateTime.UTC().Format(time.RFC3339)
	}
	var allGames []bnetRecentGame
	for _, id := range ids {
		allGames = append(allGames, bnetRecentGamesFromArchive(lead, gamesByAccount[id])...)
	}
	sortBnetToonsByRecency(detail.Toons, allGames)
	return detail
}

// featuredProSweepEvery paces the background refresh of the pros nobody has
// opened: one pro per tick, at background priority and Blizzard's 24h
// freshness, so the whole curated set costs a few dozen requests a day and
// never queues ahead of anything the user asked for.
const featuredProSweepEvery = 10 * time.Minute

func (d *Dashboard) startFeaturedProSweep(ctx context.Context) {
	go func() {
		defer crashreport.GuardNonFatal(nil)
		ticker := time.NewTicker(featuredProSweepEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.sweepOneFeaturedPro()
			}
		}
	}()
}

func (d *Dashboard) sweepOneFeaturedPro() bool {
	if d.bnetDisabled.Load() {
		return false
	}
	if addr, _ := d.bnetAddr.Load().(string); addr == "" {
		return false
	}
	for _, pro := range d.featuredPros() {
		if d.refreshFeaturedProBnet(pro, d.featuredProBnetProfiles(d.ctx, pro), bnetfacade.PriorityBackground, bnetProfileTTL) {
			return true
		}
	}
	return false
}

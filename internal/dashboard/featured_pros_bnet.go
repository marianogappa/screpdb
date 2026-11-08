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
}

type bnetToonGateway struct {
	toon    string
	gateway int64
}

// featuredProBnetProfiles returns every cached profile row of the pro's
// accounts: the rows of the pack's toons, plus rows of any other toon those
// accounts own, since a profile fetched for an alias (by the session sweep, say)
// is just as fresh an answer about the account.
func (d *Dashboard) featuredProBnetProfiles(ctx context.Context, pro *propack.Pro) []persist.BnetProfile {
	keys := make([]string, 0, len(pro.Toons))
	seen := map[string]bool{}
	for _, toon := range pro.Toons {
		if key := normalizePlayerKey(toon.Toon); key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	profiles, err := d.dbStore.ListBnetProfilesByPlayerKeys(ctx, keys)
	if err != nil {
		return nil
	}
	accounts := map[int64]bool{}
	var aliasKeys []string
	for _, p := range profiles {
		if !p.Found || p.AuroraID == 0 {
			continue
		}
		accounts[p.AuroraID] = true
		for _, t := range p.Toons {
			if key := normalizePlayerKey(t.Toon); key != "" && !seen[key] {
				seen[key] = true
				aliasKeys = append(aliasKeys, key)
			}
		}
	}
	if len(aliasKeys) == 0 {
		return profiles
	}
	aliases, err := d.dbStore.ListBnetProfilesByPlayerKeys(ctx, aliasKeys)
	if err != nil {
		return profiles
	}
	for _, p := range aliases {
		if p.Found && accounts[p.AuroraID] {
			profiles = append(profiles, p)
		}
	}
	return profiles
}

// featuredProRefreshTargets picks the (toon, gateway) requests that would bring
// the pro's picture up to maxAge. Battle.net reports games per account, so one
// request per account is enough however many toons it holds; a pack toon that
// no cached account owns is asked on its own, since it may be an account we
// have never seen. A toon Battle.net does not have keeps that answer for the
// usual 24h.
func featuredProRefreshTargets(toons []propack.Toon, profiles []persist.BnetProfile, now time.Time, maxAge time.Duration) []bnetToonGateway {
	type account struct {
		freshest time.Time
		fallback bnetToonGateway
		toon     bnetToonGateway
		toonAt   time.Time
	}
	accounts := map[int64]*account{}
	owned := map[bnetToonGateway]bool{}
	rows := map[bnetToonGateway]persist.BnetProfile{}
	for _, p := range profiles {
		rows[bnetToonGateway{normalizePlayerKey(p.Toon), p.Gateway}] = p
		if !p.Found || p.AuroraID == 0 {
			continue
		}
		a := accounts[p.AuroraID]
		if a == nil {
			a = &account{}
			accounts[p.AuroraID] = a
		}
		if a.fallback.toon == "" || p.FetchedAt.After(a.freshest) {
			a.freshest = p.FetchedAt
			a.fallback = bnetToonGateway{strings.TrimSpace(p.Toon), p.Gateway}
		}
		for _, t := range p.Toons {
			owned[bnetToonGateway{normalizePlayerKey(t.Toon), int64(t.Gateway)}] = true
		}
	}
	var targets []bnetToonGateway
	for _, toon := range toons {
		gateway := int64(toon.Gateway)
		if _, ok := bnetfacade.GatewayNames[toon.Gateway]; !ok || strings.TrimSpace(toon.Toon) == "" {
			continue
		}
		key := bnetToonGateway{normalizePlayerKey(toon.Toon), gateway}
		row, cached := rows[key]
		if cached && row.Found && row.AuroraID != 0 {
			a := accounts[row.AuroraID]
			if a.toon.toon == "" || row.FetchedAt.After(a.toonAt) {
				a.toon, a.toonAt = bnetToonGateway{strings.TrimSpace(toon.Toon), gateway}, row.FetchedAt
			}
			continue
		}
		if owned[key] {
			continue
		}
		if cached && now.Sub(row.FetchedAt) < bnetProfileTTL {
			continue
		}
		targets = append(targets, bnetToonGateway{strings.TrimSpace(toon.Toon), gateway})
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
		if a.toon.toon == "" {
			a.toon = a.fallback
		}
		targets = append(targets, a.toon)
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
	var pending []bnetToonGateway
	for _, target := range featuredProRefreshTargets(pro.Toons, profiles, now, maxAge) {
		key := bnetToonGateway{normalizePlayerKey(target.toon), target.gateway}
		if at, ok := s.attempted[key]; ok && now.Sub(at) < maxAge {
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
	}
	for key, at := range s.attempted {
		if now.Sub(at) >= bnetProfileTTL {
			delete(s.attempted, key)
		}
	}
	for _, target := range pending {
		s.attempted[bnetToonGateway{normalizePlayerKey(target.toon), target.gateway}] = now
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
			_, _ = d.getOrFetchBnetProfile(d.ctx, target.toon, target.gateway, prio, maxAge)
		}
	}()
	return true
}

// mergeFeaturedBnetDetail builds one Battle.net section out of all of a pro's
// accounts. The account that played most recently leads with its ratings and
// totals; its alias list and recent games are the union of every account's,
// newest first, so moving to a new account does not leave the page reading
// the old one.
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
	var games []persist.BnetGame
	seenGames := map[string]bool{}
	for _, id := range ids {
		if id != ids[0] {
			lead.Toons = append(lead.Toons, byAccount[id].Toons...)
		}
		for _, g := range gamesByAccount[id] {
			if g.GameID != "" && seenGames[g.GameID] {
				continue
			}
			seenGames[g.GameID] = true
			games = append(games, g)
		}
	}
	return bnetProfileDetailFromRecord(lead, games)
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

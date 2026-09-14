package dashboard

import (
	"sync/atomic"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/crashreport"
)

// regularsRefreshEvery bounds how often opening the session view re-asks
// Battle.net what the user's regulars have been playing. A sweep costs one
// bridge request per regular, so at the cap that is 20 paced requests; half an
// hour keeps a page the user leaves open from turning into a poll while still
// being fresher than the regularsRecently window it feeds.
const regularsRefreshEvery = 30 * time.Minute

// regularsObservationGoodEnough is how fresh an observation has to be for a
// request to be pointless: anything younger than one sweep interval is already
// as current as a fetch would make it. It is deliberately NOT the wider
// regularsObservationWindow the live tier tolerates — skipping everyone inside
// that tolerance would stretch the effective sweep to an hour and let the live
// tier lapse for someone who started playing in between, trading real freshness
// for savings rather than taking savings that are free.
const regularsObservationGoodEnough = regularsRefreshEvery - time.Minute

// regularsRefreshState is the deduplication for that sweep.
type regularsRefreshState struct {
	running  atomic.Bool
	lastDone atomic.Int64
}

func (s *regularsRefreshState) due(now time.Time) bool {
	last := s.lastDone.Load()
	return last == 0 || now.Sub(time.Unix(last, 0)) >= regularsRefreshEvery
}

// refreshRegularsInBackground re-fetches the Battle.net profile of each regular
// so their newest published game is current. It returns immediately: the sweep
// is paced by the facade and its results land in the archive, which the next
// read of the session view picks up. Nothing here blocks rendering, because a
// stale "last seen" is a much better outcome than a slow page.
//
// Only toons we already hold a profile for are refreshed, and each is asked on
// the gateway that profile came from, so a sweep costs at most one request per
// regular and never guesses at gateways.
//
// Regulars whose picture is already as fresh as a fetch would make it are
// skipped, and freshness counts every route the answer could have arrived by: a
// game the watcher ingested, an alt's profile fetched for some other surface, a
// game reported inside someone else's payload. A request that could only tell
// us what we already hold is a request not made, which leaves the budget for
// the regulars we are genuinely unsure about. Someone who keeps playing keeps
// re-earning the skip and costs nothing; the moment their evidence ages they
// become a target again, so nobody can be skipped indefinitely.
func (d *Dashboard) refreshRegularsInBackground(regulars []sessionRegular, now time.Time) {
	if len(regulars) == 0 || d.bnetDisabled.Load() {
		return
	}
	if addr, _ := d.bnetAddr.Load().(string); addr == "" {
		return
	}
	if !d.regularsRefresh.due(now) || !d.regularsRefresh.running.CompareAndSwap(false, true) {
		return
	}
	targets := make([]bnetProfileTarget, 0, len(regulars))
	for _, regular := range regulars {
		if regular.gateway == 0 || regular.refreshToon == "" {
			continue
		}
		if !regular.observedAt.IsZero() && now.Sub(regular.observedAt) < regularsObservationGoodEnough {
			continue
		}
		targets = append(targets, bnetProfileTarget{Toon: regular.refreshToon, Gateway: regular.gateway})
	}
	if len(targets) == 0 {
		d.regularsRefresh.running.Store(false)
		return
	}
	go func() {
		defer crashreport.GuardNonFatal(nil)
		defer d.regularsRefresh.running.Store(false)
		for _, target := range targets {
			// The same bound again, now against the profile store itself, so a
			// toon another surface already refreshed this cycle is not asked
			// for twice.
			_, _ = d.getOrFetchBnetProfile(d.ctx, target.Toon, target.Gateway, bnetfacade.PriorityBackground, regularsObservationGoodEnough)
		}
		d.regularsRefresh.lastDone.Store(time.Now().Unix())
	}()
}

// bnetProfileTarget is one (toon, gateway) pair to refresh.
type bnetProfileTarget struct {
	Toon    string
	Gateway int64
}

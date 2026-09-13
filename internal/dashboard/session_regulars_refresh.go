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
// the gateway that profile came from, so a sweep costs exactly one request per
// regular and never guesses at gateways.
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
			// A maxAge just under the sweep interval means a profile another
			// surface already refreshed this cycle is not asked for twice.
			_, _ = d.getOrFetchBnetProfile(d.ctx, target.Toon, target.Gateway, bnetfacade.PriorityBackground, regularsRefreshEvery-time.Minute)
		}
		d.regularsRefresh.lastDone.Store(time.Now().Unix())
	}()
}

// bnetProfileTarget is one (toon, gateway) pair to refresh.
type bnetProfileTarget struct {
	Toon    string
	Gateway int64
}

package dashboard

import (
	"context"
	"log"
	"time"

	"github.com/marianogappa/screpdb/internal/crashreport"
)

// bnetProfileRetention is how long an untouched profile entry stays cached.
// Anyone still being looked at refreshes on the 24h TTL, so entries this old
// belong to opponents nobody has viewed in three months.
const bnetProfileRetention = 90 * 24 * time.Hour

const bnetMaintenanceInterval = 24 * time.Hour

// startBnetCacheMaintenance prunes the Battle.net caches at startup and then
// daily: profile entries beyond the retention window are dropped (they refill
// on the next fetch), and the game archive sheds only what is provably dead —
// own-replay refs GCS no longer serves, and the log's duplicate lines. Game
// records themselves are never age-pruned: they cannot be refetched.
func (d *Dashboard) startBnetCacheMaintenance(ctx context.Context) {
	if d.library == nil {
		return
	}
	run := func() {
		if pruned, err := d.library.bnet.PruneOlderThan(bnetProfileRetention); err != nil {
			log.Printf("[bnet-prune] pruning profiles: %v", err)
		} else if pruned > 0 {
			log.Printf("[bnet-prune] dropped %d profile entries older than %d days", pruned, int(bnetProfileRetention/(24*time.Hour)))
		}
		if err := d.library.games.ClearReplayRefsOlderThan(enrichMaxAge); err != nil {
			log.Printf("[bnet-prune] clearing expired replay refs: %v", err)
		}
		if err := d.library.games.CompactIfNeeded(); err != nil {
			log.Printf("[bnet-prune] compacting the game archive: %v", err)
		}
	}
	go func() {
		defer crashreport.GuardNonFatal(nil)
		run()
		ticker := time.NewTicker(bnetMaintenanceInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

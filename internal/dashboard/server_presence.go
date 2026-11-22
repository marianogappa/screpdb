package dashboard

import (
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/sbfacade"
)

// sbRecentWindow is how long after a ShieldBattery game ended the user counts as playing there. ShieldBattery
// never shows its game to this app while it runs, so a fresh replay is the only sign of play, and ranked
// games come about every quarter of an hour.
const sbRecentWindow = 30 * time.Minute

type sbStatus struct {
	LastGameAt    string `json:"last_game_at,omitempty"`
	Recent        bool   `json:"recent"`
	RequestsToday int    `json:"requests_today"`
	CooldownUntil string `json:"cooldown_until,omitempty"`
}

// serverPresence says which servers the library has games from, so a user who only plays on one never sees
// the other.
type serverPresence struct {
	generation, version uint64
	bnetGames, sbGames  int
	sbLastGameEnd       time.Time
}

func (d *Dashboard) serverPresence() serverPresence {
	if d.library == nil {
		return serverPresence{}
	}
	snap := d.library.lib.Snapshot()
	if c := d.presence.Load(); c != nil && c.generation == snap.Generation && c.version == snap.Version {
		return *c
	}
	p := serverPresence{generation: snap.Generation, version: snap.Version}
	for _, r := range snap.Replays {
		switch {
		case r.ShieldBattery != nil:
			p.sbGames++
			if end := r.Date.Add(time.Duration(r.Duration) * time.Second); end.After(p.sbLastGameEnd) {
				p.sbLastGameEnd = end
			}
		case library.Strings.Name(r.GameSource) == bnetGameSourceName:
			p.bnetGames++
		}
	}
	d.presence.Store(&p)
	return p
}

func (d *Dashboard) sbStatus(now time.Time) *sbStatus {
	p := d.serverPresence()
	if p.sbGames == 0 {
		return nil
	}
	used, pausedUntil := sbfacade.Usage(now)
	s := &sbStatus{
		LastGameAt:    p.sbLastGameEnd.UTC().Format(time.RFC3339),
		Recent:        now.Sub(p.sbLastGameEnd) < sbRecentWindow,
		RequestsToday: used,
	}
	if !pausedUntil.IsZero() {
		s.CooldownUntil = pausedUntil.UTC().Format(time.RFC3339)
	}
	return s
}

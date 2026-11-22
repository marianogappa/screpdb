package dashboard

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/persist"
	"github.com/marianogappa/screpdb/internal/sbfacade"
)

const (
	// A profile's ladder and recent games move with every game, but a page that is opened again and again
	// must not turn into polling.
	sbProfileTTL = 30 * time.Minute
	// A page waits this long for a profile it has never seen; past it the page shows what it has.
	sbProfileFetchTimeout = 5 * time.Second
	// After a failed fetch the page stops asking for a while, so an unreachable server costs one wait.
	sbProfileFailureBackoff = 5 * time.Minute
)

var sbLadderOrder = []string{"1v1", "2v2", "2v2bgh"}

type sbLadderDetail struct {
	Type       string `json:"type"`
	Rank       int    `json:"rank"`
	Rating     int    `json:"rating"`
	Wins       int    `json:"wins"`
	Losses     int    `json:"losses"`
	LastPlayed string `json:"last_played,omitempty"`
}

type sbRecentPlayer struct {
	Name      string `json:"name"`
	Race      string `json:"race,omitempty"`
	PlayerKey string `json:"player_key"`
	Local     bool   `json:"local,omitempty"`
}

type sbRecentGame struct {
	ID              string           `json:"id"`
	PlayedAt        string           `json:"played_at"`
	MapName         string           `json:"map_name,omitempty"`
	Ranked          bool             `json:"ranked"`
	MatchmakingType string           `json:"matchmaking_type,omitempty"`
	DurationSeconds int              `json:"duration_seconds,omitempty"`
	Race            string           `json:"race,omitempty"`
	Result          string           `json:"result,omitempty"`
	Opponents       []sbRecentPlayer `json:"opponents"`
}

// sbProfileDetail is the ShieldBattery section of a ShieldBattery player's page.
type sbProfileDetail struct {
	Name          string           `json:"name"`
	Ladder        []sbLadderDetail `json:"ladder"`
	LifetimeGames int              `json:"lifetime_games"`
	LastPlayedAt  string           `json:"last_played_at,omitempty"`
	FetchedAt     string           `json:"fetched_at"`
	RecentGames   []sbRecentGame   `json:"recent_games"`
}

type sbProfiles struct {
	store    *persist.SBProfiles
	fetch    func(ctx context.Context, userID uint32) (*sbfacade.Profile, error)
	now      func() time.Time
	mu       sync.Mutex
	failedAt map[uint32]time.Time
}

func newSBProfiles(root string) *sbProfiles {
	return &sbProfiles{
		store:    persist.NewSBProfiles(root),
		fetch:    sbfacade.FetchProfile,
		now:      time.Now,
		failedAt: map[uint32]time.Time{},
	}
}

// profile serves the cached profile while it is fresh, and otherwise asks ShieldBattery, falling back to
// whatever it has when ShieldBattery does not answer in time.
func (s *sbProfiles) profile(ctx context.Context, userID uint32) (*persist.SBProfile, bool) {
	cached, ok, err := s.store.Get(userID)
	if err != nil {
		log.Printf("shieldbattery profiles: reading cache: %v", err)
	}
	now := s.now()
	if ok && now.Sub(cached.FetchedAt) < sbProfileTTL {
		return &cached, true
	}
	s.mu.Lock()
	failed, recentlyFailed := s.failedAt[userID]
	recentlyFailed = recentlyFailed && now.Sub(failed) < sbProfileFailureBackoff
	s.mu.Unlock()
	if !recentlyFailed {
		ctx, cancel := context.WithTimeout(ctx, sbProfileFetchTimeout)
		fetched, err := s.fetch(ctx, userID)
		cancel()
		if err == nil {
			fresh := persist.SBProfile{Profile: *fetched, FetchedAt: now.UTC()}
			if err := s.store.Put(userID, fresh); err != nil {
				log.Printf("shieldbattery profiles: caching %d: %v", userID, err)
			}
			return &fresh, true
		}
		log.Printf("shieldbattery profiles: %d: %v", userID, err)
		s.mu.Lock()
		s.failedAt[userID] = now
		s.mu.Unlock()
	}
	if ok {
		return &cached, true
	}
	return nil, false
}

func (d *Dashboard) sbProfileDetail(ctx context.Context, playerKey string) *sbProfileDetail {
	userID, ok := library.SBUserIDFromKey(playerKey)
	if !ok || d.sbProfiles == nil {
		return nil
	}
	cached, ok := d.sbProfiles.profile(ctx, userID)
	if !ok {
		return nil
	}
	detail := sbProfileDetailFrom(cached)
	d.markLocalSBPlayers(ctx, detail)
	return detail
}

func sbProfileDetailFrom(cached *persist.SBProfile) *sbProfileDetail {
	p := cached.Profile
	detail := &sbProfileDetail{
		Name:        p.Name,
		FetchedAt:   cached.FetchedAt.UTC().Format(time.RFC3339),
		Ladder:      []sbLadderDetail{},
		RecentGames: []sbRecentGame{},
	}
	var lastPlayed time.Time
	types := make([]string, 0, len(p.Ladder))
	for t := range p.Ladder {
		types = append(types, t)
	}
	sort.SliceStable(types, func(i, j int) bool { return ladderRank(types[i]) < ladderRank(types[j]) })
	for _, t := range types {
		l := p.Ladder[t]
		row := sbLadderDetail{Type: t, Rank: l.Rank, Rating: int(l.Rating + 0.5), Wins: l.Wins, Losses: l.Losses}
		if !l.LastPlayed.IsZero() {
			row.LastPlayed = l.LastPlayed.UTC().Format(time.RFC3339)
			if l.LastPlayed.After(lastPlayed) {
				lastPlayed = l.LastPlayed
			}
		}
		detail.Ladder = append(detail.Ladder, row)
	}
	for _, wl := range p.Lifetime {
		detail.LifetimeGames += wl.Wins + wl.Losses
	}
	for _, g := range p.RecentGames {
		if g.Canceled {
			continue
		}
		if g.StartTime.After(lastPlayed) {
			lastPlayed = g.StartTime
		}
		row := sbRecentGame{
			ID:              g.ID,
			PlayedAt:        g.StartTime.UTC().Format(time.RFC3339),
			MapName:         g.MapName,
			Ranked:          g.GameSource == sbfacade.SourceMatchmaking,
			MatchmakingType: g.MatchmakingType,
			DurationSeconds: int(g.GameLength / time.Second),
			Opponents:       []sbRecentPlayer{},
		}
		ownTeam := -1
		for _, pl := range g.Players {
			if pl.UserID == p.UserID {
				row.Race, row.Result, ownTeam = sbRaceName(pl.Race), string(pl.Result), pl.Team
			}
		}
		for _, pl := range g.Players {
			if pl.UserID == p.UserID || (pl.Team == ownTeam && len(g.Players) > 2) {
				continue
			}
			row.Opponents = append(row.Opponents, sbRecentPlayer{Name: pl.Name, Race: sbRaceName(pl.Race), PlayerKey: library.SBPlayerKey(pl.UserID)})
		}
		detail.RecentGames = append(detail.RecentGames, row)
	}
	if !lastPlayed.IsZero() {
		detail.LastPlayedAt = lastPlayed.UTC().Format(time.RFC3339)
	}
	return detail
}

func ladderRank(t string) int {
	for i, known := range sbLadderOrder {
		if t == known {
			return i
		}
	}
	return len(sbLadderOrder)
}

func sbRaceName(race string) string {
	switch race {
	case "p":
		return "Protoss"
	case "t":
		return "Terran"
	case "z":
		return "Zerg"
	case "r":
		return "Random"
	}
	return ""
}

// markLocalSBPlayers flags the opponents the user has games with, so the page can link to them.
func (d *Dashboard) markLocalSBPlayers(ctx context.Context, detail *sbProfileDetail) {
	var keys []string
	for _, g := range detail.RecentGames {
		for _, o := range g.Opponents {
			keys = append(keys, o.PlayerKey)
		}
	}
	if len(keys) == 0 {
		return
	}
	local, err := d.dbStore.HasLocalPlayers(ctx, keys)
	if err != nil {
		return
	}
	for gi := range detail.RecentGames {
		for oi := range detail.RecentGames[gi].Opponents {
			o := &detail.RecentGames[gi].Opponents[oi]
			_, o.Local = local[o.PlayerKey]
		}
	}
}

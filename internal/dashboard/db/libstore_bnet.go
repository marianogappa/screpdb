package db

import (
	"context"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/library/persist"
)

func (s *LibStore) GetBnetProfile(_ context.Context, toon string, gateway int64) (*persist.BnetProfile, error) {
	return s.bnet.Get(toon, gateway), nil
}

func (s *LibStore) UpsertBnetProfile(_ context.Context, p persist.BnetProfile) error {
	return s.bnet.Upsert(p)
}

func (s *LibStore) GetBnetCountryCodesByPlayerKeys(_ context.Context, playerKeys []string) (map[string]string, error) {
	out := map[string]string{}
	if len(playerKeys) == 0 {
		return out, nil
	}
	for key, code := range s.bnet.CountryCodesByToons(playerKeys) {
		if strings.TrimSpace(code) == "" {
			continue
		}
		out[key] = code
	}
	return out, nil
}

func (s *LibStore) GetBnetFetchedAtByPlayerKeys(_ context.Context, playerKeys []string) (map[string]time.Time, error) {
	if len(playerKeys) == 0 {
		return map[string]time.Time{}, nil
	}
	return s.bnet.FetchedAtByToons(playerKeys), nil
}

func (s *LibStore) ListBnetProfilesByPlayerKeys(_ context.Context, playerKeys []string) ([]persist.BnetProfile, error) {
	if len(playerKeys) == 0 {
		return []persist.BnetProfile{}, nil
	}
	return s.bnet.ProfilesByToons(playerKeys), nil
}

func (s *LibStore) ListBnetAuroraIDsByPlayerKeys(_ context.Context, playerKeys []string) ([]int64, error) {
	if len(playerKeys) == 0 {
		return []int64{}, nil
	}
	return s.bnet.AuroraIDsByToons(playerKeys), nil
}

func (s *LibStore) UpsertBnetGames(_ context.Context, games []persist.BnetGame) error {
	if len(games) == 0 || s.games == nil {
		return nil
	}
	return s.games.Upsert(games)
}

func (s *LibStore) ListBnetGamesByAccount(_ context.Context, auroraID int64) ([]persist.BnetGame, error) {
	if s.games == nil || auroraID == 0 {
		return []persist.BnetGame{}, nil
	}
	return s.games.GamesForAccount(auroraID), nil
}

func (s *LibStore) BnetFoundProfilesForToon(_ context.Context, toon string) ([]BnetFoundProfile, error) {
	profiles := s.bnet.FoundProfilesForToon(toon)
	out := make([]BnetFoundProfile, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, BnetFoundProfile{Gateway: p.Gateway, AuroraID: p.AuroraID})
	}
	return out, nil
}

func (s *LibStore) ListBnetGameTimes(_ context.Context, auroraID int64, since time.Time) ([]time.Time, error) {
	if s.games == nil {
		return []time.Time{}, nil
	}
	return s.games.TimesSince(auroraID, since), nil
}

// GetBnetLastGameAtByToons reports when each named toon was last seen playing
// in the game archive, whoever's profile fetch happened to report the game.
func (s *LibStore) GetBnetLastGameAtByToons(_ context.Context, toons []string) (map[string]time.Time, error) {
	if s.games == nil || len(toons) == 0 {
		return map[string]time.Time{}, nil
	}
	return s.games.LastGameByToons(toons), nil
}

// GetBnetGatewaysByToons reports a gateway we already have evidence for, per
// toon, so a backfill does not sweep gateways blind for an answer we hold.
func (s *LibStore) GetBnetGatewaysByToons(_ context.Context, toons []string) (map[string]int64, error) {
	if len(toons) == 0 {
		return map[string]int64{}, nil
	}
	return s.bnet.GatewaysByToons(toons), nil
}

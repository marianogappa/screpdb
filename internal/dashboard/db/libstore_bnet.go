package db

import (
	"context"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/library/persist"
)

func (s *LibStore) GetBnetProfile(_ context.Context, toon string, gateway int64) (*BnetProfileRow, error) {
	profile, err := s.bnet.Get(toon, gateway)
	if err != nil || profile == nil {
		return nil, err
	}
	return &BnetProfileRow{
		Toon:        profile.Toon,
		Gateway:     profile.Gateway,
		Found:       profile.Found,
		AuroraID:    profile.AuroraID,
		BattleTag:   profile.BattleTag,
		CountryCode: profile.CountryCode,
		Payload:     profile.Payload,
		FetchedAt:   profile.FetchedAt,
	}, nil
}

func (s *LibStore) UpsertBnetProfile(_ context.Context, row BnetProfileRow) error {
	return s.bnet.Upsert(persist.BnetProfile{
		Toon:        row.Toon,
		Gateway:     row.Gateway,
		Found:       row.Found,
		AuroraID:    row.AuroraID,
		BattleTag:   row.BattleTag,
		CountryCode: row.CountryCode,
		FetchedAt:   row.FetchedAt,
		Payload:     row.Payload,
	})
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

func (s *LibStore) ListBnetProfilePayloadsByPlayerKeys(_ context.Context, playerKeys []string) ([]BnetProfilePayloadRow, error) {
	if len(playerKeys) == 0 {
		return []BnetProfilePayloadRow{}, nil
	}
	profiles, err := s.bnet.PayloadsByToons(playerKeys)
	if err != nil {
		return nil, err
	}
	out := make([]BnetProfilePayloadRow, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, BnetProfilePayloadRow{Toon: profile.Toon, Gateway: profile.Gateway, Payload: profile.Payload})
	}
	return out, nil
}

func (s *LibStore) ListBnetAuroraIDsByPlayerKeys(_ context.Context, playerKeys []string) ([]int64, error) {
	if len(playerKeys) == 0 {
		return []int64{}, nil
	}
	return s.bnet.AuroraIDsByToons(playerKeys), nil
}

func (s *LibStore) UpsertBnetGameResults(_ context.Context, rows []BnetGameResultRow) error {
	if len(rows) == 0 || s.results == nil {
		return nil
	}
	converted := make([]persist.BnetGameResult, 0, len(rows))
	for _, row := range rows {
		converted = append(converted, persist.BnetGameResult{
			AuroraID:        row.AuroraID,
			GameID:          row.GameID,
			CreateTime:      row.CreateTime,
			Toon:            row.Toon,
			Gateway:         row.Gateway,
			Race:            row.Race,
			Result:          row.Result,
			APM:             row.APM,
			DurationSeconds: row.DurationSeconds,
			MapName:         row.MapName,
			MatchGUID:       row.MatchGUID,
		})
	}
	return s.results.Upsert(converted)
}

func (s *LibStore) ListBnetGameTimes(_ context.Context, auroraID int64, since time.Time) ([]time.Time, error) {
	if s.results == nil {
		return []time.Time{}, nil
	}
	return s.results.TimesSince(auroraID, since)
}

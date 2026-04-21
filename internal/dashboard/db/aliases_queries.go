package db

import (
	"context"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type PlayerAliasRow struct {
	ID                  int64  `json:"id"`
	CanonicalAlias      string `json:"canonical_alias"`
	BattleTagNormalized string `json:"battle_tag_normalized"`
	BattleTagRaw        string `json:"battle_tag_raw"`
	AuroraID            *int64 `json:"aurora_id"`
	Source              string `json:"source"`
	UpdatedAt           string `json:"updated_at"`
}

func (s *Store) ListPlayerAliases(ctx context.Context) ([]PlayerAliasRow, error) {
	sqlcRows, err := sqlcgen.New(s.defaultDB).ListPlayerAliases(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]PlayerAliasRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		result = append(result, PlayerAliasRow{
			ID:                  row.ID,
			CanonicalAlias:      strings.TrimSpace(row.CanonicalAlias),
			BattleTagNormalized: strings.TrimSpace(row.BattleTagNormalized),
			BattleTagRaw:        strings.TrimSpace(row.BattleTagRaw),
			AuroraID:            row.AuroraID,
			Source:              strings.TrimSpace(row.Source),
			UpdatedAt:           strings.TrimSpace(row.UpdatedAt),
		})
	}
	return result, nil
}

func (s *Store) UpsertPlayerAlias(ctx context.Context, canonicalAlias, battleTagRaw, battleTagNormalized string, auroraID *int64, source string) error {
	return sqlcgen.New(s.defaultDB).UpsertPlayerAlias(ctx, sqlcgen.UpsertPlayerAliasParams{
		CanonicalAlias:      strings.TrimSpace(canonicalAlias),
		BattleTagNormalized: strings.TrimSpace(battleTagNormalized),
		BattleTagRaw:        strings.TrimSpace(battleTagRaw),
		AuroraID:            auroraID,
		Source:              strings.TrimSpace(source),
	})
}

func (s *Store) DeletePlayerAliasByID(ctx context.Context, id int64) error {
	return sqlcgen.New(s.defaultDB).DeletePlayerAliasByID(ctx, id)
}

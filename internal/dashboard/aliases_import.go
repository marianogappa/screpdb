package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	aliasSourceImported = "imported"
	aliasSourceManual   = "manual"
	aliasSourceYou      = "you"
)

type aliasImportEntry struct {
	AuroraID  *int64 `json:"aurora_id"`
	BattleTag string `json:"battle_tag"`
}

type aliasUpsertRecord struct {
	CanonicalAlias      string
	BattleTagRaw        string
	BattleTagNormalized string
	AuroraID            *int64
	Source              string
}

func normalizeAliasBattleTag(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// aliasCanonicalEqualsBattleTag reports whether an alias would be a no-op (same identity for both fields).
func aliasCanonicalEqualsBattleTag(canonicalAlias, battleTagRaw string) bool {
	return normalizeAliasBattleTag(canonicalAlias) == normalizeAliasBattleTag(battleTagRaw)
}

// battleTagLookupKeys returns normalized keys for matching settings/imported battle tags to replay
// header names. Replays often omit the Battle.net numeric suffix while CSettings uses "name#1234".
// normalizedTag must already be lowercased (e.g. from normalizeAliasBattleTag).
func battleTagLookupKeys(normalizedTag string) []string {
	normalizedTag = strings.TrimSpace(normalizedTag)
	if normalizedTag == "" {
		return nil
	}
	i := strings.IndexByte(normalizedTag, '#')
	if i > 0 {
		base := strings.TrimSpace(normalizedTag[:i])
		if base != "" && base != normalizedTag {
			return []string{normalizedTag, base}
		}
	}
	return []string{normalizedTag}
}

func parseAliasImportJSON(raw []byte, source string) ([]aliasUpsertRecord, error) {
	if strings.TrimSpace(source) == "" {
		return nil, errors.New("alias source is required")
	}
	byAlias := map[string][]aliasImportEntry{}
	if err := json.Unmarshal(raw, &byAlias); err != nil {
		return nil, fmt.Errorf("failed to parse aliases JSON: %w", err)
	}

	dedup := map[string]aliasUpsertRecord{}
	for canonicalAlias, entries := range byAlias {
		canonicalAlias = strings.TrimSpace(canonicalAlias)
		if canonicalAlias == "" {
			continue
		}
		for _, entry := range entries {
			rawBattleTag := strings.TrimSpace(entry.BattleTag)
			if rawBattleTag == "" {
				continue
			}
			if aliasCanonicalEqualsBattleTag(canonicalAlias, rawBattleTag) {
				continue
			}
			normalized := normalizeAliasBattleTag(rawBattleTag)
			if normalized == "" {
				continue
			}
			record := aliasUpsertRecord{
				CanonicalAlias:      canonicalAlias,
				BattleTagRaw:        rawBattleTag,
				BattleTagNormalized: normalized,
				AuroraID:            entry.AuroraID,
				Source:              source,
			}
			key := source + "|" + normalized + "|" + canonicalAlias
			dedup[key] = record
		}
	}

	records := make([]aliasUpsertRecord, 0, len(dedup))
	for _, record := range dedup {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].BattleTagNormalized != records[j].BattleTagNormalized {
			return records[i].BattleTagNormalized < records[j].BattleTagNormalized
		}
		if records[i].Source != records[j].Source {
			return records[i].Source < records[j].Source
		}
		return records[i].CanonicalAlias < records[j].CanonicalAlias
	})
	return records, nil
}

func upsertPlayerAliases(ctx context.Context, db *sql.DB, records []aliasUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start alias upsert transaction: %w", err)
	}
	defer tx.Rollback()

	if err := upsertPlayerAliasesTx(ctx, tx, records); err != nil {
		return fmt.Errorf("failed to upsert aliases: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit alias upsert transaction: %w", err)
	}
	return nil
}

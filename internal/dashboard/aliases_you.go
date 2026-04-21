package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const youCanonicalAlias = "you"

// csettingsSearchDirs lists directories to check for CSettings.json, starting at the replay
// folder and walking toward the filesystem root. StarCraft: Remastered normally stores
// replays under .../StarCraft/Maps/Replays with CSettings.json in .../StarCraft/, but users may
// point ingest at a deeper subfolder or another tree shape, so we search ancestors instead of
// assuming a fixed depth.
func csettingsSearchDirs(replayDir string) []string {
	replayDir = strings.TrimSpace(replayDir)
	if replayDir == "" {
		return nil
	}
	var dirs []string
	cur := filepath.Clean(replayDir)
	seen := map[string]struct{}{}
	for range 20 {
		if _, dup := seen[cur]; !dup {
			seen[cur] = struct{}{}
			dirs = append(dirs, cur)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return dirs
}

func findCSettingsPathFromReplayDir(replayDir string) string {
	for _, dir := range csettingsSearchDirs(replayDir) {
		candidate := filepath.Join(dir, "CSettings.json")
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func parseCSettingsBattleTags(raw []byte) ([]string, error) {
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	tags := map[string]struct{}{}
	collectCSettingsBattleTags(root, tags)
	result := make([]string, 0, len(tags))
	for tag := range tags {
		result = append(result, tag)
	}
	return result, nil
}

func csettingsKeyIsAccountLogin(lowerKey string) bool {
	return lowerKey == "account"
}

func collectCSettingsBattleTags(node any, tags map[string]struct{}) {
	switch typed := node.(type) {
	case map[string]any:
		for key, value := range typed {
			lowerKey := strings.ToLower(strings.TrimSpace(key))
			if csettingsKeyIsAccountLogin(lowerKey) {
				if s, ok := value.(string); ok {
					clean := strings.TrimSpace(s)
					if clean != "" {
						tags[clean] = struct{}{}
					}
				}
			}
			collectCSettingsBattleTags(value, tags)
		}
	case []any:
		for _, value := range typed {
			collectCSettingsBattleTags(value, tags)
		}
	}
}

func replaceYouAliases(ctx context.Context, db *sql.DB, battleTags []string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM player_aliases WHERE source = ?`, aliasSourceYou); err != nil {
		return err
	}

	records := make([]aliasUpsertRecord, 0, len(battleTags))
	for _, battleTag := range battleTags {
		rawTag := strings.TrimSpace(battleTag)
		if rawTag == "" {
			continue
		}
		if aliasCanonicalEqualsBattleTag(youCanonicalAlias, rawTag) {
			continue
		}
		records = append(records, aliasUpsertRecord{
			CanonicalAlias:      youCanonicalAlias,
			BattleTagRaw:        rawTag,
			BattleTagNormalized: normalizeAliasBattleTag(rawTag),
			Source:              aliasSourceYou,
		})
	}
	if len(records) > 0 {
		if err := upsertPlayerAliasesTx(ctx, tx, records); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func upsertPlayerAliasesTx(ctx context.Context, tx *sql.Tx, records []aliasUpsertRecord) error {
	const sqlUpsert = `
	INSERT INTO player_aliases (
		canonical_alias,
		battle_tag_normalized,
		battle_tag_raw,
		aurora_id,
		source
	) VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(source, battle_tag_normalized, canonical_alias) DO UPDATE SET
		battle_tag_raw = excluded.battle_tag_raw,
		aurora_id = excluded.aurora_id,
		updated_at = CURRENT_TIMESTAMP
	`

	stmt, err := tx.PrepareContext(ctx, sqlUpsert)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, record := range records {
		if _, err := stmt.ExecContext(
			ctx,
			record.CanonicalAlias,
			record.BattleTagNormalized,
			record.BattleTagRaw,
			record.AuroraID,
			record.Source,
		); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dashboard) refreshYouAliasesBestEffort(ctx context.Context) {
	inputDir, err := d.getIngestInputDir(ctx)
	if err != nil {
		log.Printf("aliases: skipped you refresh, ingest dir unavailable: %v", err)
		return
	}
	if strings.TrimSpace(inputDir) == "" {
		return
	}
	csettingsPath := findCSettingsPathFromReplayDir(inputDir)
	if csettingsPath == "" {
		log.Printf("aliases: skipped you refresh, CSettings.json not found when walking up from replay dir %s", inputDir)
		return
	}
	raw, err := os.ReadFile(csettingsPath)
	if err != nil {
		log.Printf("aliases: skipped you refresh, CSettings not readable at %s: %v", csettingsPath, err)
		return
	}
	battleTags, err := parseCSettingsBattleTags(raw)
	if err != nil {
		log.Printf("aliases: skipped you refresh, CSettings parse failed: %v", err)
		return
	}
	if err := replaceYouAliases(ctx, d.db, battleTags); err != nil {
		log.Printf("aliases: skipped you refresh, DB update failed: %v", err)
	}
}

package dashboard

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

// youMarker is appended to the display name of any player who is the user. The
// index-pointing-at-the-viewer emoji says "this one is you" without spending
// the horizontal room a "(you)" suffix costs in a pill.
const youMarker = "🫵"

// csettingsMaxAncestorLevels bounds how far above the replay folder we search
// for CSettings.json. StarCraft: Remastered normally stores replays under
// .../StarCraft/Maps/Replays with CSettings.json in .../StarCraft/, but users
// may point ingest at a deeper subfolder, so we walk ancestors rather than
// assuming a fixed depth. The upward read is a sanctioned, read-only exception
// to the iofacade allowlist (see iofacade.FindAndReadAncestorFile).
const csettingsMaxAncestorLevels = 20

// youLookupKeys returns the normalized keys a battle tag should match against
// replay header names. Replays often omit the Battle.net numeric suffix while
// CSettings carries the full "name#1234", so both forms are registered.
func youLookupKeys(battleTag string) []string {
	normalized := strings.ToLower(strings.TrimSpace(battleTag))
	if normalized == "" {
		return nil
	}
	if i := strings.IndexByte(normalized, '#'); i > 0 {
		if base := strings.TrimSpace(normalized[:i]); base != "" && base != normalized {
			return []string{normalized, base}
		}
	}
	return []string{normalized}
}

// youKeySetFromBattleTags builds the lookup set held in memory.
func youKeySetFromBattleTags(battleTags []string) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, tag := range battleTags {
		for _, key := range youLookupKeys(tag) {
			keys[key] = struct{}{}
		}
	}
	return keys
}

// loadYouKeys returns the current set of names that are the user, or nil when
// it cannot be determined. It is held in memory rather than persisted: it is
// derived entirely from CSettings.json, so a stored copy could only ever go
// stale relative to the file it came from.
func (d *Dashboard) loadYouKeys() map[string]struct{} {
	if v := d.youKeys.Load(); v != nil {
		if keys, ok := v.(map[string]struct{}); ok {
			return keys
		}
	}
	return nil
}

func (d *Dashboard) isYouPlayer(name string) bool {
	keys := d.loadYouKeys()
	if len(keys) == 0 {
		return false
	}
	_, ok := keys[normalizePlayerKey(name)]
	return ok
}

// youDisplayNames maps each of names that is the user to its marked display
// name. Names that are not the user are absent from the result, so callers can
// keep treating it as an override map.
func (d *Dashboard) youDisplayNames(names []string) map[string]string {
	keys := d.loadYouKeys()
	if len(keys) == 0 {
		return map[string]string{}
	}
	display := make(map[string]string, len(names))
	for _, name := range names {
		if _, ok := keys[normalizePlayerKey(name)]; !ok {
			continue
		}
		display[name] = formatYouDisplayName(name)
	}
	return display
}

func formatYouDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, youMarker) {
		return name
	}
	return name + " " + youMarker
}

// refreshYouKeysBestEffort re-reads CSettings.json and republishes the in-memory
// set. Failures are logged and leave the previous set in place: losing the "you"
// marker is a worse outcome than briefly showing a stale one.
func (d *Dashboard) refreshYouKeysBestEffort(_ context.Context) {
	inputDir := d.library.Folder()
	if strings.TrimSpace(inputDir) == "" {
		return
	}
	csettingsPath, raw, err := iofacade.FindAndReadAncestorFile(inputDir, "CSettings.json", csettingsMaxAncestorLevels)
	if csettingsPath == "" {
		log.Printf("you: skipped refresh, CSettings.json not found when walking up from replay dir %s", inputDir)
		return
	}
	if err != nil {
		log.Printf("you: skipped refresh, CSettings not readable at %s: %v", csettingsPath, err)
		return
	}
	battleTags, err := parseCSettingsBattleTags(raw)
	if err != nil {
		log.Printf("you: skipped refresh, CSettings parse failed: %v", err)
		return
	}
	d.youKeys.Store(youKeySetFromBattleTags(battleTags))
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

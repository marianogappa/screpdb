package dashboard

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"github.com/marianogappa/screpdb/internal/library"

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
	var bnet map[string]struct{}
	if v := d.youKeys.Load(); v != nil {
		bnet, _ = v.(map[string]struct{})
	}
	sb := d.sbYouKey()
	if sb == "" {
		return bnet
	}
	keys := make(map[string]struct{}, len(bnet)+1)
	for k := range bnet {
		keys[k] = struct{}{}
	}
	keys[sb] = struct{}{}
	return keys
}

type sbYouCache struct {
	generation, version uint64
	key                 string
}

// sbYouKey is the user's ShieldBattery account: CSettings knows nothing of it, but the user is in every
// game they recorded, so it is the account found in nearly all of their ShieldBattery replays. Their
// own autosaves are the best evidence, so other files only count when there are none.
func (d *Dashboard) sbYouKey() string {
	if d.library == nil {
		return ""
	}
	snap := d.library.lib.Snapshot()
	if c := d.sbYou.Load(); c != nil && c.generation == snap.Generation && c.version == snap.Version {
		return c.key
	}
	key := sbOwnerKey(snap.Replays)
	d.sbYou.Store(&sbYouCache{generation: snap.Generation, version: snap.Version, key: key})
	return key
}

const (
	sbOwnerMinGames = 3
	sbOwnerMinShare = 0.8
)

func sbOwnerKey(replays []*library.Replay) string {
	count := func(autosaveOnly bool) (map[string]int, int) {
		counts := map[string]int{}
		games := 0
		for _, r := range replays {
			if r.ShieldBattery == nil || (autosaveOnly && !r.Flags.Has(library.FlagIsAutosave)) {
				continue
			}
			games++
			for i := range r.Players {
				if p := &r.Players[i]; !p.IsObserver() && library.IsSBKey(p.Key) {
					counts[p.Key]++
				}
			}
		}
		return counts, games
	}
	counts, games := count(true)
	if games == 0 {
		counts, games = count(false)
	}
	if games < sbOwnerMinGames {
		return ""
	}
	best, bestGames := "", 0
	for key, n := range counts {
		if n > bestGames || (n == bestGames && key < best) {
			best, bestGames = key, n
		}
	}
	if float64(bestGames) < sbOwnerMinShare*float64(games) {
		return ""
	}
	return best
}

func (d *Dashboard) isYouKey(key string) bool {
	_, ok := d.loadYouKeys()[normalizePlayerKey(key)]
	return ok
}

// youDisplayName marks name when the player key is the user's.
func (d *Dashboard) youDisplayName(key, name string) string {
	if d.isYouKey(key) {
		return formatYouDisplayName(name)
	}
	return name
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
	if gw := parseCSettingsRecentGateway(raw); gw > 0 {
		d.youGateway.Store(int64(gw))
	}
}

// parseCSettingsRecentGateway returns the gateway the user last logged into,
// or 0. The bridge's /v1/gateway endpoint lists every gateway without saying
// which one the session is on, so CSettings is the one reliable source — and
// it is the same file the you-keys come from, so whoever can be identified in
// a replay also has a gateway.
func parseCSettingsRecentGateway(raw []byte) int {
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil {
		return 0
	}
	general, ok := root["General settings"]
	if !ok {
		return 0
	}
	var section map[string]json.RawMessage
	if json.Unmarshal(general, &section) != nil {
		return 0
	}
	value, ok := section["Configuration-Recent gateway id"]
	if !ok {
		return 0
	}
	var n int
	if json.Unmarshal(value, &n) == nil && n > 0 {
		return n
	}
	var s string
	if json.Unmarshal(value, &s) == nil {
		if parsed, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
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

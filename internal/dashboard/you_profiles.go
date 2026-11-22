package dashboard

import (
	"sort"
	"strings"
	"time"

	"github.com/marianogappa/scfingerprint"
	"github.com/marianogappa/screpdb/internal/library"
)

// Where a player key's claim to being the user comes from. Definite evidence
// names the user outright; a saver is very likely the user, so fingerprinting
// against the definite profiles may still veto it.
const (
	youEvidenceCSettings     = "csettings"
	youEvidenceAccount       = "account"
	youEvidenceShieldBattery = "shieldbattery"
	youEvidenceSaver         = "saver"
)

const (
	// A name the user recorded once could be a stranger whose chat the parser
	// misread; two of the user's own autosaves never were in the study.
	youSaverMinSaves = 2
	// At most one autosave in ten may say someone else recorded a game this name
	// played in, so one misread file cannot cost the user a profile.
	youSaverMaxContradictionsPerTen = 1

	// The veto needs enough games on both sides to say "plays unlike you"; a
	// handful of games proves nothing, and a missing veto only keeps a saver.
	youVetoMinCandidateGames = 5
	youVetoMinDefiniteGames  = 20
	youVetoMaxDefiniteGames  = 200
	youVetoMinZ              = 1.5

	youSetMaxAge = time.Minute
)

type youProfileEvidence struct {
	Evidence string
	Saves    int
}

type youSetCache struct {
	generation, version uint64
	csettings           map[string]struct{}
	builtAt             time.Time
	members             map[string]youProfileEvidence
}

// youSet is every player key that is the user, with why. It is rebuilt when the
// library or CSettings changes, and at least once a minute so Battle.net
// profiles cached since then can widen it.
func (d *Dashboard) youSet() map[string]youProfileEvidence {
	csettings := d.csettingsYouKeys()
	var snap *library.Snapshot
	if view := d.corpusView(); view != nil {
		snap = view.Snapshot()
	}
	var generation, version uint64
	if snap != nil {
		generation, version = snap.Generation, snap.Version
	}
	if c := d.youSetCache.Load(); c != nil && c.generation == generation && c.version == version &&
		sameKeySet(c.csettings, csettings) && time.Since(c.builtAt) < youSetMaxAge {
		return c.members
	}
	d.youSetMu.Lock()
	defer d.youSetMu.Unlock()
	if c := d.youSetCache.Load(); c != nil && c.generation == generation && c.version == version &&
		sameKeySet(c.csettings, csettings) && time.Since(c.builtAt) < youSetMaxAge {
		return c.members
	}
	members := d.buildYouSet(csettings, snap)
	d.youSetCache.Store(&youSetCache{generation: generation, version: version, csettings: csettings, builtAt: time.Now(), members: members})
	return members
}

func (d *Dashboard) buildYouSet(csettings map[string]struct{}, snap *library.Snapshot) map[string]youProfileEvidence {
	members := map[string]youProfileEvidence{}
	for key := range csettings {
		members[key] = youProfileEvidence{Evidence: youEvidenceCSettings}
	}
	for _, key := range d.youAccountToons(csettings) {
		if _, ok := members[key]; !ok {
			members[key] = youProfileEvidence{Evidence: youEvidenceAccount}
		}
	}
	if sb := d.sbYouKey(); sb != "" {
		members[sb] = youProfileEvidence{Evidence: youEvidenceShieldBattery}
	}
	if snap == nil {
		return members
	}
	definite := make([]string, 0, len(members))
	for key := range members {
		definite = append(definite, key)
	}
	sort.Strings(definite)
	var definiteGames []scfingerprint.PlayerGame
	vetoLoaded := false
	for key, saves := range youSavers(snap.Replays) {
		if _, ok := members[key]; ok {
			continue
		}
		if !vetoLoaded {
			definiteGames = d.youFingerprintGames(definite, youVetoMaxDefiniteGames)
			vetoLoaded = true
		}
		if d.youFingerprintVetoes(key, definiteGames) {
			continue
		}
		members[key] = youProfileEvidence{Evidence: youEvidenceSaver, Saves: saves}
	}
	return members
}

// youAccountToons returns the other names on the user's Battle.net account. A
// bare name can belong to a stranger on another gateway, so an account counts
// only when it lists two of the user's CSettings names, or when CSettings
// names just one and the account is the one on the gateway CSettings says the
// user last played on.
func (d *Dashboard) youAccountToons(csettings map[string]struct{}) []string {
	if len(csettings) == 0 || d.dbStore == nil {
		return nil
	}
	keys := make([]string, 0, len(csettings))
	for key := range csettings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	byKey, _ := d.dbStore.ListBnetProfilesByPlayerKeys(d.ctx, keys)
	listing, _ := d.dbStore.ListBnetProfilesListingPlayerKeys(d.ctx, keys)
	gateway := d.youGateway.Load()

	accounts := map[int64][]string{}
	ownNames := map[int64]map[string]struct{}{}
	onGateway := map[int64]bool{}
	for _, p := range append(byKey, listing...) {
		if !p.Found || p.AuroraID == 0 {
			continue
		}
		if ownNames[p.AuroraID] == nil {
			ownNames[p.AuroraID] = map[string]struct{}{}
		}
		names := []string{p.Toon}
		for _, t := range p.Toons {
			names = append(names, t.Toon)
		}
		for _, name := range names {
			key := normalizePlayerKey(name)
			accounts[p.AuroraID] = append(accounts[p.AuroraID], key)
			if _, ok := csettings[key]; ok {
				ownNames[p.AuroraID][key] = struct{}{}
			}
		}
		if _, ok := csettings[normalizePlayerKey(p.Toon)]; ok && gateway > 0 && p.Gateway == gateway {
			onGateway[p.AuroraID] = true
		}
	}
	bareNames := 0
	for key := range csettings {
		if !strings.Contains(key, "#") {
			bareNames++
		}
	}
	var out []string
	for aurora, names := range accounts {
		if len(ownNames[aurora]) < 2 && !(bareNames == 1 && onGateway[aurora]) {
			continue
		}
		out = append(out, names...)
	}
	return out
}

// youSavers counts, per player key, the user's own autosaves that key
// recorded. Downloaded complete copies were recorded by a co-player, so they
// are no evidence. A key that other recorders' files keep showing in is a
// stranger sharing a name, not the user.
func youSavers(replays []*library.Replay) map[string]int {
	saves := map[string]int{}
	contradictions := map[string]int{}
	for _, r := range replays {
		if !r.Flags.Has(library.FlagIsAutosave) || r.Flags.Has(library.FlagIsCompleteCopy) {
			continue
		}
		saver := replaySaver(r)
		if saver < 0 {
			continue
		}
		saverKey := r.Players[saver].Key
		saves[saverKey]++
		for i := range r.Players {
			p := &r.Players[i]
			if p.Type != library.PlayerTypeHuman || p.Key == saverKey {
				continue
			}
			contradictions[p.Key]++
		}
	}
	out := map[string]int{}
	for key, n := range saves {
		if n >= youSaverMinSaves && contradictions[key]*10 <= n*youSaverMaxContradictionsPerTen {
			out[key] = n
		}
	}
	return out
}

// replaySaver is the ordinal of whoever recorded the file, or -1. Chat names
// the recorder when anyone spoke; otherwise the recording stops when its saver
// leaves, so the saver is the one human player never seen leaving, when there
// is exactly one.
func replaySaver(r *library.Replay) int {
	for i := range r.Players {
		if r.Players[i].Flags.Has(library.PlayerSaver) {
			return i
		}
	}
	found := -1
	for i := range r.Players {
		p := &r.Players[i]
		if p.Type != library.PlayerTypeHuman || p.IsObserver() || p.Flags.Has(library.PlayerLeft) {
			continue
		}
		if found >= 0 {
			return -1
		}
		found = i
	}
	return found
}

func (d *Dashboard) youFingerprintGames(keys []string, limit int) []scfingerprint.PlayerGame {
	if d.dbStore == nil {
		return nil
	}
	var games []scfingerprint.PlayerGame
	for _, key := range keys {
		rows, err := d.dbStore.ListPlayerFingerprintVectors(d.ctx, key, int64(scfingerprint.FeatureVersion()))
		if err != nil {
			continue
		}
		for _, row := range rows {
			if len(row.Vector) == 0 {
				continue
			}
			games = append(games, scfingerprint.PlayerGame{Vector: row.Vector, Race: row.Race, Toon: row.Name})
			if len(games) >= limit {
				return games
			}
		}
	}
	return games
}

// youFingerprintVetoes says a saver plays unlike the user's definite profiles.
// Without enough 1v1 games on either side it never vetoes: the veto only exists
// to throw out a misread saver, and the saver evidence stands on its own.
func (d *Dashboard) youFingerprintVetoes(key string, definite []scfingerprint.PlayerGame) bool {
	if len(definite) < youVetoMinDefiniteGames {
		return false
	}
	candidate := d.youFingerprintGames([]string{key}, youVetoMaxDefiniteGames)
	if len(candidate) < youVetoMinCandidateGames {
		return false
	}
	verdict, err := scfingerprint.Same(candidate, definite)
	if err != nil || verdict.ModelIsSynthetic {
		return false
	}
	return verdict.Z < youVetoMinZ
}

func sameKeySet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

const youProfilesMax = 8

type workflowYouProfile struct {
	PlayerKey  string `json:"player_key"`
	PlayerName string `json:"player_name"`
	LastPlayed string `json:"last_played"`
	Games      int    `json:"games"`
}

// otherYouProfiles lists the user's profiles other than playerKey that have
// games in the current view, most recently played first. It is empty unless
// playerKey is the user.
func (d *Dashboard) otherYouProfiles(playerKey string) []workflowYouProfile {
	view := d.corpusView()
	if view == nil || !d.isYouKey(playerKey) {
		return nil
	}
	self := normalizePlayerKey(playerKey)
	var out []workflowYouProfile
	for key := range d.youSet() {
		if key == self {
			continue
		}
		games := view.PlayerGames(key)
		if len(games) == 0 {
			continue
		}
		latest := games[0]
		out = append(out, workflowYouProfile{
			PlayerKey:  key,
			PlayerName: latest.Player().Name,
			LastPlayed: latest.Replay.Date.UTC().Format(time.RFC3339),
			Games:      len(games),
		})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].LastPlayed != out[b].LastPlayed {
			return out[a].LastPlayed > out[b].LastPlayed
		}
		return out[a].PlayerKey < out[b].PlayerKey
	})
	if len(out) > youProfilesMax {
		out = out[:youProfilesMax]
	}
	return out
}

func (d *Dashboard) corpusView() *library.View {
	if d.dbStore == nil {
		return nil
	}
	return d.dbStore.CorpusView()
}

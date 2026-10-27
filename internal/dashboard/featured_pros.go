package dashboard

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/marianogappa/scfingerprint"
	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/crashreport"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/propack"
)

// Built-in progamer profiles ride the same player surfaces as local players
// (players list, player page, skill-proxy distributions) but come from the
// embedded pro pack rather than the user's database. Every accessor below goes
// through featuredPros, which drops any pro the user themselves turns out to
// be: a progamer running the app must see their own data, never a frozen
// snapshot of it.

// featuredExclusionTTL bounds how long the "which pros are the user" answer is
// reused. The inputs (CSettings battle tags, cached Battle.net profiles,
// fingerprint matches) all change slowly.
const featuredExclusionTTL = time.Minute

type featuredProfile struct {
	ID           string         `json:"id"`
	Label        string         `json:"label"`
	Liquipedia   string         `json:"liquipedia,omitempty"`
	PhotoURL     string         `json:"photo_url,omitempty"`
	PhotoCredit  string         `json:"photo_credit,omitempty"`
	Country      string         `json:"country,omitempty"`
	MainRace     string         `json:"main_race,omitempty"`
	Races        map[string]int `json:"races,omitempty"`
	GamesSampled int            `json:"games_sampled"`
	Toons        []propack.Toon `json:"toons,omitempty"`
	GeneratedAt  string         `json:"generated_at,omitempty"`
	Confidence   string         `json:"confidence,omitempty"`
}

// workflowFeaturedPlayerItem is a built-in profile as the players list shows
// it: no games-played or last-played, because those describe the user's
// dataset and the pro is not in it.
type workflowFeaturedPlayerItem struct {
	PlayerKey    string  `json:"player_key"`
	PlayerName   string  `json:"player_name"`
	Race         string  `json:"race"`
	AverageAPM   float64 `json:"average_apm"`
	CountryCode  string  `json:"country_code,omitempty"`
	GamesSampled int     `json:"games_sampled"`
	Liquipedia   string  `json:"liquipedia,omitempty"`
	PhotoURL     string  `json:"photo_url,omitempty"`
}

func proPhotoURL(pro *propack.Pro) string {
	if pro == nil || pro.Photo == "" {
		return ""
	}
	return "/api/custom/pros/" + pro.ID + "/photo"
}

func (d *Dashboard) loadProPack() *propack.Pack {
	pack, err := propack.Load()
	if err != nil {
		crashreport.Handle(err, nil, false)
		return nil
	}
	return pack
}

// featuredVisibleIDs is the curated set of progamers shown on the players page
// and the bell-curve overlays. Pros not in this set stay in the embedded pack
// (their player pages still work when navigated to directly) but are hidden
// from the listing and the distribution overlays.
var featuredVisibleIDs = map[string]bool{
	"action": true, "ample": true, "artosis": true, "barracks": true,
	"best": true, "bisu": true, "dewalt": true, "effort": true,
	"flash": true, "ggaemo": true, "hero": true, "jaedong": true,
	"jyj": true, "larva": true, "leta": true, "light": true,
	"mind": true, "mong": true, "queen": true, "royal": true,
	"rush": true, "sharp": true, "shuttle": true, "snow": true,
	"soma": true, "soulkey": true, "speed": true, "ssak": true,
	"stork": true, "xiaoshuai": true,
}

// featuredPros returns the curated built-in profiles the user is not, most
// popular first: curated rank, then pros with a portrait, then label.
func (d *Dashboard) featuredPros() []*propack.Pro {
	pack := d.loadProPack()
	if pack == nil {
		return nil
	}
	excluded := d.excludedProIDs(pack)
	out := make([]*propack.Pro, 0, len(pack.Pros))
	for i := range pack.Pros {
		pro := &pack.Pros[i]
		if excluded[pro.ID] || !featuredVisibleIDs[pro.ID] {
			continue
		}
		out = append(out, pro)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := rankOrZeroLast(out[i].Rank), rankOrZeroLast(out[j].Rank)
		if ri != rj {
			return ri < rj
		}
		if (out[i].Photo != "") != (out[j].Photo != "") {
			return out[i].Photo != ""
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	return out
}

func rankOrZeroLast(rank int) int {
	if rank <= 0 {
		return int(^uint(0) >> 1)
	}
	return rank
}

// featuredPro resolves a pro player key, or nil when the key is not a pro or
// names one the user is.
func (d *Dashboard) featuredPro(playerKey string) *propack.Pro {
	id, ok := propack.IDFromKey(playerKey)
	if !ok {
		return nil
	}
	pack := d.loadProPack()
	if pack == nil {
		return nil
	}
	pro := pack.ByID(id)
	if pro == nil || d.excludedProIDs(pack)[pro.ID] {
		return nil
	}
	return pro
}

// excludedProIDs answers "which built-in profiles are the user?" from three
// signals, any of which suffices: one of the user's account names is a known
// toon of the pro; a cached Battle.net profile of a user account carries one of
// the pro's aurora IDs; or the user's own replays fingerprint-match the pro at
// high confidence. Only the CSettings-derived "you" accounts are ever tested.
func (d *Dashboard) excludedProIDs(pack *propack.Pack) map[string]bool {
	d.featuredExclMu.Lock()
	defer d.featuredExclMu.Unlock()
	if d.featuredExcl != nil && time.Since(d.featuredExclAt) < featuredExclusionTTL {
		return d.featuredExcl
	}
	excluded := map[string]bool{}
	youKeys := d.loadYouKeys()
	if len(youKeys) > 0 && pack != nil {
		keys := make([]string, 0, len(youKeys))
		for k := range youKeys {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		auroraIDs := map[int64]bool{}
		if ids, err := d.dbStore.ListBnetAuroraIDsByPlayerKeys(d.ctx, keys); err == nil {
			for _, id := range ids {
				auroraIDs[id] = true
			}
		}
		for i := range pack.Pros {
			pro := &pack.Pros[i]
			for _, toon := range pro.Toons {
				if _, ok := youKeys[normalizePlayerKey(toon.Toon)]; ok {
					excluded[pro.ID] = true
				}
			}
			for _, id := range pro.AuroraIDs {
				if auroraIDs[id] {
					excluded[pro.ID] = true
				}
			}
		}
		for _, key := range keys {
			match, err := d.matchFingerprint(key, scfingerprint.FeatureVersion())
			if err != nil || match == nil || (match.Tier != fingerprintTierConfirmed && match.Tier != fingerprintTierHigh) {
				continue
			}
			if pro := pack.ByLabel(match.Label); pro != nil {
				excluded[pro.ID] = true
			}
		}
	}
	d.featuredExcl = excluded
	d.featuredExclAt = time.Now()
	return excluded
}

func (d *Dashboard) featuredPlayersList(nameFilter string) []workflowFeaturedPlayerItem {
	needle := normalizePlayerKey(nameFilter)
	items := []workflowFeaturedPlayerItem{}
	for _, pro := range d.featuredPros() {
		if needle != "" && !strings.Contains(strings.ToLower(pro.Label), needle) && !strings.Contains(pro.ID, needle) {
			continue
		}
		item := workflowFeaturedPlayerItem{
			PlayerKey:    pro.Key(),
			PlayerName:   pro.Label,
			Race:         pro.MainRace,
			CountryCode:  pro.Country,
			GamesSampled: pro.GamesSampled,
			Liquipedia:   pro.Liquipedia,
			PhotoURL:     proPhotoURL(pro),
		}
		if pro.APM != nil {
			item.AverageAPM = pro.APM.Value
		}
		items = append(items, item)
	}
	return items
}

func featuredProfileOf(pack *propack.Pack, pro *propack.Pro) *featuredProfile {
	profile := &featuredProfile{
		ID:           pro.ID,
		Label:        pro.Label,
		Liquipedia:   pro.Liquipedia,
		PhotoURL:     proPhotoURL(pro),
		Country:      pro.Country,
		MainRace:     pro.MainRace,
		Races:        pro.Races,
		GamesSampled: pro.GamesSampled,
		Toons:        pro.Toons,
		Confidence:   pro.Confidence,
	}
	if pro.Photo != "" {
		profile.PhotoCredit = pro.Liquipedia
	}
	if pack != nil {
		profile.GeneratedAt = pack.GeneratedAt
	}
	return profile
}

// buildFeaturedPlayerOverview is the player-page overview of a built-in
// profile. Local-dataset fields (wins, matchups, tech orders, chat) stay empty;
// the Battle.net section is served from the profile cache keyed on the pro's
// known toons, and a background fetch fills it when the bridge is connected.
func (d *Dashboard) buildFeaturedPlayerOverview(playerKey string) (workflowPlayerOverview, error) {
	pro := d.featuredPro(playerKey)
	if pro == nil {
		return workflowPlayerOverview{}, dashboarddb.ErrNotFound
	}
	pack := d.loadProPack()
	result := workflowPlayerOverview{
		SummaryVersion:     workflowSummaryVersion,
		PlayerKey:          pro.Key(),
		PlayerName:         pro.Label,
		GamesPlayed:        int64(pro.GamesSampled),
		CountryCode:        pro.Country,
		Featured:           featuredProfileOf(pack, pro),
		RaceBreakdown:      []workflowPlayerRaceBreakdown{},
		FingerprintMetrics: []workflowComparativeMetric{},
		RecentGames:        []workflowGameListItem{},
		NarrativeHints:     []string{},
		Matchups:           []workflowPlayerMatchupCell{},
		RaceOrders:         []workflowRaceOrderSummary{},
		MatchupOrders:      []workflowMatchupOrderSummary{},
		EarlyTimings:       []workflowPlayerEarlyTiming{},
	}
	if pro.APM != nil {
		result.AverageAPM = pro.APM.Value
	}
	for race, games := range pro.Races {
		result.RaceBreakdown = append(result.RaceBreakdown, workflowPlayerRaceBreakdown{Race: race, GameCount: int64(games)})
	}
	sort.Slice(result.RaceBreakdown, func(i, j int) bool {
		if result.RaceBreakdown[i].GameCount != result.RaceBreakdown[j].GameCount {
			return result.RaceBreakdown[i].GameCount > result.RaceBreakdown[j].GameCount
		}
		return result.RaceBreakdown[i].Race < result.RaceBreakdown[j].Race
	})

	toonKeys := make([]string, 0, len(pro.Toons))
	for _, toon := range pro.Toons {
		toonKeys = append(toonKeys, normalizePlayerKey(toon.Toon))
	}
	details := d.bnetProfileDetailsByPlayerKeys(d.ctx, toonKeys)
	for _, detail := range details {
		// Misses reach the page now so ordinary players can be told their name
		// is not on Battle.net. A pro page has nothing to say with one: it
		// decorates a known account, so only a found profile counts.
		if !detail.Found {
			continue
		}
		if result.BnetProfile == nil || bnetProfileDetailScore(detail) > bnetProfileDetailScore(result.BnetProfile) {
			result.BnetProfile = detail
		}
	}
	if result.BnetProfile != nil {
		result.BnetGames = int64(pro.GamesSampled)
		if result.CountryCode == "" {
			result.CountryCode = result.BnetProfile.CountryCode
		}
		result.BnetProfile.Habits = d.bnetPlayHabitsFor(d.ctx, result.BnetProfile.AuroraID, result.CountryCode, time.Now())
	}
	d.backfillBnetProfilesForToons(pro.Toons)
	return result, nil
}

// backfillBnetProfilesForToons fetches, in the background, the profiles of
// known (toon, gateway) accounts that are not cached yet. Unlike
// backfillBnetProfiles it never has to guess the gateway, so it spends at most
// one bridge request per account.
func (d *Dashboard) backfillBnetProfilesForToons(toons []propack.Toon) {
	if d.bnetDisabled.Load() {
		return
	}
	addr, _ := d.bnetAddr.Load().(string)
	if addr == "" || len(toons) == 0 {
		return
	}
	var pending []propack.Toon
	for _, toon := range toons {
		if _, ok := bnetfacade.GatewayNames[toon.Gateway]; !ok || strings.TrimSpace(toon.Toon) == "" {
			continue
		}
		row, err := d.dbStore.GetBnetProfile(d.ctx, toon.Toon, int64(toon.Gateway))
		if err != nil {
			continue
		}
		if row != nil && time.Since(row.FetchedAt) < bnetProfileTTL {
			continue
		}
		pending = append(pending, toon)
	}
	if len(pending) == 0 {
		return
	}
	d.bnetBackfillActive.Add(1)
	go func() {
		defer crashreport.GuardNonFatal(nil)
		defer d.bnetBackfillActive.Add(-1)
		for _, toon := range pending {
			_, _ = d.getOrFetchBnetProfile(d.ctx, toon.Toon, int64(toon.Gateway), bnetfacade.PriorityBackground, 0)
		}
	}()
}

func (d *Dashboard) featuredHotkeySignature(pro *propack.Pro) hotkeySignaturePayload {
	payload := hotkeySignaturePayload{Cards: pro.Hotkeys, GamesByRace: map[string]int{}}
	if payload.Cards == nil {
		payload.Cards = []*hotkeystream.Signature{}
	}
	for race, games := range pro.Races {
		payload.GamesByRace[race] = games
	}
	return payload
}

// The whole pack overlaid on one histogram is a wall, not a reference: seventy
// markers collide into each other and bury the local population underneath, and
// no single pro stays readable. The overlay is capped here rather than in the
// frontend so the payload shrinks too. Cutting by curated rank alone is
// race-skewed (the top 15 are 8 Terran, 5 Zerg and 2 Protoss), so the cap is
// taken per race. Everyone left out keeps their own player page, which shows
// that pro's placement on the same distributions.
// featuredOverlayIDs is the subset of progamers shown on the bell-curve
// skill-proxy distributions. Smaller than the players-page list to keep the
// overlays readable.
var featuredOverlayIDs = map[string]bool{
	"barracks": true, "best": true, "bisu": true, "effort": true,
	"hero": true, "jaedong": true, "jyj": true, "larva": true,
	"leta": true, "light": true, "mind": true, "queen": true,
	"royal": true, "rush": true, "sharp": true, "shuttle": true,
	"snow": true, "soma": true, "speed": true, "ssak": true,
}

const (
	featuredOverlayPerRace = 5
	featuredOverlayLimit   = 20
)

// featuredOverlayPros picks the pros to draw on one skill-proxy distribution:
// the best curated ranks per race among those hasValue accepts, so a pro with
// no value for that particular metric does not cost the overlay a slot.
func (d *Dashboard) featuredOverlayPros(hasValue func(*propack.Pro) bool) []*propack.Pro {
	perRace := map[string]int{}
	out := make([]*propack.Pro, 0, featuredOverlayLimit)
	for _, pro := range d.featuredPros() {
		if len(out) >= featuredOverlayLimit {
			break
		}
		if !featuredOverlayIDs[pro.ID] || !hasValue(pro) || perRace[pro.MainRace] >= featuredOverlayPerRace {
			continue
		}
		perRace[pro.MainRace]++
		out = append(out, pro)
	}
	return out
}

var errFeaturedPlayerHasNoLocalData = errors.New("built-in profiles have no local games")

// handlerProPhoto serves a pro's embedded Liquipedia portrait.
func (d *Dashboard) handlerProPhoto(w http.ResponseWriter, r *http.Request) {
	pack := d.loadProPack()
	if pack == nil {
		http.NotFound(w, r)
		return
	}
	pro := pack.ByID(mux.Vars(r)["proID"])
	data, mime, ok := propack.Photo(pro)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=604800")
	_, _ = w.Write(data)
}

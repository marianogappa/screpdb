package dashboard

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/models"
)

// playerSummaryTopBOPerMatchup caps how many BO/marker entries each
// matchup card surfaces. Five is enough to spot patterns without making
// the card a wall of pills.
const playerSummaryTopBOPerMatchup = 5
const playerSummaryTopMarkersPerMatchup = 5

// neverAlliedMultiTeamEligible decides whether the "🚫 alliances" pill
// fires for the player. The threshold is intentionally one game — the user
// asked for any signal — but the pill body still surfaces the sample size
// so a user with one game knows it's a one-game observation.
func neverAlliedMultiTeamEligible(multiTeamGames, allianceCommands int64) bool {
	return multiTeamGames >= 1 && allianceCommands == 0
}

// neverHotkeysEligible reads the precomputed used_hotkey_groups marker rate
// (a value in [0,100]) and fires the pill when the player has at least one
// recorded game and the rate is exactly zero. The marker has its own 7+
// minute gate at ingest, so short games never affect the average.
func neverHotkeysEligible(totalGames int64, hotkeyGamesRatePercent float64) bool {
	return totalGames >= 1 && hotkeyGamesRatePercent == 0
}

// playerSummaryOutlierPillCap caps the number of distinctive-outlier pills
// surfaced on the Summary tab. The Skill proxies tab keeps the full list.
const playerSummaryOutlierPillCap = 12

// outlierIconKey resolves the unit/building icon key for an outlier pill.
// Build/Train/Morph: the name IS the unit type. Order: lookup the casting
// unit via models.UnitOrderToUnit. Tech/Upgrade: a small hand-rolled map
// to the unit most associated with the tech (e.g. Psionic Storm -> High
// Templar) — leaving it empty is fine, the pill just renders text-only.
//
// Returns the normalized key the frontend's getUnitIcon expects (i.e. the
// same key used by /api/custom/game-assets/*). Empty when nothing fits.
func outlierIconKey(category, name string) string {
	switch category {
	case "Build", "Train", "Morph":
		key, _, ok := resolveGameAssetIconQuery(name)
		if ok {
			return key
		}
		return ""
	case "Order":
		if origin, ok := models.UnitOrderToUnit[name]; ok && origin.Unit != "" {
			key, _, ok := resolveGameAssetIconQuery(origin.Unit)
			if ok {
				return key
			}
		}
		return ""
	case "Tech":
		if unit := outlierTechToUnit(name); unit != "" {
			key, _, ok := resolveGameAssetIconQuery(unit)
			if ok {
				return key
			}
		}
		return ""
	case "Upgrade":
		if unit := outlierUpgradeToUnit(name); unit != "" {
			key, _, ok := resolveGameAssetIconQuery(unit)
			if ok {
				return key
			}
		}
		return ""
	}
	return ""
}

// outlierTechToUnit maps a Tech command name to the unit most associated
// with it. Used purely for icon attribution on the Summary pills row —
// missing entries fall back to no-icon, never an error.
var outlierTechMap = map[string]string{
	"Stim Packs":       "Marine",
	"Lockdown":         "Ghost",
	"EMP Shockwave":    "Science Vessel",
	"Spider Mines":     "Vulture",
	"Tank Siege Mode":  "Siege Tank",
	"Defensive Matrix": "Science Vessel",
	"Irradiate":        "Science Vessel",
	"Yamato Gun":       "Battlecruiser",
	"Cloaking Field":   "Wraith",
	"Personnel Cloaking": "Ghost",
	"Restoration":      "Medic",
	"Optical Flare":    "Medic",
	"Healing":          "Medic",

	"Burrowing":        "Hydralisk",
	"Infestation":      "Queen",
	"Spawn Broodlings": "Queen",
	"Parasite":         "Queen",
	"Ensnare":          "Queen",
	"Dark Swarm":       "Defiler",
	"Plague":           "Defiler",
	"Consume":          "Defiler",
	"Lurker Aspect":    "Hydralisk",

	"Psionic Storm":  "High Templar",
	"Hallucination":  "High Templar",
	"Recall":         "Arbiter",
	"Stasis Field":   "Arbiter",
	"Archon Warp":    "High Templar",
	"Disruption Web": "Corsair",
	"Mind Control":   "Dark Archon",
	"Dark Archon Meld": "Dark Archon",
	"Feedback":       "Dark Archon",
	"Maelstrom":      "Dark Archon",
}

func outlierTechToUnit(name string) string { return outlierTechMap[name] }

// outlierUpgradeToUnit picks the unit most strongly associated with an
// upgrade. For ambiguous global upgrades (Terran Vehicle Weapons etc.) we
// pick the canonical recipient; for the parenthetical-tagged upgrades we
// extract from the parenthesis.
func outlierUpgradeToUnit(name string) string {
	if unit := outlierUpgradeMap[name]; unit != "" {
		return unit
	}
	// Many upgrades have the unit in parens, e.g. "Singularity Charge (Dragoon Range)".
	if open := strings.Index(name, "("); open >= 0 {
		closeIdx := strings.Index(name[open:], ")")
		if closeIdx > 0 {
			inner := strings.TrimSpace(name[open+1 : open+closeIdx])
			fields := strings.Fields(inner)
			if len(fields) >= 1 {
				return fields[0]
			}
		}
	}
	return ""
}

var outlierUpgradeMap = map[string]string{
	"Terran Infantry Armor":   "Marine",
	"Terran Vehicle Plating":  "Siege Tank",
	"Terran Ship Plating":     "Battlecruiser",
	"Terran Infantry Weapons": "Marine",
	"Terran Vehicle Weapons":  "Siege Tank",
	"Terran Ship Weapons":     "Battlecruiser",
	"Zerg Carapace":           "Zergling",
	"Zerg Flyer Carapace":     "Mutalisk",
	"Zerg Melee Attacks":      "Zergling",
	"Zerg Missile Attacks":    "Hydralisk",
	"Zerg Flyer Attacks":      "Mutalisk",
	"Protoss Ground Armor":    "Zealot",
	"Protoss Air Armor":       "Scout",
	"Protoss Ground Weapons":  "Zealot",
	"Protoss Air Weapons":     "Scout",
	"Protoss Plasma Shields":  "Zealot",
	"Scarab Damage":           "Reaver",
	"Reaver Capacity":         "Reaver",
	"Carrier Capacity":        "Carrier",
	"Defiler Energy":          "Defiler",

	// Parenthesised upgrades whose unit name doesn't match the icon
	// registry verbatim. The fallback parser extracts the first word in
	// parens (e.g. "Templar" from "Khaydarin Amulet (Templar Energy)")
	// but the icon registry only knows "hightemplar" / "darkarchon" etc.
	// Pin these explicitly so each upgrade pill picks up its proper
	// caster icon.
	"Khaydarin Amulet (Templar Energy)":     "High Templar",
	"Khaydarin Core (Arbiter Energy)":       "Arbiter",
	"Argus Jewel (Corsair Energy)":          "Corsair",
	"Argus Talisman (Dark Archon Energy)":   "Dark Archon",
	"Caduceus Reactor (Medic Energy)":       "Medic",
	"Apollo Reactor (Wraith Energy)":        "Wraith",
	"Colossus Reactor (Battle Cruiser Energy)": "Battlecruiser",
	"Titan Reactor (Science Vessel Energy)": "Science Vessel",
	"Moebius Reactor (Ghost Energy)":        "Ghost",
	"Ocular Implants (Ghost Sight)":         "Ghost",
	"U-238 Shells (Marine Range)":           "Marine",
	"Ion Thrusters (Vulture Speed)":         "Vulture",
	"Charon Boosters (Goliath Range)":       "Goliath",
	"Singularity Charge (Dragoon Range)":    "Dragoon",
	"Leg Enhancement (Zealot Speed)":        "Zealot",
	"Gravitic Drive (Shuttle Speed)":        "Shuttle",
	"Sensor Array (Observer Sight)":         "Observer",
	"Gravitic Booster (Observer Speed)":     "Observer",
	"Apial Sensors (Scout Sight)":           "Scout",
	"Gravitic Thrusters (Scout Speed)":      "Scout",
	"Antennae (Overlord Sight)":             "Overlord",
	"Pneumatized Carapace (Overlord Speed)": "Overlord",
	"Ventral Sacs (Overlord Transport)":     "Overlord",
	"Metabolic Boost (Zergling Speed)":      "Zergling",
	"Adrenal Glands (Zergling Attack)":      "Zergling",
	"Muscular Augments (Hydralisk Speed)":   "Hydralisk",
	"Grooved Spines (Hydralisk Range)":      "Hydralisk",
	"Gamete Meiosis (Queen Energy)":         "Queen",
	"Chitinous Plating (Ultralisk Armor)":   "Ultralisk",
	"Anabolic Synthesis (Ultralisk Speed)":  "Ultralisk",
}

// outlierMoneyBag is the suffix shown on Money-segment pills instead of
// the verbose "· Money" label.
const outlierMoneyBag = "💰"

// outlierPillLabel composes the user-facing label for a pill. Money-
// segment pills get a money-bag emoji suffix; Regular and all-maps pills
// share the same plain label since the user only needs to disambiguate
// money-map signatures from everything else.
func outlierPillLabel(prettyName, mapKind string) string {
	if mapKind == "Money" {
		return prettyName + " " + outlierMoneyBag
	}
	return prettyName
}

// playerSummaryOutlierMapKindSegments enumerates the segments we evaluate
// outliers under. "" means the all-maps corpus; the others restrict the
// corpus to that map kind so e.g. Carrier-on-Money pills don't get diluted
// by Carrier-on-Regular baselines. We compute all three from a single
// combined SQL query per spec — see ListOutlierPlayerCountsSegmented.
var playerSummaryOutlierMapKindSegments = []string{"", "Regular", "Money"}

func (d *Dashboard) buildWorkflowPlayerSummaryPerMatchup(playerKey string) (workflowPlayerSummaryPerMatchup, error) {
	result := workflowPlayerSummaryPerMatchup{
		SummaryVersion: workflowSummaryVersion,
		PlayerKey:      playerKey,
		Cards:          []workflowPlayerSummaryCard{},
	}
	playerName, err := d.playerNameForKey(playerKey)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(playerName) == "" {
		return result, sql.ErrNoRows
	}
	result.PlayerName = playerName

	sortAndCap := func(entries []workflowPlayerMatchupPatternCount, cap int) []workflowPlayerMatchupPatternCount {
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].Count != entries[j].Count {
				return entries[i].Count > entries[j].Count
			}
			return entries[i].PatternName < entries[j].PatternName
		})
		if len(entries) > cap {
			entries = entries[:cap]
		}
		return entries
	}
	emptyIfNil := func(s []workflowPlayerMatchupPatternCount) []workflowPlayerMatchupPatternCount {
		if s == nil {
			return []workflowPlayerMatchupPatternCount{}
		}
		return s
	}

	cards := []workflowPlayerSummaryCard{}

	// 1v1 matchup cards
	aggRows, err := d.dbStore.ListPlayerMatchupAggregates(d.ctx, playerKey)
	if err != nil {
		return result, fmt.Errorf("failed to load matchup aggregates: %w", err)
	}
	if len(aggRows) > 0 {
		markerRows, err := d.dbStore.ListPlayerMatchupMarkerCounts(d.ctx, playerKey)
		if err != nil {
			return result, fmt.Errorf("failed to load matchup marker counts: %w", err)
		}
		type matchupKey struct {
			own string
			opp string
		}
		bos := map[matchupKey][]workflowPlayerMatchupPatternCount{}
		markers := map[matchupKey][]workflowPlayerMatchupPatternCount{}
		for _, row := range markerRows {
			key := matchupKey{own: row.OwnRace, opp: row.OppRace}
			entry := workflowPlayerMatchupPatternCount{PatternName: row.PatternName, Count: row.ReplayCount}
			if strings.HasPrefix(row.PatternName, "bo_") {
				bos[key] = append(bos[key], entry)
			} else {
				markers[key] = append(markers[key], entry)
			}
		}
		for _, row := range aggRows {
			var winRate float64
			if row.Games > 0 {
				winRate = float64(row.Wins) / float64(row.Games)
			}
			key := matchupKey{own: row.OwnRace, opp: row.OppRace}
			cards = append(cards, workflowPlayerSummaryCard{
				Kind:           "matchup",
				Key:            "m:" + row.OwnRace + ":" + row.OppRace,
				OwnRace:        row.OwnRace,
				OppRace:        row.OppRace,
				Games:          row.Games,
				Wins:           row.Wins,
				WinRate:        winRate,
				Confidence:     matchupConfidenceForGames(row.Games),
				AvgAPM:         row.AvgAPM,
				AvgEAPM:        row.AvgEAPM,
				TopBuildOrders: emptyIfNil(sortAndCap(bos[key], playerSummaryTopBOPerMatchup)),
				TopMarkers:     emptyIfNil(sortAndCap(markers[key], playerSummaryTopMarkersPerMatchup)),
			})
		}
	}

	// By-format cards: each is per-(format_class, map_kind, own_race) so a
	// Random player gets distinct cards (and so distinct top-N BO/marker
	// pills) for each race they play in a given format. Multiple raw
	// team_formats may roll up to a single bucket (any team_format with
	// 2+ 'v's becomes "multi-team"), in which case we re-average APM/EAPM
	// weighted by games and sum game/win counts.
	byFormatAgg, err := d.dbStore.ListPlayerByFormatAggregates(d.ctx, playerKey)
	if err != nil {
		return result, fmt.Errorf("failed to load by-format aggregates: %w", err)
	}
	if len(byFormatAgg) > 0 {
		byFormatMarkers, err := d.dbStore.ListPlayerByFormatMarkerCounts(d.ctx, playerKey)
		if err != nil {
			return result, fmt.Errorf("failed to load by-format marker counts: %w", err)
		}
		type fmtKey struct {
			format string
			mk     string
			race   string
		}
		bos := map[fmtKey]map[string]int64{}
		markers := map[fmtKey]map[string]int64{}
		for _, row := range byFormatMarkers {
			cls := teamFormatToClass(row.TeamFormat)
			if cls == "" {
				continue
			}
			key := fmtKey{format: cls, mk: row.MapKind, race: row.OwnRace}
			target := markers
			if strings.HasPrefix(row.PatternName, "bo_") {
				target = bos
			}
			if target[key] == nil {
				target[key] = map[string]int64{}
			}
			target[key][row.PatternName] += row.ReplayCount
		}
		type aggBucket struct {
			games   int64
			wins    int64
			apmSum  float64
			eapmSum float64
			apmN    int64
			eapmN   int64
		}
		buckets := map[fmtKey]*aggBucket{}
		for _, row := range byFormatAgg {
			if row.Games <= 0 {
				continue
			}
			cls := teamFormatToClass(row.TeamFormat)
			if cls == "" {
				continue
			}
			key := fmtKey{format: cls, mk: row.MapKind, race: row.OwnRace}
			b := buckets[key]
			if b == nil {
				b = &aggBucket{}
				buckets[key] = b
			}
			b.games += row.Games
			b.wins += row.Wins
			if row.AvgAPM > 0 {
				b.apmSum += row.AvgAPM * float64(row.Games)
				b.apmN += row.Games
			}
			if row.AvgEAPM > 0 {
				b.eapmSum += row.AvgEAPM * float64(row.Games)
				b.eapmN += row.Games
			}
		}
		toEntries := func(m map[string]int64) []workflowPlayerMatchupPatternCount {
			out := make([]workflowPlayerMatchupPatternCount, 0, len(m))
			for name, count := range m {
				out = append(out, workflowPlayerMatchupPatternCount{PatternName: name, Count: count})
			}
			return out
		}
		for key, b := range buckets {
			var winRate, avgAPM, avgEAPM float64
			if b.games > 0 {
				winRate = float64(b.wins) / float64(b.games)
			}
			if b.apmN > 0 {
				avgAPM = b.apmSum / float64(b.apmN)
			}
			if b.eapmN > 0 {
				avgEAPM = b.eapmSum / float64(b.eapmN)
			}
			cards = append(cards, workflowPlayerSummaryCard{
				Kind:           "format",
				Key:            "f:" + key.format + ":" + key.mk + ":" + key.race,
				OwnRace:        key.race,
				FormatClass:    key.format,
				MapKind:        key.mk,
				Games:          b.games,
				Wins:           b.wins,
				WinRate:        winRate,
				Confidence:     matchupConfidenceForGames(b.games),
				AvgAPM:         avgAPM,
				AvgEAPM:        avgEAPM,
				TopBuildOrders: emptyIfNil(sortAndCap(toEntries(bos[key]), playerSummaryTopBOPerMatchup)),
				TopMarkers:     emptyIfNil(sortAndCap(toEntries(markers[key]), playerSummaryTopMarkersPerMatchup)),
			})
		}
	}

	// Sort by games desc; tie-break by key for stable ordering.
	sort.SliceStable(cards, func(i, j int) bool {
		if cards[i].Games != cards[j].Games {
			return cards[i].Games > cards[j].Games
		}
		return cards[i].Key < cards[j].Key
	})
	result.Cards = cards
	return result, nil
}

// teamFormatToClass collapses raw team_format strings into the three
// Summary-tab buckets. "1v1" returns "" so the caller can skip — 1v1 has
// its own dedicated matchup cards. Any team_format with at least two 'v's
// is "multi-team" regardless of player counts (so 2v2v2v2, 1v1v1v1,
// 3v2v1, etc. all roll up together).
func teamFormatToClass(teamFormat string) string {
	switch teamFormat {
	case "1v1", "":
		return ""
	case "2v2":
		return "2v2"
	case "3v3":
		return "3v3"
	}
	if strings.Count(teamFormat, "v") >= 2 {
		return "multi-team"
	}
	return ""
}

// buildWorkflowPlayerSummarySpecial returns ONLY the cheap eligibility
// flags (never-allied, never-hotkeys). Outlier pills are now served by
// per-category endpoints (/summary/outliers?category=...) so the FE can
// stream them in independently and the heavy aggregate work isn't all
// gated behind a single 60-90s response. The OutlierPills slice stays
// in the response shape but is always empty here — the FE merges results
// from the per-category endpoints.
func (d *Dashboard) buildWorkflowPlayerSummarySpecial(playerKey string) (workflowPlayerSummarySpecial, error) {
	result := workflowPlayerSummarySpecial{
		SummaryVersion: workflowSummaryVersion,
		PlayerKey:      playerKey,
		OutlierPills:   []workflowPlayerSummaryOutlierPill{},
	}
	playerName, err := d.playerNameForKey(playerKey)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(playerName) == "" {
		return result, sql.ErrNoRows
	}
	result.PlayerName = playerName

	multiTeamGames, err := d.dbStore.CountPlayerMultiTeamMeleeGames(d.ctx, playerKey)
	if err != nil {
		return result, fmt.Errorf("failed to count multi-team melee games: %w", err)
	}
	allianceCmds := int64(0)
	if multiTeamGames > 0 {
		allianceCmds, err = d.dbStore.CountPlayerAllianceCommandsInMultiTeamMelee(d.ctx, playerKey)
		if err != nil {
			return result, fmt.Errorf("failed to count alliance commands: %w", err)
		}
	}
	result.NeverAlliedMultiTeam = workflowPlayerSpecialEligibleStat{
		Eligible: neverAlliedMultiTeamEligible(multiTeamGames, allianceCmds),
		Games:    multiTeamGames,
	}

	hotkeyGamesRate, err := d.dbStore.ListHotkeyGamesRateByPlayer(d.ctx)
	if err != nil {
		return result, fmt.Errorf("failed to load hotkey usage rates: %w", err)
	}
	gamesByRace, err := d.playerGamesByRace(playerKey)
	if err != nil {
		return result, err
	}
	totalGames := int64(0)
	for _, n := range gamesByRace {
		totalGames += n
	}
	result.NeverHotkeys = workflowPlayerSpecialEligibleStat{
		Eligible: neverHotkeysEligible(totalGames, hotkeyGamesRate[playerKey]),
		Games:    totalGames,
	}
	return result, nil
}

// playerSummaryPillsRaceMinGames thresholds the per-race pill computation
// so micro-tail races (a Random player with 4 Terran games and 200 Zerg)
// don't produce false-positive pills off small denominators. Below this
// threshold we fall back to the player's most-played race so very-low-
// volume players still get something.
const playerSummaryPillsRaceMinGames = 10

// buildWorkflowPlayerSummaryOutliersForCategory runs the segmented outlier
// computation for a single category (Order, Build, Train, Morph, Tech,
// Upgrade) so the frontend can fan out to one HTTP request per category
// and render pills incrementally as each finishes.
//
// Within a category we iterate over ALL races the player plays
// meaningfully (>= playerSummaryPillsRaceMinGames). This is what gives
// Random players coverage across all three races — without it chobo86
// would see only Zerg pills (his most-played race) and miss
// Protoss-only signatures like "Khaydarin Amulet" or Terran-only ones
// like "EMP Shockwave". The per-category split keeps each individual
// HTTP response under the WriteTimeout budget even with 3× the
// per-race work, because there are 6 parallel requests instead of one
// monolith.
func (d *Dashboard) buildWorkflowPlayerSummaryOutliersForCategory(playerKey, category string) (workflowPlayerSummaryOutliers, error) {
	result := workflowPlayerSummaryOutliers{
		SummaryVersion: workflowSummaryVersion,
		PlayerKey:      playerKey,
		Category:       category,
		Pills:          []workflowPlayerSummaryOutlierPill{},
	}
	spec, ok := lookupOutlierSpec(category)
	if !ok {
		return result, errUnknownOutlierCategory
	}
	playerName, err := d.playerNameForKey(playerKey)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(playerName) == "" {
		return result, sql.ErrNoRows
	}
	gamesByRace, err := d.playerGamesByRace(playerKey)
	if err != nil {
		return result, err
	}
	if len(gamesByRace) == 0 {
		return result, nil
	}
	races := []string{}
	for race, games := range gamesByRace {
		if games >= playerSummaryPillsRaceMinGames {
			races = append(races, race)
		}
	}
	if len(races) == 0 {
		// Fall back to most-played race for low-volume players.
		bestRace, bestGames := "", int64(0)
		for race, games := range gamesByRace {
			if games > bestGames {
				bestRace = race
				bestGames = games
			}
		}
		if bestRace == "" {
			return result, nil
		}
		races = []string{bestRace}
	}

	popGamesByRace, err := d.populationGamesByRace()
	if err != nil {
		return result, err
	}
	popDistinctPlayersByRace, err := d.populationDistinctPlayersByRace()
	if err != nil {
		return result, err
	}
	playerGamesByRaceMK, err := d.playerGamesByRaceMapKind(playerKey)
	if err != nil {
		return result, err
	}
	popGamesByRaceMK, err := d.populationGamesByRaceMapKind()
	if err != nil {
		return result, err
	}
	popDistinctPlayersByRaceMK, err := d.populationDistinctPlayersByRaceMapKind()
	if err != nil {
		return result, err
	}

	thresholds := workflowOutlierThresholds{
		TFIDFMin: workflowOutlierTFIDFMin,
		RatioMin: workflowOutlierRatioMin,
	}
	all := []workflowPlayerSummaryOutlierPill{}
	for _, race := range races {
		pills, err := d.segmentedOutliersForSpec(playerKey, race, spec,
			gamesByRace, popGamesByRace, popDistinctPlayersByRace,
			playerGamesByRaceMK, popGamesByRaceMK, popDistinctPlayersByRaceMK,
			thresholds)
		if err != nil {
			return result, err
		}
		all = append(all, pills...)
	}
	// Dedup within this category: same (race, name) should pick the most
	// informative pill (segmented over all-maps, then highest TFIDF).
	// Different races' same-named items stay distinct.
	type dedupKey struct {
		race string
		name string
	}
	best := map[dedupKey]int{}
	for i, p := range all {
		key := dedupKey{race: p.Race, name: p.Name}
		if existing, ok := best[key]; !ok {
			best[key] = i
		} else {
			cur := all[existing]
			curSegmented := cur.MapKind != ""
			newSegmented := p.MapKind != ""
			switch {
			case newSegmented && !curSegmented:
				best[key] = i
			case curSegmented == newSegmented && p.TFIDF > cur.TFIDF:
				best[key] = i
			}
		}
	}
	deduped := make([]workflowPlayerSummaryOutlierPill, 0, len(best))
	for _, idx := range best {
		deduped = append(deduped, all[idx])
	}
	sort.SliceStable(deduped, func(i, j int) bool {
		if deduped[i].TFIDF == deduped[j].TFIDF {
			return deduped[i].RatioToBaseline > deduped[j].RatioToBaseline
		}
		return deduped[i].TFIDF > deduped[j].TFIDF
	})
	result.Pills = deduped
	return result, nil
}

var errUnknownOutlierCategory = errors.New("unknown outlier category")

func lookupOutlierSpec(category string) (workflowOutlierCategorySpec, bool) {
	for _, spec := range workflowOutlierSpecs() {
		if strings.EqualFold(spec.CategoryLabel, category) {
			return spec, true
		}
	}
	return workflowOutlierCategorySpec{}, false
}


// workflowOutlierSpecs is the canonical list of outlier specs used by both
// the Skill-proxies tab and the Summary-tab pills row. Kept in one place so
// the two surfaces stay in sync.
func workflowOutlierSpecs() []workflowOutlierCategorySpec {
	return []workflowOutlierCategorySpec{
		{CategoryLabel: "Order", ActionTypes: []string{"Targeted Order"}, NameColumn: "order_name", UseInstanceShare: true},
		{CategoryLabel: "Build", ActionTypes: []string{"Build", "Building Morph"}, NameColumn: "unit_type"},
		{CategoryLabel: "Train", ActionTypes: []string{"Train"}, NameColumn: "unit_type"},
		{CategoryLabel: "Morph", ActionTypes: []string{"Unit Morph"}, NameColumn: "unit_type"},
		{CategoryLabel: "Tech", ActionTypes: []string{"Tech"}, NameColumn: "tech_name"},
		{CategoryLabel: "Upgrade", ActionTypes: []string{"Upgrade"}, NameColumn: "upgrade_name"},
	}
}

// segmentedOutliersForSpec runs ONE player query + ONE corpus query (each
// returning all-maps + per-mapkind segment columns) and computes outlier
// pills for the all-maps, Regular and Money segments in a single pass.
// This keeps the round-trip count at 2 per spec (vs 6 for the naive
// 3-segments-3-roundtrips approach), so the Summary tab pills row matches
// the cost profile of the existing Skill-proxies Outliers tab.
func (d *Dashboard) segmentedOutliersForSpec(
	playerKey string,
	primaryRace string,
	spec workflowOutlierCategorySpec,
	playerGamesByRaceAll map[string]int64,
	popGamesByRaceAll map[string]int64,
	popDistinctPlayersByRaceAll map[string]float64,
	playerGamesByRaceMK map[string]map[string]int64,
	popGamesByRaceMK map[string]map[string]int64,
	popDistinctPlayersByRaceMK map[string]map[string]float64,
	thresholds workflowOutlierThresholds,
) ([]workflowPlayerSummaryOutlierPill, error) {
	playerRows, err := d.dbStore.ListOutlierPlayerCountsSegmented(d.ctx, playerKey, primaryRace, spec.NameColumn, spec.UseInstanceShare, spec.ActionTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to query player segmented outliers for %s: %w", spec.CategoryLabel, err)
	}
	globalRows, err := d.dbStore.ListOutlierGlobalRowsSegmented(d.ctx, primaryRace, spec.NameColumn, spec.UseInstanceShare, spec.ActionTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to query global segmented outliers for %s: %w", spec.CategoryLabel, err)
	}

	type pair struct {
		race string
		name string
	}
	type playerSegCounts struct {
		all     int64
		regular int64
		money   int64
	}
	type globalSegCounts struct {
		gamesAll       int64
		gamesRegular   int64
		gamesMoney     int64
		playersAll     float64
		playersRegular float64
		playersMoney   float64
	}
	playerCounts := map[pair]playerSegCounts{}
	for _, row := range playerRows {
		playerCounts[pair{race: strings.TrimSpace(row.Race), name: strings.TrimSpace(row.Name)}] = playerSegCounts{
			all: row.GamesAll, regular: row.GamesRegular, money: row.GamesMoney,
		}
	}
	globalCounts := map[pair]globalSegCounts{}
	for _, row := range globalRows {
		key := pair{race: strings.TrimSpace(row.Race), name: strings.TrimSpace(row.Name)}
		globalCounts[key] = globalSegCounts{
			gamesAll: row.GamesAll, gamesRegular: row.GamesRegular, gamesMoney: row.GamesMoney,
			playersAll: row.PlayersAll, playersRegular: row.PlayersRegular, playersMoney: row.PlayersMoney,
		}
	}

	// Targeted-order denominators: when spec.UseInstanceShare we score by
	// share of total order *instances*, not replay incidence. The totals
	// must come from the same filtered universe (player's primary race,
	// allowed for race, not a generic order) — computed per-segment so each
	// pill has a denominator that matches its numerator.
	var pTotalAll, pTotalRegular, pTotalMoney int64
	var gTotalAll, gTotalRegular, gTotalMoney int64
	if spec.UseInstanceShare {
		for key, counts := range playerCounts {
			if !strings.EqualFold(strings.TrimSpace(key.race), strings.TrimSpace(primaryRace)) {
				continue
			}
			if !workflowItemAllowedForPrimaryRace(primaryRace, spec, key.name) {
				continue
			}
			if workflowSkipGenericTargetedOrder(key.name) {
				continue
			}
			pTotalAll += counts.all
			pTotalRegular += counts.regular
			pTotalMoney += counts.money
		}
		for key, counts := range globalCounts {
			if !strings.EqualFold(strings.TrimSpace(key.race), strings.TrimSpace(primaryRace)) {
				continue
			}
			if !workflowItemAllowedForPrimaryRace(primaryRace, spec, key.name) {
				continue
			}
			if workflowSkipGenericTargetedOrder(key.name) {
				continue
			}
			gTotalAll += counts.gamesAll
			gTotalRegular += counts.gamesRegular
			gTotalMoney += counts.gamesMoney
		}
	}

	out := []workflowPlayerSummaryOutlierPill{}
	for key, pc := range playerCounts {
		if !strings.EqualFold(strings.TrimSpace(key.race), strings.TrimSpace(primaryRace)) {
			continue
		}
		if !workflowItemAllowedForPrimaryRace(primaryRace, spec, key.name) {
			continue
		}
		if spec.UseInstanceShare && workflowSkipGenericTargetedOrder(key.name) {
			continue
		}
		gc := globalCounts[key]

		// Evaluate each segment independently and keep any pill that
		// qualifies. Within the same (category, name) pair the dedup pass
		// at the call site picks the best to surface.
		for _, seg := range []struct {
			mapKind         string
			playerSegGames  int64
			globalSegGames  int64
			globalSegPlayers float64
			playerRaceGames  int64
			popRaceGames     int64
			popRacePlayers   float64
			playerInstanceTotal int64
			globalInstanceTotal int64
		}{
			{
				mapKind: "", playerSegGames: pc.all, globalSegGames: gc.gamesAll, globalSegPlayers: gc.playersAll,
				playerRaceGames: playerGamesByRaceAll[key.race], popRaceGames: popGamesByRaceAll[key.race], popRacePlayers: popDistinctPlayersByRaceAll[key.race],
				playerInstanceTotal: pTotalAll, globalInstanceTotal: gTotalAll,
			},
			{
				mapKind: "Regular", playerSegGames: pc.regular, globalSegGames: gc.gamesRegular, globalSegPlayers: gc.playersRegular,
				playerRaceGames: playerGamesByRaceMK[key.race]["Regular"], popRaceGames: popGamesByRaceMK[key.race]["Regular"], popRacePlayers: popDistinctPlayersByRaceMK[key.race]["Regular"],
				playerInstanceTotal: pTotalRegular, globalInstanceTotal: gTotalRegular,
			},
			{
				mapKind: "Money", playerSegGames: pc.money, globalSegGames: gc.gamesMoney, globalSegPlayers: gc.playersMoney,
				playerRaceGames: playerGamesByRaceMK[key.race]["Money"], popRaceGames: popGamesByRaceMK[key.race]["Money"], popRacePlayers: popDistinctPlayersByRaceMK[key.race]["Money"],
				playerInstanceTotal: pTotalMoney, globalInstanceTotal: gTotalMoney,
			},
		} {
			if seg.playerSegGames < 3 || seg.globalSegGames <= 0 {
				continue
			}
			if seg.playerRaceGames <= 0 || seg.popRaceGames <= 0 || seg.popRacePlayers <= 0 {
				continue
			}
			playerDen := float64(seg.playerRaceGames)
			baselineDen := float64(seg.popRaceGames)
			if spec.UseInstanceShare {
				if seg.playerInstanceTotal <= 0 || seg.globalInstanceTotal <= 0 {
					continue
				}
				playerDen = float64(seg.playerInstanceTotal)
				baselineDen = float64(seg.globalInstanceTotal)
			}
			playerRate := float64(seg.playerSegGames) / playerDen
			baselineRate := float64(seg.globalSegGames) / baselineDen
			if baselineRate <= 0 || playerRate < 0.15 {
				continue
			}
			ratio := playerRate / baselineRate
			idf := math.Log((1.0+seg.popRacePlayers)/(1.0+seg.globalSegPlayers)) + 1.0
			tfidf := playerRate * idf
			qualifiedBy := []string{}
			if tfidf >= thresholds.TFIDFMin {
				qualifiedBy = append(qualifiedBy, "Rare signature")
			}
			if ratio >= thresholds.RatioMin {
				qualifiedBy = append(qualifiedBy, "Much more frequent than peers")
			}
			if len(qualifiedBy) == 0 {
				continue
			}
			pretty := prettySplitUppercase(key.name)
			out = append(out, workflowPlayerSummaryOutlierPill{
				Category:        spec.CategoryLabel,
				Name:            key.name,
				PrettyName:      pretty,
				PrettyLabel:     outlierPillLabel(pretty, seg.mapKind),
				IconKey:         outlierIconKey(spec.CategoryLabel, key.name),
				Race:            key.race,
				MapKind:         seg.mapKind,
				PlayerGames:     seg.playerSegGames,
				PlayerRate:      playerRate,
				BaselineRate:    baselineRate,
				RatioToBaseline: ratio,
				TFIDF:           tfidf,
				QualifiedBy:     qualifiedBy,
			})
		}
	}
	return out, nil
}

func (d *Dashboard) playerGamesByRaceMapKind(playerKey string) (map[string]map[string]int64, error) {
	rows, err := d.dbStore.ListPlayerGamesByRaceMapKind(d.ctx, playerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load player games by race+map_kind: %w", err)
	}
	out := map[string]map[string]int64{}
	for _, row := range rows {
		race := strings.TrimSpace(row.Race)
		mk := strings.TrimSpace(row.MapKind)
		if out[race] == nil {
			out[race] = map[string]int64{}
		}
		out[race][mk] = row.Count
	}
	return out, nil
}

func (d *Dashboard) populationGamesByRaceMapKind() (map[string]map[string]int64, error) {
	rows, err := d.dbStore.ListPopulationGamesByRaceMapKind(d.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load population games by race+map_kind: %w", err)
	}
	out := map[string]map[string]int64{}
	for _, row := range rows {
		race := strings.TrimSpace(row.Race)
		mk := strings.TrimSpace(row.MapKind)
		if out[race] == nil {
			out[race] = map[string]int64{}
		}
		out[race][mk] = row.Count
	}
	return out, nil
}

func (d *Dashboard) populationDistinctPlayersByRaceMapKind() (map[string]map[string]float64, error) {
	rows, err := d.dbStore.ListPopulationDistinctPlayersByRaceMapKind(d.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load population distinct players by race+map_kind: %w", err)
	}
	out := map[string]map[string]float64{}
	for _, row := range rows {
		race := strings.TrimSpace(row.Race)
		mk := strings.TrimSpace(row.MapKind)
		if out[race] == nil {
			out[race] = map[string]float64{}
		}
		out[race][mk] = row.Value
	}
	return out, nil
}


package core

import (
	"encoding/json"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// AlgorithmVersion is the current version of the pattern detection algorithm
// Increment this when the algorithm changes to trigger re-detection
//
// 26: build-order overhaul — Zerg 5/6/7/8/10/11 Pool rungs, loosened FFE &
// 1 Rax FE timings, widened Protoss expand/core matchups, Bunker Rush, per-race
// residual "… (Other)" catch-alls, and the "Opener unresolved" N/A marker.
//
// 27: Build dedup now requires the same build tile, not just a 3s window —
// stops merging distinct same-type buildings placed seconds apart at different
// spots (the time-only heuristic mis-merged ~55% of its collapses). The dead
// non-streaming ApplyBuildDedup mirror was also removed.
//
// 28: Selection-tag build dedup (internal/unittags + internal/builddedup),
// applied ahead of earlyfilter: provable worker one-at-a-time drops (Terran SCV
// / Zerg Drone redirected to a different-tile build before the prior could
// finish) and never-produced production buildings within the build-order
// window. Removes redundant Build commands so building counts reflect reality.
//
// 29: Terran build-order revamp (issue #155). The topology openers 1 Rax 1 Fac
// / 1 Rax FE / 2 Rax CC and the style markers Mech / 1-1-1 / SK Terran / Mech
// transition are replaced by composition-based initial BOs classified at 10:00:
// Wraith, Goliath, N-Rax Bio, 1-1-1 (+ into Mech), N-Fac Mech, N-Fac Tankless
// Mech — split by Barracks/Factory count and bio-vs-mech predominance. New DSL
// primitives (Predominant, time-bounded produce/build counts) and a non-1v1
// matchup gate back them. CC First / BBS / Bunker Rush are kept; the Terran
// residual is now "Terran (Other)" (bo_terran_other).
//
// 30: Expert milestone timings for the composition-based Terran BOs (issue
// #158) — Wraith, Goliath, N-Rax Bio, N-Fac Mech, N-Fac Tankless Mech, 1-1-1
// (+ into Mech) now carry Expert events, so the detector persists their
// expert_actuals payload. Bumped so replays analyzed under v29 (which stored an
// empty payload for these BOs) re-analyze and populate the Build Orders chart's
// actual-vs-expert markers.
//
// 31: Coordinate enrichment (issue #175). Production / research / cancel
// commands (Train, Unit Morph, Tech, Upgrade, Building Morph, Cancel Train) —
// previously spatially blank — now carry their producing building's inferred
// pixel location (internal/unittags.Coordinates), recovered via selection-tag
// state and, for Zerg larva, frequency-confirmed Hatchery tags. These coords
// flow through ownership (a producing building further refreshes its base's
// inactivity clock) and inflate Viewport Multitasking; the recall destination
// inference excludes them. Re-ingest so stored coords/events reflect the change.
//
// 32: Preferred build-order tier. Specific, scene-named openers (e.g. "3 Hatch
// Muta", "2 Gate Reaver", "Factory Expand") are added as tier-1 markers that take
// precedence over the broad buckets they overlap (tier 2) and the residual
// "… (Other)" catch-alls (tier 3); only the best-tier opener is persisted per
// player (internal/patterns markers.Tier + Orchestrator.selectBestTierOpeners).
// Re-ingest so stored openers reflect the more specific classification.
//
// 33: Terran composition + cliff-drop accuracy fixes. (a) The Goliath opener now
// requires no Siege Tanks ("with tanks it's Mech"), so tank-heavy mech in non-TvZ
// games is no longer subtracted from the residual and lost. (b) 1-Rax Bio also
// admits a pure-Barracks opening with no Factory/Starport transition (covers a
// Marine opener cut short below the 8-Marine floor). (c) Cliff drops now require a
// Dropship (not just a Bunker's UnloadAll), ignore tile-unit Build coords in the
// unload-location fallback, use a tightened 150px corner box, and classify on
// individual unload points instead of the cluster centroid. Re-ingest so stored
// build orders and drop events reflect the corrected classification.
//
// 34: Coordinate-unit normalization. Build commands carry TILE-unit
// coordinates while every other command is pixels; the enriched stream now
// converts Build coords to pixels once in cmdenrich.Classify, so the whole
// detection pipeline is uniformly pixel-space and per-consumer conversions are
// removed. This also fixes Viewport Multitasking, which counted every Build as
// a viewport teleport to the map origin (tile coords read as pixels) and so
// over-reported switches_per_minute — most for build-heavy players. Re-ingest
// so stored Viewport Multitasking values are corrected.
//
// 35: 1v1 attack detection rewritten (issue #186). For exactly-two-opposing-
// player games, attacks are detected as bilateral space-time command clusters
// (a real fight needs both sides active in one neighbourhood), located by
// point-in-polygon (correct base kind/clock/owner) or by inter-base-axis
// relational prose in open field ("in the middle", "near X's base", with drift
// direction), and gated on per-side command count + duration — replacing the
// pressure-tracker + unit-novelty filter, which over-fired on one-sided pokes
// and mislabeled locations. Multiplayer keeps the existing per-base path.
// Also: a "starting"-kind polygon that is not an actual player main (extra
// start locations on an N-player map played 1v1) now labels as "expansion".
// Re-ingest so stored attack events reflect the new model.
//
// 36: Removed dt_drop classification (issue #185). "DT produced near a drop"
// is a weak proxy — DTs are commonly walked in cloaked or built for a later
// drop — so DT drops are no longer inferred; such unloads stay plain "drop"
// (a real DT drop is under-classified, never mislabeled). Re-ingest so stored
// dt_drop rows become "drop".
//
// 37: Removed reaver_drop classification too (issue #185 follow-up). Reavers
// are usually a-moved, leaving no reaver-specific order to confirm the drop, so
// the "Reaver produced nearby" proxy mislabeled e.g. PvT speedlot-drops-on-Tanks
// while a reaver was merely in production; only cliff_drop remains a subtype.
// Also replaced the one-reaver_drop-per-player suppression (which dropped ~300
// real target-confirmed drops corpus-wide) with a per-target time-window dedup
// (dropDedupWindowSec) so drop-heavy games stay readable without hiding distinct
// attacks. Re-ingest so stored reaver_drop rows become "drop" and dedup applies.
//
// 38: Offensive-nydus detection (issue #193). A forward BuildNydusExit placed in
// enemy territory and confirmed as an army insertion (attack-coincidence or
// post-placement activity) now emits a "nydus_attack" game event at the exit and
// an "offensive_nydus" marker. Required surfacing the BuildNydusExit order in
// cmdenrich (it arrives as an ActionType="Build" command, so it was previously
// classified as a plain building) with tile→pixel coordinate normalization.
// Re-ingest so offensive nydus pushes surface.
//
// 39: Mutalisk hit-and-run detection (issue #194). Selection/hotkey state is
// reconstructed to find the oscillating dart-in/pull-back volley signature; a
// conservative per-game-player confidence bar (strongest window ≥30 volleys over
// ≥20s) drives a presence-only "Muta hit-n-run" marker (no timeline/timing, which
// is too error-prone — a microed muta attack is geometrically indistinguishable).
// Re-ingest so the marker + games-list filter surface.
//
// 40: Build-order accuracy + modifiers. Multi-larva Zerg morphs are now counted
// by selection size (one morph command morphs every selected larva), fixing the
// supply undercount that read 11 Hatch as 10. 1 Gate Reaver no longer matches a
// 2-gate build, CC First requires canonical topology, and "Siege Expand" is
// renamed "Factory Expand" (key bo_t_factory_expand). New orthogonal build-order
// modifiers ("all-in", "proxy") ride in the marker payload. Re-ingest so corrected
// openers, the new feature key, and modifier tags surface.
//
// 41: Bio is classified by base count (1-Base / 2-Base, keys bo_t_bio_1base /
// bo_t_bio_2base) instead of Barracks count — the rax count drifts through the
// game so 1-Rax…6-Rax were unstable. The all-in modifier is subsumed by 1-Base
// and removed; a 1 Gate Reaver "expand" modifier (Nexus before the first Reaver)
// is added. Re-ingest so bio re-buckets and the new keys/modifiers surface.
//
// 42: build-dedup (Tier B never-produced) now collapses same-tile Build commands
// into one distinct building before matching them to producing tags. A spammed
// placement at one spot no longer consumes several producer matches and strands
// real buildings elsewhere as "never produced" — which had dropped real later
// gateways and made a 3-gate build read as 1 Gate Reaver. Re-ingest so building
// counts (and the openers keyed on them) are correct.
//
// 43: earlyfilter backtrack no longer force-drops income-producing worker trains
// to fund a re-admitted proven-real building unless re-admission actually
// overdraws minerals at that frame. Dropping affordable workers froze the
// simulated worker count and collapsed income into a spiral that wiped out an
// early worker line (an SCV-from-frame-0 opening read as no workers until ~4
// min) and undercounted build-order supply (a 9 Hatch read as 4 Hatch). Bunker
// rush detection now bounds its base-snap fallback, so an open-ground proxy /
// simcity bunker on a money map is no longer misread as a rush on a distant
// base. Re-ingest so early worker timing, BO supply counts, and bunker-rush
// events are correct.
//
// 44: Terran air/specialist openers (#228). The TvZ "Wraith" composition opener
// is folded into a matchup-shared "2 Port Wraith" (TvT+TvZ; 1 Rax/1 Fac into two
// Starports, wraith-dominant) with an "expand" modifier; "2 Fact Vults" is
// renamed "2 Fact before Expa" (key bo_t_2fact_vults -> bo_t_2fact_expa; exactly
// two Factories before the expansion, no vulture requirement). New "Wraith Cloak
// timing" pill (first Cloaking Field research) and a new "proxy_starport"
// game-event with a "proxy" modifier on 2 Port Wraith. The proxy spatial gate is
// now player-aware, so a forward building near the enemy main (not just midfield)
// fires proxy_gate/_rax/_factory/_starport. Re-ingest so the renamed/unified
// openers, the new pill, and proxy events surface.
//
// 46: new strategic markers — Made Maelstrom (PvZ Dark Archon cast), Crazy Zerg
// (TvZ Mutalisk->Ultralisk with Zerg Carapace and no Lurker before the first
// Ultralisk), and Guardians (TvZ, >=1 Guardian — required adding Guardian to
// subjectsOfInterest so the rule-path fact isn't dropped). New per-player timing
// pills First Observer (PvP/PvT) and First Mine (PvT), the latter backed by a new
// cmdenrich KindLayMine fact for the PlaceMine / VultureMine orders. Dashboard
// also gained a "beta" tag on uncurated markers (hotkey markers exempt). Re-ingest
// so the new markers and pills surface.
//
// 47: Zerg pool/hatch supply-count fix. ProduceCountBeforeBuild now counts
// produces by their game-second relative to the building, not by observation
// order — the build-dedup tail (player_marker.go) held a Build fact for a few
// seconds, during which a unit morphed just after the building was miscounted
// as before it (a Drone morphed 2s after a 9-supply Pool inflated 9 Overpool
// into 10 Pool). Re-ingest so Zerg openers re-classify correctly.
//
// 48: add the 13 Hatch hatch-first rung (9 Drone morphs + Overlord before the
// expansion Hatchery) — previously fell into the Pool/Hatch (Other) residual.
//
// 49: 3 Hatch Muta converted from a build-order opener to a TvZ composition
// marker (key three_hatch_muta) so the hatch-first opener underneath (11/12
// Hatch) surfaces on its own.
//
// 50: fuzzy Zerg opener (bo_z_fuzzy). When a multi-unit-selection Drone morph
// before the Pool/Hatchery makes the supply rung indeterminate, exact rungs no
// longer fire (they require an unambiguous count) and a "~N Pool/Overpool/Hatch"
// label is emitted at the floor instead.
//
// 51: Terran mech taxonomy reformulated (#226/#227). Mech is now named by the
// number of Factories built STRICTLY BEFORE the first expansion ("N Fact Expa
// Mech", deterministic) instead of a by-deadline factory count that conflated
// pre- and post-expansion factories. Expand-first (CC before any Factory) is
// plain "Mech"; a one-base mech with no expansion is "Mech (no expa)"; each has
// a "Tankless Mech" variant (no Siege Tanks). The tank/tankless bucket families
// (bo_t_mech_Nfac / bo_t_tankless_Nfac) are replaced by bo_t_mech_expa_Nfac /
// bo_t_tankless_expa_Nfac / _expand / _noexpa. "1-1-1 into Mech" -> "1-1-1 Mech"
// (+ "1-1-1 Tankless Mech"). "2 Port Wraith" -> "2 Starport Wraith" (cluster of
// two Starports, ignoring the opening before them) plus a new "2 Starport
// Valkyrie". "Factory Expand" and "2 Fact before Expa" retired — they are the
// 1- and 2-Factory expands and fold into "N Fact Expa Mech". Also fixes a
// builddedup bug the new naming exposed: a Terran worker re-placing one building
// a tile over (a Factory misclick) had Tier A drop the earlier placement while
// the resource sim dropped the later, zeroing a building that actually stood (a
// Factory that trained Vultures read as 0 Factories). Tier A now drops the later
// same-type duplicate for Terran (matching the resource sim's kept-earliest);
// Zerg is unchanged. Round-9 curation also added: a Goliath composition flavor
// (Goliath-dominant + no tanks by 10:00 → "Goliath" / "N Fact Expa Goliath",
// folding the former standalone Goliath opener); "2 Port Wraith" → "2 Starport
// Wraith" plus "3 Starport Wraith/Valkyrie" (cluster size names it, Wraith floor
// lowered to 3); and Bunker Rush loosened to TWO+ forward Bunkers (≤240s) even
// into an expand/tech follow-up (a single early bunker is a poke). Re-ingest so
// Terran build orders re-classify.
//
// 52: round-10 curation. "Mech (no expa)" family renamed to "1-Base Mech" /
// "1-Base Goliath" / "1-Base Tankless Mech" (parallel to 1-Base Bio). Manner
// pylon no longer fires against a Zerg opponent (creep prevents building inside
// their base — always a false positive). Re-ingest so the renamed BOs and
// corrected manner-pylon events apply.
// 53: N Hatch Hydra counts bases up to the 1st Hydralisk + a 30s grace window
// (an expansion placed as hydra production begins is part of the commit; later
// macro expansions are not) — corrects 2jd (3 Hatch, not 2). Re-ingest so Zerg
// hydra build orders re-count.
const AlgorithmVersion = 53

// DetectorLevel indicates at which level a pattern detector operates
type DetectorLevel string

const (
	LevelReplay DetectorLevel = "replay"
	LevelPlayer DetectorLevel = "player"
)

// PatternResult represents the result of a pattern detection
type PatternResult struct {
	PatternName    string
	Level          DetectorLevel
	ReplayID       int64
	PlayerID       *int64 // nil for replay-level patterns (database ID)
	ReplayPlayerID *byte  // Temporary: replay player ID (byte) for player-level results, converted to PlayerID later

	// DetectedAtSecond is the replay second at which the marker fired.
	// Stored in replay_events.seconds_from_game_start. Source depends on marker family:
	//   Rule markers       → second of the fact that flipped Decision→Matched
	//   First-event markers → second of the first qualifying narrative event
	//   Absence markers     → replay duration (marker commits at end-of-replay)
	//   Viewport/Hotkeys    → documented per-evaluator
	DetectedAtSecond int

	// Payload is the optional JSON blob persisted to replay_events.payload.
	// Empty for presence-only markers. Populated only by markers that carry extra data
	// beyond presence (currently: used_hotkey_groups, viewport_multitasking).
	Payload json.RawMessage
}

// Detector is the interface that all pattern detectors must implement
type Detector interface {
	// Name returns the unique name of this pattern detector
	Name() string

	// Level returns the level at which this detector operates
	Level() DetectorLevel

	// Initialize is called once at the start of replay parsing
	// It receives the replay and all players
	Initialize(replay *models.Replay, players []*models.Player)

	// ProcessCommand is called for each command during replay parsing
	// Returns true if the detector is finished and no longer needs commands
	ProcessCommand(command *models.Command) bool

	// Finalize is called after all commands were processed.
	// Detectors that require full-replay context can complete here.
	Finalize()

	// IsFinished returns true if the detector has finished and won't change
	IsFinished() bool

	// GetResult returns the pattern result if the detector is finished
	// Returns nil if the pattern was not detected or should not be saved
	GetResult() *PatternResult

	// ShouldSave returns true if the result should be saved to the database
	ShouldSave() bool
}

// WorldStateConsumer can receive orchestrator-owned runtime world state context.
type WorldStateConsumer interface {
	SetWorldState(worldState *worldstate.Engine)
}

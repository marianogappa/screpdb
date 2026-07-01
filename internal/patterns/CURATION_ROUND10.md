# Curation round 10 — remove all betas (ledger)

Watching the `zc10_` batch in the StarCraft watch folder (46 replays / 26 beta BOs).
Confirmed = wire as tier-1 fixture (copy into `markers/testdata/replays/`, UPDATE_GOLDEN,
add to `curatedFeatureKeys` + GOLDEN_TIERS). Verdicts as they come in:

## Confirmed (wire as tier-1)
- `zc10_1gatenoexpa_566IIlllllllll_BB36A264` — **1 Gate (no expa)** (PvZ) ✓
- `zc10_1gatenoexpa_broodwarisbest_C15F4714` — **1 Gate (no expa)** (PvT; opp F1SSasad) ✓
- `zc10_3starportvalk_as2QS_3C03155E` — **3 Starport Valkyrie** (TvZ) ✓
- `zc10_7pool_3050sdsd_045D5DC4` — **7 Pool** (ZvT) ✓
- `zc10_7pool_HerWater_219A57AA` — **7 Pool** (ZvP) ✓
- `zc10_8pool_Coffeegene_28C8510C` — **8 Pool** (ZvT) ✓
- `zc10_8pool_loveaddio_12B312B2` — **8 Pool** (ZvT) ✓

## Corrections (detector wrong — fix before wiring)
- `zc10_3starportvalk_4023b_2CD5D7E6` — said **3 Starport Valk**, actually **2 Starport Valk** (already
  curated). 3-Starport cluster over-fired on a 2-Starport build (extra/cancelled Starport counted as 3rd).
- `zc10_8hatch_lIIIlIllllIlIl_AE0CEF98` — said **8 Hatch**, actually **9 Hatch** (supply count off by one).
- `zc10_4hatch_2jd23jdjd234_8C678524` — should be **4 Hatch Hydra** (midgame mostly Hydra + Hydra
  speed AND range; scourge/spire OK as long as no substantial Muta).
- `zc10_4hatch_SYC_CE63DF12` — **4 Hatch Hydra** (made scourge, still hydra-based).
- `zc10_6hatch_PingcoJerry_52377CB6` — said "6 Hatch", should be **3 Hatch Hydra** (see redesign).

## ⚠️ Zerg hatch-BO redesign (from PingcoJerry + the 4-hatch cases)
Hatch count/name are wrong because the count uses a hardcoded time threshold. Correct model (user):
the BO is the **economy→army transition** — mostly Drones (+ a few Lings) on N Hatcheries, then **cut
Drone production and start substantial Hydra/Muta**. That point sets BOTH the count N (Hatcheries at
transition) AND the suffix: **"N Hatch Hydra" / "N Hatch Muta"**. Later Drone rounds / extra Hatcheries
are not BO (PingcoJerry reaches 5-6 total but transitions at 3 → "3 Hatch Hydra"). Scourge/Spire alone
≠ Muta; only substantial Muta production is. Dynamic threshold (like game-phase), NOT a fixed second.
Big change — validate against curated round-8 Zerg fixtures so they hold.

## Zerg redesign — design + open decisions (NOT yet implemented; needs sign-off)
Investigated the current detection (definitions.go ~1003-1165 + dsl.go). Findings:
- Plain **"N Hatch" (9-13) = SUPPLY** (4 start + N drone morphs before the first expansion Hatchery),
  RuleDeadline 180. These are hatch-first ECONOMIC openers (curated, keep).
- **"2/3 Hatch Muta/Lurker/Hydra" = HATCHERY COUNT** via `CountBuildsBefore(Hatchery, 2, 360)` — a
  HARDCODED 360s. This is the axis the user's feedback targets.
- No DSL primitive detects an economy→army transition; a **custom evaluator** is required (like
  `zergOpenerFuzzyEvaluator`).

Target model (user): "N Hatch {Hydra|Muta|Lurker}" where **N = bases (Build(Hatchery) count + 1) at the
economy→army transition** = when Drone morphs cut AND substantial tech-unit production starts. Suffix by
the dominant tech unit (Spire+substantial-Muta → Muta; Den+Hydra → Hydra; Lurker aspect+Lurkers → Lurker).
Scourge/Spire alone ≠ Muta (need real Muta). Later Drones/Hatcheries after the transition don't count
(PingcoJerry hits 5-6 Hatch total but transitions at 3 → "3 Hatch Hydra").

DECISIONS NEEDED before implementing:
1. Replace the hardcoded 360s with a dynamic transition. Define "transition" precisely — proposal: the
   moment of the Kth tech-unit morph (K = 4 Muta / 6 Hydra / 2 Lurker, the existing thresholds); N =
   Build(Hatchery) count seen strictly before that moment, +1. Confirm K and the "+1 base" convention.
2. Extend the family beyond 2/3 to any N (add 3/4 Hatch Hydra, 4 Hatch Muta, etc.) — dynamic count makes
   this automatic (one marker per N, or a single dynamic-name evaluator).
3. Keep the plain supply "N Hatch" (9-13) openers as-is (they're a different, economic axis) and let the
   composition BO win by tier precedence when a transition is detected? (Recommend yes — mirrors how
   2 Hatch Muta already coexists with 12 Hatch today.)
4. Risk: the dynamic count may reclassify the curated 2/3 Hatch Muta/Lurker/Hydra fixtures — must re-validate.

## Supply-count accuracy (separate from the redesign)
- `zc10_8hatch_lIIIlIllllIlIl` (9 Hatch): raw stream has 7 Drone-morph commands before the Hatchery with
  same-second pairs (1s, 23s); dedup collapsed to 4 → exact "8 Hatch". Truth 5 drones → 9 Hatch. Same
  multi-larva morph-count ambiguity round 8 handled with `bo_z_fuzzy`; here it fired an exact wrong count
  instead of going fuzzy. Fix belongs with the round-8 supply-count logic — risky, defer.

## Pending watch (~38 remaining)
Zerg openers (4/6/8 Hatch, 7/8 Pool), Protoss (Forge Cannon no-expa, Forge-Gate-Cannon, Carriers,
Double Stargate, First Corsair, Sair/Speedlot, Speedlot timing), Zerg comp (Muta hit-n-run, Muta
timing), Terran (Nukes, Turret/Wraith-Cloak timing, Wraiths, BCs, 2/4 Fact Expa Mech, Mech no-expa,
Tankless 1 Fact Expa, Goliath 3 Fact Expa). See watch-folder `_CURATION_NOTES.txt` for the full list.

Deterministic-fact betas already exempted on this branch (became_*, *_game_starts, viewport_multitasking,
never_*); catch-all residuals (`bo_*_other`, `opener_unresolved`) intentionally stay beta.

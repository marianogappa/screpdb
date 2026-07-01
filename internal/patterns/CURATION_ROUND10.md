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

## Batch 3 verdicts (2026-07-01)
Confirmed & WIRED (curated): Carriers, Battlecruisers, Forge Cannon (no expa), Forge Gate Cannon,
2 Fact Expa Mech (F1SSasad), Threw Nukes, Sair/Speedlot, 1 Fact Expa Tankless Mech, Wraith Cloak timing.
Rename + wired: "Mech (no expa)" → **"1-Base Mech"** (family: 1-Base Goliath / 1-Base Tankless Mech);
NaMu/WJDDSU confirmed 1-Base Mech.

Corrections still to FIX (detector wrong):
- **Factory-before-expa OVER-count**: `mech2fac_SSTJumJaJungJi` (said 2, is 1) and `mech4fac_Terran3`
  (said 4, is 1) — user: "only one Factory before the expa." Raw shows extra Factory build commands
  before the first CC (Terran3: 151,362,463,464 before CC@481) that were cancelled/spread re-placements.
  The round-9 builddedup fix drops LATER same-type dups but only for near-simultaneous re-placements;
  these are spread over minutes → not caught. Needs a standing-factory (produced/completed) count for
  the before-expa tally. RISKY (opposite direction to the round-9 under-count fix) — investigate carefully.
- **Double Stargate** over-fires: it's a PvZ early multi-corsair technique (observe / supply-block via
  killing Overlords / cloak-DT support), NOT a carrier build. Corpus survey of the 2nd-Starport second:
  bulk 4-7 min (median 5:51, p10 4:48), long tail 8min→35min (~14 matches = carrier transitions / late
  = false positives). Fix = (a) 2nd Starport within ~7:00 window, (b) count STANDING Starports so a
  re-placed single Starport (89EFD77C: 2 builds 3s apart, only 1 stood) doesn't fire, (c) corsair-, not
  carrier-, focused. `82A3FA04` (2nd SP @8:23 → carriers) and `89EFD77C` (1 real SP) both correctly drop.

## Timing-marker detected values (for the user to confirm against the replay)
| Marker | Replay | Player | Detected |
| --- | --- | --- | --- |
| First Corsair | 55DD7250 | Tomson`net | 4:58 |
| Speedlot timing | 55DD7250 / 632A5226 | Tomson`net / o-jing | 6:12 / 4:37 |
| Mutalisk timing | AF8C1D90 / F26080FE | IlIlIllIlllIlll | 5:40 (6 muta burst) / 5:35 (3 muta) |
| Turret timing | F26080FE / AF8C1D90 | 1235sdfdfhg / gimoddak7 | 6:03 (Ebay 4:59) / 5:49 (Ebay 4:47) |

## Manner pylon — suspected over-fire (user)
169 events across 71 replays (~2.4/replay) — high for a griefing pylon. Staged candidates for the user
to verify; if it's mis-firing on ordinary forward pylons, tighten + possibly un-curate.

## Batch 4 verdicts (manner pylon + first corsair)
Manner pylon (was over-firing, 169/71): FIXED the Zerg class — `manner_pylon` no longer fires vs a Zerg
opponent (creep blocks it), v52. Verdicts:
- `llllIIIlllIl` (PvZ) — impossible vs Zerg → now 0 ✓ (fix).
- `SKT1JSPARK` — proxy Gateway at the player's OWN natural, mis-attributed to enemy start. STILL fires
  (own-base false positive). DEFERRED: needs a "pylon inside the enemy polygon (not nearest-base
  fallback)" fix — a raw own-vs-enemy start-distance guard regressed the genuine case, so reverted.
- `132SDFSDFSD` (PvT) — genuine manner pylon, ok (still fires ✓).
- `I11II11II` — weird replay, drop the candidate.

**first_corsair BUG**: reports the Stargate-finish second (4:58), not the first Corsair train (5:08).
Likely the other timing pills (speedlot/muta/turret) mis-report tech-building-finish vs the unit too —
AUDIT the timing-marker definitions (don't ask the user to eyeball seconds). Fix to report the unit.

## Open backlog (implement + validate; no user decisions blocking — proceed with defaults)
1. first_corsair (+ audit speedlot/muta/turret timing) — report the unit, not the tech building.
2. Manner pylon own-base/proxy-gate false positive (polygon-inside gate).
3. Factory-before-expa over-count (mech2fac_SST / mech4fac_Terran3 → 1 Fact) — standing-factory count.
4. Double Stargate — 2nd-Starport ≤~7:00 window + standing-Starport + corsair-not-carrier.
5. Zerg "N Hatch Hydra/Muta" dynamic transition (defaults: transition = Kth tech-unit morph [4 Muta /
   6 Hydra / 2 Lurker]; N = Build(Hatchery) before it +1; keep supply openers; validate round-8 fixtures).

## Pending watch (remaining) — only these two markers still need the user
Composition/behavior (watchable): `wraiths` ×2, `mutaharass` ×2. Timing pills are being audited by code,
not eyeballed.
Zerg openers (4/6/8 Hatch, 7/8 Pool), Protoss (Forge Cannon no-expa, Forge-Gate-Cannon, Carriers,
Double Stargate, First Corsair, Sair/Speedlot, Speedlot timing), Zerg comp (Muta hit-n-run, Muta
timing), Terran (Nukes, Turret/Wraith-Cloak timing, Wraiths, BCs, 2/4 Fact Expa Mech, Mech no-expa,
Tankless 1 Fact Expa, Goliath 3 Fact Expa). See watch-folder `_CURATION_NOTES.txt` for the full list.

Deterministic-fact betas already exempted on this branch (became_*, *_game_starts, viewport_multitasking,
never_*); catch-all residuals (`bo_*_other`, `opener_unresolved`) intentionally stay beta.

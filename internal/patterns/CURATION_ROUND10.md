# Curation round 10 — remove all betas (ledger)

Watching the `zc10_` batch in the StarCraft watch folder (46 replays / 26 beta BOs).
Confirmed = wire as tier-1 fixture (copy into `markers/testdata/replays/`, UPDATE_GOLDEN,
add to `curatedFeatureKeys` + GOLDEN_TIERS). Verdicts as they come in:

## Confirmed (wire as tier-1)
- `zc10_1gatenoexpa_566IIlllllllll_BB36A264` — **1 Gate (no expa)** (PvZ, 566IIllllllllll) ✓
- `zc10_1gatenoexpa_broodwarisbest_C15F4714` — **1 Gate (no expa)** (PvT, broodwarisbest; opp F1SSasad) ✓
- `zc10_3starportvalk_as2QS_3C03155E` — **3 Starport Valkyrie** (TvZ, as2QS) ✓

## Corrections (detector wrong — fix before wiring)
- `zc10_3starportvalk_4023b_2CD5D7E6` — detector said **3 Starport Valkyrie**, actually **2 Starport
  Valkyrie**. The 3-Starport cluster over-fired on a 2-Starport build (likely an extra / cancelled
  Starport placement counted as the 3rd — investigate, same family as the builddedup re-placement issue).

## Pending watch (~42 remaining)
Zerg openers (4/6/8 Hatch, 7/8 Pool), Protoss (Forge Cannon no-expa, Forge-Gate-Cannon, Carriers,
Double Stargate, First Corsair, Sair/Speedlot, Speedlot timing), Zerg comp (Muta hit-n-run, Muta
timing), Terran (Nukes, Turret/Wraith-Cloak timing, Wraiths, BCs, 2/4 Fact Expa Mech, Mech no-expa,
Tankless 1 Fact Expa, Goliath 3 Fact Expa). See watch-folder `_CURATION_NOTES.txt` for the full list.

Deterministic-fact betas already exempted on this branch (became_*, *_game_starts, viewport_multitasking,
never_*); catch-all residuals (`bo_*_other`, `opener_unresolved`) intentionally stay beta.

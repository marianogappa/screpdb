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

## Pending watch (~38 remaining)
Zerg openers (4/6/8 Hatch, 7/8 Pool), Protoss (Forge Cannon no-expa, Forge-Gate-Cannon, Carriers,
Double Stargate, First Corsair, Sair/Speedlot, Speedlot timing), Zerg comp (Muta hit-n-run, Muta
timing), Terran (Nukes, Turret/Wraith-Cloak timing, Wraiths, BCs, 2/4 Fact Expa Mech, Mech no-expa,
Tankless 1 Fact Expa, Goliath 3 Fact Expa). See watch-folder `_CURATION_NOTES.txt` for the full list.

Deterministic-fact betas already exempted on this branch (became_*, *_game_starts, viewport_multitasking,
never_*); catch-all residuals (`bo_*_other`, `opener_unresolved`) intentionally stay beta.

# Zerg pool/hatch curation — round 8 (issues #222 / #223 / #224)

Human-verified ground truth from watching the candidate replays. This is the
authoritative record of the openers; the detector does **not** yet output all of
these (see "Detector bug" below), so promotion to tier-1 fixtures in
`GOLDEN_TIERS.md` + `markers_golden.json` is blocked on the supply-count fix.

## Verified verdicts (player → true opener)

Detector column = what the classifier currently outputs. ✗ = disagreement.

| Replay (prefix · player) | True opener | Detector | |
| --- | --- | --- | --- |
| MM-D2D45CFA gaemalline | 9 Pool | 9 Pool | ✓ |
| MM-026A8DC0 scgotoboy | 9 Pool | 9 Pool | ✓ |
| MM-3C1578EA INFIPRO | 9 Pool | 9 Pool | ✓ |
| MM-0120EE30 mentalgap | 9 Overpool | 9 Overpool | ✓ |
| MM-4F6FE1E6 Ljkrjtkejt | 9 Overpool | 9 Overpool | ✓ |
| MM-3FB30740 fdhfthfghgdhn | 9 Overpool | 9 Overpool | ✓ |
| MM-DEBFBBFE BBBuuuUU[kS] | 9 Overpool | 10 Pool | ✗ multi-larva over-count |
| MM-7F585434 mentalgap | 9 Overpool | 10 Pool | ✗ build-fact ordering |
| MM-0BAED626 UtataneLeina | 9 Overpool | 10 Pool | ✗ build-fact ordering |
| MM-E0638086 hommage88 | 12 Pool | 12 Pool | ✓ |
| MM-0F17A2FC sd1sd234gag | 12 Pool | 12 Pool | ✓ |
| MM-9B91E242 lillljilililili | 12 Pool | 12 Pool | ✓ |
| MM-422852F6 IIIlIlllllllIlI | 4 Pool | 4 Pool | ✓ |
| MM-E6609AB8 BBBuuuUU[kS] | 4 Pool | 4 Pool | ✓ |
| MM-9AD07620 Eulsann | 5 Pool | 5 Pool | ✓ |
| MM-D7866556 hohojojo3 | 9 Overpool | 9 Overpool | ✓ (issue mislabeled as 6 Pool) |
| MM-D35B0248 lototete | 11 Pool | 11 Pool | ✓ (issue mislabeled as 6 Pool) |
| MM-ED69B8D6 Skins_ | 9 Pool | 9 Pool | ✓ (issue mislabeled as 6 Pool) |
| MM-63A24E0C BBBuuuUU[kS] | 9 Pool | 9 Pool | ✓ (issue mislabeled as 7 Pool) |
| MM-DC96F3CC 3050_KzerG | 9 Pool | 9 Pool | ✓ (issue mislabeled as 7 Pool) |
| MM-69D0916C mentalgap | 9 Pool | 9 Pool | ✓ (issue mislabeled as 7 Pool) |
| MM-4D3FF3A0 BBBuuuUU[kS] | 9 Pool | 9 Pool | ✓ (issue mislabeled as 8 Pool) |
| MM-2FDAA8D0 BisuSnow | 9 Pool | 9 Pool | ✓ (issue mislabeled as 8 Pool) |
| MM-E6AA2324 IlIlIllIlllIlll | 9 Overpool | 9 Overpool | ✓ (issue mislabeled as 8 Pool) |
| MM-132913C2 lIlIlllIIlIlll | 9 Hatch | 4 Hatch | ✗ drone under-count |
| MM-83179234 3050_KzerG | 9 Hatch | 9 Hatch | ✓ |
| MM-3D2A8A3C 165123141231241 | 9 Hatch | 9 Hatch | ✓ |
| MM-93F6E4B8 Foreigner70 | 13 Hatch (new rung) | Pool/Hatch (Other) | ✗ rung missing |
| MM-5639B7E6 LYX2008 | 12 Hatch | Pool/Hatch (Other) | ✗ |
| MM-EDBD3CB6 lillljilililili | 12 or 13 Hatch (commands too close to tell visually; decide by first-command order) | Pool/Hatch (Other) | ✗ |

### 3 Hatch Muta → make it a composition marker (not an opener)

The opener underneath is a hatch-first build; "3 Hatch Muta" should be a marker
layered on top (like Crazy Zerg), set when the player reaches 3 hatcheries into
Mutalisks.

| Replay · player | True opener | + marker |
| --- | --- | --- |
| MM-FE0149BC chillibeans | 12 Hatch | 3 Hatch Muta |
| MM-CC4246B6 eezxcq1 | 12 Hatch | 3 Hatch Muta |
| MM-6B443B78 llIIll1ll1lI | 11 Hatch | 3 Hatch Muta |

## Requested structural changes

- **Create a 13 Hatch rung** (`zergHatchBO(13, …)`).
- **Convert 3 Hatch Muta** from `KindInitialBuildOrder` to a `KindMarker`
  composition marker; the opener becomes the underlying 11/12 Hatch.
- **Eliminate the `Pool/Hatch (Other)` residual** — with 13 Hatch added, its
  members classify as real rungs.

## Detector bug (blocks promoting the ✗ rows as fixtures)

The drones-before-pool / drones-before-hatch count the rung predicates see is
off (usually +1), so 9 Overpool reads as 10 Pool and 9 Hatch reads as 4 Hatch.
Two compounding causes:

1. **Multi-larva over-count** — `internal/earlyfilter/sim.go` `producedCount`
   credits the full larva-selection size, so a 2-larva drone morph adds +2
   supply even when only one extra drone lands (and after the pool). Seen on
   BBBuuuUU (`MM-DEBFBBFE`): morph @0:44 has count 2 → 6 drones before pool.
2. **Build-fact observation ordering** — `internal/patterns/detectors/player_marker.go`
   `enqueueDedup` holds `KindMakeBuilding` facts in a dedup tail while
   `KindMakeUnit` (drone) facts are observed immediately, so a drone morphed
   seconds after the real Pool/Hatch can be counted as before it.
   Seen on mentalgap/UtataneLeina: 5 drones + overlord before pool in the
   filtered stream (confirmed via the early-filter trace) yet classified 10 Pool.

Fix touches the shared dedup/counting path used by **every** build order →
requires live-predicate instrumentation and a full re-validation of the existing
curated Zerg fixtures (11/12 Hatch, 11 Pool, 2H Muta, 3H Lurker, 9 Pool) before
the ✗ rows above can be promoted to tier-1.

## Progress

**Landed (committed on `fix/zerg-pool-hatch-curation`):**
- Supply-count ordering fix (`ProduceCountBeforeBuild` counts by game-second) —
  fixes 9 Overpool (mentalgap, UtataneLeina). 2 tier-2 fixture shifts only.
- New **13 Hatch** rung — Foreigner70 ✓.
- Promoted 10 tier-1 fixtures (`bo_z_*pool_*`, `bo_z_*hatch_*`) + marked their
  rungs curated (beta tag off): 9 Pool, 9 Overpool, 12 Pool, 4 Pool, 5 Pool,
  11 Pool, 9 Hatch, 12 Hatch, 13 Hatch.
- **3 Hatch Muta** converted from opener to TvZ composition marker
  (`three_hatch_muta`); the hatch opener surfaces underneath (chillibeans/eezxcq1
  → 12 Hatch, llIIll → 11 Hatch). Promoted 2 fixtures; marked curated.

**Not done — eliminate the `Pool/Hatch (Other)` residual:** deferred. With the
edge-case games below still misclassifying, removing the residual would push them
to `Opener unresolved`. Revisit after the edge cases are resolved.

**Still ✗ — human re-confirmed truth (screenshots), root-caused, not yet fixed:**
- `BBBuuuUU[kS]` (MM-DEBFBBFE) → 10 Pool; truth **9 Overpool**.
- `LYX2008` (MM-5639B7E6) → 13 Hatch; truth **12 Hatch**.
  Both are the SAME multi-larva over-count: `internal/unittags/morphcount.go`
  `MorphSelectionSizes` records `len(s.cur)` (the whole current selection) as the
  morph count, not the number of *larvae* in the selection. When a larva is
  selected alongside any non-larva unit (a Drone, the Hatchery) and morphed, only
  the larva becomes a Drone but it's counted as 2 → +1 supply before the
  Pool/Hatch. Secondary: `earlyfilter/sim.go` `consumeLarva()` drains only 1 larva
  per morph regardless of how many were produced.
  FIX (focused, regression-risky): count only larvae in the morph selection —
  needs per-tag unit-type tracking; must NOT re-introduce the multi-larva
  UNDER-count this code originally fixed (11 Hatch→10). Re-validate all curated
  Zerg fixtures.
- `lIlIlllIIlIlll` (MM-132913C2) → 4 Hatch; truth **9 Hatch**. DIFFERENT bug: the
  player's early Drone morphs are absent from the filtered stream (0 drones before
  the hatch). Needs early-filter drop-reason investigation (the SCREPDB_EARLY_
  FILTER_DEBUG_DIR trace).

**Structural changes still TODO:** add 13 Hatch rung; convert 3 Hatch Muta to a
marker; drop the Pool/Hatch (Other) residual; promote confirmed games to tier-1
fixtures + `curatedFeatureKeys` + GOLDEN_TIERS rows.

# Golden test tiers

The integration goldens (`markers/testdata/markers_golden.json`,
`worldstate/testdata/drops_golden.json`) mix two kinds of assertion. Treat them
differently when a change moves a golden value.

## Tier 2 — inferred / auto-generated (changes tolerated)

Most golden values are produced by `UPDATE_GOLDEN=1` and were never eyeballed by
a human against the actual replay. They exist to catch *unintended* drift, not to
encode a verified truth. When a deliberate change moves them and the new values
are explainable, refreshing with `UPDATE_GOLDEN=1` is fine — no human sign-off
needed.

Examples of tier-2 assertions: Viewport Multitasking `switches_per_minute`,
hotkey/upgrade markers, expert-milestone `expert_actuals`, regular `drop` /
`reaver_drop` records, and every assertion on the pre-existing marker fixtures
(`battlecruisers.rep`, `bo_*_hatch.rep`, `bo_2_gate_carriers.rep`,
`carriers_recalls.rep`, `threw_nukes.rep`, …).

## Tier 1 — human-curated premises (changes are regressions)

A small set of fixtures encode a *specific premise a human verified by watching
the replay*. If a change breaks one of these premises, that is a **regression**:
do not blindly `UPDATE_GOLDEN`. Either fix the code, or get human re-verification
before accepting the new value.

Important: tiering is **per-premise, not per-file**. A tier-1 fixture's golden
JSON still contains tier-2 assertions (e.g. its players' Viewport Multitasking
numbers) that may change freely. Only the listed premise is protected.

### Build-order classification — `markers_golden.json`

Fixture `bo_team_mech_111.rep` (from `AutoSave/20260614/174024,(8)Big Game
Hunters.rep`). The author watched the match; these per-player build orders are
the verified premise:

| Player (idx) | Must classify as | Why (author) |
| --- | --- | --- |
| chobo86 (P0) | `Build Order: 5-Fac Mech` | "clear mech build with 5 factories" |
| ALT+F4 (P4) | `Build Order: 1-1-1 into Mech` | "alt+f4 did a 1-1-1" |
| UranAsol (P6) | `Build Order: 1-Rax Bio` | 1-rax marine opening, left early under attack |
| Mr.Cordelius (P5) | `Opener unresolved` | "fair since they didn't play" |

(The other players' BOs in this fixture are tier-2.)

### Cliff-drop detection — `drops_golden.json`

Each fixture below was confirmed by watching the replay. The premise is the
presence/absence of a `cliff_drop` subtype record.

| Fixture (source replay) | Premise | Verified |
| --- | --- | --- |
| `drops_cliff_bgh_truepos.rep` (`AutoSave/20260301/215111`) | chobo86 cliff drop **present** (~6:30, bottom-right) | "a classic example of it, correct" |
| `drops_cliff_bgh_centroid_tp.rep` (`oldAutosave/20171118/211035`) | crazybigcup cliff drop **present** (~7:26, top-left) | "This one is a true positive!" — guards centroid-pollution recovery |
| `drops_cliff_bgh_bunker_fp.rep` (`AutoSave/20251207/203130`) | **zero** cliff drops (16:53 is a Bunker unload, no Starport) | "they didn't even have a starport so that's impossible … quite wrong" |
| `drops_cliff_bgh_offcliff_fp.rep` (`AutoSave/20260214/160159`) | **zero** cliff drops (drop lands close to but not on the cliff) | "not cliff drops but the drops happen very close to the cliff" |

A change that adds a cliff_drop to a *_fp fixture, or removes the one from a
*_tp fixture, breaks a human premise → regression.

### Recall target inference — `recalls_golden.json`

All six `recalls_*.rep` fixtures (introduced with the recall-destination feature,
#118) are hand-curated: the author hand-annotated each recall's **target** base.
The golden test header says so directly — "the source of truth for the user's
hand-annotated targets," with labels in the author's annotation style ("9",
"9's natural", "center base"). Scenario names encode the cases probed:
`arbiter_died`, `multi_target`, `no_teleport_case`, `single_natural`,
`sustained_9_then_11`, `with_defensive`.

The protected premise per recall cluster is the **target attribution**
(`target_label` / `target_owner_pid`) — i.e. where the recall was inferred to go.
The mechanical fields (`second`, `count`, `source_label`) are derived. A change
that moves a recall's target away from the annotated base is a regression → it
needs the author to re-annotate, not a blind `UPDATE_GOLDEN`.

### Rush detection — `rushes_golden.json`

Six `rush_*.rep` fixtures (issue #189), each a real rush confirmed by watching
the replay: three `zergling_rush` and three `cannon_rush`. The protected premise
is the **presence** of the rush (subtype + rusher) — dropping a detection or
adding a spurious one is a regression. Two of the zergling fixtures are ZvZ and
correctly capture *both* players' mutual ling rush (the author verified the
named rusher; the opponent's ling rush in ZvZ is the same well-understood case).

| Fixture | Premise |
| --- | --- |
| `rush_zergling_thelasthydra.rep` | ZvZ — both players zergling_rush (~1:56 / ~2:06) |
| `rush_zergling_llil.rep` | ZvZ — both players zergling_rush (~1:54 / ~2:05) |
| `rush_zergling_asdas.rep` | zergling_rush (~1:53) |
| `rush_cannon_afdjkdsfaf.rep` | cannon_rush (~3:14) |
| `rush_cannon_undertaker.rep` | cannon_rush (~2:46) |
| `rush_cannon_lyx2008.rep` | cannon_rush (~2:51) |

## Additional human-verified ground truth (not yet fixtured)

From the same review, verified but not (yet) encoded as fixtures — candidates if
more tier-1 coverage is wanted, and useful context when judging changes:

- True cliff drops that are currently detected: Blast. (`AutoSave/20260323/000311`,
  ~5:58), Pro-THC (`AutoSave/20251005/231527`, ~6:56), gdtyjk
  (`AutoSave/20250330/183144`, ~5:43), DeCartonPiedra (`...20171230/171658`,
  cliff_drop @361), BULLSHlT (`...20180204/195036`, cliff_drop @491).
- Genuinely-missed cliff drops (purely coordless plain-`Unload`), tracked in
  `worldstate/cliff_drop_todo.md`: bombom (`...20170527/150854`, ~19:45),
  JustPassingThru (`AutoSave/20251116/225058`, ~19:08).

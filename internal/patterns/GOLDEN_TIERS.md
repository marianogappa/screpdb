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
hotkey/upgrade markers, expert-milestone `expert_actuals`, and every assertion
on the pre-existing marker fixtures (`battlecruisers.rep`, `bo_*_hatch.rep`,
`bo_2_gate_carriers.rep`, `carriers_recalls.rep`, `threw_nukes.rep`, …).

**Dashboard "beta" tag.** The set of tier-1 feature keys is mirrored, machine-readably,
in `internal/patterns/markers/curation.go` (`curatedFeatureKeys`). The dashboard flags any
marker / build-order NOT in that set with a "beta" superscript. When you promote a detection
to tier-1 below, add its FeatureKey to `curatedFeatureKeys` so the beta tag disappears.

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
| UranAsol (P6) | `Build Order: 1-Base Bio` | one-base marine opening, left early under attack |
| Mr.Cordelius (P5) | `Opener unresolved` | "fair since they didn't play" |

(The other players' BOs in this fixture are tier-2.)

Fixture `bo_bunker_simcity_bgh_fp.rep` (from `AutoSave/20260607/225451,(8)Big
Game Hunters.rep`, issue #164). The verified premise is a single negative
assertion:

| Player (idx) | Must NOT classify as | Why |
| --- | --- | --- |
| P3 | `Build Order: Bunker Rush` | Defensive sim-city Bunker walled at the player's own base, not a rush. On a Money map the no-expansion topology is meaningless (nobody takes a second CC), so topology alone misread it; the offensive `bunker_rush` spatial gate now keeps it out. Classifies as `Build Order: 1-Base Bio`. |

A change that classifies P3 as Bunker Rush breaks this premise → regression.
(The other players' BOs in this fixture are tier-2.)

Zerg opener batch (`bo_11hatch_*`, `bo_12hatch_*`, `bo_11pool_*`, `bo_2hmuta_*`,
`bo_3hlurker_*`). Fifteen non-mirror 1v1 ladder replays, each watched and
confirmed 100% correct after the multi-larva-morph supply fix (a single
larva-morph command morphs every selected larva, so counting commands had been
undercounting supply — see the `fix(markers): count multi-larva Zerg morphs`
commit). The verified premise is the Zerg player's build order:

| Fixtures | Zerg player must classify as |
| --- | --- |
| `bo_11hatch_tvz_lyx2008`, `bo_11hatch_tvz_mentalgap`, `bo_11hatch_pvz_bbbuuuuuks` | `Build Order: 11 Hatch` |
| `bo_12hatch_tvz_junja`, `bo_12hatch_tvz_attheendpl`, `bo_12hatch_tvz_mbushine` | `Build Order: 12 Hatch` |
| `bo_11pool_tvz_ililillill`, `bo_11pool_pvz_ililillill`, `bo_11pool_pvz_mbushine` | `Build Order: 11 Pool` |
| `bo_2hmuta_tvz_mbushine`, `bo_2hmuta_tvz_mentalgap`, `bo_2hmuta_tvz_skins` | `Build Order: 2 Hatch Muta` |
| `bo_3hlurker_tvz_honjr`, `bo_3hlurker_tvz_lyx2008`, `bo_3hlurker_tvz_puuuuuma` | `Build Order: 3 Hatch Lurker` |

The 11 Hatch / 12 Hatch / 11 Pool fixtures specifically guard the supply-rung
boundary the fix corrected — a regression there (e.g. an 11 Hatch sliding back to
10) means multi-larva morphs are being undercounted again. The opponent's
(Terran / Protoss) BO in each fixture is tier-2.

Protoss/Terran opener batch (round 3). Thirteen non-mirror 1v1 ladder replays,
each watched and confirmed correct. The verified premise is the named player's
build order (and, where noted, a modifier):

| Fixtures | Player must classify as |
| --- | --- |
| `bo_factory_expand_sst`, `bo_factory_expand_dsfsd`, `bo_factory_expand_ncs` | `Build Order: Factory Expand` (1 Rax → Factory + vultures → natural CC; no siege research — this is why "Siege Expand" was renamed) |
| `bo_ccfirst_111113`, `bo_ccfirst_ilill`, `bo_ccfirst_illill` | `Build Order: CC First` (canonical: Depot then CC, no Barracks or 2nd Depot before the CC) |
| `bo_1base_bio_sst` | `Build Order: 1-Base Bio` (bio all-in, no natural CC in the opening) |
| `bo_2base_bio_cabeiri` | `Build Order: 2-Base Bio` (bio that takes a natural CC in the opening) |
| `bo_2hatch_hydra_mbushine`, `bo_2hatch_hydra_lilli`, `bo_2hatch_hydra_mentalgap` | `Build Order: 2 Hatch Hydra` |
| `bo_1gate_reaver_flashrilla` | `Build Order: 1 Gate Reaver`, **no** `expand` modifier (Nexus only after the Reaver) |
| `bo_1gate_reaver_minimaxii` | `Build Order: 1 Gate Reaver` **with** `expand` modifier (Nexus before the first Reaver) |

The 1-Base/2-Base pair guards the bio base-count split; the two 1 Gate Reaver
fixtures guard the `expand` modifier (present vs absent). The opponent's BO in
each fixture is tier-2.

Protoss opener batch (round 4). Thirteen non-mirror 1v1 ladder replays, each
watched and confirmed. The verified premise is the Protoss player's opener (and,
where noted, a modifier):

| Fixtures | Protoss player must classify as |
| --- | --- |
| `bo_1gatecore_pvt_pporoktoss`, `bo_1gatecore_pvt_231314`, `bo_1gatecore_pvt_dp6`, `bo_1gatecore_pvt_paralyze` | `Build Order: 1 Gate Core` |
| `bo_2gate_pvt_duongdallas` | `Build Order: 2 Gate` |
| `bo_2gate_pvt_proxy_iiii` | `Build Order: 2 Gate` **with** `proxy` modifier (gateways near the enemy) |
| `bo_nexusfirst_pvt_bbbukae`, `bo_nexusfirst_pvt_bysnow`, `bo_nexusfirst_pvt_kong` | `Build Order: Nexus First` |
| `bo_gateexpand_pvt_horang2` | `Build Order: Gate Expand` |
| `bo_forgeexpand_pvz_jgquickly`, `bo_forgeexpand_pvz_femaleval`, `bo_forgeexpand_pvz_llilil` | `Build Order: Forge Expand` |

`bo_1gatecore_pvt_paralyze` specifically guards the early-filter tech-prerequisite
backstop: it is a 1 Gate Core → Dragoon opener whose Cybernetics Core the mineral
sim had wrongly dropped (starved by a phantom spend), making it misread as Gate
Expand; a kept Robotics Facility now re-admits the Core (see
`internal/cmdenrich/techtree.go`). A regression to Gate Expand there means the
backstop broke. The opponent's BO in each fixture is tier-2.

Protoss opener batch (round 5, issue #225). Tech/contain openers, each watched
and confirmed. Round 5 also retired the `2 Gate DT`, `2 Gate Reaver` and
`Sair/Speedlot` openers (they described post-opening tech composition, not the
opening) — those replays fall to their true opener below.

| Fixtures | Protoss player must classify as |
| --- | --- |
| `bo_1gatecore_pvt_23asd`, `bo_1gatecore_pvt_dicltoss`, `bo_1gatecore_pvt_dotk` | `Build Order: 1 Gate Core` (former false "2 Gate DT" — no real DT/Templar opening) |
| `bo_gate_forge_cannon_pvz_horang2`, `bo_gate_forge_cannon_pvz_lyx2008` | `Build Order: Gate Forge Cannon before expa` |
| `bo_forge_cannon_gate_pvz_231314` | `Build Order: Forge Cannon Gate before expa` |
| `bo_gateexpand_pvz_asdzzz` | `Build Order: Gate Expand` (1 Gate FE; former "Sair/Speedlot") |
| `bo_forgeexpand_pvz_dkdlt` | `Build Order: Forge Expand` (FFE; former "Sair/Speedlot", only 1 Corsair so not a Sair build) |

The cannon-contain fixtures guard the {Gate, Forge, Cannon}-before-core-and-expa
build-order permutations. The opponent's BO in each fixture is tier-2.

Manner pylon (round 5, issue #225). Fixture `manner_pylon_pvp_llilil` (PvP):
llIIlIIIIIIllll places a Pylon inside LYX2008's main mineral line at ~2:17 to
block worker mining. The verified premise is the `Manner pylon` marker (and its
worldstate `manner_pylon` game_event) on llIIlIIIIIIllll. The same fixture also
guards the `First Reaver` timing marker (llIIlIIIIIIllll, ~5:25) and the
`1 Gate Core` opener. A change that drops the manner pylon or reclassifies the
opener is a regression.

Terran air/specialist opener batch (round 6, issue #228). Thirteen non-mirror
or named-player 1v1 ladder replays, each watched and confirmed. Round 6 also
redefined two openers and retired one: the TvZ "Wraith" composition opener was
folded into a matchup-shared `2 Port Wraith` (1 Rax / 1 Fac into two Starports,
wraith-dominant — the same build in TvT and TvZ), and "2 Fact Vults" became
`2 Fact before Expa` (exactly two Factories before the expansion; mech is implied
in TvT, so it's a vulture/tank/goliath mix, not pure vultures). The verified
premise is the named player's opener (and, where noted, a modifier):

| Fixtures | Player must classify as |
| --- | --- |
| `bo_2port_wraith_tvz_qwejlkqwen`, `bo_2port_wraith_tvz_hanatan`, `bo_2port_wraith_tvz_lllii` | `Build Order: 2 Port Wraith` (TvZ — the former "Wraith") |
| `bo_2port_wraith_tvt_kuri`, `bo_2port_wraith_tvt_boogeeyoon` | `Build Order: 2 Port Wraith` (TvT) |
| `bo_2fact_expa_tvt_c9flash` (opponent `IlllllllIlIIIll`, P1) | `Build Order: 2 Port Wraith` **with** `proxy` modifier |
| `bo_goliath_tvz_iilliii1`, `bo_goliath_tvz_f1ssasad`, `bo_goliath_tvz_lilliill` | `Build Order: Goliath` |
| `bo_2fact_expa_tvt_c9flash` (P0), `bo_2fact_expa_tvt_ybscan`, `bo_2fact_expa_tvt_tipkofe` | `Build Order: 2 Fact before Expa` |
| `bo_bbs_tvp_chocobo12`, `bo_bbs_tvp_standordie`, `bo_bbs_tvp_sstjumja` | `Build Order: BBS` **with** `proxy` modifier (Barracks planted forward at the enemy) |

`bo_2port_wraith_tvz_qwejlkqwen` additionally guards the `expand` modifier (a
Command Center taken before the two Starports). `bo_2fact_expa_tvt_c9flash` is a
single TvT game carrying two protected premises: P0 `C9_FlaSh` = 2 Fact before
Expa, and P1 `IlllllllIlIIIll` = 2 Port Wraith **with `proxy`** — its two
Starports are planted forward at the enemy, which fires the `proxy_starport`
game-event (and the `proxy` BO modifier). That fixture is the guard for proxy
Starport detection. The three BBS fixtures guard the `proxy` modifier on every
BBS: `bo_bbs_tvp_standordie` specifically guards the proxy spatial gate's
at-the-enemy case — its Barracks sit across the map on the opponent's half, which
the old midfield-only band missed (it now fires because the gate is player-aware:
far from the builder's own main, within reach of the enemy's). The opponent's BO
in each fixture is tier-2.

TvZ Zerg composition (round 7). Fixture `crazy_zerg_guardians_tvz_lyx2008.rep`
(TvZ; LYX2008 = Zerg, P1), watched and confirmed. Two protected premises on
LYX2008:

| Premise | Detail |
| --- | --- |
| `Crazy Zerg` | Mutalisk → Ultralisk with Zerg Carapace and no Lurker before the first Ultralisk (first Ultralisk ~13:54) |
| `Guardians` | at least one Guardian morphed (~9:28) — guards the `subjectsOfInterest` fix that lets the Guardian fact reach the rule predicate |

A regression that drops either marker for LYX2008 is a regression. The opponent's
BO is tier-2.

Timing / cast pills (round 7), each watched and confirmed. These pills report the
first *production/cast command* (not the unit's completion / the cast's effect) —
the same convention as First Reaver / First Corsair — so an early re-click or an
energy-less cast can set the time slightly before the visible action.

| Fixture | Protected premise |
| --- | --- |
| `maelstrom_pvz_bysnow.rep` | `Made Maelstrom` on By.Snow1\` (P0, Protoss) — Dark Archon Maelstrom cast |
| `first_observer_pvt_0sawon.rep` | `First Observer` on 0sawon (P0, Protoss) |
| `first_mine_pvt_f1ssasad.rep` | `First Mine` on F1SSasad11s32dd (P1, Terran) — first Vulture spider mine |

The opponent's BO in each is tier-2.

Zerg pool/hatch openers (round 8, issues #222/#223/#224). Non-mirror ladder
replays, each watched and confirmed; the verified premise is the Zerg player's
opener. Several were promoted alongside the supply-count fix (drones counted by
game-second relative to the building, not observation order) and the new 13 Hatch
rung.

| Fixture | Zerg player must classify as |
| --- | --- |
| `bo_z_9pool_gaemalline` | `9 Pool` |
| `bo_z_9overpool_mentalgap`, `bo_z_9overpool_utataneleina` | `9 Overpool` (5 Drones + Overlord before Pool; utataneleina guards the dedup-ordering fix — a 6th Drone 2s after the Pool used to inflate it to 10 Pool) |
| `bo_z_12pool_hommage88` | `12 Pool` |
| `bo_z_4pool_iiilil` | `4 Pool` |
| `bo_z_5pool_eulsann` | `5 Pool` |
| `bo_z_11pool_lototete` | `11 Pool` |
| `bo_z_9hatch_3050kzerg` | `9 Hatch` |
| `bo_z_12hatch_lllji` | `12 Hatch` |
| `bo_z_13hatch_foreigner70` | `13 Hatch` (new rung — was the Pool/Hatch (Other) residual) |

The opponent's BO in each fixture is tier-2. Edge cases still pending human
re-review (multi-larva over-count, a missing-drone replay) are logged in
`CURATION_ZERG_ROUND8.md`, not yet promoted.

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

### Drops — `drops_golden.json`

Six `drops_{reg,notreaver,notdt}_*.rep` fixtures (issue #185), each confirmed by
watching the replay. The premise is a real `drop` at the verified location. The
`notreaver` and `notdt` fixtures are regression guards: their replays contain a
reaver / Dark Templar near the drop, but neither subtype is inferred anymore, so
they must classify as plain `drop` — never `reaver_drop` or `dt_drop`.

| Fixture | Premise |
| --- | --- |
| `drops_reg_ilil_11m36.rep` | `drop` (~11:36, ~5 o'clock) |
| `drops_reg_ilil_8m50.rep` | `drop` (~8:50, ~7 o'clock) |
| `drops_notreaver_fafa_9m06.rep` | plain `drop`, **not** `reaver_drop` (reaver a-moved; no reaver-specific order to confirm) |
| `drops_notreaver_eoks_16m32.rep` | plain `drop`, **not** `reaver_drop` |
| `drops_notdt_wwwboo_14m03.rep` | plain `drop`, **not** `dt_drop` (DTs walked; the unload was a reaver) |
| `drops_notdt_llli_19m05.rep` | plain `drop`, **not** `dt_drop` (multi-shuttle Zealot/Archon/HT drop; DTs were for a later drop) |

Only `drop` and `cliff_drop` subtypes exist now — a `reaver_drop` or `dt_drop`
record appearing in any fixture is a regression. Note these fixtures also exercise
the per-target time-window dedup (`dropDedupWindowSec`), which collapses repeat
drops onto the same base.

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

### Offensive-nydus detection — `nydus_golden.json`

Two `nydus_*.rep` fixtures (issue #193), each confirmed by watching the replay.
The protected premise is the **presence of an offensive nydus** (a forward
`BuildNydusExit` into enemy territory, surfaced as a `nydus_attack` event) by the
named player onto the named target — dropping a detection or adding a spurious
one is a regression. The conservative narrative is deliberate: the forward exit
placement is what we observe; the army may never cross (the canal can be killed
first), so the event asserts the offensive nydus, not a completed army transfer.

| Fixture (source replay) | Premise | Verified |
| --- | --- | --- |
| `nydus_1v1_matchpoint_defiler.rep` (`replays-cwal-dl/30-MORE-mentalgap/MM-6D64D05C-3EED-11F1`) | 1v1 — mentalgap makes one offensive nydus onto the opponent's main (1 o'clock) ~23:00, Zergling/Defiler army | author-curated |
| `nydus_bgh_team_aggression.rep` (`AutoSave/20260503/230946,(8)Big Game Hunters.rep`) | BGH 2v2v2v1v1 — chobo86 makes repeated offensive nydus onto fire-n-blood and SubTERRANeum (~27:00–39:00) | author-curated |

The `target_via` field (`a` attack-coincidence / `p` post-placement activity) and
`second` are derived; the protected premise is presence + target attribution
(`target_label` / `target_owner_pid`). A change that removes an offensive nydus
from either fixture, or adds one onto a base the player never nydused, is a
regression → re-verify by watching, do not blindly `UPDATE_GOLDEN`.

### Nuke detection — `nuke_golden.json` (PENDING human review — not yet tier-1)

Two `nuke_*.rep` fixtures (issue #187). These are **not tier-1 yet**: the golden
captures the detector's current nuke output as a drift guard, but the premise has
not been confirmed by watching the replay. The candidates are in the
`000_screpdb_watch_me` review folder. Once verified in-game, promote them here
with the confirmed per-nuke premise (which launches actually landed, on which
base).

| Fixture (source replay) | Candidate (detector) | Status |
| --- | --- | --- |
| `nuke_tvz_attitude.rep` (`30-MORE-BBBuuuUU[kS]/MM-87075E8E-3C91-11F1`) | ZvT on Attitude — Terran (P1) nukes the Zerg's 12 o'clock expansion @20:14 and @21:33 | awaiting review |
| `nuke_tvp_polestar.rep` (`30-NEW-Horang2[._.]/MM-F96E7920-1E3D-11F1`) | TvP on Pole Star — Terran (P0) nukes the Protoss 7 o'clock expansion @11:51 and 6 o'clock natural @15:19 | awaiting review |

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

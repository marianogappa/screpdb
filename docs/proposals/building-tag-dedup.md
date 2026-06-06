# Build-order dedup: worker one-at-a-time (provable) + never-produced buildings (inferential)

## Summary

Make Build-Order building counts (4-rax vs 5-rax, 2-gate vs 3-gate, #expansions)
reflect what actually happened, via two dedup mechanisms of different confidence:

1. **Worker one-building-at-a-time (provable, 100%, applied whole-game).** If the same
   worker tag issues another build before its previous building could finish, the previous
   one never completed. Terran SCV / Zerg Drone only (Protoss probes warp-in and are freed).

2. **Never-produced production buildings (inferential, applied within a 10-min window).**
   By reconstructing selection state we know which unit-producing buildings ever actually
   produced a unit. A production-capable building (in the BO window) that never produced is
   dropped.

Both rely on a selection-state pre-pass that recovers unit tags from `Select`/`Hotkey`
commands — data screpdb currently discards.

## Motivation

Today we cannot confidently say "this was a 4-Barracks build": the `Build` stream contains
spam re-issues, placement mind-changes, cancelled buildings, and buildings made only for
add-ons/tech that never produce. A `Build` command carries position but no producing-
structure identity, and `Train`/`UnitMorph`/`BuildingMorph` carry only the produced unit —
never the source building's tag. But `Select` commands **do** carry the selected units'
tags (`repcmd.SelectCmd.UnitTags []UnitTag`), so tracking selection lets us bind both
production and worker-build actions to specific unit tags.

## Confidence tiers (why only one is windowed)

| Tier | Rule | Confidence | Scope |
|------|------|-----------|-------|
| A | worker one-building-at-a-time | provable (100%) | **whole game** |
| B | never-produced production building | inferential | **BO window (10 min)** |

A provable rule has no reason to be time-limited; an inferential one does, because a
late-game expansion/production building that never pumped is probably real (just unused),
not spam. Measured: 25–28% of the provable worker drops occur *after* 10 min — windowing
tier A would silently discard them.

## Measured impact (prototype `cmd/tagresearch` over the local ladder corpus)

Corpus: `~/Library/Application Support/Blizzard/StarCraft/Maps/Replays`, 1v1 only:
676 Protoss + 523 Terran + 1441 Zerg player-games. Counts are AFTER current
`earlyfilter`+`cmddedup`, i.e. genuine incremental impact.

**Tier A — provable worker one-at-a-time drops (100%, whole game):**

| Race | provable drops | of which after 10 min |
|------|---------------:|----------------------:|
| Protoss | 0 (law N/A) | – |
| Terran | 3,428 | 943 (28%) |
| Zerg | 2,310 | 526 (23%) |

**Tier B — never-produced production buildings.** `built` = production buildings screpdb
counts today; `produced` = those that ever trained/morphed a unit; `DROP%` whole-game vs
the BO-window (≤10 min) cut that the rule actually uses:

| Building | built | produced | drop% (all) | **BO≤10m drop%** |
|----------|------:|---------:|------------:|-----------------:|
| P Gateway | 3889 | 3103 | 20% | **9.7%** |
| P Robotics | 448 | 340 | 24% | **21.0%** |
| P Stargate | 450 | 381 | 15% | **10.2%** |
| P Nexus | 1270 | 763 | 40% | **13.2%** |
| T Barracks | 1320 | 1097 | 17% | **11.0%** |
| T Factory | 1501 | 1100 | 27% | **21.1%** |
| T Starport | 557 | 446 | 20% | **16.7%** |
| T Command Center | 983 | 624 | 37% | **11.6%** |

Whole-game drop% is high but dominated by *late-game macro* (expansions/extra production
built after the BO that the game ended before they pumped) — expected, not spam, hence the
window. Worker buildings (Nexus/CC) validate the method: their starting instance is seeded
(it produces but has no Build command). Factory/Robotics are higher even in-window because
they are frequently built for tech/add-ons rather than to produce units.

**Spot checks (why buildings drop):**

```
[T] Factory: 1 built, 0 produced => DROP 1
   DROP @3:45  (1 dropped had an add-on => provably existed but idle)   # built only a Machine Shop
[P] Gateway: 4 built, 3 produced => DROP 1
   keep @1:18, @2:47, @5:37   DROP @14:09 (58,116)                      # lone late gate, made nothing
[T] Factory: 12 built, 9 produced => DROP 3
   keep @10:01 (109,13)  DROP @10:07 (109,13)                           # same tile, 6s apart = rebuild spam
   DROP @11:11 (114,21)  @11:21 (114,21)                                # same tile, 10s apart = rebuild spam
```

## Enabling data: selection-state pre-pass

Selection tags are needed but the parser discards `Select` commands
(`internal/parser/commands/registry.go` `ignoredCommandTypes`). Run a pre-pass over the
raw screp stream (`rep.Commands.Cmds`) before/alongside command extraction; tags need not
be persisted, only the derived evidence. Per player maintain `curSel []UnitTag` and
`groups map[byte][]UnitTag`:

- `Select` / `…121` → replace; `Select Add` → union; `Select Remove` → remove.
- `Hotkey{Assign,g}` → `groups[g]=copy(curSel)`; `{Select,g}` → `curSel=copy(groups[g])`;
  `{Add,g}` → `curSel=union(curSel,groups[g])`.

(IDs: `repcmd.TypeIDSelect`=0x09/Add/Remove + 0x63–0x65 `121` variants; `TypeIDHotkey`=0x13
with `HotkeyType.Name` ∈ {Assign,Select,Add}.)

Coverage measured: Train/Morph commands issued with exactly one unit selected (so the
producing tag is unambiguous) — **Protoss 100%, Terran 100%, Zerg 57%**. Build commands
issued with a single selected worker tag are similarly near-total for P/T.

## Tier A — worker one-building-at-a-time (provable)

On each `Build` with `len(curSel)==1`, record `(workerTag, sec, building, tile)`. Group by
worker tag (per player, Terran/Zerg only); sort by sec. For consecutive builds `a,b` by the
same tag, if `b.sec < a.sec + BuildTimeOf(a.building)` then `a` was abandoned (the worker
was redirected before it could finish) → drop `a`. Unit tags carry a recycle counter, so a
tag reused after a unit's death does not collide. Apply whole-game.

**Same-tile guard (required).** Only fire when `b` is at a *different tile* than `a`. Two
builds by one worker at the *same* tile are a re-click of the same building (the worker
keeps building it; never redirected), and command construction already collapses same-tile
doubles to one command. Without this guard Tier A drops that surviving collapsed instance,
deleting a real building — observed as a same-tile Academy double-click leaving a ComSat
with no Academy (tech-tree impossible). A different-tile redirect is the only case that
proves abandonment, since BW cannot queue building construction.

## Tier B — never-produced production buildings (inferential, windowed)

Producer evidence: on `Train`/`UnitMorph` with `len(curSel)==1`, the single tag is the
producing building; map produced unit → building type
(Probe→Nexus, Zealot/Dragoon/HT/DT→Gateway, Reaver/Shuttle/Observer→Robotics,
Scout/Carrier/Arbiter/Corsair→Stargate; SCV→CC, Marine/Firebat/Ghost/Medic→Barracks,
Vulture/Tank/Goliath→Factory, Wraith/Dropship/SciVessel/Valkyrie→Starport). Record the tag
and its first-production frame.

Drop decision, per player × production-capable building type, restricted to builds issued
within the 10-min window:
- Seed `startInstances` (1 Nexus / 1 CC) as a producer with no Build command.
- Assign each build, earliest first, to a distinct producing tag whose first production is
  `>= build.sec` (a building cannot produce before it is commanded). Builds left unmatched
  never produced → **drop**. This both yields the count and respects the existence law
  (below), so it never drops more than `builds − builtThatProduced`.

Production-capable types in scope: `Nexus, Gateway, Robotics Facility, Stargate` (P);
`Command Center, Barracks, Factory, Starport` (T). Non-production buildings are untouched.

Semantics (maintainer decision): drop every production-capable building (in window) that
never produced — this intentionally also drops *real but idle* buildings (e.g. a Factory
built only for a Machine Shop), which do not count for build-order purposes.

## Existence laws (keep-guards, prevent over-dropping)

- **produced ⇒ existed:** a tag that produced at frame T must have been built by
  `T − BuildTimeOf(type)`; the assignment above enforces this.
- **add-on ⇒ parent existed:** a single-select `Build` of a Terran add-on proves its parent
  (Machine Shop→Factory, Control Tower→Starport, Comsat/Nuclear Silo→Command Center). Used
  as a do-not-drop guard if we ever move off the "drop all non-producers" semantics; today
  it is reported for visibility (e.g. 152 of the dropped Factories had an add-on).

## Scope & limitations

- **Zerg production buildings (Hatch/Lair/Hive) are out of scope for tier B.** Unit morphs
  select *larva*, not the Hatchery, so producer tags are larvae, not buildings. (Tier A —
  the Drone one-at-a-time rule — does apply to Zerg: 2,310 provable drops.)
- **Protoss is exempt from tier A** (probes are freed on warp-in).
- **Idle-but-real buildings are dropped** by the chosen tier-B semantics.
- **Unit-tag recycle** can split one physical building across two tags, making the keep
  count conservative (fewer drops) — acceptable and rare in the BO window.

## Integration

Complementary to `internal/earlyfilter` (resource-impossibility) and `internal/cmddedup`
(tech/upgrade dedup). Proposed: a new package (e.g. `internal/buildingtags`) producing the
selection-derived evidence and the two drop sets, invoked from `internal/parser/replay.go`
around the existing `earlyfilter.Apply` call. Confirmed producers can additionally feed
earlyfilter's backtrack as must-exist constraints.

## Validation

- Keep `cmd/tagresearch` as a metrics harness (`-detail` for per-game spot checks); track
  drop%, BO-window drop%, provable counts, and Train/Build single-select coverage per race.
- Guardrail: tier B must never drop a build needed to satisfy an existence law; worker-
  building gaps in-window stay modest after seeding the starting instance.
- Spot-check against known BO fixtures in `internal/patterns/markers/testdata/replays`.

## Out of scope / follow-ups

- Zerg hatchery counting via a different signal (larva→hatchery spatial inference).
- Promoting the "add-on ⇒ parent" guard to an alternative, more conservative tier-B mode.

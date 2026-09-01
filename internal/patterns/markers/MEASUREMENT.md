# How expert golden-line timings are measured

Every `Marker.Expert` `TargetSecond` / `Tolerance` in `definitions.go` should
trace back to a run of this procedure. Where a value does not, its comment says
so ("unmeasured legacy").

The procedure is committed as `scripts/expert-mine` — see its package comment
for usage. It stages + ingests the labelled pro subset into a scratch DB and
emits per-milestone percentiles, the in-band% of the current definitions, and
proposed values, plus the fuzzy-opener and phase-2 (non-BO constant)
measurements. Re-running it with `-stage=false -ingest=false` after editing
`definitions.go` recomputes in-band% against the new bands — the acceptance
check below.

## Corpus

Aurora-ID-labelled progamer player-games from the local `screpharvest` harvest.
A row counts as a progamer game iff:

```
row.auroraId != 0
  ∧ row.auroraId ∈ pros_merged.json[name]
  ∧ row.auroraId ∉ pro_exclusions.json[name]
```

`auroraId == 0` is CWAL's unidentified-player sentinel. Filtering on the
harvest's own `proName` field instead is **wrong**: 5,189 rows carry
`auroraId == 0` and inherit a pro label, which alone accounts for 56% of the
raw pro-labelled set.

Restrict to `duration >= 240s` and 1v1 melee. Current size: **5,229 labelled
player-games, 114 pro aurora IDs, 4,746 replay files, median MMR 2,275**
(corpus hash and per-run tallies land in the tool's `meta.json`).

## Joining a label to a `(replay, player)` row

`replays.file_name` ↔ `<matchId>.rep`, then, in order:

1. `players.name == row.toon`
2. else eliminate by `row.oppToon` matching the other slot (1v1 only)
3. else the unique player of the pro's race (fails on mirrors)

Drop what none of the three resolve — currently ~6%. Never guess.

## Measurement source

```sql
SELECT r.file_name, p.name, e.event_type, e.payload
FROM replay_events e
JOIN players p ON p.id = e.source_player_id
JOIN replays r ON r.id = e.replay_id
WHERE e.event_kind = 'marker' AND e.event_type LIKE 'bo_%';
```

`payload.expert_actuals[i].second` is milestone `i` of `Marker.Expert`, in
declaration order. This is the same resolution path the Build Orders tab
compares against, so the measured distribution is by construction the
distribution the band scores.

**The measurement is `AlgorithmVersion`-bound.** Actuals depend on
`earlyfilter` / `builddedup` / `cmdenrich`; re-run after any detection change.

## Statistics

- Percentiles are nearest-rank over the sorted sample: `sorted[round(q·(n−1))]`.
- `TargetSecond` = median.
- `Tolerance` = `Asym(median - p10, p90 - median)`, floored at 2s per side so
  neither side degenerates to zero width.
- By construction ~80% of progamer games land in band. A milestone sitting near
  100% has a band too wide to say anything; near 0% is mis-centred.
- **Do not bake a value with `n < 20`.** Fall back to the shared family table,
  or leave the existing value with an "unmeasured" comment.
- Never derive one milestone from another. `secAfter(pool, BuildTimeSpawningPool)`
  was 2s short of the observed pool→zergling gap, because building time does not
  include larva availability.

## Shared tables

A table shared across buckets (e.g. the mech opening backbone) must be measured
**pooled over exactly the buckets that use it**, not on its largest bucket. If a
sub-bucket's own median disagrees with the pooled median, that is a signal the
table is modelling two different things and should split — this is how the
fact-first / expand-first mech split was found.

Pooling shares the *target* legitimately; it does not always share the
*spread*. A rare sub-bucket can sit outside a band derived from a common one.
That is acceptable — the band is meant to say "this is unusual" — but check it
is not hiding a missing split.

## The fuzzy Zerg opener is measured from the commands table

`bo_z_fuzzy` has an empty `Expert` list — its rung ("~11 Hatch") is only known
at detection time — so there are no `expert_actuals` to read. Its golden bands
live in `fuzzyZergExpertByLabel` (zerg_fuzzy_opener.go), keyed by the persisted
payload label, and the Build Orders tab resolves them at render time against
the raw-command timings of `ListEarlyZergMorphsForBOTimings` — the same rows it
draws the actual ticks from. Measure those labels from the commands table with
the same filters (first `Build` of Spawning Pool / Hatchery, `< 600s`), which
`scripts/expert-mine` does into `fuzzy.tsv`. Because both the band and the
actuals live outside the payload, changing them needs no `AlgorithmVersion`
bump either.

## Changing these values

Targets and tolerances are presentation, not detection: `expert_actuals`
positions are unchanged, so `markers_golden.json` does not move and **no
`AlgorithmVersion` bump / re-ingest is required** — as long as no milestone is
added to or removed from a marker's `Expert` list. Adding or removing one shifts
payload positions and does require a bump.

Run `go generate ./...` to refresh SPECIFICATION.md.

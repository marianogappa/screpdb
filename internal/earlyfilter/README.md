# earlyfilter

Spam-aware filter for the first 5 minutes of a StarCraft: Brood War replay.

## Why this package exists

The replay format records every command the player issued, including spam,
misclicks, and re-routed worker builds. In the early game this gives you
two concrete failures:

1. **Inflated worker counts.** Replays show ~40 workers at 2:24 when the
   actual game had ~15. Build-order detection and the worldstate engine's
   worker tally read garbage.
2. **Inflated building counts.** Patterns like 2-Gate fire on single-Gate
   games because the same probe re-routed to two placement spots looks
   like two Gateways being built simultaneously.

This package runs unconditionally during ingestion (see
[`internal/parser`](../parser)) and produces a cleaner command stream the
pattern orchestrator and the database both consume.

## What the filter does

For each player, simulate the early game frame by frame:

* Start with race-correct state (50 minerals, 4 workers, 9 or 10 supply
  cap, race-specific gather rate from `cmdenrich.GatherRatePerMinute`).
* Walk the command stream in time order. For each `Build`, `Train`, or
  `Unit Morph`:
  - Look up cost / build time / supply via `cmdenrich.EconOf`.
  - **Tech-tree gate**: a Protoss `Gateway` ordered without a completed
    `Pylon` is dropped. Same for any building whose `cmdenrich.PrereqsOf`
    isn't yet satisfied.
  - **Resource gate**: drop if minerals or supply is short.
  - **Heuristic**: an Evolution Chamber Build inside the window is always
    dropped (players cancel it more often than they keep it).
  - Otherwise keep, debit, and schedule the completion (workers gain at
    completion; Pylon / Supply Depot / Overlord bumps `supplyMax` at
    completion; Zerg Drones used to build are consumed at order time).

This forward pass on its own catches most worker spam.

## Backtracking on tech-tree evidence

The forward pass can over-filter when fake worker spend has consumed
minerals that a real building needed. The `backtrack` pass corrects
this using a strong invariant: **the StarCraft engine refuses to execute
orders without their prerequisites**, so a kept Train / Morph implies its
producer chain.

Concretely:

| Kept consequent | Implies (transitively) |
|---|---|
| `Zealot` | `Gateway` ⇒ `Pylon` |
| `Marine` | `Barracks` |
| `Zergling` | `Spawning Pool` |

When a kept consequent's producer is missing from the player's completed
buildings, the backtrack pass:

1. Re-admits the latest dropped Build of the missing producer (and its
   transitive prereqs via `cmdenrich.PrereqsOf`).
2. Reconciles the budget by *retroactively dropping the latest kept
   worker train* before that Build. Workers are the dominant source of
   fake early-game spend — dropping one frees 50 minerals and 1 supply.
3. Re-runs the forward simulation. If the violation persists, the loop
   adds more drops.

The loop is bounded: at most 5 outer iterations, and per iteration the
re-admission adds at most one worker drop per missing prereq. The filter
explicitly errs on admitting commands; if backtracking can't balance the
budget, the consequent stays kept and the prereq stays dropped (logged
in the trace).

## Public API

```go
import "github.com/marianogappa/screpdb/internal/earlyfilter"

result := earlyfilter.Apply(replay, players, commands, earlyfilter.Options{
    DebugDir: os.Getenv("SCREPDB_EARLY_FILTER_DEBUG_DIR"),
})
filteredCommands := result.Commands
```

`Apply` is pure: it does not mutate the input slice and writes nothing
to disk except the optional JSON trace. Failures inside the trace path
never break ingestion.

## Debug trace

Set the env var `SCREPDB_EARLY_FILTER_DEBUG_DIR` to a directory and the
filter will write `<replay-checksum>.json` per replay. Schema (abridged):

```json
{
  "replay": "Somavssnow.rep",
  "max_second": 300,
  "iterations": 1,
  "players": [{
    "player_id": 0,
    "race": "Protoss",
    "ticks": [
      {"second": 0,  "minerals": 50,  "supply": "4/9",  "workers": 4},
      {"second": 60, "minerals": 105, "supply": "9/9",  "workers": 8},
      {"second": 120,"minerals": 165, "supply": "16/25","workers": 12}
    ],
    "decisions": [
      {"frame": 285, "second": 12, "action": "Train",
       "subject": "Probe", "verdict": "kept", "minerals_after": 55},
      {"frame": 312, "second": 13, "action": "Train",
       "subject": "Probe", "verdict": "dropped",
       "reason": "supply_blocked", "minerals_after": 105},
      {"frame": 940, "second": 39, "action": "Build",
       "subject": "Gateway", "verdict": "readmitted",
       "reason": "tech_tree_readmit", "minerals_after": -50}
    ],
    "summary": {"Total": 53, "Kept": 41, "Dropped": 12, "Readmitted": 3,
                "WorkerDropsForBacktrack": 2}
  }]
}
```

`ticks` is sampled every 30 seconds from a re-run of the final
simulation. `decisions` covers every Build / Train / Morph the filter
recognised. `verdict` is one of `kept`, `dropped`, `readmitted`,
`dropped_by_backtrack`. `reason` is empty for clean keeps.

## Knobs

`Options.MaxSecond` defaults to 300. Past that second every command
passes through unchanged.

## Known limitations

* SCV "busy building" downtime is ignored. Terran income is slightly
  over-estimated, biasing toward admitting more commands. This matches
  the design preference to filter less, not more.
* Tech-tree coverage is currently Tier-1 only (Pylon → Gateway → Zealot,
  Barracks → Marine, Spawning Pool → Zergling). Lair-tier and beyond
  rely on the existing `internal/patterns/markers/dedup.go`.
* Workers' producers (Nexus / Command Center / Hatchery) are assumed
  present from frame 0. The filter never raises a violation on a kept
  worker.
* The backtrack pass operates one violation at a time per iteration. A
  pathological replay with many tightly-coupled violations could fail
  to fully reconcile within 5 iterations; in that case some consequents
  remain kept without their full producer chain. The trace records this.

## Tests

`go test ./internal/earlyfilter/...` covers:

* The cost & tech-tree lookup tables.
* Forward-pass over-spam (≥20 Probe trains in 10s drop to ≤5).
* Evolution Chamber heuristic.
* Past-window pass-through.
* Tech-tree backtrack: a kept Zealot re-admits Gateway and Pylon and
  records the worker drop count.

The parser-level golden test
(`internal/parser/parser_test.go::TestParserGolden`) covers the
end-to-end command-count delta and pattern-detection stability across
the 5 replay fixtures in `internal/testdata/replays/`. Refresh with
`UPDATE_GOLDEN=1 go test ./internal/parser/...`.

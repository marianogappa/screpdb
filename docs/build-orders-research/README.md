# Brood War Build-Order Reference for screpdb (2025–2026 meta)

Reference data to improve build-order detection in `screpdb`. **Brood War only — no StarCraft 2.** Every entry is a single, disjoint **opening**, described as a **building timeline** (the signal a replay parser detects reliably).

## Files

- **`Brood_War_Build_Orders.md`** — main reference. 39 disjoint openings (14 Terran, 12 Protoss, 13 Zerg) as building tables with **supply** + any **sourced game-clock timings**, plus creator/popularizer metadata and recent pro-game sources. **Read the "supply vs seconds" section at the top first.**
- **`building_events.csv`** — the matcher-friendly dataset: one row per `(build_id, step, building, supply, sourced_seconds, trigger_note)`. Direct input for detection logic.
- **`Build_Orders_Quick_Reference.csv`** — one row per build (id, names EN/KR, creator, confidence, `has_sourced_clock`, source URL).
- **`Sources_and_Scene_Context.md`** — current players, ASL 19/20/21 results, recent pro VODs, full Liquipedia link list.

## The key finding (changes how you should match)

Liquipedia expresses BW builds in **supply counts**, not seconds. After reading ~40 BO pages directly, **only ~7 builds publish any absolute m:ss timing** (all consolidated in a table at the top of the main doc, and carried in `building_events.csv` as `sourced_seconds`).

So, per the "only sourced timings" decision:

1. **Building SEQUENCE is the strong, fully-populated fingerprint** — order + identity of structures, which you already timestamp exactly. Supply numbers give expected ordering/spacing.
2. **Sourced seconds are sparse** and used as hard anchors where present.
3. **For true second-level reference timings, derive them from your own labelled replay corpus** — `screpdb` is ideal for this. `building_events.csv` gives the schema and supply/sequence priors to seed it. (Ask if you want help scripting the median-timestamp extraction in Go.)

## What changed from the first version

Added building-keyed timelines with sourced timings; split the combined "12 Hatch / Overpool" into separate disjoint entries; removed non-openings (Carrier/Arbiter, SK Terran, Hive Lurker/Defiler, the 2-Factory adaptation, Light's Fast Arbiter) and the empty "5 Hatch Hydra into Muta" page; corrected 2 Gate DT to PvT.

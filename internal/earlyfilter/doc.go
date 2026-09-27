// Package earlyfilter cleans the first ~5 minutes of a replay's command
// stream by simulating per-player resources (minerals, supply, workers) and
// dropping commands that were resource-impossible at the moment they were
// issued.
//
// A backtrack pass uses tech-tree invariants to reverse course when the
// forward pass over-filtered: a kept Train Zealot proves a Gateway must
// have existed, which in turn proves a Pylon must have existed; a kept
// Marine proves a Barracks; a kept Zergling proves a Spawning Pool. The
// backtrack unconditionally re-admits those prerequisite Build commands:
// the kept consequent is engine evidence that the building existed, so
// the re-admission needs no compensating worker drop. The next forward
// pass re-simulates income with the re-admitted building's cost charged
// and all workers intact, which self-corrects any temporary deficit.
//
// The filter is deliberately imperfect:
//
//   - It assumes the player plays perfectly in the gather/economic sense
//     (full mineral rate, no idle workers, no probe-walk delay).
//   - It only filters Build / Train / Morph commands. Move, Attack,
//     Hotkey, Right-Click etc. are passed through unchanged.
//   - It only operates on the first 5 minutes of game time.
//   - It errs on admitting commands. False keeps (kept spam) are cheaper
//     for downstream consumers than false drops (real builds removed).
//
// Apply is the public entry point. It is pure: it does not mutate the
// input command slice and has no I/O outside the optional JSON debug
// trace written to Options.DebugDir.
package earlyfilter

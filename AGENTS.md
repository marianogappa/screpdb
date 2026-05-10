# Agent Build Rules

- React dashboard artifacts from `internal/dashboard/frontend/build` are embedded into the Go binary with `embed`.
- Never run `npm run dev` in production paths (`screpdb` default run or `screpdb dashboard`).
- Always use `make build` for local builds so UI assets are rebuilt before `go build`.
- CI/release workflows enforce UI build before Go build.

# Detection Changes — bump `core.AlgorithmVersion`

Whenever you change anything that affects the *output* of replay detection (game-event composition, marker firing rules, attack/scout/recall/drop heuristics, ownership inference, base resolution, payload shape on `replay_events`, etc.), bump `AlgorithmVersion` in `internal/patterns/core/types.go`. The ingest pipeline stamps each replay's `analyzer_algorithm_version`; replays older than the current constant are re-detected on next ingest. Forgetting this leaves stale detections in users' DBs.

If you only changed presentation (frontend rendering, descriptions, overlays) without touching what's persisted, no bump is needed.

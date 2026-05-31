# Agent Build Rules

- React dashboard artifacts from `internal/dashboard/frontend/build` are embedded into the Go binary with `embed`.
- Never run `npm run dev` in production paths (`screpdb` default run or `screpdb dashboard`).
- Always use `make build` for local builds so UI assets are rebuilt before `go build`.
- CI/release workflows enforce UI build before Go build.

# I/O must go through the facades (issue #135)

- All real filesystem access goes through `internal/iofacade`; the Go binary makes **no external outbound network calls** and only `internal/netfacade` performs a localhost readiness probe. Never call `os.Open/Create/ReadFile/WriteFile/Mkdir*/Remove*/ReadDir/Rename/Stat`, `filepath.Walk/WalkDir/Glob`, `io/ioutil`, or `net`/`net/http` clients directly outside those packages.
- `TestNoDirectIOOutsideFacades` (in `internal/iofacade`) enforces this on every `go test`. If you genuinely need new I/O, add a wrapper to the facade rather than bypassing it, and keep the test green.
- Do not widen the iofacade allowlist (currently: working dir, replays folder, OS cache dir) or add a dependency with broad filesystem/network capability without explicit review — these expand the attack surface the facades exist to contain.
- **When authoring a commit, update the "I/O Safety Audit" log in `README.md`**: add a dated line with a one-word verdict (`OK` / `REVIEW` / `CONCERN`) and a brief justification of whether the change could weaken the I/O rules (new direct os/net calls, a widened allowlist, an outbound network call, a weakened enforcement test, or a dependency with broad I/O capability). You — the authoring LLM — perform this assessment; it is an honour-system receipt that makes tampering visible in the diff. `TestIOSafetyAuditPresent` fails CI if the log is empty, so the entry is not optional. The enforcement test is the real guard.

# Detection Changes — bump `core.AlgorithmVersion`

Whenever you change anything that affects the *output* of replay detection (game-event composition, marker firing rules, attack/scout/recall/drop heuristics, ownership inference, base resolution, payload shape on `replay_events`, etc.), bump `AlgorithmVersion` in `internal/patterns/core/types.go`. The ingest pipeline stamps each replay's `analyzer_algorithm_version`; replays older than the current constant are re-detected on next ingest. Forgetting this leaves stale detections in users' DBs.

If you only changed presentation (frontend rendering, descriptions, overlays) without touching what's persisted, no bump is needed.

# Pull Requests

- **Always use Conventional Commits format for the PR title** (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, etc.). Releases are automated from the squash-merged commit message via release-please-style tooling — a non-conventional title means no release on merge.
- **Check open GitHub issues before opening a PR** (`gh issue list`). If any issue describes the work, start the PR body with `fixes https://github.com/marianogappa/screpdb/issues/<N>` so merging closes the issue. If no matching issue exists, surface that and offer to either open one or proceed without.
- PR descriptions are bullet-point based. Lead with the user-visible behaviour change, then notable implementation details, then anything reviewers should be aware of.
- Do not merge — open the PR and stop. The user wants to review and may want to add commits before merging.

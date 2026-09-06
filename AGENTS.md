# Agent Build Rules

- React dashboard artifacts from `internal/dashboard/frontend/build` are embedded into the Go binary with `embed`.
- Never run `npm run dev` in production paths (`screpdb` default run or `screpdb dashboard`).
- Always use `make build` for local builds so UI assets are rebuilt before `go build`.
- CI/release workflows enforce UI build before Go build.

# I/O must go through the facades (issue #135)

- All real filesystem access goes through `internal/iofacade`; the Go binary's outbound network calls are confined to three sanctioned packages: `internal/netfacade` (loopback readiness probes), `internal/selfupdate` (GitHub Releases, minisign-verified), and `internal/bnetfacade` (SC:R local bridge on loopback + GCS replay downloads allowlisted to `storage.googleapis.com/starcraft-user-uploads-prod/S1-replays/`). Never call `os.Open/Create/ReadFile/WriteFile/Mkdir*/Remove*/ReadDir/Rename/Stat`, `filepath.Walk/WalkDir/Glob`, `io/ioutil`, or `net`/`net/http` clients directly outside those packages.
- `TestNoDirectIOOutsideFacades` (in `internal/iofacade`) enforces this on every `go test`. If you genuinely need new I/O, add a wrapper to the facade rather than bypassing it, and keep the test green.
- Do not widen the iofacade allowlist (currently: the app-data dir `internal/appdata` resolves — `%LOCALAPPDATA%\screpdb` on Windows, `~/Library/Application Support/screpdb` on macOS, `$XDG_CONFIG_HOME/screpdb` on Linux — plus the user's replays folder, which is read-only in practice) or add a dependency with broad filesystem/network capability without explicit review — these expand the attack surface the facades exist to contain. On Windows the app-data dir is the single grantable root the Low-integrity worker can write to (issue #237); `internal/winsandbox` is a sanctioned raw-syscall surface (process spawn + integrity labeling + watch-me broker) on the enforcement-test skip list alongside `internal/selfupdate`.
- **When authoring a commit, update the "I/O Safety Audit" log in `README.md`**: the log is a fenced code block (newest entry shown, older ones in a collapsed `<details>`); add a new dated line at the top in the form `YYYY-MM-DD  OK. <justification>` with a one-word verdict (`OK` / `REVIEW` / `CONCERN`) and a brief justification of whether the change could weaken the I/O rules (new direct os/net calls, a widened allowlist, an outbound network call, a weakened enforcement test, or a dependency with broad I/O capability). You — the authoring LLM — perform this assessment; it is an honour-system receipt that makes tampering visible in the diff. `TestIOSafetyAuditPresent` fails CI if the log is empty, so the entry is not optional. The enforcement test is the real guard.

# Code comments — default to none

The code is the documentation. Write a comment only when it carries a fact a
careful reader cannot recover from the code itself, and then keep it to the
shortest form that carries it — usually one line, occasionally three.

**Delete or don't write:**

- Doc comments that restate the declaration (`// Close closes the connection`,
  `// Options holds options`, `// FileInfo represents a replay file`). Go's
  exported-symbol convention does not override this rule here.
- Step narration inside a function (`// Parse players`, `// Step 3: insert
  commands`, `// Convert nullable int fields`). If the block needs a label,
  extract it into a named function instead.
- Section banners whose text repeats the declaration or selector below them
  (`/* Footer */` above `.app-footer`, `// ----- Combinators -----`).
- Changelogs, version history and curation notes. Those belong in a file —
  see `docs/DETECTOR_VERSIONS.md` for the `core.DetectorVersion` log.
- Restatements of a rule the code expresses declaratively. A marker's `Rule:`
  already says which buildings must precede which.

**Keep, in one pithy line where possible:**

- Why a threshold, window, tolerance or magic number is *that* value, and what
  broke at the other value. Cite the issue (`#227`) or corpus/measurement
  source when there is one.
- A non-obvious invariant or ordering constraint a caller must honour
  (`// Must be called before Finalize.`, `// Markers setting this MUST use the
  endOfReplaySentinel RuleDeadline, because …`).
- Why the obvious implementation was rejected — a measured performance result,
  a false-positive class, an upstream (screp / SC:BW engine) quirk.
- A pointer to where the other half of a mechanism lives, when the coupling is
  not visible from here.
- Units and coordinate spaces when they are not in the identifier (tiles vs
  pixels, frames vs seconds).

**When editing existing code:** if you touch a function whose comments violate
the above, trim them in the same change. Do not preserve a stale comment just
because it was already there, and do not restore a comment the code now makes
obvious.

**Tests are more lenient.** A comment naming what a case proves, or recording
the replay/fixture a premise came from, is worth keeping.

# Detection changes — bump `core.DetectorVersion`

`DetectorVersion` in `internal/patterns/core/types.go` has exactly one job: it
keeps the embedded progamer pack honest. `scripts/pro-pack` stamps the pack it
builds with the value, and `internal/propack`'s test fails when the pack is
stamped older than the code. That matters because the dashboard plots the pack's
precomputed APM, cadence and viewport-switch figures against the *same figures
computed locally from the user's replays*, and two detector versions are not
comparable.

Nothing re-detects or re-ingests on it. The corpus is read into memory and
detected on every launch, so a detection change reaches users on their next
launch whether or not you touch the constant.

**Bump it when, and only when, a change would alter what `scripts/pro-pack`
computes** — that is, the APM / cadence / viewport-switch-rate aggregates, or
the sampling that feeds them. Regenerate the pack in the same change (this needs
the expert-mine scratch corpus; if you cannot, say so in the PR rather than
bumping and leaving the test red). Log the entry in `docs/DETECTOR_VERSIONS.md`.

Changing marker firing rules, game-event composition, or a `FeatureKey` does
*not* need a bump on its own: none of it is persisted, and the pack does not
carry marker data. Renaming a `FeatureKey` or `event_type` is still a breaking
change across the API, the frontend pill registry and the locale catalogs.

# Pull Requests

- **Always use Conventional Commits format for the PR title** (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, etc.). Releases are automated from the squash-merged commit message via release-please-style tooling — a non-conventional title means no release on merge.
- **Check open GitHub issues before opening a PR** (`gh issue list`). If any issue describes the work, start the PR body with `fixes https://github.com/marianogappa/screpdb/issues/<N>` so merging closes the issue. If no matching issue exists, surface that and offer to either open one or proceed without.
- PR descriptions are bullet-point based. Lead with the user-visible behaviour change, then notable implementation details, then anything reviewers should be aware of.
- Do not merge — open the PR and stop. The user wants to review and may want to add commits before merging.

# screpdb

[![Release](https://img.shields.io/github/v/release/marianogappa/screpdb)](https://github.com/marianogappa/screpdb/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/marianogappa/screpdb)](go.mod)

screpdb is an advanced Starcraft replay reporting tool.

## Features
### Filtering/finding replays by high-level semantic features & staging them for watching on the game client
<img width="1671" height="854" alt="Screenshot 2026-05-04 at 23 36 24" src="https://github.com/user-attachments/assets/33b28969-10fd-4226-96b2-1507f99f829c" />

### Rich game events browser with map overlays
<img width="1656" height="873" alt="Screenshot 2026-05-04 at 23 41 24" src="https://github.com/user-attachments/assets/9e31dc50-55fd-459b-9628-d3ce847af67b" />

###  Build Order detection with charts and for comparing with progamer timings
<img width="1657" height="860" alt="Screenshot 2026-05-04 at 23 42 20" src="https://github.com/user-attachments/assets/b3d909fd-17c6-410c-9bc9-fcba1cbf2313" />

###  Skill proxies measurements: Viewport Multitasking, Unit Production Cadence, First Unit Efficiency
<img width="1665" height="841" alt="Screenshot 2026-05-04 at 23 43 39" src="https://github.com/user-attachments/assets/aa2db88d-0e12-430c-ba08-97474d462a0c" />

###  Alias list support for progamer replays (built-in, editable, importable/exportable), and automatic aliasing for local user's player names
<img width="1133" height="629" alt="Screenshot 2026-05-04 at 23 44 27" src="https://github.com/user-attachments/assets/592e773a-5691-4841-9d0e-5c53d8f22db4" />

### Sophisticated command de-duping on the early game to facilitate precise build order detection and timing comparisons
<img width="1665" height="877" alt="Screenshot 2026-05-04 at 23 46 48" src="https://github.com/user-attachments/assets/fcf5c796-89a8-4536-8d41-2ab4d868676c" />

### Alliance timeline and team stacking detection on multiplayer melee games
<img width="1557" height="872" alt="Screenshot 2026-05-13 at 22 59 15" src="https://github.com/user-attachments/assets/ce38f46a-89c8-4a9a-b9f9-6489afd9c05b" />



## Installation

Download the latest release from the [Releases page](https://github.com/marianogappa/screpdb/releases). See [CHANGELOG.md](CHANGELOG.md) for release notes.

As a convenience for non-technical Windows users, a special Windows GUI binary is included in releases (look for screpdb-dashboard).

### Windows: install & upgrade via Scoop

If you use [Scoop](https://scoop.sh), you can install screpdb (and upgrade it with one command) instead of re-downloading the binary each release:

```powershell
scoop bucket add screpdb https://github.com/marianogappa/screpdb
scoop install screpdb
```

This installs both the `screpdb` CLI and the `screpdb-dashboard` GUI. To upgrade later:

```powershell
scoop update screpdb
```

The bucket manifest lives at [`bucket/screpdb.json`](bucket/screpdb.json) and is bumped automatically on each release.

> ⚠️ **Warning:** screpdb runs as an unsandboxed binary. To reduce risk it now routes all I/O through in-process facades — filesystem access is confined to the working directory, the replays folder, and the OS cache dir, and the binary makes no outbound network calls (see [Security / I/O model](#security--io-model)). These are best-effort guardrails, not an OS sandbox, so still exercise judgement before running it.

If you prefer to build from source, you'll need Go 1.25.2 or later:

```bash
git clone https://github.com/marianogappa/screpdb.git
cd screpdb
go build .
```

## Developer features
- CLI for ingestion onto SQLite database. No need to use UI: just ingest and query the database.

```bash
./screpdb ingest

- `-i, --input-dir`: Input directory containing replay files (default: system replay directory)
- `-s, --sqlite-path`: SQLite database file path (default: screp.db)
- `-n, --stop-after-n-reps`: Stop after processing N replay files (0 = no limit)
- `-d, --up-to-yyyy-mm-dd`: Only process files up to this date (YYYY-MM-DD format)
- `-m, --up-to-n-months`: Only process files from the last N months (0 = no limit)
- `--store-right-clicks`: Store `Right Click` commands (disabled by default to reduce command-table volume)
- `--skip-hotkeys`: Skip storing `Hotkey` commands (disabled by default)
- `--clean`: Drop all non-dashboard tables before ingesting to start over (useful for migrations)
```

- MCP server: expose the replay database to an MCP client so you can query any game/player.

```bash
./screpdb mcp

# Specify custom database file
./screpdb mcp -s /path/to/custom.db
```

- All UI functionality exposed as API: [OpenAPI schema available](api/openapi/dashboard.v1.yaml)

## Specification — how the numbers are computed

screpdb makes a lot of derived claims: "this is a **9 Pool**", "your Spawning
Pool was 6s late", "a Zealot takes 25.2s". Skeptical? Audit them.

[**SPECIFICATION.md**](SPECIFICATION.md) documents every golden value the app
relies on — unit names, build times, expert timings, costs, tech-tree rules,
detection thresholds, and more. It's:

- **Generated** from the Go source of truth (`go generate ./...`), so it can't drift from the code.
- **Test-backed** — CI fails if any value is wrong or the file is stale.

In short: not aspirational docs that rot, but a provably-accurate description of
what the app actually does.

## Running on Windows

The Windows binaries are **not code-signed**. On first launch you will see one or more of the following — none of them mean the binary is malicious:

- **SmartScreen "Windows protected your PC"** dialog. Click **More info → Run anyway**.
- **Microsoft Defender or third-party antivirus** may flag the binary as suspicious or silently quarantine it. Unsigned Go binaries that read files and make network requests are a known false-positive pattern. If the binary disappears from your Downloads folder, check Defender's Protection History and restore it (or add an exclusion).
- **Enterprise machines** running AppLocker or Windows Defender Application Control may block execution outright. There is no workaround without code signing.

The dashboard binary (`screpdb-dashboard-windows-amd64.exe`) is a GUI app — if you dismiss the SmartScreen dialog, it simply won't start and won't print any error.

You can always [build from source](#installation) to bypass these warnings.

## Reporting a bug

If screpdb misbehaves or crashes, please [open an issue](https://github.com/marianogappa/screpdb/issues/new/choose) — the bug-report form asks for the few things that make a report actionable (version, OS, and ideally the replay that triggers it).

To make this painless, screpdb helps you out:

- **Version is always visible.** The exact version and commit SHA are shown in the dashboard footer (e.g. `v1.3.0 (abc1234)`) — paste that into the issue.
- **Crashes are caught.** If the app panics, it writes a `screpdb-crash-<timestamp>.log` file next to the binary (containing the version, OS, and full stack trace) and prints a pre-filled "open an issue" link. The Windows dashboard GUI — which has no console — additionally opens that pre-filled issue in your browser automatically and writes a `screpdb-dashboard.log` next to the binary. Attach those files to the issue.

### Verifying downloads

Each release publishes a `SHA256SUMS` file and a `SHA256SUMS.minisig` minisign signature alongside the binaries.

**Verify the checksum** (Linux/macOS):

```bash
sha256sum -c SHA256SUMS --ignore-missing
```

**Verify the checksum** (Windows PowerShell):

```powershell
Get-FileHash screpdb-windows-amd64.exe -Algorithm SHA256
# Compare the printed hash against the line in SHA256SUMS
```

**Verify the signature** (requires [minisign](https://jedisct1.github.io/minisign/)):

```bash
minisign -Vm SHA256SUMS -P 'RWS9gPPOydPD/tR8JBOelXKhif526NoAKY18dau7QHR4dqg84QMhJ5L/'
```

## Security / I/O model

screpdb minimizes its attack surface by routing all I/O through facades and keeping dependencies small (see [#135](https://github.com/marianogappa/screpdb/issues/135)):

- **Filesystem** — all disk access goes through `internal/iofacade`, which permits reads/writes only within: the current working directory (the SQLite database), the configured replays folder (read replays, write "watch me" replays), and the OS user-cache directory (cached game-asset images). A narrow, read-only exception walks up from the replays folder to find StarCraft's `CSettings.json`.
- **Network** — the Go binary makes **no external outbound network calls**. The dashboard server binds to `localhost` only. The single "new version available" check is a browser `fetch()` to the GitHub releases API from the dashboard UI, not from the binary. `internal/netfacade` houses the only network-client operation (a localhost readiness probe).
- **Enforcement** — `TestNoDirectIOOutsideFacades` (in `internal/iofacade`) parses the whole module on every `go test` run and fails the build if any package reaches the filesystem or network directly instead of through the facades.

This is a best-effort, in-process guard, not an OS sandbox: paths handed to trusted dependencies (the SQLite driver, the screp parser, scmapanalyzer) are opened inside those libraries. The guarantee is that screpdb's own code keeps its I/O behind these chokepoints.

### I/O Safety Audit

Changes to screpdb are authored by an LLM coding agent (e.g. Claude Code). As part of authoring a change, that LLM re-assesses whether the change could weaken the I/O rules above and records a dated, one-line verdict in the log below (see `AGENTS.md`). It's an honour-system receipt written by the same LLM that wrote the code — it can be tampered with, just as the facades themselves can — but recording it makes any tampering visible in the diff. `TestIOSafetyAuditPresent` fails CI (`go test ./...`) if the log has no entry, so a change cannot land with an empty audit. The authoritative guard remains the enforcement test above.

<!-- IO-AUDIT:START -->
- **2026-06-09** — `OK`. Debugging/crash-reporting improvements (issue #165): new `internal/crashreport` writes a crash log via `iofacade.WriteFile`, and the Windows GUI binary opens a `screpdb-dashboard.log` via `iofacade.Create` and registers cwd with `iofacade.AllowDir` (already an allowed root). No new direct os/net calls, no allowlist widening, no enforcement-test changes; the crash handler's browser-open uses `pkg/browser` (process exec, not a net/fs primitive).
- **2026-06-07** — `OK`. Early-game event overlay rework (issue #159): consolidated BO timeline events + map overlays. Pure presentation/dashboard-response changes (Go struct field, frontend rendering); no new os/net calls, no allowlist or enforcement-test changes.
- **2026-05-31** — `OK`. Introduced the `iofacade`/`netfacade` chokepoints, the enforcement test, and removed the AI + fswatch surfaces; this change establishes the I/O rules rather than weakening them.
<!-- IO-AUDIT:END -->

_(Most recent first. The authoring LLM adds a dated line each time; keep the last few.)_

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions welcome. Open a [Pull Request](https://github.com/marianogappa/screpdb/pulls) or file an [Issue](https://github.com/marianogappa/screpdb/issues).

## Acknowledgments

- Built using the [github.com/icza/screp](https://github.com/icza/screp) library for StarCraft replay parsing. This project would have been impossible without [András Belicza](https://github.com/icza)'s work.

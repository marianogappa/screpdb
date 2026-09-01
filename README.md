# screpdb

screpdb is an advanced Starcraft replay reporting tool.

[![Release](https://img.shields.io/github/v/release/marianogappa/screpdb)](https://github.com/marianogappa/screpdb/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/marianogappa/screpdb)](go.mod)
[![Coverage](https://img.shields.io/badge/coverage-81%25-brightgreen)](scripts/coverage.sh)
[![Ingestion throughput](https://img.shields.io/badge/ingestion-4.7%20replays%2Fsec-brightgreen)](.github/workflows/bench-ingest.yml)

<!-- ingest-bench-start -->
<sub>212.43 ms/replay · corpus: 150 replays · GitHub-hosted 2-core runner · updated automatically on merge to main</sub>
<!-- ingest-bench-end -->

## Features
### Filtering/finding replays by high-level semantic features
<img width="1670" alt="Game list — filter and find replays by high-level semantic features" src="docs/images/game-list.png" />

### Game summary, with one-click staging of a replay for watching on the game client
<img width="1660" alt="Game summary — per-game overview and staging a replay for watching on the game client" src="docs/images/game-summary.png" />

### Rich game events browser with map overlays
<img width="1582" alt="Rich game events browser with map overlays" src="docs/images/game-events.png" />

###  Build Order detection with charts and for comparing with progamer timings
<img width="1657" height="860" alt="Screenshot 2026-05-04 at 23 42 20" src="https://github.com/user-attachments/assets/b3d909fd-17c6-410c-9bc9-fcba1cbf2313" />

###  Skill proxies measurements: Viewport Multitasking, Unit Production Cadence, First Unit Efficiency
<img width="1643" alt="Skill proxies — viewport multitasking, unit production cadence, first unit efficiency" src="docs/images/skill-proxies.png" />

###  Alias list support for progamer replays (built-in, editable, importable/exportable), and automatic aliasing for local user's player names
<img width="1133" height="629" alt="Screenshot 2026-05-04 at 23 44 27" src="https://github.com/user-attachments/assets/592e773a-5691-4841-9d0e-5c53d8f22db4" />

### Sophisticated command de-duping on the early game to facilitate precise build order detection and timing comparisons
<img width="1665" height="877" alt="Screenshot 2026-05-04 at 23 46 48" src="https://github.com/user-attachments/assets/fcf5c796-89a8-4536-8d41-2ab4d868676c" />

### Alliance timeline and team stacking detection on multiplayer melee games
<img width="1557" height="872" alt="Screenshot 2026-05-13 at 22 59 15" src="https://github.com/user-attachments/assets/ce38f46a-89c8-4a9a-b9f9-6489afd9c05b" />



## Installation

See [CHANGELOG.md](CHANGELOG.md) for release notes.

> ⚠️ **Security:** On **Windows**, screpdb runs its worker at **Low integrity** — the OS confines all of screpdb's writes to a single app-data folder, so even a compromised replay/map parser cannot write elsewhere on your machine (see [Security / I/O model](#security--io-model)). On **macOS and Linux** there is no OS sandbox yet: screpdb routes all its own I/O through in-process facades (writes confined to the app-data dir and the replays folder, no outbound network calls beyond user-initiated self-update), but these are best-effort guardrails rather than an OS boundary, so exercise judgement before running it.

<details>
<summary><strong>Windows</strong> — recommended: install with Scoop</summary>

**👉 Recommended: install with [Scoop](https://scoop.sh).** Open **PowerShell** and paste these commands:

```powershell
scoop install git   # required by 'scoop bucket add' (skip if you already have git)
scoop bucket add screpdb https://github.com/marianogappa/screpdb
scoop install screpdb
```

That's it. Now run **`screpdb-gui`** (the app opens in your browser), or `screpdb` for the CLI.

To upgrade later, just run:

```powershell
scoop update screpdb
```

> 💡 **Seeing an old version, or `install` fails on a missing file?** Your local
> copy of the bucket is a git clone that only refreshes on `scoop update`. Run
> `scoop update` (no package name) first to pull the latest manifest, then
> `scoop install screpdb` / `scoop update screpdb`.

Scoop is the happy path because it downloads without a browser, so Windows **won't** show the "unidentified developer" / SmartScreen warning, and upgrades are one command. Don't have Scoop yet? Install it first (one line, from [scoop.sh](https://scoop.sh)):

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
irm get.scoop.sh | iex
```

<details>
<summary>Prefer a direct download? (expect a SmartScreen warning)</summary>

Grab **`screpdb-gui-windows-amd64.exe`** (the GUI; `screpdb-windows-amd64.exe` is the CLI) from the [Releases page](https://github.com/marianogappa/screpdb/releases) and double-click it.

The binaries are **not code-signed**, so on first launch Windows may warn you — none of these mean the binary is malicious:

- **SmartScreen "Windows protected your PC".** Click **More info → Run anyway**.
- **Microsoft Defender or third-party antivirus** may flag or silently quarantine the binary. Unsigned Go binaries that read files and make network requests are a known false-positive pattern. If the file vanishes from Downloads, check Defender's Protection History and restore it (or add an exclusion).
- **Enterprise machines** running AppLocker or Windows Defender Application Control may block it outright. There's no workaround without code signing.

The GUI binary is a windowed app with no console — if you dismiss the SmartScreen dialog it simply won't start and won't print an error. Scoop avoids all of this. You can also [build from source](#building-from-source).

> 💡 **Want the in-app Update button to work?** Put the `.exe` in a folder you can write to without admin rights — e.g. create `%LOCALAPPDATA%\Programs\screpdb\` and drop it there. screpdb can only replace its own binary when its folder is user-writable, so `C:\Program Files\` (needs admin) won't self-update. Otherwise the app just shows the download link instead.

</details>

The Scoop manifest lives at [`bucket/screpdb.json`](bucket/screpdb.json) and is bumped automatically on each release.

</details>

<details>
<summary><strong>Linux</strong> — one-line installer, or Homebrew</summary>

**Install with one command** (downloads the right binary, verifies it against the release's signed `SHA256SUMS`, drops it on your PATH):

```bash
curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh | sh
```

Then run `screpdb`. To upgrade, re-run the same command (or use the in-app **Update** button).

> 🔍 **Don't pipe scripts you haven't read.** [`install.sh`](install.sh) is deliberately short and dependency-free so you can audit it in under a minute — it only downloads the binary for your OS/arch, checks it against the release's signed `SHA256SUMS`, and copies it to `~/.local/bin`. To read it first, then run your local copy:
>
> ```bash
> curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh -o screpdb-install.sh
> less screpdb-install.sh   # audit it
> sh screpdb-install.sh
> ```

Prefer **[Homebrew](https://brew.sh) / Linuxbrew**?

```bash
brew install marianogappa/screpdb/screpdb   # upgrade later: brew upgrade screpdb
```

Or download the binary for your architecture from the [Releases page](https://github.com/marianogappa/screpdb/releases), make it executable, and move it onto your `PATH` — put it in a writable folder (not a Homebrew prefix) so the in-app **Update** button works:

```bash
chmod +x screpdb-linux-amd64                              # or screpdb-linux-arm64
mkdir -p ~/.local/bin && mv screpdb-linux-amd64 ~/.local/bin/screpdb
```

`~/.local/bin` is the one-line installer's default — any writable folder on your `PATH` works. Binaries fetched via curl/brew carry no quarantine flag, so they just run.

> 💡 screpdb self-updates only when its folder is user-writable and not owned by a package manager. A binary you run from `~/Downloads` or a Homebrew prefix won't auto-update — the app falls back to showing the download command instead.

</details>

<details>
<summary><strong>macOS</strong> — Homebrew, or one-line installer</summary>

**Install with [Homebrew](https://brew.sh):**

```bash
brew install marianogappa/screpdb/screpdb   # upgrade later: brew upgrade screpdb
```

Or the one-line installer (verifies it against the release's signed `SHA256SUMS`, installs to `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh | sh
```

Wary of piping to `sh`? It's the same [`install.sh`](install.sh) shown in the Linux section above — read it first, then run your local copy.

Then run `screpdb`. **No Gatekeeper "unidentified developer" block** with either method — `brew` and `curl` don't attach the quarantine attribute that triggers it, so the binary just runs (no notarization needed).

<details>
<summary>Prefer a direct download? (this one <em>does</em> hit Gatekeeper)</summary>

Download the binary for your architecture from the [Releases page](https://github.com/marianogappa/screpdb/releases), then clear the quarantine flag and move it onto your `PATH` — a writable folder (not a Homebrew prefix) so the in-app **Update** button works:

```bash
chmod +x screpdb-darwin-arm64                          # or screpdb-darwin-amd64
xattr -d com.apple.quarantine screpdb-darwin-arm64     # clear the browser-download quarantine
mkdir -p ~/.local/bin && mv screpdb-darwin-arm64 ~/.local/bin/screpdb
```

(Or right-click the binary → **Open** to approve it once.) `~/.local/bin` matches the one-line installer's default.

> 💡 screpdb self-updates only when its folder is user-writable and not owned by a package manager. A binary you run straight from `~/Downloads` or a Homebrew prefix won't auto-update — the app falls back to showing the download command instead.

</details>

</details>

### Building from source

You'll need Go 1.25.2 or later. Use `make build` (not a bare `go build`) so the embedded dashboard UI assets are rebuilt first:

```bash
git clone https://github.com/marianogappa/screpdb.git
cd screpdb
make build
```

## Uninstall

**1. Remove the binary**

| Installed with | Command |
| --- | --- |
| Scoop (Windows) | `scoop uninstall screpdb` |
| Homebrew (macOS/Linux) | `brew uninstall screpdb` |
| One-line installer / manual | Delete the binary you placed (e.g. `~/.local/bin/screpdb`) |

**2. Delete the data folder** (optional — skip this to keep your data for a future reinstall).

```bash
# Windows (PowerShell)
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\screpdb"

# macOS
rm -rf "$HOME/Library/Application Support/screpdb"

# Linux
rm -rf "${XDG_CONFIG_HOME:-$HOME/.config}/screpdb"
```

## Developer features

<details>
<summary>CLI ingestion, MCP server, and full OpenAPI — click to expand</summary>

- CLI for ingestion onto SQLite database. No need to use UI: just ingest and query the database.

```bash
./screpdb ingest

- `-i, --input-dir`: Input directory containing replay files (default: system replay directory)
- `-s, --sqlite-path`: SQLite database file path (default: screp.db)
- `-n, --stop-after-n-reps`: Stop after processing N replay files (0 = no limit)
- `-d, --up-to-yyyy-mm-dd`: Only process files up to this date (YYYY-MM-DD format)
- `-m, --up-to-n-months`: Only process files from the last N months (0 = no limit)
- `--store-right-clicks`: Store `Right Click` commands (disabled by default to reduce command-table volume)
- `--clean`: Drop all non-dashboard tables before ingesting to start over (useful for migrations)
```

- MCP server: point an MCP client (Claude Desktop, Claude Code, Cursor, …) at the replay database and ask questions in natural language about any game, player, matchup, build order, or event. The client's model turns your question into read-only SQL over the ingested data. The server exposes tools to run queries (`query_database`), inspect the schema (`get_database_schema`), read StarCraft domain knowledge (`get_starcraft_knowledge`), and discover players and derived events (`list_top_players`, `list_event_types`).

```bash
./screpdb mcp

# Specify custom database file
./screpdb mcp -s /path/to/custom.db
```

- Server / API: `./screpdb dashboard` (also the default when run with no subcommand) starts the HTTP server and opens the dashboard UI. All UI functionality is exposed as a JSON API — [OpenAPI schema available](api/openapi/dashboard.v1.yaml). Run it headless as an API-only server (no UI, no browser) with `--headless`:

```bash
./screpdb dashboard --headless -p 8000 -s /path/to/custom.db
# then: curl http://localhost:8000/api/health
```

</details>

## Specification — how the numbers are computed

<details>
<summary>Every golden value — unit stats, build times, expert timings, detection thresholds — is generated from source and test-backed (see <code>SPECIFICATION.md</code>)</summary>

screpdb makes a lot of derived claims: "this is a **9 Pool**", "your Spawning
Pool was 6s late", "a Zealot takes 25.2s". Skeptical? Audit them.

[**SPECIFICATION.md**](SPECIFICATION.md) documents every golden value the app
relies on — unit names, build times, expert timings, costs, tech-tree rules,
detection thresholds, and more. It's:

- **Generated** from the Go source of truth (`go generate ./...`), so it can't drift from the code.
- **Test-backed** — CI fails if any value is wrong or the file is stale.

In short: not aspirational docs that rot, but a provably-accurate description of
what the app actually does.

</details>

<details>
<summary><strong>Verifying downloads</strong> — checksums + minisign signature</summary>

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

</details>

## Security / I/O model

screpdb minimizes its attack surface by routing all I/O through facades and keeping dependencies small (see [#135](https://github.com/marianogappa/screpdb/issues/135)). On **macOS and Linux** this is a best-effort, in-process guard; on **Windows** a Low-integrity worker adds a real OS write boundary.

<details>
<summary><strong>How the I/O model works</strong> — filesystem, Windows sandbox, network, self-update, enforcement</summary>

- **Filesystem** — all disk access goes through `internal/iofacade`, which permits reads/writes only within: a single per-OS **app-data directory** (`%LOCALAPPDATA%\screpdb` on Windows, `~/Library/Application Support/screpdb` on macOS, `$XDG_CONFIG_HOME/screpdb` on Linux) that holds the SQLite database, game-asset cache, logs, crash reports, and extracted sample replays; and the configured replays folder (read replays, write "watch me" replays). A narrow, read-only exception walks up from the replays folder to find StarCraft's `CSettings.json`.
- **Windows OS sandbox** — on Windows the app splits into a Medium-integrity **launcher** and a **Low-integrity worker** ([#237](https://github.com/marianogappa/screpdb/issues/237)). The launcher marks the app-data directory Low-writable and relaunches the real worker at Low integrity; the worker keeps read-down access to replays anywhere but can only *write* into that one Low-labeled folder — every other write is refused by the OS, even from a compromised `screp`/`scmapanalyzer` parser. The launcher retains self-update (it must overwrite the install `.exe`) and brokers the single "watch me" write into the read-only replays folder on the worker's behalf. This does **not** stop a compromised parser from *reading* private files (Low integrity can read up-level); blocking reads needs AppContainer + a broker process, a deferred "Tier 2" follow-up.
- **Network** — the dashboard server binds to `localhost` only. The binary's outbound calls are confined to three sanctioned packages: **`internal/selfupdate`** ([#212](https://github.com/marianogappa/screpdb/issues/212)) queries GitHub Releases for self-update, verifying every byte against a minisign-signed `SHA256SUMS` (embedded public key) before any swap; **`internal/bnetfacade`** ([#317](https://github.com/marianogappa/screpdb/issues/317)) talks to SC:R's local web-api bridge (loopback only, path-prefixed to `/web-api/`) and downloads replays from `storage.googleapis.com` (allowlisted to the single path prefix `/starcraft-user-uploads-prod/S1-replays/`, with length + `seRS` magic-byte validation on every download); **`internal/netfacade`** houses localhost readiness probes.
- **Self-update** — updates are always user-initiated, never automatic. Package-manager installs (Scoop on Windows, Homebrew/Linuxbrew on macOS/Linux) and non-writable install directories are detected and excluded so the updater never fights `scoop update` / `brew upgrade` or needs elevation; those installs are pointed back at their package manager. The `curl | sh` installer drops into a writable dir (`~/.local/bin`), so in-app self-update keeps working there. Self-written binaries carry no macOS quarantine xattr / Windows Mark-of-the-Web, so Gatekeeper/SmartScreen don't re-prompt after an update.
- **Enforcement** — `TestNoDirectIOOutsideFacades` (in `internal/iofacade`) parses the whole module on every `go test` run and fails the build if any package reaches the filesystem or network directly instead of through the facades. `internal/selfupdate`, `internal/bnetfacade`, and `internal/winsandbox` (the Windows process-spawn / integrity-labeling / broker surface) are the documented exceptions.

On **macOS and Linux** this is a best-effort, in-process guard, not an OS sandbox: paths handed to trusted dependencies (the SQLite driver, the screp parser, scmapanalyzer) are opened inside those libraries, and the facade only constrains screpdb's own code. On **Windows** the Low-integrity worker adds a real OS write boundary on top of the same facades.

</details>

### I/O Safety Audit

The LLM that authors each change records a dated, one-line verdict on whether it could weaken the I/O rules above (see `AGENTS.md`); `TestIOSafetyAuditPresent` fails CI if the log is empty, and the enforcement test above stays the authoritative guard.

<!-- IO-AUDIT:START -->
```
2026-09-01  OK. Per-option game counts for the games-list featuring filters (issue #359, omnibar groundwork). New Store.CountWorkflowFeaturingGames resolves each UI filter key to a count via workflowFeaturingCountShapeFor, which mirrors the existing workflowFeaturingExistsSQL switch so a count and its filter always agree (two new guard tests assert the two switches stay in step). Cost is a fixed three read-only aggregate queries (GROUP BY over replay_events, a payload-label breakdown, one replays COUNT) plus one COUNT(DISTINCT) per composite key, all through the existing replay-scoped store helpers; counts are corpus-wide and take no user input, so no new query parameters are interpolated. Read-only: no writes, no os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test change, no AlgorithmVersion bump.
```

<details>
<summary>Older I/O safety audit entries (click to expand)</summary>

```
2026-09-01  OK. Replaced the two literal NUL (0x00) bytes in internal/dashboard/frontend/src/lib/compositionPill.jsx with the \x00 escape. The raw bytes made git classify the whole 251-line file as binary, so every change to it rendered as "Bin <n> -> <m> bytes" and could not be reviewed in a diff or a PR. The escape produces the identical character at runtime, so the composite (unit, spell) map keys stay byte-identical (verified: the built JS bundle hash is unchanged). Source-encoding only: no behaviour change, no os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test change, no AlgorithmVersion bump.
```

```
2026-09-01  OK (net reduction: one fetch and one endpoint consumer removed). Dashboard visual-hierarchy rebalance, first tranche (issue #359). Presentation-only: an emphasis-ladder token block in styles.css, games-list column widths (Featuring no longer crops behind a "..." toggle), player-name pill fills/flags/crowns dropped in the list with the winning side rendered as weight, a single Players cell component for every player count (teams chunked into balanced rows from the already-loaded players array, replacing the separate 1v1/team/no-team-info branches), unit-composition segment fills changed to a greyscale ramp, and the replay player colour moved from the name's text colour to an adjacent swatch (which retires legendTextStyle and its black|navy|darkblue text-shadow allowlist, fixing 4 of 8 names below WCAG AA). The /api/player-colors rank palette is no longer consumed: the api.js client method and its loader are deleted, so the frontend makes one fewer request at boot; the Go endpoint is untouched and now unused. No Go changes at all, no detection change and no AlgorithmVersion bump, no new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test change.
```

```
2026-09-01  OK. Hotkey commands now stored as an encoded blob column on players (issue #357, AlgorithmVersion 67). Each player's Hotkey commands are delta-varint-encoded in memory (new internal/hotkeystream package) and written into the new players.hotkey_stream BLOB by the existing player insert; Hotkey rows are no longer written to commands_low_value, and the --skip-hotkeys CLI flag and skip_hotkeys ingest API field are removed, which narrows the API surface. All inputs are already-parsed in-memory commands; the schema change is one additive migration. No new os/net calls, endpoints, facade exemptions, allowlist widening, hosts or paths.
```

```
2026-09-01  OK. Mass-disconnect games no longer credit the replay saver a phantom win (issue #358, AlgorithmVersion 66). All changes consume data already parsed in memory: the detector reads the in-memory players/commands slices, winner clearing touches the same structs, and the two new replay_events types (player_dropped, mass_disconnect) flow through the existing worldstate emit + storage insert path with the type allowlist widened accordingly. No new os/net calls, endpoints, facade exemptions, hosts or paths.
```

```
2026-09-01  OK. Country flags fixed on Windows + country-name tooltips (issue #361). Windows ships no flag glyphs, so the dashboard now vendors the Twemoji Country Flags woff2 (committed at frontend/src/assets, sha256-pinned) into the frontend build, where the existing go:embed of frontend/build serves it from the app's own origin — no page fetches a remote asset. A canvas feature-probe activates the font only where the platform lacks native flags; tooltips resolve names via the browser's built-in Intl.DisplayNames. Presentation only: no new endpoints, os/net calls, facade exemptions, iofacade allowlist widening, hosts or paths; no AlgorithmVersion bump (no detection change).
```

```
2026-09-01  OK. Expert golden-line timings re-derived from the aurora-ID-labelled progamer corpus (issue #362). All shipped changes are data-only: Marker.Expert targets/tolerances in definitions.go, a render-time fuzzyZergExpertByLabel table, and the Build Orders tab resolving fuzzy-opener bands from the already-persisted payload label over the existing ListEarlyZergMorphsForBOTimings query. The mining pipeline itself lives in scripts/expert-mine — dev-only, never shipped, exempt from the facade guard like the rest of scripts/ — and reads the local screpharvest/scfingerprint corpus paths passed by flag, writing only under its -workdir. No new shipped os/net calls, endpoints, facade exemptions, allowlist changes, hosts or paths; no AlgorithmVersion bump (targets are presentation, not detection).
```

```
2026-08-30  OK. Battle.net profile details surfaced on the player page and the Gaming Session players tab. All of it parses payloads already cached in the bnet_profiles table by the existing fetch path: one new read-only query (ListBnetProfilePayloadsByPlayerKeys, bounded by an IN clause over the keys on screen) and a decoder for the identity, alternate-toon and matchmaking fields. Nothing here fetches, so no bridge request is issued and no rate-limit budget is spent; the avatar URLs in the payload are decoded away rather than rendered, so no page pulls a remote asset. No new endpoints, os/net calls, facade exemptions, allowlist changes, hosts or paths; no AlgorithmVersion bump (no detection change).
```

```
2026-08-30  OK. Money-map classification fixed and the Gaming Session view wired up. The map rule now takes the median mineral field rather than whichever field the map file stored first, and compares against a standard 1500 patch instead of a strict > 10000, so "Big Game Hunters - Remastered" stops reading as a Regular map; this reads already-parsed map data and adds no I/O. core.AlgorithmVersion 63 to 64 so map_kind is recomputed on re-ingest. New env var SCREPDB_SESSION_RECENCY widens the session window for development; it is read with os.Getenv, touches no filesystem, and cannot name a path. No new os/net calls, no facade exemptions, no allowlist widening, no new hosts or paths.
```

```
2026-08-30  OK. Manual player aliasing removed; Feature Flags and a flag-gated Gaming Session added. The player_aliases table is dropped by a new settings migration (000002) and its queries, endpoints, OpenAPI operations and UI are gone, which narrows the API surface rather than widening it. The one piece kept, the "you" players, still reads CSettings.json through the same iofacade.FindAndReadAncestorFile call as before, at the same path, read-only; the result is now held in memory instead of persisted, so this removes a DB write and adds no I/O. Feature flags live in a new settings column. Two new read-only endpoints (GET /api/custom/feature-flags, GET /api/custom/gaming-session) and one write (PUT /api/custom/feature-flags) that accepts only allowlisted flag keys; all read the local DB only. The oapi-codegen generator is now pinned instead of @latest, which stops a floating generator emitting code against the pinned runtime. No new os/net calls, no facade exemptions, no allowlist widening, no new hosts or paths.
```

```
2026-08-30  OK. Bridge connection state now watches the port instead of polling HTTP. New bnetfacade.BridgePortOpen opens and immediately closes a loopback TCP connection to an address the same isLocalAddr guard already vets; it sends zero bytes, so SC:R has nothing to forward upstream and no rate-limit budget is spent, and it is unmetered for the same reason ProbeBridge is. The monitor spends a real HTTP probe only when the port's presence flips or on a 60s floor, which is strictly fewer bridge requests than the previous unconditional 30s poll. Profile backfill is now capped at 20 players per page view (previously unbounded, and a players page could have queued 25 players x 5 gateways). One new read-only endpoint, GET /api/custom/bnet/country-codes, serves the existing profile cache and never fetches, so the UI can poll it for free; its IN clause is bounded at 200 keys. No new hosts, paths, facade exemptions or iofacade allowlist changes; no AlgorithmVersion bump (no detection change).
```

```
2026-08-30  OK. Player-pill glyph sizing, games-list Featuring column, fingerprint domain gate, and scmapanalyzer bump. All presentation and query-shape changes: CSS only for the pill adornments; one SQL predicate added to ListPlayerFingerprintVectors restricting fingerprint input to 2-human non-money games (narrows what is read, never widens it); build-order markers dropped from the games-list Featuring column. scmapanalyzer bumped to 2026-08-16 for base recognition on newer maps — an existing dependency at an existing call site, no new I/O capability — with core.AlgorithmVersion 62 to 63 so bases re-resolve on re-ingest. No new os/net calls, no facade exemptions, no iofacade allowlist widening, no new hosts or paths.
```

```
2026-08-30  OK. Fetch and cache SC:R aurora profiles (issue #329). New bnetfacade.FetchAuroraProfile goes through the already-metered BridgeGet (loopback-only, /web-api/ prefix, #319 budgets apply — no new hosts, paths, or facade exemptions) and normalizes the payload to UTF-8 before parsing. Responses are cached in a new bnet_profiles table (dashboard migration 000002) keyed on (toon, gateway) per #344, 24h TTL per Blizzard's Cache-Control max-age=86400, with the unknown-toon 200/aurora_id-0 response negative-cached so misses don't re-spend budget; failed refetches serve the stale row. One new hand-written endpoint (GET /api/custom/bnet/profile). No iofacade allowlist widening, no AlgorithmVersion bump (no detection change).
```

```
2026-08-30  OK. Two-budget rate limiter at the bnetfacade boundary (issue #319). BridgeGet and DownloadReplay now spend separate in-package budgets (token buckets with priority queues, persisted daily caps, exponential cooldown on the bridge's explicit rate-limit signal), so no caller can bypass them; ProbeBridge stays unmetered (local liveness only). New file write: bnet_budget.json (daily counters + cooldown) inside the app-data root, read/written strictly through iofacade — no allowlist widening, no new hosts or paths, no AlgorithmVersion bump (no detection change). Dashboard surfaces a requests-today meter via the existing /api/custom/bnet/status endpoint.
```

```
2026-08-29  OK. SC:R local web-api detection and connection-state UI (issue #318). Adds platform-specific port discovery (macOS: lsof, Linux: /proc/net/tcp, Windows: GetExtendedTcpTable via iphlpapi.dll) and a ProbeBridge function to bnetfacade — all loopback-only, one GET to /web-api/v1/gateway every 10s. Dashboard gains a background monitor goroutine (atomic-cached state), two new hand-written endpoints (GET/POST /api/custom/bnet/{status,toggle}), and a nav pill. The global off-switch disables all probing. No new outbound hosts (all 127.0.0.1), no iofacade allowlist widening, no AlgorithmVersion bump. Platform-specific files use os/exec (macOS lsof) and os.ReadFile (Linux /proc/net/tcp) inside the already-exempt bnetfacade package.
```

```
2026-08-29  REVIEW (deliberate, documented widening). New internal/bnetfacade package (issue #317): third sanctioned network surface added to the enforcement-test skip list. Loopback client restricted to 127.0.0.1 + /web-api/ prefix for SC:R's local bridge. Outbound client allowlisted to exactly storage.googleapis.com + /starcraft-user-uploads-prod/S1-replays/ prefix for GCS replay downloads. Downloaded replays validated (length ≥ 16 bytes + seRS magic at offset 12). Bridge payloads decoded leniently (cp949/ISO-8859-1 fallback) for Korean map titles. Host, path, and loopback restrictions all covered by tests. This widens the attack surface: the binary now makes one genuine outbound call to a third-party host, guarded by host+path allowlist at the facade boundary. golang.org/x/text promoted from indirect to direct (cp949/charmap decoders). No iofacade allowlist widening; no AlgorithmVersion bump (no detection change).
```

```
2026-08-29  OK. Derive game_source and lobby_kind from replay content at ingest (issue #346). Reads only in-memory screp replay struct fields (rep.ShieldBattery, rep.RepFormat, rep.Header.Players, rep.Header.Title). Two new replays columns via migration 000004, surfaced on the API/MCP schema. No new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test change; AlgorithmVersion bumped 61→62 to drive backfill via the existing re-ingest hint.
```

```
2026-08-27  OK. Fingerprint feature vectors extracted at ingest and stored in a new player_fingerprint_vectors table. New dependency github.com/marianogappa/scfingerprint is pure computation over the already-parsed replay (embedded model/dataset via go:embed, no filesystem or network I/O in the code paths used — Extract/FeatureVersion/ModelTag). No new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test change; AlgorithmVersion bumped 60→61 to drive vector backfill via the existing re-ingest hint.
```

```
2026-07-04  OK (net reduction in the SQL surface's capability). MCP-server modernization + dashboard headless API mode. MCP: query_database now rejects non-read-only SQL (only SELECT/WITH/EXPLAIN/PRAGMA, single statement, comment-stripped) so an MCP client can no longer mutate the corpus; corrected tool descriptions/annotations, expanded GetDatabaseSchema introspection to replay_events/player_aliases, refreshed the domain-knowledge text, added two read-only discovery tools (list_top_players, list_event_types), and bumped mcp-go v0.41.1→v0.55.1. Dashboard: new `--headless` flag serves the JSON API only (no embedded SPA, no browser-open — one fewer os call in that mode); documented 8 operational endpoints (game-assets, debug map-layout, markers definitions, sample-set load, self-update status/apply) in the OpenAPI spec, excluded from code generation, with the validator middleware deferring method-less spec paths to their hand-written handlers while still returning 405 for genuine wrong-method calls. All DB access stays through the storage/dashboard layer; no new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test change, no AlgorithmVersion bump (no detection change).
2026-07-04  OK. Zerg opener supply fix: larva morphs cancelled before the player's first Overlord are dropped from the "N Pool"/"N Hatch" count (a cancelled egg that early is provably a Drone, so it refunds a supply) — fixes e.g. a 5 Pool with a cancelled drone reading as 6 Pool. New commands.DropCancelledMorphs runs on the already-filtered stream in the parser; AlgorithmVersion 58→59 (re-ingest), SPECIFICATION.md regenerated. Reads the in-memory command slice only: no os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test change.
2026-07-04  OK. Beta-exempt the catch-all residual buckets (bo_zerg_other / bo_protoss_other / bo_terran_other / opener_unresolved) so the dashboard stops flagging them "beta" — they claim whatever the named openers leave over, so there is no premise to verify. Added the keys to markers.betaExemptFeatureKeys plus a guard test that every exempt key names a live marker. Display-time curation metadata only (beta tag is computed from FeatureKey at the definitions endpoint): no detection/ingest change, no AlgorithmVersion bump, no os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test change.
2026-07-04  OK. "Update available" UX polish: managed/not-writable installs now show a copyable upgrade command with a Copy button and a Changelog link, the loud (major) banner is dismissable like the quiet one, and the not-writable macOS/Linux case surfaces the `curl | sh` install-script re-run. Go change is additive-only — a new runtime.GOOS-derived `OS` field on selfupdate.Status so the frontend can pick a platform-correct command; plus README direct-download placement tips. No new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes; the self-update mechanism (minisign-verified, user-initiated, writable-dir/package-manager detection) is unchanged.
2026-07-03  OK. Removed the Go Report Card README badge (goreportcard.com no longer rendering it) and added a static coverage badge (81%, from scripts/coverage.sh — generated code excluded). Docs-only: no code, os/net calls, or facade allowlist change.
2026-07-03  OK. Fixed the player-outliers endpoint returning 500 (not 404) for an unknown player: GetOutlierPlayerSummary now COALESCEs the NULL-name aggregate to '' (sqlc query + regenerated sqlcgen). Query/codegen only, still through the dashboard store; no os/net calls, no facade allowlist change.
2026-07-03  OK. scmapanalyzer bumped to v0.0.0-20260702193642 (upstream copyright-only change) + Wraith Cloak timing now reports Cloaking Field completion (start+63s, drops if the game ends first), AlgorithmVersion 58. Dependency/detection only: map analysis still runs through the iofacade allowlist, no os/net calls added, no allowlist widening, no TestNoDirectIOOutsideFacades change; 193 no-cache tests across the scmapanalyzer-dependent packages pass.
2026-07-03  OK. Speedlot-timing now reports Leg Enhancement research completion (and drops when the game ends first) + "9 Pool 9 Hatch" relabel; AlgorithmVersion 57, goldens + SPECIFICATION.md regenerated. Pure detection/pattern logic: no os/net calls, no iofacade/netfacade allowlist change, no TestNoDirectIOOutsideFacades change.
2026-07-03  OK. Toolchain bump to Go 1.26.4 (go.mod go directive) plus a repo-wide gofmt pass. Tooling/formatting only: no os/net calls added, no iofacade/netfacade allowlist widening, no change to the TestNoDirectIOOutsideFacades enforcement.
2026-07-02  OK. README presentation pass, no behaviour change: collapsed the per-OS install sections, baked the measured ingestion-throughput figure into the badge, trimmed the Security/I/O-model prose, and reformatted this audit log into a code block. The TestIOSafetyAuditPresent regex was loosened to also match the new plain-date log line. Docs/test-format only: no os/net calls, no iofacade/netfacade allowlist widening, no change to the TestNoDirectIOOutsideFacades enforcement test.
2026-07-02  OK. Follow-up to #248 (PR #255): dashboard-frontend copy only — for package-manager (Homebrew/Scoop) installs the "update available" control shows the exact upgrade command (brew upgrade screpdb / scoop update screpdb) in its label, renders as a non-link (a download page is useless when the fix is a terminal command), and surfaces it via an instant hover/focus tooltip. No Go changes, no os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes.
2026-07-02  OK. BO/marker label render fixes (issue #251) + ingestion-speed benchmark tracking (issue #249). #251 is render-only: a new markers.DecodePayloadLabel decoder resolves the persisted {"label":...} value so the games-list Featuring strip and game-detail openers show "3 Hatch Muta"/"~9 Overpool" instead of the placeholder name — reads existing payloads, no new os/net calls. #249 adds a make bench-ingest target, scripts/bench-ingest.sh / scripts/update-readme-bench.sh, and a bench-ingest.yml workflow — all shell/CI tooling that runs outside the screpdb binary (not part of the Go module the enforcement test parses), plus a checksum-dedup guard in the existing storage benchmark test. No iofacade/netfacade allowlist widening, no enforcement-test changes.
2026-07-02  OK. Free first-install UX for macOS/Linux + GUI asset rename (issue #248). New files are the standalone install.sh (curl | sh installer) and scripts/update-homebrew-formula.sh/.github release wiring — these run outside the screpdb binary (they are shell installers/CI, not part of the Go module the enforcement test parses), so they touch no iofacade/netfacade surface. In-binary Go changes are a pure rename: the Windows GUI release asset screpdb-dashboard-windows-amd64.exe → screpdb-gui-windows-amd64.exe and the buildinfo.Variant value dashboard → gui (self-update asset-name string in internal/selfupdate + a one-line dashboard-frontend message), plus the GUI log file screpdb-dashboard.log → screpdb-gui.log. No new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes. Self-update mechanism is unchanged (still minisign-verified, user-initiated); the curl/Homebrew install paths reuse the existing package-manager / writable-dir detection.
2026-07-02  OK (with a deliberate, documented allowlist change + one new sanctioned surface). Windows Low-integrity sandbox (issue #237). Filesystem: writes are consolidated under a single per-OS app-data root via the new internal/appdata package (DB, game-asset cache, logs, crash reports, sample replays) — the iofacade allowlist changes, not widens: the working-directory and OS-user-cache roots are removed and replaced by the one app-data root (the read-only replays root is unchanged). Windows-only: a new internal/winsandbox package performs raw golang.org/x/sys/windows calls (duplicate-token → Low integrity level → CreateProcessAsUser; SetNamedSecurityInfo to Low-label the app-data dir) and a file-drop broker so the Medium launcher performs the one "watch me" write into the read-only replays folder on the Low worker's behalf; it is added to the enforcement-test skip list alongside internal/selfupdate and documented as a chokepoint. golang.org/x/sys is promoted from indirect to direct. Self-update is unchanged in mechanism (still minisign-verified, user-initiated) — on Windows it now runs in the Medium launcher rather than the worker. Net effect is a reduction in attack surface: even a compromised screp/scmapanalyzer parser can no longer write outside the single app-data dir on Windows. Residual risk documented: a compromised Low worker can request one fixed-path (000_screpdb_watch_me/watch_me.rep) brokered write into the replays folder — low impact, no arbitrary paths.
2026-07-02  OK. "N Hatch <tech>" redesign (issue #245): Hydra/Muta/Lurker become composition markers (any N) layered on the supply opener, counted by town-hall builds at the economy→army transition. New internal/unittags.TownHallBuildSeconds reads the already-parsed raw command stream (no new I/O), threaded through the orchestrator into a new worldstate.Engine.TownHallBuildSeconds getter. Pure detection-logic + dashboard-response + testdata changes; no new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes.
2026-07-01  OK. Round-10 follow-up: N Hatch Hydra base count uses a +30s grace at hydra-production start (2jd fix); curate wraiths / muta hit-n-run / 2jd fixtures. Pure detection-logic + testdata changes; no new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes.
2026-07-01  OK. Round-10 curation: beta-exempt deterministic facts (became_, game-phase, viewport, never_); curate 18 BOs/markers (1 Gate no-expa, 7/8 Pool, 3 Starport Valk, Carriers, BCs, Forge Cannon/Forge-Gate-Cannon, 2 Fact Expa Mech, Nukes, Sair/Speedlot, 1 Fact Expa Tankless Mech, Wraith Cloak, 1-Base Mech) with watched fixtures; rename "Mech (no expa)" family → "1-Base"; fix manner_pylon firing vs Zerg opponents. Pure detection-logic + dashboard-response + testdata changes (marker curation registry, worldstate manner-pylon race gate, golden fixtures); no new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes.
2026-06-30  OK. Terran mech taxonomy reformulated (issues #226/#227): mech named by Factories before the first expansion ("N Fact Expa Mech" + Tankless/plain/no-expa variants), a Goliath composition flavor ("Goliath" / "N Fact Expa Goliath", folding the standalone Goliath opener), "2/3 Starport Wraith/Valkyrie" cluster openers, Bunker Rush loosened to 2+ forward bunkers, retired "Factory Expand"/"2 Fact before Expa". New marker-DSL predicates (BuildCountBeforeFirstBuildOf, BuildCountAtLeastBeforeFirstBuildOf, NthBuildWithinGapOfFirst) + a builddedup Tier-A fix for Terran rapid re-placements; definition/allowlist/curation edits, AlgorithmVersion 50→51. Pure detection-logic + dashboard-response changes; no new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes.
2026-06-29  OK. Fuzzy Zerg opener: when a multi-larva Drone morph makes the supply rung indeterminate, emit a "~N Pool/Hatch" label instead of an exact rung (new Custom evaluator + 13 Hatch rung + 3 Hatch Muta → marker). Pure detection-logic + dashboard-response changes; no new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes.
2026-06-29  OK. Zerg pool/hatch supply-count fix: ProduceCountBeforeBuild now counts produces by game-second relative to the building rather than observation order, correcting a dedup-tail miscount (9 Overpool read as 10 Pool). Pure detection-logic change in the marker DSL; no new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes.
2026-06-27  OK. New markers (Maelstrom, Crazy Zerg, Guardians) + timing pills (First Observer, First Mine), proxy-building map overlays, and a "beta" tag on uncurated markers/BOs. Pure detection + dashboard-response changes: marker definitions/evaluators, a new cmdenrich.KindLayMine fact for the PlaceMine / VultureMine orders, a subjectsOfInterest addition, a curated-feature-key registry surfaced via the markers-definitions endpoint, and frontend rendering. No new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes.
2026-06-27  OK. Terran air/specialist openers (issue #228): redefined/renamed build-order markers, a new Wraith Cloak timing pill, a new proxy_starport game-event, and a player-aware proxy spatial gate. Pure detection + dashboard-response changes (marker/worldstate logic, event-type allowlists, frontend rendering); no new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes.
2026-06-25  OK (with a deliberate, documented widening). In-binary self-update (issue #212) introduces the binary's first sanctioned outbound network calls (GitHub Releases API + asset download) and its first writes outside the iofacade roots (atomically swapping the running binary in its own install dir). Both are confined to the new internal/selfupdate package, which is added to the enforcement test's skip list alongside iofacade/netfacade and documented as a chokepoint. Integrity is guaranteed by verifying a minisign signature (embedded public key) over SHA256SUMS and the asset's SHA-256 before any swap; updates are user-initiated only, and package-manager/non-writable installs are excluded. No other package gained os/net access; the rest of the binary stays behind the facades.
2026-06-09  OK. Ingestion crash resilience (issue #165): added a per-replay panic guard in internal/ingest (recover → per-file error) and a guarded type assertion in the parser. Pure control-flow/error-handling change; no new os/net calls, no allowlist or enforcement-test changes. Audited the concurrent parse/detect path and its screp/scmapanalyzer deps for shared mutable state (none found unguarded).
2026-06-09  OK. Debugging/crash-reporting improvements (issue #165): new internal/crashreport writes a crash log via iofacade.WriteFile, and the Windows GUI binary opens a screpdb-dashboard.log via iofacade.Create and registers cwd with iofacade.AllowDir (already an allowed root). No new direct os/net calls, no allowlist widening, no enforcement-test changes; the crash handler's browser-open uses pkg/browser (process exec, not a net/fs primitive).
2026-08-29  OK. Fingerprint-based player identification on the player detail page (issue #322). New read-only SQL query (ListPlayerFingerprintVectors) against the existing player_fingerprint_vectors table; scfingerprint.MatchMany runs in-process on already-stored vectors. No new os/net calls, no allowlist changes.
2026-06-07  OK. Early-game event overlay rework (issue #159): consolidated BO timeline events + map overlays. Pure presentation/dashboard-response changes (Go struct field, frontend rendering); no new os/net calls, no allowlist or enforcement-test changes.
2026-05-31  OK. Introduced the iofacade/netfacade chokepoints, the enforcement test, and removed the AI + fswatch surfaces; this change establishes the I/O rules rather than weakening them.
```

</details>
<!-- IO-AUDIT:END -->


## License, Contributing & Acknowledgements

- This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
- Due to security safeguards I can no longer accept PRs or other code contributions, but please feel free to file an [Issue](https://github.com/marianogappa/screpdb/issues), and you're more than welcome to contribute non-code improvements.
- Built using the [github.com/icza/screp](https://github.com/icza/screp) library for StarCraft replay parsing. This project would have been impossible without [András Belicza](https://github.com/icza)'s work.
- Country flags on platforms that lack them (Windows) are drawn with the [Twemoji Country Flags](https://github.com/talkjs/country-flag-emoji-polyfill) font, whose artwork comes from [Twemoji](https://github.com/jdecked/twemoji) and is used under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).

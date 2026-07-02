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

Pick your OS below. See [CHANGELOG.md](CHANGELOG.md) for release notes.

> ⚠️ **Security:** On **Windows**, screpdb runs its worker at **Low integrity** — the OS confines all of screpdb's writes to a single app-data folder, so even a compromised replay/map parser cannot write elsewhere on your machine (see [Security / I/O model](#security--io-model)). On **macOS and Linux** there is no OS sandbox yet: screpdb routes all its own I/O through in-process facades (writes confined to the app-data dir and the replays folder, no outbound network calls beyond user-initiated self-update), but these are best-effort guardrails rather than an OS boundary, so exercise judgement before running it.

### Windows

**👉 Recommended: install with [Scoop](https://scoop.sh).** Open **PowerShell** and paste these two commands:

```powershell
scoop bucket add screpdb https://github.com/marianogappa/screpdb
scoop install screpdb
```

That's it. Now run **`screpdb-gui`** (the app opens in your browser), or `screpdb` for the CLI.

To upgrade later, just run:

```powershell
scoop update screpdb
```

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

</details>

The Scoop manifest lives at [`bucket/screpdb.json`](bucket/screpdb.json) and is bumped automatically on each release.

### Linux

**Install with one command** (downloads the right binary, verifies its checksum, drops it on your PATH):

```bash
curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh | sh
```

Then run `screpdb`. To upgrade, re-run the same command (or use the in-app **Update** button).

Prefer **[Homebrew](https://brew.sh) / Linuxbrew**?

```bash
brew install marianogappa/tap/screpdb   # upgrade later: brew upgrade screpdb
```

Or download the binary for your architecture from the [Releases page](https://github.com/marianogappa/screpdb/releases) and `chmod +x screpdb-linux-amd64` (or `screpdb-linux-arm64`). Binaries fetched via curl/brew carry no quarantine flag, so they just run.

### macOS

**Install with [Homebrew](https://brew.sh):**

```bash
brew install marianogappa/tap/screpdb   # upgrade later: brew upgrade screpdb
```

Or the one-line installer (verifies the checksum, installs to `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh | sh
```

Then run `screpdb`. **No Gatekeeper "unidentified developer" block** with either method — `brew` and `curl` don't attach the quarantine attribute that triggers it, so the binary just runs (no notarization needed).

<details>
<summary>Prefer a direct download? (this one <em>does</em> hit Gatekeeper)</summary>

Download the binary for your architecture from the [Releases page](https://github.com/marianogappa/screpdb/releases), then:

```bash
chmod +x screpdb-darwin-arm64   # or screpdb-darwin-amd64
xattr -d com.apple.quarantine screpdb-darwin-arm64   # clear the browser-download quarantine
./screpdb-darwin-arm64
```

(Or right-click the binary → **Open** to approve it once.)

</details>

### Building from source

You'll need Go 1.25.2 or later. Use `make build` (not a bare `go build`) so the embedded dashboard UI assets are rebuilt first:

```bash
git clone https://github.com/marianogappa/screpdb.git
cd screpdb
make build
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

## Reporting a bug

If screpdb misbehaves or crashes, please [open an issue](https://github.com/marianogappa/screpdb/issues/new/choose) — the bug-report form asks for the few things that make a report actionable (version, OS, and ideally the replay that triggers it).

To make this painless, screpdb helps you out:

- **Version is always visible.** The exact version and commit SHA are shown in the dashboard footer (e.g. `v1.3.0 (abc1234)`) — paste that into the issue.
- **Crashes are caught.** If the app panics, it writes a `screpdb-crash-<timestamp>.log` file in the app-data folder (containing the version, OS, and full stack trace) and prints a pre-filled "open an issue" link. The Windows GUI — which has no console — additionally opens that pre-filled issue in your browser automatically and writes a `screpdb-gui.log` in the same folder. Attach those files to the issue.

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

- **Filesystem** — all disk access goes through `internal/iofacade`, which permits reads/writes only within: a single per-OS **app-data directory** (`%LOCALAPPDATA%\screpdb` on Windows, `~/Library/Application Support/screpdb` on macOS, `$XDG_CONFIG_HOME/screpdb` on Linux) that holds the SQLite database, game-asset cache, logs, crash reports, and extracted sample replays; and the configured replays folder (read replays, write "watch me" replays). A narrow, read-only exception walks up from the replays folder to find StarCraft's `CSettings.json`.
- **Windows OS sandbox** — on Windows the app splits into a Medium-integrity **launcher** and a **Low-integrity worker** ([#237](https://github.com/marianogappa/screpdb/issues/237)). The launcher marks the app-data directory Low-writable and relaunches the real worker at Low integrity; the worker keeps read-down access to replays anywhere but can only *write* into that one Low-labeled folder — every other write is refused by the OS, even from a compromised `screp`/`scmapanalyzer` parser. The launcher retains self-update (it must overwrite the install `.exe`) and brokers the single "watch me" write into the read-only replays folder on the worker's behalf. This does **not** stop a compromised parser from *reading* private files (Low integrity can read up-level); blocking reads needs AppContainer + a broker process, a deferred "Tier 2" follow-up.
- **Network** — the dashboard server binds to `localhost` only. The binary's only outbound calls are to **GitHub Releases for self-update** ([#212](https://github.com/marianogappa/screpdb/issues/212)): on launch it reads the latest release to surface an update notice, and — only when you click Update — it downloads the matching asset. Every downloaded byte is verified against a minisign-signed `SHA256SUMS` (embedded public key) before the binary is swapped, so a tampered or man-in-the-middled download is rejected regardless of which host served it. All of this lives in the single sanctioned `internal/selfupdate` package; `internal/netfacade` houses the only other network-client operation (a localhost readiness probe).
- **Self-update** — updates are always user-initiated, never automatic. Package-manager installs (Scoop on Windows, Homebrew/Linuxbrew on macOS/Linux) and non-writable install directories are detected and excluded so the updater never fights `scoop update` / `brew upgrade` or needs elevation; those installs are pointed back at their package manager. The `curl | sh` installer drops into a writable dir (`~/.local/bin`), so in-app self-update keeps working there. Self-written binaries carry no macOS quarantine xattr / Windows Mark-of-the-Web, so Gatekeeper/SmartScreen don't re-prompt after an update.
- **Enforcement** — `TestNoDirectIOOutsideFacades` (in `internal/iofacade`) parses the whole module on every `go test` run and fails the build if any package reaches the filesystem or network directly instead of through the facades. `internal/selfupdate` and `internal/winsandbox` (the Windows process-spawn / integrity-labeling / broker surface) are the documented exceptions.

On **macOS and Linux** this is a best-effort, in-process guard, not an OS sandbox: paths handed to trusted dependencies (the SQLite driver, the screp parser, scmapanalyzer) are opened inside those libraries, and the facade only constrains screpdb's own code. On **Windows** the Low-integrity worker adds a real OS write boundary on top of the same facades.

### I/O Safety Audit

Changes to screpdb are authored by an LLM coding agent (e.g. Claude Code). As part of authoring a change, that LLM re-assesses whether the change could weaken the I/O rules above and records a dated, one-line verdict in the log below (see `AGENTS.md`). It's an honour-system receipt written by the same LLM that wrote the code — it can be tampered with, just as the facades themselves can — but recording it makes any tampering visible in the diff. `TestIOSafetyAuditPresent` fails CI (`go test ./...`) if the log has no entry, so a change cannot land with an empty audit. The authoritative guard remains the enforcement test above.

<!-- IO-AUDIT:START -->
- **2026-07-02** — `OK`. Free first-install UX for macOS/Linux + GUI asset rename (issue #248). New files are the standalone `install.sh` (`curl | sh` installer) and `scripts/update-homebrew-formula.sh`/`.github` release wiring — these run *outside* the screpdb binary (they are shell installers/CI, not part of the Go module the enforcement test parses), so they touch no `iofacade`/`netfacade` surface. In-binary Go changes are a pure rename: the Windows GUI release asset `screpdb-dashboard-windows-amd64.exe` → `screpdb-gui-windows-amd64.exe` and the `buildinfo.Variant` value `dashboard` → `gui` (self-update asset-name string in `internal/selfupdate` + a one-line dashboard-frontend message), plus the GUI log file `screpdb-dashboard.log` → `screpdb-gui.log`. No new os/net calls, no `iofacade`/`netfacade` allowlist widening, no enforcement-test changes. Self-update mechanism is unchanged (still minisign-verified, user-initiated); the curl/Homebrew install paths reuse the existing package-manager / writable-dir detection.
- **2026-07-02** — `OK` (with a deliberate, documented allowlist change + one new sanctioned surface). Windows Low-integrity sandbox (issue #237). Filesystem: writes are **consolidated** under a single per-OS app-data root via the new `internal/appdata` package (DB, game-asset cache, logs, crash reports, sample replays) — the iofacade allowlist **changes**, not widens: the working-directory and OS-user-cache roots are removed and replaced by the one app-data root (the read-only replays root is unchanged). Windows-only: a new `internal/winsandbox` package performs raw `golang.org/x/sys/windows` calls (duplicate-token → Low integrity level → `CreateProcessAsUser`; `SetNamedSecurityInfo` to Low-label the app-data dir) and a file-drop **broker** so the Medium launcher performs the one "watch me" write into the read-only replays folder on the Low worker's behalf; it is added to the enforcement-test skip list alongside `internal/selfupdate` and documented as a chokepoint. `golang.org/x/sys` is promoted from indirect to direct. Self-update is unchanged in mechanism (still minisign-verified, user-initiated) — on Windows it now runs in the Medium launcher rather than the worker. Net effect is a *reduction* in attack surface: even a compromised `screp`/`scmapanalyzer` parser can no longer write outside the single app-data dir on Windows. Residual risk documented: a compromised Low worker can request one fixed-path (`000_screpdb_watch_me/watch_me.rep`) brokered write into the replays folder — low impact, no arbitrary paths.
- **2026-07-02** — `OK`. "N Hatch <tech>" redesign (issue #245): Hydra/Muta/Lurker become composition markers (any N) layered on the supply opener, counted by town-hall builds at the economy→army transition. New `internal/unittags.TownHallBuildSeconds` reads the already-parsed raw command stream (no new I/O), threaded through the orchestrator into a new `worldstate.Engine.TownHallBuildSeconds` getter. Pure detection-logic + dashboard-response + testdata changes; no new os/net calls, no `iofacade`/`netfacade` allowlist widening, no enforcement-test changes.
- **2026-07-01** — `OK`. Round-10 follow-up: N Hatch Hydra base count uses a +30s grace at hydra-production start (2jd fix); curate wraiths / muta hit-n-run / 2jd fixtures. Pure detection-logic + testdata changes; no new os/net calls, no `iofacade`/`netfacade` allowlist widening, no enforcement-test changes.
- **2026-07-01** — `OK`. Round-10 curation: beta-exempt deterministic facts (`became_*`, game-phase, viewport, `never_*`); curate 18 BOs/markers (1 Gate no-expa, 7/8 Pool, 3 Starport Valk, Carriers, BCs, Forge Cannon/Forge-Gate-Cannon, 2 Fact Expa Mech, Nukes, Sair/Speedlot, 1 Fact Expa Tankless Mech, Wraith Cloak, 1-Base Mech) with watched fixtures; rename "Mech (no expa)" family → "1-Base"; fix `manner_pylon` firing vs Zerg opponents. Pure detection-logic + dashboard-response + testdata changes (marker curation registry, worldstate manner-pylon race gate, golden fixtures); no new os/net calls, no `iofacade`/`netfacade` allowlist widening, no enforcement-test changes.
- **2026-06-30** — `OK`. Terran mech taxonomy reformulated (issues #226/#227): mech named by Factories before the first expansion ("N Fact Expa Mech" + Tankless/plain/no-expa variants), a Goliath composition flavor ("Goliath" / "N Fact Expa Goliath", folding the standalone Goliath opener), "2/3 Starport Wraith/Valkyrie" cluster openers, Bunker Rush loosened to 2+ forward bunkers, retired "Factory Expand"/"2 Fact before Expa". New marker-DSL predicates (`BuildCountBeforeFirstBuildOf`, `BuildCountAtLeastBeforeFirstBuildOf`, `NthBuildWithinGapOfFirst`) + a builddedup Tier-A fix for Terran rapid re-placements; definition/allowlist/curation edits, AlgorithmVersion 50→51. Pure detection-logic + dashboard-response changes; no new os/net calls, no `iofacade`/`netfacade` allowlist widening, no enforcement-test changes.
- **2026-06-29** — `OK`. Fuzzy Zerg opener: when a multi-larva Drone morph makes the supply rung indeterminate, emit a "~N Pool/Hatch" label instead of an exact rung (new Custom evaluator + 13 Hatch rung + 3 Hatch Muta → marker). Pure detection-logic + dashboard-response changes; no new os/net calls, no `iofacade`/`netfacade` allowlist widening, no enforcement-test changes.
- **2026-06-29** — `OK`. Zerg pool/hatch supply-count fix: `ProduceCountBeforeBuild` now counts produces by game-second relative to the building rather than observation order, correcting a dedup-tail miscount (9 Overpool read as 10 Pool). Pure detection-logic change in the marker DSL; no new os/net calls, no `iofacade`/`netfacade` allowlist widening, no enforcement-test changes.
- **2026-06-27** — `OK`. New markers (Maelstrom, Crazy Zerg, Guardians) + timing pills (First Observer, First Mine), proxy-building map overlays, and a "beta" tag on uncurated markers/BOs. Pure detection + dashboard-response changes: marker definitions/evaluators, a new `cmdenrich.KindLayMine` fact for the PlaceMine / VultureMine orders, a subjectsOfInterest addition, a curated-feature-key registry surfaced via the markers-definitions endpoint, and frontend rendering. No new os/net calls, no `iofacade`/`netfacade` allowlist widening, no enforcement-test changes.
- **2026-06-27** — `OK`. Terran air/specialist openers (issue #228): redefined/renamed build-order markers, a new `Wraith Cloak timing` pill, a new `proxy_starport` game-event, and a player-aware proxy spatial gate. Pure detection + dashboard-response changes (marker/worldstate logic, event-type allowlists, frontend rendering); no new os/net calls, no `iofacade`/`netfacade` allowlist widening, no enforcement-test changes.
- **2026-06-25** — `OK` (with a deliberate, documented widening). In-binary self-update (issue #212) introduces the binary's first sanctioned outbound network calls (GitHub Releases API + asset download) and its first writes outside the `iofacade` roots (atomically swapping the running binary in its own install dir). Both are confined to the new `internal/selfupdate` package, which is added to the enforcement test's skip list alongside `iofacade`/`netfacade` and documented as a chokepoint. Integrity is guaranteed by verifying a minisign signature (embedded public key) over `SHA256SUMS` and the asset's SHA-256 before any swap; updates are user-initiated only, and package-manager/non-writable installs are excluded. No other package gained os/net access; the rest of the binary stays behind the facades.
- **2026-06-09** — `OK`. Ingestion crash resilience (issue #165): added a per-replay panic guard in `internal/ingest` (recover → per-file error) and a guarded type assertion in the parser. Pure control-flow/error-handling change; no new os/net calls, no allowlist or enforcement-test changes. Audited the concurrent parse/detect path and its `screp`/`scmapanalyzer` deps for shared mutable state (none found unguarded).
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

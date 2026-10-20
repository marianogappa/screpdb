# screpdb

screpdb is an advanced Starcraft replay reporting tool.

[English](README.md) | [한국어](README.ko.md)

[![Release](https://img.shields.io/github/v/release/marianogappa/screpdb)](https://github.com/marianogappa/screpdb/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/marianogappa/screpdb)](go.mod)
[![Coverage](https://img.shields.io/badge/coverage-82%25-brightgreen)](scripts/coverage.sh)
[![Replay load throughput](https://img.shields.io/badge/replay%20load-34.6%20replays%2Fsec-brightgreen)](.github/workflows/bench-load.yml)

<!-- load-bench-start -->
<sub>28.88 ms/replay · corpus: 166 replays · GitHub-hosted 2-core runner · updated automatically on merge to main</sub>
<!-- load-bench-end -->

**▶️ [Try it in your browser](https://marianogappa.github.io/screpdb/)** — the full dashboard on sample replays, no install, runs entirely client side.

**⬇️ Install on Windows:** [download screpdb](https://github.com/marianogappa/screpdb/releases/latest/download/screpdb-gui-windows-amd64.exe) and open it. If Windows says it protected your PC, click **More info → Run anyway**.

macOS, Linux and other options: see [Installation](#installation).

## Features
### Filtering and finding replays by high-level semantic features
<img width="1680" alt="Game list: filter and find replays by high-level semantic features" src="docs/images/game-list.png" />

### Game summary: each player's noteworthy semantic events and unit composition
<img width="1680" alt="Game summary: per-player semantic events, spellcasts and unit composition" src="docs/images/game-summary.png" />

### Rich game events browser with map overlays
<img width="1680" alt="Game events browser with map overlays" src="docs/images/game-events.png" />

### Hotkey intel: what each player keeps on which key, all game long
<img width="1520" alt="Per-key hotkey timeline for both players" src="docs/images/game-hotkeys.png" />

### …and the hotkeyed buildings drawn on the map where they were built
<img width="293" alt="Map overlay of hotkeyed buildings, labelled with their hotkey" src="docs/images/game-hotkeys-map.png" />

### Build order detection, de-duped and charted against usual progamer timings
<img width="1680" alt="Build order detection with charts and progamer timing bands" src="docs/images/build-orders.png" />

### Built-in progamer profiles alongside the players in your own library
<img width="1680" alt="Players list with built-in progamer profiles" src="docs/images/players-list.png" />

### Skill proxies: viewport multitasking, unit production cadence, first unit efficiency
<img width="1680" alt="Skill proxies: population distribution with progamers for reference" src="docs/images/skill-proxies.png" />

### Per-player hotkey signature, including the built-in progamers
<img width="1680" alt="Per-player hotkey signature" src="docs/images/player-hotkey-signature.png" />

### Alliance timeline and team stacking detection on multiplayer melee games
<img width="1680" alt="Alliance timeline and team stacking detection" src="docs/images/alliances.png" />

## Installation

**Windows**

[Download screpdb](https://github.com/marianogappa/screpdb/releases/latest/download/screpdb-gui-windows-amd64.exe) and open it. If Windows says it protected your PC, click **More info → Run anyway**. screpdb moves itself out of Downloads, adds Start Menu and Desktop shortcuts, and opens in your browser.

<details>
<summary>Trouble opening it?</summary>

The app is not code-signed, so:

- **Defender or other antivirus** may quarantine it (a known false positive for unsigned Go binaries). Restore it from Protection History.
- **Work PCs** with AppLocker or WDAC may block it outright.

</details>

<details>
<summary><strong>Linux</strong></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh | sh
```

Or with Homebrew / Linuxbrew:

```bash
brew install marianogappa/screpdb/screpdb   # upgrade later: brew update && brew upgrade screpdb
```

Then run `screpdb`. Upgrade by re-running the installer or with the in-app **Update** button.

<details>
<summary>Read the installer first, or install manually</summary>

[`install.sh`](install.sh) is short and dependency-free: it downloads the binary for your OS/arch, checks it against the release's signed `SHA256SUMS`, and copies it to `~/.local/bin`. To audit before running:

```bash
curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh -o screpdb-install.sh
less screpdb-install.sh
sh screpdb-install.sh
```

Or download the binary from the [Releases page](https://github.com/marianogappa/screpdb/releases) yourself:

```bash
chmod +x screpdb-linux-amd64                              # or screpdb-linux-arm64
mkdir -p ~/.local/bin && mv screpdb-linux-amd64 ~/.local/bin/screpdb
```

</details>

</details>

<details>
<summary><strong>macOS</strong></summary>

```bash
brew install marianogappa/screpdb/screpdb   # upgrade later: brew update && brew upgrade screpdb
```

Or, without Homebrew:

```bash
curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh | sh
```

Then run `screpdb`. Neither method triggers the Gatekeeper "unidentified developer" block.

<details>
<summary>Prefer a direct download? (this one <em>does</em> hit Gatekeeper)</summary>

Download the binary for your Mac from the [Releases page](https://github.com/marianogappa/screpdb/releases), then:

```bash
chmod +x screpdb-darwin-arm64                          # or screpdb-darwin-amd64
xattr -d com.apple.quarantine screpdb-darwin-arm64     # clear the browser-download quarantine
mkdir -p ~/.local/bin && mv screpdb-darwin-arm64 ~/.local/bin/screpdb
```

Keep it in a writable folder (not `~/Downloads` or a Homebrew prefix) so the in-app Update button works.

</details>

</details>

<details>
<summary><strong>Windows: Scoop or the CLI</strong></summary>

<details>
<summary>Prefer Scoop? (also installs the <code>screpdb</code> CLI)</summary>

```powershell
scoop install git   # required by 'scoop bucket add' (skip if you already have git)
scoop bucket add screpdb https://github.com/marianogappa/screpdb
scoop install screpdb
```

Run **`screpdb-gui`** for the app or `screpdb` for the CLI. Upgrade with `scoop update screpdb`. Seeing an old version? Run `scoop update` (no package name) first to refresh the bucket.

No Scoop yet? Install it first:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
irm get.scoop.sh | iex
```

</details>

<details>
<summary>Want the CLI without Scoop?</summary>

Grab **`screpdb-windows-amd64.exe`** from the [Releases page](https://github.com/marianogappa/screpdb/releases). The same SmartScreen and antivirus notes apply. You can also [build from source](#building-from-source).

</details>

</details>

🛡️ **Secured:** on Windows, screpdb runs sandboxed at Low integrity, so even a compromised replay parser can only write to screpdb's own data folder. On macOS and Linux there is no OS sandbox yet; writes are confined in-process to the app-data and replays folders. See [Security / I/O model](#security--io-model).

See [CHANGELOG.md](CHANGELOG.md) for release notes.

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
| Windows download or PowerShell installer | `Remove-Item -Recurse -ErrorAction SilentlyContinue "$env:LOCALAPPDATA\Programs\screpdb", "$([Environment]::GetFolderPath('Programs'))\screpdb.lnk", "$([Environment]::GetFolderPath('Desktop'))\screpdb.lnk"` |
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
<summary>MCP server and full OpenAPI — click to expand</summary>

- Server / API: `./screpdb dashboard` (also the default when run with no subcommand) starts the HTTP server and opens the dashboard UI. All UI functionality is exposed as a JSON API — [OpenAPI schema available](api/openapi/dashboard.v1.yaml). Run it headless as an API-only server (no UI, no browser) with `--headless`:

```bash
./screpdb dashboard --headless -p 8000
# then: curl http://localhost:8000/api/health
```

- MCP server: point an MCP client (Claude Desktop, Claude Code, Cursor, …) at your replays and ask questions in natural language about any game, player, matchup, build order, or event. A curated read-only subset of the API is reachable; nothing that changes local state is.

```bash
./screpdb mcp
```

If you want to build something on top of screpdb's API or MCP, let me know. I'm open to facilitating this, but I won't add untested features otherwise.

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

- **Filesystem** — all disk access goes through `internal/iofacade`, which permits reads/writes only within: a single per-OS **app-data directory** (`%LOCALAPPDATA%\screpdb` on Windows, `~/Library/Application Support/screpdb` on macOS, `$XDG_CONFIG_HOME/screpdb` on Linux) that holds settings, the Battle.net and game-asset caches, logs, crash reports, and extracted sample replays; and the configured replays folder (read replays, write "watch me" replays). A narrow, read-only exception walks up from the replays folder to find StarCraft's `CSettings.json`.
- **Windows OS sandbox** — on Windows the app splits into a Medium-integrity **launcher** and a **Low-integrity worker** ([#237](https://github.com/marianogappa/screpdb/issues/237)). The launcher marks the app-data directory Low-writable and relaunches the real worker at Low integrity; the worker keeps read-down access to replays anywhere but can only *write* into that one Low-labeled folder — every other write is refused by the OS, even from a compromised `screp`/`scmapanalyzer` parser. The launcher retains self-update (it must overwrite the install `.exe`) and brokers the single "watch me" write into the read-only replays folder on the worker's behalf. This does **not** stop a compromised parser from *reading* private files (Low integrity can read up-level); blocking reads needs AppContainer + a broker process, a deferred "Tier 2" follow-up.
- **Network** — the dashboard server binds to `localhost` only. The binary's outbound calls are confined to three sanctioned packages: **`internal/selfupdate`** ([#212](https://github.com/marianogappa/screpdb/issues/212)) queries GitHub Releases for self-update, verifying every byte against a minisign-signed `SHA256SUMS` (embedded public key) before any swap; **`internal/bnetfacade`** ([#317](https://github.com/marianogappa/screpdb/issues/317)) talks to SC:R's local web-api bridge (loopback only, path-prefixed to `/web-api/`) and downloads replays from `storage.googleapis.com` (allowlisted to the single path prefix `/starcraft-user-uploads-prod/S1-replays/`, with length + `seRS` magic-byte validation on every download); **`internal/netfacade`** houses localhost readiness probes.
- **Self-update** — updates are always user-initiated, never automatic. Package-manager installs (Scoop on Windows, Homebrew/Linuxbrew on macOS/Linux) and non-writable install directories are detected and excluded so the updater never fights `scoop update` / `brew upgrade` or needs elevation; those installs are pointed back at their package manager. The `curl | sh` and `install.ps1` installers drop into writable dirs (`~/.local/bin`, `%LOCALAPPDATA%\Programs\screpdb`), so in-app self-update keeps working there. On Windows, an app opened from the Downloads folder copies itself to `%LOCALAPPDATA%\Programs\screpdb`, writes Start Menu and Desktop shortcuts, relaunches from there and deletes the download. Self-written binaries carry no macOS quarantine xattr / Windows Mark-of-the-Web, so Gatekeeper/SmartScreen don't re-prompt after an update.
- **Enforcement** — `TestNoDirectIOOutsideFacades` (in `internal/iofacade`) parses the whole module on every `go test` run and fails the build if any package reaches the filesystem or network directly instead of through the facades. `internal/selfupdate`, `internal/bnetfacade`, and `internal/winsandbox` (the Windows process-spawn / integrity-labeling / broker surface) are the documented exceptions.

On **macOS and Linux** this is a best-effort, in-process guard, not an OS sandbox: paths handed to trusted dependencies (the screp parser, scmapanalyzer) are opened inside those libraries, and the facade only constrains screpdb's own code. On **Windows** the Low-integrity worker adds a real OS write boundary on top of the same facades.

</details>

### I/O Safety Audit

The LLM that authors each change records a dated, one-line verdict on whether it could weaken the I/O rules above (see `AGENTS.md`); `TestIOSafetyAuditPresent` fails CI if the log is empty, and the enforcement test above stays the authoritative guard.

<!-- IO-AUDIT:START -->
```
2026-10-20  OK. The terrain crop endpoint takes layer=terrain or layer=resources (scmapanalyzer renders each from the same replay file it already reads), so creep draws under minerals; the event map drops its whole-map toggle and caption. No new files, paths, network or writes.
```

<details>
<summary>Older I/O safety audit entries (click to expand, five most recent)</summary>

```
2026-10-20  OK. Overlay art from the bumped scmapanalyzer (shadows, sprite anchors, composited units, tileset creep and edge frames) served from embedded data through two new read-only endpoints; the engine snapshot records the map tileset. No new files, paths, network or writes beyond the existing icon cache.
2026-10-20  OK. Game Events: one curated list per game from either the engine or the command stream (never mixed), outcome-aware sentences, focused regions, and an overlay with true-scale sprites, construction bars and creep. The on-demand snapshot pass now opens the engine with its own copy of the already extracted game data (the same app-data bwdata directory openbw already reads); no new files, paths, network or writes beyond the existing snapshot cache.
2026-10-20  OK. Failed updates fall back to a browser download: the Windows launcher relaunches the worker with SCREPDB_SELFUPDATE_FAILED=1 when applying an update fails, and the update status carries that flag plus the latest release's download link. The dashboard shows the link instead of retrying. No new files, reads, writes or network calls.
2026-10-20  OK. Windows self-install: the launcher, when opened from the Downloads folder, copies its own binary to %LOCALAPPDATA%\Programs\screpdb, writes Start Menu and Desktop .lnk files through the Windows Shell, relaunches the copy and deletes the download. All of it lives in internal/selfupdate (the sanctioned binary-swap exception), runs before the Low worker exists, makes no network calls, and dev builds, Scoop installs and binaries outside Downloads are left alone.
2026-10-20  OK. Docs only: Windows install is now a browser download of the app instead of the PowerShell one-liner, which antivirus software can block. No code changes.
```

</details>

<!-- IO-AUDIT:END -->


## License, Contributing & Acknowledgements

- This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
- Due to security safeguards I can no longer accept PRs or other code contributions, but please feel free to file an [Issue](https://github.com/marianogappa/screpdb/issues), and you're more than welcome to contribute non-code improvements.
- Built using the [github.com/icza/screp](https://github.com/icza/screp) library for StarCraft replay parsing. This project would have been impossible without [András Belicza](https://github.com/icza)'s work.
- Country flags on platforms that lack them (Windows) are drawn with the [Twemoji Country Flags](https://github.com/talkjs/country-flag-emoji-polyfill) font, whose artwork comes from [Twemoji](https://github.com/jdecked/twemoji) and is used under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).

# screpdb

screpdb is an advanced Starcraft replay reporting tool.

[English](README.md) | [한국어](README.ko.md)

**▶️ [Try it in your browser](https://marianogappa.github.io/screpdb/)** — the full dashboard on sample replays, no install, runs entirely client side.

[![Release](https://img.shields.io/github/v/release/marianogappa/screpdb)](https://github.com/marianogappa/screpdb/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/marianogappa/screpdb)](go.mod)
[![Coverage](https://img.shields.io/badge/coverage-82%25-brightgreen)](scripts/coverage.sh)
[![Replay load throughput](https://img.shields.io/badge/replay%20load-34.6%20replays%2Fsec-brightgreen)](.github/workflows/bench-load.yml)

<!-- load-bench-start -->
<sub>28.88 ms/replay · corpus: 166 replays · GitHub-hosted 2-core runner · updated automatically on merge to main</sub>
<!-- load-bench-end -->

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

### Korean UI: the dashboard follows your system language (English / 한국어) and has a switcher in the footer



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
brew install marianogappa/screpdb/screpdb   # upgrade later: brew update && brew upgrade screpdb
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
brew install marianogappa/screpdb/screpdb   # upgrade later: brew update && brew upgrade screpdb
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
- **Self-update** — updates are always user-initiated, never automatic. Package-manager installs (Scoop on Windows, Homebrew/Linuxbrew on macOS/Linux) and non-writable install directories are detected and excluded so the updater never fights `scoop update` / `brew upgrade` or needs elevation; those installs are pointed back at their package manager. The `curl | sh` installer drops into a writable dir (`~/.local/bin`), so in-app self-update keeps working there. Self-written binaries carry no macOS quarantine xattr / Windows Mark-of-the-Web, so Gatekeeper/SmartScreen don't re-prompt after an update.
- **Enforcement** — `TestNoDirectIOOutsideFacades` (in `internal/iofacade`) parses the whole module on every `go test` run and fails the build if any package reaches the filesystem or network directly instead of through the facades. `internal/selfupdate`, `internal/bnetfacade`, and `internal/winsandbox` (the Windows process-spawn / integrity-labeling / broker surface) are the documented exceptions.

On **macOS and Linux** this is a best-effort, in-process guard, not an OS sandbox: paths handed to trusted dependencies (the screp parser, scmapanalyzer) are opened inside those libraries, and the facade only constrains screpdb's own code. On **Windows** the Low-integrity worker adds a real OS write boundary on top of the same facades.

</details>

### I/O Safety Audit

The LLM that authors each change records a dated, one-line verdict on whether it could weaken the I/O rules above (see `AGENTS.md`); `TestIOSafetyAuditPresent` fails CI if the log is empty, and the enforcement test above stays the authoritative guard.

<!-- IO-AUDIT:START -->
```
2026-09-06  OK. Retires the detection-algorithm version machinery the SQLite removal left behind, and finishes the comment cleanup. `core.AlgorithmVersion` is renamed `core.DetectorVersion` and re-documented against its one real consumer: scripts/pro-pack stamps the embedded pack with it and internal/propack's test refuses a pack stamped older than the code, because the dashboard plots the pack's precomputed APM / cadence / viewport figures against the same figures computed locally from the user's replays. The dead `algorithm_version` field on /api/custom/markers/definitions is deleted together with the frontend property that stored it and never read it; its own doc comment described a re-fetch-on-version-change cache that was never implemented, and the field is absent from the OpenAPI spec. SPECIFICATION.md's row no longer claims the value triggers re-detection, AGENTS.md's rule is rewritten to the narrow condition that actually holds, and its claim that an scfingerprint FeatureVersion bump makes the pack's vectors stale is removed as false (the pack carries no vectors). Four comments that still described a re-ingest are corrected, and this log is trimmed to the five most recent archived entries, now enforced by TestIOSafetyAuditArchiveBounded. Rename and removal only: no new os/net calls, roots, hosts, endpoints, queries, facade exemptions or enforcement-test weakening. The pack's stamped value stays 68, so no regeneration is needed.
```

<details>
<summary>Older I/O safety audit entries (click to expand, five most recent)</summary>

```
2026-09-06  OK. Comment-only cleanup pass, plus the AGENTS.md rule that keeps it that way. Trimmed or removed redundant comments across the Go packages and the dashboard frontend, and moved the ~300-line `core.AlgorithmVersion` history out of a comment above the constant into `docs/ALGORITHM_VERSIONS.md` (the constant itself, 68, is unchanged, and the comment left behind states what it now gates). Rebased onto the SQLite removal above, which deleted three of the files this pass had touched (internal/ingest, internal/storage); those edits went with them, and internal/mcp/server.go was re-trimmed against its rewritten form. No statements were added, removed or reordered by this pass: every edit replaced a comment range with a shorter comment or with nothing, `gofmt` reformatted the touched Go files, and the build plus the patterns/cmdenrich/parser/earlyfilter test suites stay green. Also shortens eight user-facing strings that carried implementation detail or a redundant hedge (the two insight descriptions, the APM description, the build-order and mutalisk chart legends, the example-replay confirm, and the never_upgraded / never_researched pill titles), in the Go source and both locale catalogs together so the serverExact lookup keeps matching; the catalog parity test passes. No new os/net calls, roots, hosts, endpoints, queries, facade exemptions or enforcement-test changes, and no AlgorithmVersion bump (the pill titles are presentation only and nothing persisted changed).
2026-09-06  OK. Frontend only, and it removes work rather than adding any. The Alliances tab's column-ordering search moves out of AllianceTimeline.jsx into internal/dashboard/frontend/src/lib/allianceLayout.js, where it memoises per-row suffix scores and runs its inner loop on dense player ordinals in preallocated typed arrays instead of re-simulating the whole timeline once per permutation (8 players = 40,320 of them, which blocked the main thread for seconds on an 8-player melee). Scoring, tie-breaking and the permutation enumeration order are unchanged, so the layout is byte-identical; a new test pins that against the previous exhaustive implementation over generated topologies, and it was confirmed against the committed bgh.rep replay in the running dashboard. No Go code, no os/net calls, no new roots, hosts, endpoints, paths or facade exemptions, no enforcement-test change, and no AlgorithmVersion bump (detection and everything persisted are untouched).
2026-09-06  OK. WASM demo published to GitHub Pages: the demo now references its assets relatively (Vite --base=./ for the WASM build only, relative wasm_exec.js/fs-shim.js/screpdb.wasm/assets URLs in the demo index.html) so it works from the /screpdb/ project subpath, and its workflow also builds on pull requests. Build/packaging and static-asset-path changes only; no new os/net calls, no iofacade/netfacade allowlist widening, no enforcement-test changes, and the native build path is untouched.
2026-09-06  REVIEW. Removes SQLite from the binary: the ingest command, internal/ingest, internal/storage and internal/migrations are deleted, and screpdb mcp stops opening a database and reads the headless dashboard's JSON API instead, so one process owns the corpus and MCP holds no state. Two new capabilities, both narrow. (1) netfacade.LocalAPIGet, a loopback-only GET added to the existing network facade and refusing any non-loopback address, so the MCP tools cannot be pointed at a remote host by the model driving them; MCP reaches only a hand-written allowlist of 16 read-only GET paths (games, players, hotkeys, insights, marker definitions, health), cross-checked against the OpenAPI document by test, with every mutating and UI-only operation excluded and /api/games/{id}/see, which launches the game client, explicitly out. (2) screpdb mcp spawns `screpdb dashboard --headless` with os/exec when no screpdb answers on localhost:8000-8009, and kills it on exit; it is the binary re-executing itself with fixed arguments, os.Executable resolves the path, and no argument comes from the model. internal/legacyimport keeps its read-only (mode=ro) open of the pre-2.0 screp.db, and stays the only database reader and the only reason modernc.org/sqlite is still required. The per-package no-database guards in the dashboard and the replay library are replaced by one module-wide TestBinaryHasNoDatabaseDependencies in internal/iofacade, next to the existing enforcement test, exempting only internal/legacyimport and pinned to it by a second test. scripts/expert-mine, the reproducible provenance of the SPECIFICATION golden lines, is ported from the scratch database onto the in-memory library and reads the same staged folder through the same loader. github.com/fatih/color drops out of go.mod with the ingest logger. No new roots, no new hosts, no outbound calls off loopback, no weakened enforcement test, no AlgorithmVersion bump (detection is untouched).
2026-09-06  OK. Narrows I/O, does not widen it. Follow-up to the #384 fix below: loopback bridge requests (discovery probes, state probes and real bridge calls) now go out on a connection bnetfacade owns end to end (net.Dialer + Request.Write + http.ReadResponse, one shot) instead of an http.Transport, and loopbackTransport goes away. DisableKeepAlives closed only one of the two ways into net/http's "Unsolicited response received on idle HTTP channel": the other is a freshly dialled connection, because dialConn starts the read loop before roundTrip claims it, so a peer that greets on connect lands its banner while no request is counted in flight and nothing was ever pooled. Measured against such a peer, the unpooled transport still logged 27 lines in 20,000 probes and an owned connection logged none. Owning the connection also stops an http.Client following a redirect off 127.0.0.1 on a probe. The GCS download client keeps its pooled transport unchanged, as do the skiplist and every loopback-only, /web-api/ prefix and rate-limit check; no new os/net calls, roots, hosts, endpoints, facade exemptions, enforcement-test change, or AlgorithmVersion bump.
```

</details>

<!-- IO-AUDIT:END -->


## License, Contributing & Acknowledgements

- This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
- Due to security safeguards I can no longer accept PRs or other code contributions, but please feel free to file an [Issue](https://github.com/marianogappa/screpdb/issues), and you're more than welcome to contribute non-code improvements.
- Built using the [github.com/icza/screp](https://github.com/icza/screp) library for StarCraft replay parsing. This project would have been impossible without [András Belicza](https://github.com/icza)'s work.
- Country flags on platforms that lack them (Windows) are drawn with the [Twemoji Country Flags](https://github.com/talkjs/country-flag-emoji-polyfill) font, whose artwork comes from [Twemoji](https://github.com/jdecked/twemoji) and is used under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).

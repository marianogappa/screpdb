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
### Filtering and finding replays by high-level semantic features
<img width="1680" alt="Game list: filter and find replays by high-level semantic features" src="docs/images/game-list.png" />

### Game summary: each player's noteworthy semantic events and unit composition
<img width="1680" alt="Game summary: per-player semantic events, spellcasts and unit composition" src="docs/images/game-summary.png" />

### Rich game events browser with map overlays
<img width="1680" alt="Game events browser with map overlays" src="docs/images/game-events.png" />

### Hotkey intel: what each player keeps on which key, all game long
<img width="1520" alt="Per-key hotkey timeline for both players" src="docs/images/game-hotkeys.png" />

### …and the hotkeyed buildings drawn on the map where they were built
<img width="731" alt="Map overlay of hotkeyed buildings, labelled with their hotkey" src="docs/images/game-hotkeys-map.png" />

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
2026-09-16  OK. Folded the allied-collapse special case into the survivor rule it duplicated. The previous commit added a step that credited a single remaining coalition outright; it turned out to be reachable only because the "fewer than two coalitions" guard would otherwise reject it, and in every case it fired the ordinary "sole coalition still holding a player who never left" rule reaches the same answer. The guard now simply exempts alliance-derived groups, so the procedure loses a step. No behaviour change: a single allied coalition with a survivor is credited by the survivor rule, and one with none by the saver or last-leaver fallback, which name the same group. Also measured what the remaining guard actually rejects: 228 of 2,898 corpus games, every one of them humans against computers, since computer slots are excluded from the count and leave the humans as the only side. SPECIFICATION.md regenerated. No new reads, roots, hosts, endpoints, facade exemptions, enforcement-test weakening, or DetectorVersion bump.
```

<details>
<summary>Older I/O safety audit entries (click to expand, five most recent)</summary>

```
2026-09-16  OK. Two corrections to winner determination, both found by reading the generated specification. A disconnect is now its own outcome (models.OutcomeDisconnected), not a loss and not an unknown: when the saver drops, the game carries on and resolves off the recording, so their own result is the disconnect and nobody else's is known. That verdict is also applied last now rather than mid-pipeline, because the elimination signature's new credit mode could otherwise re-credit a winner off the phantom leave cluster a drop writes. Second, a single remaining coalition no longer means nobody wins: StarCraft will not start a one-sided game, so when the end-of-game alliance topology collapses to one group the survivors allied into it and everybody in it won. The rule is gated on the groups coming from that topology, since a single static team means a malformed replay instead. Per-player statistics keep disconnects out of the loss column, the session record folds them into its existing dropped bucket, and library PlayerFlags gains one bit. Swept the 3,364-replay corpus: 26 saver disconnects, 1 everybody-allied game, team result known in 64.1%. No new reads, roots, hosts, endpoints, facade exemptions, enforcement-test weakening, or DetectorVersion bump.
2026-09-16  OK. Session game list shows your own result per game. workflowGameListPlayer gains outcome (the tri-state models.GameOutcome as a string) and dropped, both read from the in-memory library record the row already loaded - no new query, no new endpoint, and neither field is in the OpenAPI schema so no regeneration. A one-glyph column between Played and Players renders it, shown only where the session view asks for it. The session record above it now reads the same field instead of the team result, so the tally and the glyphs can no longer disagree; the crown replaces the check mark for a win, matching the idiom used beside winning player names everywhere else. Column widths in that table moved from th:nth-child to header classes, because the optional column shifted every positional rule one to the right and silently gave Players the Map column's width. No new user-facing strings: the four result labels and the four record labels already existed. No new reads, roots, hosts, endpoints, facade exemptions, enforcement-test weakening, or DetectorVersion bump.
2026-09-16  OK. is_winner is retired for two named results. A game has two outcomes per player - what happened to the player, and what happened to their side - and they coincide only in 1v1; models.GameOutcome (unknown/won/lost) now carries each as Player.Outcome and Player.TeamOutcome, replacing the boolean that silently meant the second while every reader assumed the first. internal/parser/player_outcome.go derives the personal result from the same command stream already in memory: a player who quit, or whose exit ended the recording, while a rival was still in the game conceded. It needs only that player's own exit, so it is knowable in 84% of a 3,364-replay corpus against 64% for the team result, and the two differ in 649 games. library.PlayerFlags widens from uint8 to uint16 to hold both tri-states (in-memory only, never serialised, rebuilt from parsing). Per-player dashboard statistics - overview wins, matchup wins, race breakdown, games-list rows - now read the personal result, which is what they always claimed to mean; winner labels and Battle.net enrichment eligibility keep reading the team result, which is what they need. Losing is recorded explicitly rather than inferred from not-winning, because a replay routinely cannot tell those apart. No new reads, roots, hosts, endpoints, facade exemptions, enforcement-test weakening, or DetectorVersion bump.
2026-09-16  OK. Winner attribution stops treating the replay saver's unrecorded exit as evidence about their team. screp's "largest remaining team wins" credits a side whenever that exit leaves the tally uneven, which is an artefact of the recording stopping; internal/parser/alliances.go now credits only the sole coalition still holding a player who never left, with the saver excluded from the count, falling back to the saver's own coalition when everyone else quit and to the last leaver when no saver is known. screpdb no longer reads rep.Computed.WinnerTeam at all, so the same rule covers 1v1 and Top-vs-Bottom rather than only melee. The elimination signature gained a second mode: where the procedure declines and two or more coalitions still hold players, it credits the one that is not production-dead, since a destroyed player issues no Leave Game. All of it reads the command stream already in memory - no re-parse, no file or network access, no new field. Verified by sweeping a 3,364-replay corpus twice: 0 panics, 517 games move from a credited winner to undecided, 35 the other way, and 24 change winner; 18 of 22 hand-labelled expectations match, with 2 of the 4 mismatches being cases the new code gets right and Battle.net confirms. Undecided rises from 25.8% to 40.1%, which is the honest figure: those games end before the game does. SPECIFICATION.md regenerated from the live constants. No new reads, roots, hosts, endpoints, facade exemptions, enforcement-test weakening, or DetectorVersion bump - is_winner is derived at ingest and the aggregates DetectorVersion gates are untouched.
2026-09-16  OK. Catalogued a replay the StarCraft engine cannot play back. internal/parser/testdata/broken/ holds the file plus a README recording what was observed in-game (playback already wrong by ~15s), which cannot be recovered from the file later. Two tests keep the catalogue honest: the entries must keep parsing, and the anomaly that distinguishes this one stays pinned - three of eight players issue 20-31 Train/Unit Morph commands inside a 5-second window in the opening minute, against a 14-command ceiling in a normal 8-player game and a 50-mineral starting bank that affords one worker. Measured on the raw screp stream because screpdb's early filter already discards these as opening spam. Reads are test-only, from a path under the package's own testdata; no runtime code path touches the directory, and screpdb has no feature that consumes it yet. No new roots, hosts, endpoints, facade exemptions, enforcement-test weakening, or DetectorVersion bump.
```

</details>

<!-- IO-AUDIT:END -->


## License, Contributing & Acknowledgements

- This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
- Due to security safeguards I can no longer accept PRs or other code contributions, but please feel free to file an [Issue](https://github.com/marianogappa/screpdb/issues), and you're more than welcome to contribute non-code improvements.
- Built using the [github.com/icza/screp](https://github.com/icza/screp) library for StarCraft replay parsing. This project would have been impossible without [András Belicza](https://github.com/icza)'s work.
- Country flags on platforms that lack them (Windows) are drawn with the [Twemoji Country Flags](https://github.com/talkjs/country-flag-emoji-polyfill) font, whose artwork comes from [Twemoji](https://github.com/jdecked/twemoji) and is used under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).

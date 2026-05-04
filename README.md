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
<img width="1666" height="641" alt="Screenshot 2026-05-04 at 23 47 28" src="https://github.com/user-attachments/assets/8c9dad2b-45d1-4280-be8d-a1147e01c688" />


## Installation

Download the latest release from the [Releases page](https://github.com/marianogappa/screpdb/releases). See [CHANGELOG.md](CHANGELOG.md) for release notes.

As a convenience for non-technical Windows users, a special Windows GUI binary is included in releases (look for screpdb-dashboard).

> ⚠️ **Warning:** screpdb is currently distributed as a binary with full filesystem read/write access and unrestricted Internet access. Treat it as high-risk software and think twice before executing it. Safety guardrails are being investigated.

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
- `-w, --watch`: Watch for new files and ingest them as they appear
- `-n, --stop-after-n-reps`: Stop after processing N replay files (0 = no limit)
- `-d, --up-to-yyyy-mm-dd`: Only process files up to this date (YYYY-MM-DD format)
- `-m, --up-to-n-months`: Only process files from the last N months (0 = no limit)
- `--store-right-clicks`: Store `Right Click` commands (disabled by default to reduce command-table volume)
- `--skip-hotkeys`: Skip storing `Hotkey` commands (disabled by default)
- `--clean`: Drop all non-dashboard tables before ingesting to start over (useful for migrations)
```

- MCP server: ask AI anything about any game/player.

```bash
./screpdb mcp

# Specify custom database file
./screpdb mcp -s /path/to/custom.db
```

- All UI functionality exposed as API: [OpenAPI schema available](api/openapi/dashboard.v1.yaml)

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions welcome. Open a [Pull Request](https://github.com/marianogappa/screpdb/pulls) or file an [Issue](https://github.com/marianogappa/screpdb/issues).

## Acknowledgments

- Built using the [github.com/icza/screp](https://github.com/icza/screp) library for StarCraft replay parsing. This project would have been impossible without [András Belicza](https://github.com/icza)'s work.

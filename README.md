# screpdb

screpdb is an advanced Starcraft replay reporting tool.

## Features
- Filtering/finding replays by high-level semantic features & staging them for watching on the game client
- Rich game events browser with map overlays
- Build Order detection with charts and for comparing with progamer timings
- Skill proxies measurements: Viewport Multitasking, Unit Production Cadence, First Unit Efficiency
- Alias list support for progamer replays (built-in, editable, importable/exportable), and automatic aliasing for local user's player names
- Alliance timeline and team stacking detection on multiplayer melee games

## Installation

Download the latest release from the [Releases page](https://github.com/marianogappa/screpdb/releases).


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
```

- MCP server: ask AI anything about any game/player.

```bash
./screpdb mcp

# Specify custom database file
./screpdb mcp -s /path/to/custom.db
```

- All UI functionality exposed as API: [OpenAPI schema available](api/openapi/dashboard.v1.yaml)

## License

This project is licensed under the MIT License. See the LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgments

- Built using the `github.com/icza/screp` library for StarCraft replay parsing
- MCP server implementation powered by `github.com/mark3labs/mcp-go`

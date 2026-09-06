# README screenshots

Regenerates every image under `docs/images`, in English and Korean, by driving a
real dashboard over a staged replay corpus. Dev-only tooling: nothing here ships
in the binary, and no repo source is modified.

```bash
./run.sh
```

That stages the corpus, runs `make build`, starts the dashboard on port 8123,
captures both languages into `docs/images`, and stops the server.

## Prerequisites

- **Node 20+** and `npm install` in this folder, plus `npx playwright install chromium`.
- **A [screpharvest](https://github.com/marianogappa/screpharvest) harvest**, for
  the progamer ladder games. Defaults to
  `~/Code/go/src/github.com/marianogappa/screpharvest/harvest`; override with
  `--harvest DIR`.

## What gets staged

`stage.mjs` writes a 500-replay folder — the loader's own cap, so nothing is
silently dropped:

- **Fixtures first.** The five bundled example replays plus
  `replays/bgh_team_stacking.rep`, the multiplayer melee game the alliance and
  team-stacking shot needs (the 1v1 ladder harvest has no BGH games).
- **Up to 20 games per curated progamer handle** (`FEATURED_HANDLES`). The
  dashboard names a barcode from play style alone, but only with three or more
  games for that handle, so volume per handle is what makes the "Possibly …"
  lines and the built-in-progamer comparisons appear.
- **A hash-shuffled spread of other playable games** to fill the rest, so the
  game list reads like a real library rather than a highlight reel.

Selection is deterministic, and replay ids are derived from file checksums, so
re-running produces the same corpus and the same ids.

## Adding or changing a shot

Edit `shots.mjs`. A shot names the route, how to find its replay, and how to
frame it:

| field | meaning |
| --- | --- |
| `pick` | picks the replay out of `/api/games`, by player handle or file name — never by id |
| `route` | the dashboard URL (see `internal/dashboard/frontend/src/lib/mainRoute.js` for the valid `view` / tab values) |
| `click` | optional `{ selector, hasText }` to select something before the shot |
| `clip` | framing; an object, or a function of the picked replay |

`clip` supports the element's own box (`{ selector, nth, hasText }`), that box
trimmed to an inner element (`trimToBottomOf`), the page from the top down to an
element's bottom (`throughSelector`), or a fixed `height`.

Capture one shot at a time while iterating:

```bash
node capture.mjs --base http://localhost:8123 --out out --only game-hotkeys
```

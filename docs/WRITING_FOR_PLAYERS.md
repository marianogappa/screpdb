# Writing for players

PR titles become the lines of `CHANGELOG.md`, and `site/news.json` becomes the
News section of the website. Both are read by StarCraft players, not by
developers. This guide covers both.

## PR titles

The title of a `feat:` or `fix:` PR is copied word for word into the changelog.
Write it as the line a player would want to read.

| Do | Don't |
| --- | --- |
| `feat: search the games list by player name` | `feat(dashboard): omnibar player-name predicate` |
| `fix: Bunker unloads no longer show up as drops` | `fix(drops): stop reporting Bunker unloads as drops` |
| `fix: games where your worker count was too low early on now show the right opener` | `fix(earlyfilter): stop dropping workers during backtrack re-admission` |
| `chore: reuse per-tick unit maps in the engine pass` | `perf(engine): reuse per-tick unit maps` |

- **Say what the player sees or can now do.** Lead with the feature, not the code that changed.
- **Use present tense, lower case, no trailing period.** release-please capitalises nothing for you, so write it as it should appear.
- **No internal names.** Package names, file names, flags, table names and code words (`earlyfilter`, `worldstate`, `facade`, `ingest`, `digest`, `corpus`, `toon`, `WASM`, `MCP`, `SQLite`) mean nothing to players.
- **Use a scope only when it is a place in the app:** `dashboard`, `skill`, `events`, `hotkeys`, `install`, `update`. Otherwise leave it out.
- **Internal work is not `feat:` or `fix:`.** Releases, CI, tests, refactors, crash report plumbing, docs and dependency bumps use `chore:`, `ci:`, `test:`, `refactor:`, `docs:` or `build:`. Those stay out of the changelog. `perf:` is shown, so use it only when players will notice the speedup.
- **One change per title.** "and tidy the I/O audit" belongs in the PR body.

### Fixing a title after merge

If a title slipped through, edit the merged PR's body (not the title) and add:

```
BEGIN_COMMIT_OVERRIDE
fix: Bunker unloads no longer show up as drops
END_COMMIT_OVERRIDE
```

The open release PR picks up the new text on the next push to `main`.

## News entries (`site/news.json`)

Each release gets one hand-written entry before its release PR is merged.
Articles and videos go in the same file.

```json
{
  "date": "2026-11-22",
  "kind": "release",
  "version": "2.5.0",
  "title": "ShieldBattery support",
  "items": ["Your ShieldBattery games show up next to your Battle.net ones."]
}
```

- `kind` is `release`, `video` or `article`. Videos and articles need a `url`; releases need a `version` and `items`.
- An optional `"ko": { "title": "...", "items": [...], "summary": "..." }` gives the Korean version. Without it, Korean visitors see the English text.
- Newest first. `go test ./scripts/news` checks the shape, order and wording.
- CI keeps it in sync with releases: the newest release in `CHANGELOG.md` must have an entry, and every release entry's version and date must match `CHANGELOG.md`. So the release PR fails CI until its entry is added.

`go run ./scripts/news draft` prints a draft entry for the newest release in
`CHANGELOG.md` that has no entry yet. Rewrite it with the rules below before
committing it.

### How to write an entry

- **Title: the one thing worth telling a friend.** "ShieldBattery support", not "2.5.0" and not a list.
- **Two to five items.** Drop anything a player would not notice. Merge small fixes into one line ("Smaller fixes to the Skill card and chat") or leave them out.
- **Write to the player:** "You can now…", "Your games…", "screpdb now…".
- **StarCraft words are fine, code words are not.** Mutalisk, Hatch first, natural, drop, ZvT: yes. Marker, pill, detector, engine pass, digest: no.
- **Name the place in the app** where the change lives, using the names the app shows: the Skill tab, Game Events, the games list.
- **No dashes as punctuation.** No em dashes, en dashes or " - ". Use a colon, a comma or a new sentence.
- **Every release gets an entry.** Only `feat:`, `fix:` and `perf:` cut a release, so there is always something to say. An older patch release can still be folded into the next entry.

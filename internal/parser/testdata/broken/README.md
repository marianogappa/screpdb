# Replays StarCraft itself refuses to play back

Replays that parse cleanly here but break inside the game engine. screpdb has no
feature that uses them yet; they are kept so that whoever builds one has real
examples to work from, and so the signature stays reproducible.

Each entry records what was observed in-game, because that observation cannot be
recovered from the file later.

## 20240207_bgh_2v2v2v2_engine_desync.rep

Big Game Hunters, 2v2v2v2, 8 humans, 41m46s, Brood War 1.21+, saved by chobo86.

**Observed in-game:** playback is already wrong by roughly 15 seconds and never
recovers; the game cannot be watched. Reported 2026-09-16.

**Parses fine.** screp reads the full 41-minute stream, 8 players, ~32k commands,
no unknown command types, no decode errors.

**The anomaly is impossible production.** Counting Train and Unit Morph commands
in each player's worst 5-second window during the first minute:

| player | trains in first 60s | worst 5s burst | at |
| --- | --- | --- | --- |
| Jackal. | 40 | **31** | 0:08 |
| \|200Spartans\| | 66 | **29** | 0:24 |
| PainXG | 34 | **20** | 0:09 |
| PEROBEAR | 11 | 3 | 0:37 |
| -=FallenAngel=- | 9 | 5 | 0:00 |
| SimpleScOrp | 9 | 3 | 0:18 |
| chobo86 | 6 | 2 | 0:06 |
| YAPEPLIN | 5 | 1 | 0:01 |

A player starts with 50 minerals. Nine seconds in, one worker is affordable and a
fast human queues perhaps five. Twenty to thirty-one is not reachable by playing,
so those commands were not produced by an unmodified client. The first burst lands
at 0:08-0:09, immediately before the reported breakage.

Four of the eight players are clean, which rules out a decode-side fault: the same
parse produces sane counts for them from the same file.

**Not proven:** that the flood is what desyncs the engine. It is the only anomaly
found in the stream, and it precedes the breakage, but the causal link is
untested. A second broken replay showing the same burst signature would settle it.

For comparison, a normal 8-player BGH game from the same corpus tops out at a
14-command burst, and most players sit between 2 and 7.

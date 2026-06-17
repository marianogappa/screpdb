# Cliff-drop detection — known limitation & TODO

## Known limitation: coordless plain-`Unload` drops (deferred)

A cliff drop is detected from the unload's location. When the player uses
`MoveUnload` (right-click unload at a spot) the command carries pixel
coordinates and detection is accurate. When the player uses plain `Unload`
(unit-by-unit eject, no coordinates) the drop pass falls back through:

1. the unload's own coords — absent for plain `Unload`,
2. the freshest paired `Load`'s transport tag coord — this is the *load*
   position (source base), not where the dropship later flew,
3. the player's last spatial command coord — often an unrelated `Train`
   rally point or move.

So a cliff drop performed with plain `Unload` after the dropship moved to the
cliff resolves to the wrong place and is missed.

The fix is transport-position tracking: reconstruct selection state and track
each transport tag's position as it moves (mirroring `unittags.Coordinates`
for production coords, issue #175), then resolve a coordless `Unload` to the
selected transport's actual position. Deferred.

## Genuinely-missed cliff drops (future goldens once the above is fixed)

A cliff drop is only missed when it is performed *entirely* with plain `Unload`
(no `MoveUnload` anywhere in the cluster). Drops that include even one corner
`MoveUnload` are already detected via the per-unload check in
`isCliffDropForCluster`. Human-verified purely-coordless misses:

- `oldAutosave/20170527/150854- (8)Big Game Hunters.rep` — bombom — ~19:45
  (plain Unloads at 1181-1185; the 1177 MoveUnload at (3902,3959) is off-cliff).
- `AutoSave/20251116/225058,(8)Big Game Hunters.rep` — JustPassingThru — ~19:08
  (plain Unloads at 1149-1165, no MoveUnload at all).

Note: two replays first thought to be missed are in fact ALREADY detected by the
per-unload check — DeCartonPiedra (`...20171230/171658...`, cliff_drop @361,
t≈[76,15]) and BULLSHlT (`...20180204/195036...`, cliff_drop @491, t≈[110,21]).
The off-cliff drops in those same games (445 / 502) are correctly rejected.

## Note: centroid pollution (fixed)

A cluster can merge a corner cliff drop with a nearby edge unload, dragging the
centroid off the cliff. Cliff classification therefore tests individual unload
points, not the centroid (see `isCliffDropForCluster`). The displayed target
arrow still uses the centroid, so a recovered cliff drop may render its arrow
slightly off the actual cliff — cosmetic only.

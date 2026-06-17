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

## Human-verified missed cliff drops (future goldens once the above is fixed)

These were manually confirmed by watching the replays; the detector currently
misses them because the drop was a plain `Unload`:

- `oldAutosave/20171230/171658- (8)Big Game Hunters.rep` — DeCartonPiedra — ~6:11
- `oldAutosave/20180204/195036-(8)Big Game Hunters.rep` — BULLSHlT — ~8:13
- `oldAutosave/20170527/150854- (8)Big Game Hunters.rep` — bombom — ~19:45

(Each of these replays ALSO has an off-cliff `MoveUnload` that the 150px corner
box correctly rejects — the misses above are separate, real cliff drops.)

## Note: centroid pollution (fixed)

A cluster can merge a corner cliff drop with a nearby edge unload, dragging the
centroid off the cliff. Cliff classification therefore tests individual unload
points, not the centroid (see `isCliffDropForCluster`). The displayed target
arrow still uses the centroid, so a recovered cliff drop may render its arrow
slightly off the actual cliff — cosmetic only.

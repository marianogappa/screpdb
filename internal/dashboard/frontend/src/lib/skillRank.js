// spreadShares places players on a submetric strip against each other: the worst at 0, the best at 1 and the
// rest in proportion between them, so two close players stay close and a standout stands apart. Players who
// all sit together go to the middle.
export function spreadShares(positions) {
  if (!positions.length) return [];
  const lo = Math.min(...positions);
  const hi = Math.max(...positions);
  if (hi - lo < 1e-9) return positions.map(() => 0.5);
  return positions.map((p) => (p - lo) / (hi - lo));
}

// A tug bar reaches its end at this gap, in tiers: a wider gap is already as clear a lead as a metric shows.
const TUG_FULL_GAP = 1.5;

// tugOf is how far a two player bar reaches from the middle, as a share of the strip, and toward which side:
// toward the player ahead, in proportion to their lead.
export function tugOf(leftPosition, rightPosition) {
  const gap = rightPosition - leftPosition;
  return { side: gap >= 0 ? 'right' : 'left', share: Math.min(1, Math.abs(gap) / TUG_FULL_GAP) / 2 };
}

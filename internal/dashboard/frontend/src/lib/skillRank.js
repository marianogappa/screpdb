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

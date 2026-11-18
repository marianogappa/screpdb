// simplify drops the small cliff-hugging steps of an open run of points (Douglas-Peucker).
const simplify = (points, tolerance) => {
  if (points.length < 3) return points;
  const [a, b] = [points[0], points[points.length - 1]];
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const len = Math.hypot(dx, dy);
  let far = 0;
  let farIdx = 0;
  for (let i = 1; i < points.length - 1; i += 1) {
    const p = points[i];
    const d = len === 0 ? Math.hypot(p.x - a.x, p.y - a.y) : Math.abs(dy * p.x - dx * p.y + b.x * a.y - b.y * a.x) / len;
    if (d > far) {
      far = d;
      farIdx = i;
    }
  }
  if (far <= tolerance) return [a, b];
  return [...simplify(points.slice(0, farIdx + 1), tolerance).slice(0, -1), ...simplify(points.slice(farIdx), tolerance)];
};

// baseOutline is a base polygon with its staircase steps smoothed out, in the polygon's own vertex order.
// Following the ring keeps concave bases (a natural's long arm) intact.
export const baseOutline = (polygon, tolerance) => {
  const ring = [];
  for (const p of polygon || []) {
    const last = ring[ring.length - 1];
    if (!last || last.x !== p.x || last.y !== p.y) ring.push({ x: p.x, y: p.y });
  }
  while (ring.length > 1 && ring[0].x === ring[ring.length - 1].x && ring[0].y === ring[ring.length - 1].y) ring.pop();
  if (ring.length < 3) return ring;
  let farIdx = 0;
  let far = -1;
  ring.forEach((p, i) => {
    const d = Math.hypot(p.x - ring[0].x, p.y - ring[0].y);
    if (d > far) {
      far = d;
      farIdx = i;
    }
  });
  return [
    ...simplify(ring.slice(0, farIdx + 1), tolerance).slice(0, -1),
    ...simplify([...ring.slice(farIdx), ring[0]], tolerance).slice(0, -1),
  ];
};

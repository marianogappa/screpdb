// placeLabels picks where each base's owner label goes: the spot inside the base, on screen, that covers
// the least of what the map draws (buildings, units, the event's focus, labels already placed), with a
// small pull towards the base's centre. Everything is in map pixels; the result is deterministic.

const CELL = 16;
const STEP = 32;
const CENTRE_PULL = 0.25;
const LABEL_WEIGHT = 4;

export const pointInPolygon = (poly, x, y) => {
  let inside = false;
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i, i += 1) {
    const [a, b] = [poly[i], poly[j]];
    if ((a.y > y) !== (b.y > y) && x < ((b.x - a.x) * (y - a.y)) / (b.y - a.y) + a.x) inside = !inside;
  }
  return inside;
};

class Occupancy {
  constructor(width, height) {
    this.cols = Math.ceil(width / CELL);
    this.rows = Math.ceil(height / CELL);
    this.cells = new Float32Array(this.cols * this.rows);
  }

  add(r, weight) {
    const c0 = Math.max(0, Math.floor(r.x / CELL));
    const c1 = Math.min(this.cols - 1, Math.floor((r.x + r.w) / CELL));
    const r0 = Math.max(0, Math.floor(r.y / CELL));
    const r1 = Math.min(this.rows - 1, Math.floor((r.y + r.h) / CELL));
    for (let row = r0; row <= r1; row += 1) {
      const oy = Math.min(r.y + r.h, (row + 1) * CELL) - Math.max(r.y, row * CELL);
      for (let col = c0; col <= c1; col += 1) {
        const ox = Math.min(r.x + r.w, (col + 1) * CELL) - Math.max(r.x, col * CELL);
        if (ox > 0 && oy > 0) this.cells[row * this.cols + col] += (weight * ox * oy) / (CELL * CELL);
      }
    }
  }

  // covered is how much of r is occupied, as a share of its area.
  covered(r) {
    let sum = 0;
    const c0 = Math.max(0, Math.floor(r.x / CELL));
    const c1 = Math.min(this.cols - 1, Math.floor((r.x + r.w - 1) / CELL));
    const r0 = Math.max(0, Math.floor(r.y / CELL));
    const r1 = Math.min(this.rows - 1, Math.floor((r.y + r.h - 1) / CELL));
    for (let row = r0; row <= r1; row += 1) {
      for (let col = c0; col <= c1; col += 1) sum += Math.min(1, this.cells[row * this.cols + col]);
    }
    return sum / Math.max(1, (c1 - c0 + 1) * (r1 - r0 + 1));
  }
}

const centroidOf = (poly) => ({
  x: poly.reduce((s, p) => s + p.x, 0) / poly.length,
  y: poly.reduce((s, p) => s + p.y, 0) / poly.length,
});

// labels: [{ key, outline: [{x, y}], w, h }]; obstacles and avoid: [{ x, y, w, h }] by their top-left corner.
export function placeLabels({ labels, obstacles, avoid = [], view, width, height }) {
  const grid = new Occupancy(width, height);
  obstacles.forEach((r) => grid.add(r, 1));
  avoid.forEach((r) => grid.add(r, 0.5));
  const out = new Map();
  for (const label of labels) {
    const { outline, w, h } = label;
    if (!outline || outline.length < 3) continue;
    const c = centroidOf(outline);
    const onScreen = (x, y) => x - w / 2 >= view.x && y - h / 2 >= view.y && x + w / 2 <= view.x + view.w && y + h / 2 <= view.y + view.h;
    const fits = (x, y) => [[0, 0], [-1, -1], [1, -1], [1, 1], [-1, 1]].every(([dx, dy]) => pointInPolygon(outline, x + (dx * w) / 2, y + (dy * h) / 2));
    const xs = outline.map((p) => p.x);
    const ys = outline.map((p) => p.y);
    const reach = Math.max(w, Math.hypot(Math.max(...xs) - Math.min(...xs), Math.max(...ys) - Math.min(...ys)) / 2);
    let best = null;
    for (const strict of [true, false]) {
      for (let y = Math.min(...ys); y <= Math.max(...ys); y += STEP) {
        for (let x = Math.min(...xs); x <= Math.max(...xs); x += STEP) {
          if (!onScreen(x, y) || !(strict ? fits(x, y) : pointInPolygon(outline, x, y))) continue;
          const score = grid.covered({ x: x - w / 2, y: y - h / 2, w, h }) + (CENTRE_PULL * Math.hypot(x - c.x, y - c.y)) / reach;
          if (!best || score < best.score) best = { x, y, score };
        }
      }
      if (best) break;
    }
    if (!best) best = c;
    out.set(label.key, { x: best.x, y: best.y });
    grid.add({ x: best.x - w / 2, y: best.y - h / 2, w, h }, LABEL_WEIGHT);
  }
  return out;
}

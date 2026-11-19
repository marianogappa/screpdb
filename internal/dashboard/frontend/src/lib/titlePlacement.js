import { Occupancy, pointInPolygon } from './labelPlacement.js';

// A title is placed where the map shows nothing of interest: no buildings, units, owner labels, owned
// base tint or the event's focus. It wraps into up to MAX_LINES lines to fit such a patch; a box covering
// at most GOOD_ENOUGH of anything (covered) is accepted as is.
const MAX_LINES = 4;
const STEP_CSS = 16;
const MARGIN_CSS = 12;
const MIN_BOX_CSS = 220;
const LINE_PENALTY = 0.015;
export const GOOD_ENOUGH = 0.04;

// basesGrid marks owned base polygons, softly: their tint is colour on the map, but less than a unit.
const addPolygon = (grid, poly, weight, cell) => {
  const xs = poly.map((p) => p.x);
  const ys = poly.map((p) => p.y);
  for (let y = Math.min(...ys); y < Math.max(...ys); y += cell) {
    for (let x = Math.min(...xs); x < Math.max(...xs); x += cell) {
      if (pointInPolygon(poly, x + cell / 2, y + cell / 2)) grid.add({ x, y, w: cell, h: cell }, weight);
    }
  }
};

// A title in the lower half is anchored by its bottom edge, so text that wraps to more lines than estimated
// grows away from the frame's bottom edge and the progress bar along it.
export const titleAnchor = (box) => {
  const viewH = box.y + box.h + box.bottom;
  return box.y + box.h / 2 > viewH / 2 ? { bottom: `${box.bottom}px` } : { top: `${box.y}px` };
};

// placeTitle finds the emptiest box for a title of textWidthCss on one line, wrapped to 1..MAX_LINES lines,
// in a view of the map shown at mapPerCss map pixels per CSS pixel. obstacles and avoid are rectangles in
// map pixels; polygons are owned base outlines. The box comes back in CSS pixels from the view's corner.
export function placeTitle({ view, width, height, mapPerCss, textWidthCss, lineHeightCss, padCss, obstacles = [], avoid = [], polygons = [] }) {
  const grid = new Occupancy(width, height);
  obstacles.forEach((r) => grid.add(r, 1));
  avoid.forEach((r) => grid.add(r, 2));
  polygons.forEach((p) => addPolygon(grid, p, 0.35, 32));
  const viewW = view.w / mapPerCss;
  const viewH = view.h / mapPerCss;
  let best = null;
  for (let lines = 1; lines <= MAX_LINES; lines += 1) {
    const w = Math.max(MIN_BOX_CSS, Math.min(viewW - 2 * MARGIN_CSS, (textWidthCss / lines) * 1.12 + 2 * padCss));
    const h = lines * lineHeightCss + 2 * padCss;
    if (h > viewH - 2 * MARGIN_CSS) break;
    for (let y = MARGIN_CSS; y + h <= viewH - MARGIN_CSS; y += STEP_CSS) {
      for (let x = MARGIN_CSS; x + w <= viewW - MARGIN_CSS; x += STEP_CSS) {
        const box = { x: view.x + x * mapPerCss, y: view.y + y * mapPerCss, w: w * mapPerCss, h: h * mapPerCss };
        const covered = grid.covered(box);
        const score = covered + LINE_PENALTY * (lines - 1);
        if (!best || score < best.score) best = { x, y, w, h, lines, score, covered, bottom: viewH - y - h };
      }
    }
  }
  return best;
}

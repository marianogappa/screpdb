import test from 'node:test';
import assert from 'node:assert/strict';
import { placeTitle, GOOD_ENOUGH, titleAnchor } from './titlePlacement.js';

const view = { x: 0, y: 0, w: 1600, h: 1200 };
const base = { view, width: 4096, height: 4096, mapPerCss: 2, lineHeightCss: 36, padCss: 10 };

test('the title goes where the map is empty', () => {
  const busy = { x: 0, y: 0, w: 1600, h: 600 };
  const t = placeTitle({ ...base, textWidthCss: 500, obstacles: [busy] });
  assert.ok(t.covered <= GOOD_ENOUGH);
  assert.ok(t.y * 2 >= 600, `title at y=${t.y * 2} overlaps the busy top half`);
});

test('a long title wraps to fit a narrow empty column', () => {
  const left = { x: 0, y: 0, w: 1100, h: 1200 };
  const t = placeTitle({ ...base, textWidthCss: 700, obstacles: [left] });
  assert.ok(t.lines > 1);
  assert.ok(t.x * 2 >= 1100 && t.covered <= GOOD_ENOUGH);
});

test('a crowded view says so, so the map can zoom out', () => {
  const t = placeTitle({ ...base, textWidthCss: 500, obstacles: [{ x: 0, y: 0, w: 1600, h: 1200 }] });
  assert.ok(t.covered > GOOD_ENOUGH);
});

test('a title in the lower half grows upward, away from the progress bar', () => {
  assert.deepEqual(titleAnchor({ y: 500, h: 80, bottom: 20 }), { bottom: '20px' });
  assert.deepEqual(titleAnchor({ y: 20, h: 80, bottom: 500 }), { top: '20px' });
});

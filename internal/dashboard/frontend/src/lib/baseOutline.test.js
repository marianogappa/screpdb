import test from 'node:test';
import assert from 'node:assert/strict';
import { baseOutline } from './baseOutline.js';

const inside = (poly, x, y) => {
  let c = false;
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i, i += 1) {
    const a = poly[i];
    const b = poly[j];
    if ((a.y > y) !== (b.y > y) && x < ((b.x - a.x) * (y - a.y)) / (b.y - a.y) + a.x) c = !c;
  }
  return c;
};

test('a concave base keeps its notch instead of spiking out to it', () => {
  const l = [{ x: 0, y: 0 }, { x: 1000, y: 0 }, { x: 1000, y: 300 }, { x: 300, y: 300 }, { x: 300, y: 1000 }, { x: 0, y: 1000 }];
  const out = baseOutline(l, 16);
  assert.equal(out.length, 6);
  assert.equal(inside(out, 800, 800), false);
  assert.equal(inside(out, 100, 100), true);
});

test('one-tile staircase steps along an edge are smoothed away', () => {
  const steps = [];
  for (let x = 0; x <= 640; x += 32) steps.push({ x, y: (x / 32) % 2 === 0 ? 0 : 32 });
  const poly = [...steps, { x: 640, y: 640 }, { x: 0, y: 640 }];
  assert.ok(baseOutline(poly, 32).length <= 5);
});

test('repeated vertices do not break the outline', () => {
  const poly = [{ x: 0, y: 0 }, { x: 0, y: 0 }, { x: 500, y: 0 }, { x: 500, y: 500 }, { x: 500, y: 500 }, { x: 0, y: 500 }, { x: 0, y: 0 }];
  assert.deepEqual(baseOutline(poly, 16), [{ x: 0, y: 0 }, { x: 500, y: 0 }, { x: 500, y: 500 }, { x: 0, y: 500 }]);
});

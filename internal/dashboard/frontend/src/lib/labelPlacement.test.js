import test from 'node:test';
import assert from 'node:assert/strict';
import { placeLabels } from './labelPlacement.js';

const square = (x, y, side) => [{ x, y }, { x: x + side, y }, { x: x + side, y: y + side }, { x, y: y + side }];
const view = { x: 0, y: 0, w: 2000, h: 2000 };

test('a label moves off the buildings at the centre of its base, and stays inside it', () => {
  const spots = placeLabels({
    labels: [{ key: 'a', outline: square(0, 0, 640), w: 160, h: 40 }],
    obstacles: [{ x: 160, y: 160, w: 320, h: 320 }],
    view,
    width: 2000,
    height: 2000,
  });
  const { x, y } = spots.get('a');
  const overlaps = x + 80 > 160 && x - 80 < 480 && y + 20 > 160 && y - 20 < 480;
  assert.equal(overlaps, false);
  assert.ok(x - 80 >= 0 && x + 80 <= 640 && y - 20 >= 0 && y + 20 <= 640);
});

test('two labels in one base do not stack, and placement is deterministic', () => {
  const args = {
    labels: [{ key: 'a', outline: square(0, 0, 640), w: 160, h: 40 }, { key: 'b', outline: square(0, 0, 640), w: 160, h: 40 }],
    obstacles: [],
    view,
    width: 2000,
    height: 2000,
  };
  const first = placeLabels(args);
  const [a, b] = [first.get('a'), first.get('b')];
  assert.ok(Math.abs(a.x - b.x) >= 160 || Math.abs(a.y - b.y) >= 40);
  assert.deepEqual(placeLabels(args), first);
});

test('a base off screen keeps its centre', () => {
  const spots = placeLabels({ labels: [{ key: 'a', outline: square(3000, 3000, 400), w: 160, h: 40 }], obstacles: [], view, width: 4096, height: 4096 });
  assert.deepEqual(spots.get('a'), { x: 3200, y: 3200 });
});

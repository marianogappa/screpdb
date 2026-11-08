import test from 'node:test';
import assert from 'node:assert/strict';
import { spreadShares } from './skillRank.js';

test('the worst player sits on the left edge and the best on the right', () => {
  assert.deepEqual(spreadShares([0.4, 3.6, 2.0]), [0, 1, 0.5]);
});

test('players keep their distances between the edges', () => {
  const shares = spreadShares([1, 1.5, 3]);
  assert.deepEqual(shares, [0, 0.25, 1]);
});

test('players who all sit together go to the middle', () => {
  assert.deepEqual(spreadShares([2, 2, 2]), [0.5, 0.5, 0.5]);
  assert.deepEqual(spreadShares([1.3]), [0.5]);
  assert.deepEqual(spreadShares([]), []);
});

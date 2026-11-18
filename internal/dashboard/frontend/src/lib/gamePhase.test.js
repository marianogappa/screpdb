import test from 'node:test';
import assert from 'node:assert/strict';
import { gamePhaseFor, gamePhaseStarts } from './gamePhase.js';

test('a game that ended in the mid game has a mid game and no late game', () => {
  assert.equal(gamePhaseFor(600, 520, 0), 'mid');
  assert.deepEqual(gamePhaseStarts(520, 0), { mid: 520 });
});

test('a game with both boundaries has all three phases', () => {
  assert.deepEqual([100, 600, 900].map((s) => gamePhaseFor(s, 520, 800)), ['early', 'mid', 'late']);
  assert.deepEqual(gamePhaseStarts(520, 800), { mid: 520, late: 800 });
});

test('with only the late boundary the mid game is folded into the early game', () => {
  assert.deepEqual([100, 900].map((s) => gamePhaseFor(s, 0, 800)), ['early', 'late']);
  assert.deepEqual(gamePhaseStarts(0, 800), { late: 800 });
});

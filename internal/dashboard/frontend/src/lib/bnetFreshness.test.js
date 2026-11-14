import test from 'node:test';
import assert from 'node:assert/strict';
import { bnetFreshness } from './bnetFreshness.js';

const now = Date.parse('2026-11-14T12:00:00Z');
const daysAgo = (d) => new Date(now - d * 86400000).toISOString();

test('a profile refreshed this week is fresh', () => {
  assert.equal(bnetFreshness(daysAgo(0), now), 'fresh');
  assert.equal(bnetFreshness(daysAgo(6.9), now), 'fresh');
});

test('a profile a week old is outdated, and two weeks old expired', () => {
  assert.equal(bnetFreshness(daysAgo(7), now), 'outdated');
  assert.equal(bnetFreshness(daysAgo(13.9), now), 'outdated');
  assert.equal(bnetFreshness(daysAgo(14), now), 'expired');
});

test('a profile with no refresh date is not trusted', () => {
  assert.equal(bnetFreshness(undefined, now), 'expired');
  assert.equal(bnetFreshness('garbage', now), 'expired');
});

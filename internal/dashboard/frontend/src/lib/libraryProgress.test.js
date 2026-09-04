import test from 'node:test';
import assert from 'node:assert/strict';

import {
  formatCount,
  formatLoaded,
  formatLoadingShort,
  isLibraryLoading,
  percent,
  phaseLabel,
  statusLabel,
  stillLoadingCopy,
} from './libraryProgress.js';

test('formatLoaded uses thousands separators', () => {
  assert.equal(formatLoaded(1240, 8102), 'Loaded 1,240 of 8,102 replays');
  assert.equal(formatLoaded(0, 0), 'Loaded 0 of 0 replays');
});

test('formatLoaded tolerates missing or garbage counts', () => {
  assert.equal(formatLoaded(undefined, null), 'Loaded 0 of 0 replays');
  assert.equal(formatLoaded('12', 'abc'), 'Loaded 12 of 0 replays');
  assert.equal(formatLoaded(-5, 3.9), 'Loaded 0 of 3 replays');
});

test('formatLoadingShort is the nav badge text', () => {
  // Counts are deliberately absent: the folder is read in one go and telling
  // the page about every batch made it re-render for as long as the read took.
  assert.equal(formatLoadingShort(), 'Loading');
});

test('stillLoadingCopy is the partial-corpus empty state', () => {
  assert.equal(
    stillLoadingCopy(),
    'Still reading your replay folder. This fills in when it finishes.',
  );
});

test('formatCount formats plain numbers', () => {
  assert.equal(formatCount(1000000), '1,000,000');
  assert.equal(formatCount(null), '0');
});

test('phaseLabel maps every wire phase and falls back to Idle', () => {
  assert.equal(phaseLabel('scanning'), 'Scanning');
  assert.equal(phaseLabel('recent'), 'Loading recent games');
  assert.equal(phaseLabel('backfill'), 'Loading older games');
  assert.equal(phaseLabel('ready'), 'Ready');
  assert.equal(phaseLabel('watching'), 'Watching for new replays');
  assert.equal(phaseLabel('failed'), 'Failed');
  assert.equal(phaseLabel('BACKFILL'), 'Loading older games');
  assert.equal(phaseLabel(''), 'Idle');
  assert.equal(phaseLabel(undefined), 'Idle');
  assert.equal(phaseLabel('bogus'), 'Idle');
});

test('statusLabel maps every wire status', () => {
  assert.equal(statusLabel('idle'), 'Idle');
  assert.equal(statusLabel('loading'), 'Loading');
  assert.equal(statusLabel('watching'), 'Watching for new replays');
  assert.equal(statusLabel('failed'), 'Failed');
  assert.equal(statusLabel(null), 'Idle');
});

test('percent is a clamped integer', () => {
  assert.equal(percent(0, 0), 0);
  assert.equal(percent(0, 100), 0);
  assert.equal(percent(50, 100), 50);
  assert.equal(percent(1240, 8102), 15);
  assert.equal(percent(100, 100), 100);
  assert.equal(percent(150, 100), 100);
  assert.equal(percent(1, 3), 33);
  assert.equal(percent(undefined, 10), 0);
});

test('isLibraryLoading is true only for loading', () => {
  assert.equal(isLibraryLoading('loading'), true);
  assert.equal(isLibraryLoading('LOADING'), true);
  assert.equal(isLibraryLoading('watching'), false);
  assert.equal(isLibraryLoading('idle'), false);
  assert.equal(isLibraryLoading('failed'), false);
  assert.equal(isLibraryLoading(undefined), false);
});

test('no dashes in user-facing copy', () => {
  const samples = [
    formatLoaded(1, 2),
    formatLoadingShort(1, 2),
    stillLoadingCopy(1, 2),
    ...['scanning', 'recent', 'backfill', 'ready', 'watching', 'failed', ''].map(phaseLabel),
    ...['idle', 'loading', 'watching', 'failed'].map(statusLabel),
  ];
  for (const s of samples) {
    assert.doesNotMatch(s, /[–—]|\w-\w/, `dash found in: ${s}`);
  }
});

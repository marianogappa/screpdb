import test from 'node:test';
import assert from 'node:assert/strict';

import {
  formatLoadingShort,
  isLibraryLoading,
  stillLoadingCopy,
} from './libraryProgress.js';

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

test('isLibraryLoading only matches the loading status', () => {
  assert.equal(isLibraryLoading('loading'), true);
  assert.equal(isLibraryLoading('LOADING'), true);
  assert.equal(isLibraryLoading('watching'), false);
  assert.equal(isLibraryLoading(undefined), false);
});

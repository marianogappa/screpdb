import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

import {
  formatLoadingShortWith,
  isLibraryLoading,
  stillLoadingCopyWith,
} from './libraryProgress.js';

const catalog = JSON.parse(readFileSync(
  path.join(path.dirname(fileURLToPath(import.meta.url)), '../locales/en/app.json'),
  'utf8',
));
const translate = (key) => catalog[key] ?? key;

test('formatLoadingShortWith is the nav badge text', () => {
  // Counts are deliberately absent: the folder is read in one go and telling
  // the page about every batch made it re-render for as long as the read took.
  assert.equal(formatLoadingShortWith(translate), 'Loading');
});

test('stillLoadingCopyWith is the partial-corpus empty state', () => {
  assert.equal(
    stillLoadingCopyWith(translate),
    'Still reading your replay folder. This fills in when it finishes.',
  );
});

test('isLibraryLoading only matches the loading status', () => {
  assert.equal(isLibraryLoading('loading'), true);
  assert.equal(isLibraryLoading('LOADING'), true);
  assert.equal(isLibraryLoading('watching'), false);
  assert.equal(isLibraryLoading(undefined), false);
});

import test from 'node:test';
import assert from 'node:assert/strict';
import { releaseNotesUrl } from './releaseNotes.js';

test('links to the release on the website', () => {
  assert.equal(releaseNotesUrl('2.6.0'), 'https://marianogappa.github.io/screpdb/#v2.6.0');
  assert.equal(releaseNotesUrl('v2.6.0'), 'https://marianogappa.github.io/screpdb/#v2.6.0');
});

test('falls back to the news section without a version', () => {
  assert.equal(releaseNotesUrl(''), 'https://marianogappa.github.io/screpdb/#news');
});

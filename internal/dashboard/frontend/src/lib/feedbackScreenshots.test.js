import test from 'node:test';
import assert from 'node:assert/strict';
import { base64FromDataURL, scaledSize } from './feedbackScreenshots.js';

test('scaledSize keeps small images and shrinks the long side to the cap', () => {
  assert.deepEqual(scaledSize(1920, 1080), { width: 1920, height: 1080 });
  assert.deepEqual(scaledSize(5120, 2880), { width: 2560, height: 1440 });
  assert.deepEqual(scaledSize(1000, 4000, 2000), { width: 500, height: 2000 });
});

test('base64FromDataURL strips the data URL prefix', () => {
  assert.equal(base64FromDataURL('data:image/png;base64,iVBORw0K'), 'iVBORw0K');
});

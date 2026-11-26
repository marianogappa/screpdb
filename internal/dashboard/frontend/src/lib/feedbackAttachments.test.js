import test from 'node:test';
import assert from 'node:assert/strict';
import { attachmentBytes, base64FromDataURL, isReplayFile, scaledSize } from './feedbackAttachments.js';

test('scaledSize keeps small images and shrinks the long side to the cap', () => {
  assert.deepEqual(scaledSize(1920, 1080), { width: 1920, height: 1080 });
  assert.deepEqual(scaledSize(5120, 2880), { width: 2560, height: 1440 });
  assert.deepEqual(scaledSize(1000, 4000, 2000), { width: 500, height: 2000 });
});

test('base64FromDataURL strips the data URL prefix', () => {
  assert.equal(base64FromDataURL('data:image/png;base64,iVBORw0K'), 'iVBORw0K');
});

test('isReplayFile matches .rep names case-insensitively', () => {
  assert.equal(isReplayFile({ name: 'LastReplay.rep' }), true);
  assert.equal(isReplayFile({ name: 'game.REP' }), true);
  assert.equal(isReplayFile({ name: 'shot.png' }), false);
});

test('attachmentBytes sums screenshots and replays', () => {
  assert.equal(attachmentBytes([{ size: 10 }, { size: 5 }], [{ size: 100 }]), 115);
});

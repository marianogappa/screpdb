import test from 'node:test';
import assert from 'node:assert/strict';
import { crc32, withDpi } from './pngDpi.js';

const chunk = (type, data) => {
  const out = new Uint8Array(12 + data.length);
  const view = new DataView(out.buffer);
  view.setUint32(0, data.length);
  out.set([...type].map((ch) => ch.charCodeAt(0)), 4);
  out.set(data, 8);
  view.setUint32(8 + data.length, crc32(out.subarray(4, 8 + data.length)));
  return out;
};
const concat = (...parts) => {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let at = 0;
  parts.forEach((p) => { out.set(p, at); at += p.length; });
  return out;
};
const SIGNATURE = new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]);
const tinyPng = (...extra) => concat(SIGNATURE, chunk('IHDR', new Uint8Array(13)), ...extra, chunk('IDAT', new Uint8Array([1, 2, 3])), chunk('IEND', new Uint8Array()));

const chunksOf = (png) => {
  const view = new DataView(png.buffer, png.byteOffset, png.byteLength);
  const out = [];
  for (let at = 8; at < png.length;) {
    const length = view.getUint32(at);
    out.push({ type: String.fromCharCode(...png.subarray(at + 4, at + 8)), data: png.subarray(at + 8, at + 8 + length), crc: view.getUint32(at + 8 + length), raw: png.subarray(at + 4, at + 8 + length) });
    at += 12 + length;
  }
  return out;
};

test('crc32 matches the PNG reference value', () => {
  assert.equal(crc32(new TextEncoder().encode('IEND')), 0xae426082);
});

test('a PNG marked at 144 dpi carries one valid pHYs chunk right after IHDR', () => {
  const chunks = chunksOf(withDpi(tinyPng(), 144));
  assert.deepEqual(chunks.map((c) => c.type), ['IHDR', 'pHYs', 'IDAT', 'IEND']);
  const phys = chunks[1];
  const view = new DataView(phys.data.buffer, phys.data.byteOffset, 9);
  assert.equal(view.getUint32(0), 5669);
  assert.equal(view.getUint32(4), 5669);
  assert.equal(phys.data[8], 1);
  assert.equal(phys.crc, crc32(phys.raw));
});

test('a density the PNG already declared is replaced, not doubled', () => {
  const old = chunk('pHYs', new Uint8Array([0, 0, 11, 19, 0, 0, 11, 19, 1]));
  const chunks = chunksOf(withDpi(tinyPng(old), 144));
  assert.equal(chunks.filter((c) => c.type === 'pHYs').length, 1);
});

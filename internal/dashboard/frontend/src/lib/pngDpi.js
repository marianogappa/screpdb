const CRC_TABLE = Array.from({ length: 256 }, (_, n) => {
  let c = n;
  for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  return c >>> 0;
});

export function crc32(bytes) {
  let c = 0xffffffff;
  for (const b of bytes) c = CRC_TABLE[(c ^ b) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

const INCHES_PER_METER = 39.3701;
const IHDR_END = 8 + 4 + 4 + 13 + 4;

// withDpi marks a PNG as drawn at dpi, so image viewers show a picture drawn at twice the pixels at its real size
// rather than twice as large; it replaces any density the PNG already declared.
export function withDpi(png, dpi) {
  const ppm = Math.round(dpi * INCHES_PER_METER);
  const chunk = new Uint8Array(4 + 4 + 9 + 4);
  const view = new DataView(chunk.buffer);
  view.setUint32(0, 9);
  chunk.set([0x70, 0x48, 0x59, 0x73], 4);
  view.setUint32(8, ppm);
  view.setUint32(12, ppm);
  chunk[16] = 1;
  view.setUint32(17, crc32(chunk.subarray(4, 17)));
  const parts = [png.subarray(0, IHDR_END), chunk];
  const source = new DataView(png.buffer, png.byteOffset, png.byteLength);
  for (let at = IHDR_END; at < png.length;) {
    const length = source.getUint32(at);
    const end = at + 12 + length;
    const type = String.fromCharCode(...png.subarray(at + 4, at + 8));
    if (type !== 'pHYs') parts.push(png.subarray(at, end));
    at = end;
  }
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let offset = 0;
  parts.forEach((p) => { out.set(p, offset); offset += p.length; });
  return out;
}

import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import {
  needsFlagFontFallback,
  applyFlagFontFallback,
  FLAG_FONT_FALLBACK_CLASS,
} from './flagFont.js';

const SAMPLE_EMOJI = '\u{1F60A}';
const SAMPLE_FLAG = '\u{1F1E8}\u{1F1ED}';

// A canvas stub that paints the glyphs in colorGlyphs as colour-emoji ink
// (same pixel whatever the fill) and everything else as monochrome ink that
// tracks the fill colour, which is exactly how the real probe tells a native
// flag from Windows drawing the bare letters.
const makeFakeDocument = (colorGlyphs) => {
  const classes = new Set();
  let lastText = null;
  const ctx = {
    textBaseline: '',
    font: '',
    fillStyle: '',
    scale() {},
    clearRect() { lastText = null; },
    fillText(text) { lastText = text; },
    getImageData() {
      let data;
      if (lastText === null) data = [0, 0, 0, 0];
      else if (colorGlyphs.has(lastText)) data = [255, 204, 0, 255];
      else data = ctx.fillStyle === '#fff' ? [255, 255, 255, 255] : [0, 0, 0, 255];
      return { data };
    },
  };
  return {
    documentElement: {
      classList: {
        add(cls) { classes.add(cls); },
        contains(cls) { return classes.has(cls); },
      },
    },
    createElement() {
      return { width: 0, height: 0, getContext: () => ctx };
    },
  };
};

test('windows (colour smiley, monochrome flag) needs the fallback', () => {
  const doc = makeFakeDocument(new Set([SAMPLE_EMOJI]));
  assert.equal(needsFlagFontFallback(doc), true);
});

test('macOS/Linux (colour smiley and flag) does not need the fallback', () => {
  const doc = makeFakeDocument(new Set([SAMPLE_EMOJI, SAMPLE_FLAG]));
  assert.equal(needsFlagFontFallback(doc), false);
});

test('no colour emoji at all (headless) does not need the fallback', () => {
  const doc = makeFakeDocument(new Set());
  assert.equal(needsFlagFontFallback(doc), false);
});

test('applyFlagFontFallback tags <html> only where the fallback is needed', () => {
  const windowsDoc = makeFakeDocument(new Set([SAMPLE_EMOJI]));
  assert.equal(applyFlagFontFallback(windowsDoc), true);
  assert.equal(windowsDoc.documentElement.classList.contains(FLAG_FONT_FALLBACK_CLASS), true);

  const macDoc = makeFakeDocument(new Set([SAMPLE_EMOJI, SAMPLE_FLAG]));
  assert.equal(applyFlagFontFallback(macDoc), false);
  assert.equal(macDoc.documentElement.classList.contains(FLAG_FONT_FALLBACK_CLASS), false);
});

// Drift guard: the probe is useless unless the font ships, the @font-face
// points at it, a rule activates it under the fallback class, and main.jsx
// actually runs the probe.
test('font, @font-face, activation rule and main.jsx wiring all exist', () => {
  const srcDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

  const woff2 = readFileSync(path.join(srcDir, 'assets', 'TwemojiCountryFlags.woff2'));
  assert.ok(woff2.length > 0);
  assert.equal(woff2.subarray(0, 4).toString('latin1'), 'wOF2');

  const css = readFileSync(path.join(srcDir, 'styles.css'), 'utf8');
  const fontFace = css.match(/@font-face\s*{[^}]*Twemoji Country Flags[^}]*}/);
  assert.ok(fontFace, 'styles.css must declare the Twemoji Country Flags @font-face');
  assert.match(fontFace[0], /unicode-range:[^;]*U\+1F1E6-1F1FF/);
  assert.match(fontFace[0], /url\('\.\/assets\/TwemojiCountryFlags\.woff2'\)/);
  assert.match(css, new RegExp(`\\.${FLAG_FONT_FALLBACK_CLASS} \\.country-flag\\s*{[^}]*'Twemoji Country Flags'`));

  const main = readFileSync(path.join(srcDir, 'main.jsx'), 'utf8');
  assert.match(main, /applyFlagFontFallback\(\)/);
});

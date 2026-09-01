// Windows ships no flag glyphs: Segoe UI Emoji covers the regional-indicator
// letters but not the pairs, so 🇦🇷 draws as two lettered boxes ("AR") instead
// of a flag. There is no font-stack fix — no font on a stock Windows install
// has them — so we ship our own (Twemoji Country Flags) and switch to it only
// where the platform is broken, leaving macOS/Linux with their native flags.

const EMOJI_STACK =
  '"Apple Color Emoji","Segoe UI Emoji","Segoe UI Symbol","Noto Color Emoji","EmojiOne Color","Android Emoji",sans-serif';

const SAMPLE_EMOJI = '\u{1F60A}';
const SAMPLE_FLAG = '\u{1F1E8}\u{1F1ED}';

export const FLAG_FONT_FALLBACK_CLASS = 'flag-font-fallback';

const pixelFor = (ctx, text, fillStyle) => {
  ctx.clearRect(0, 0, 100, 100);
  ctx.fillStyle = fillStyle;
  ctx.fillText(text, 0, 0);
  return ctx.getImageData(0, 0, 1, 1).data.join(',');
};

// A colour-emoji glyph paints its own palette, so it looks identical whether
// the fill is black or white. A monochrome fallback (Windows drawing the bare
// regional-indicator letters, or tofu) tracks the fill colour instead.
const rendersAsColorEmoji = (ctx, text) => {
  const onWhite = pixelFor(ctx, text, '#fff');
  const onBlack = pixelFor(ctx, text, '#000');
  return onWhite === onBlack && !onBlack.startsWith('0,0,0,');
};

export function needsFlagFontFallback(doc = globalThis.document) {
  try {
    const canvas = doc.createElement('canvas');
    canvas.width = 1;
    canvas.height = 1;
    const ctx = canvas.getContext('2d', { willReadFrequently: true });
    if (!ctx) return false;
    ctx.textBaseline = 'top';
    ctx.font = `100px ${EMOJI_STACK}`;
    ctx.scale(0.01, 0.01);
    // Gate on the smiley too: a browser that renders no colour emoji at all
    // (headless, a font-less container) would otherwise look like Windows.
    return rendersAsColorEmoji(ctx, SAMPLE_EMOJI) && !rendersAsColorEmoji(ctx, SAMPLE_FLAG);
  } catch {
    return false;
  }
}

export function applyFlagFontFallback(doc = globalThis.document) {
  if (!doc?.documentElement || !needsFlagFontFallback(doc)) return false;
  doc.documentElement.classList.add(FLAG_FONT_FALLBACK_CLASS);
  return true;
}

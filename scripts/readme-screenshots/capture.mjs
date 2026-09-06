// capture.mjs — drive a running screpdb dashboard and write the README shots.
//
// Usage:
//   node capture.mjs --base http://localhost:8123 --out ../../docs/images \
//                    [--locale en] [--width 1680] [--only game-hotkeys,alliances]
import fs from 'node:fs';
import path from 'node:path';
import { chromium } from 'playwright';

import { SHOTS } from './shots.mjs';

const args = new Map();
for (let i = 2; i < process.argv.length; i += 1) {
  const [k, v] = process.argv[i].replace(/^--/, '').split('=');
  args.set(k, v ?? process.argv[++i]);
}
const base = args.get('base') ?? 'http://localhost:8123';
const outDir = path.resolve(args.get('out') ?? 'out');
const locale = args.get('locale') ?? 'en';
const width = Number(args.get('width') ?? 1680);
const height = Number(args.get('height') ?? 1000);
const only = (args.get('only') ?? '').split(',').map((s) => s.trim()).filter(Boolean);
// 1x is deliberate: GitHub renders README images at roughly half this width, so
// a 2x capture only quadruples what the repository carries forever.
const scale = Number(args.get('scale') ?? 1);

const suffix = locale === 'en' ? '' : `.${locale}`;

// /api/games caps a page well below the corpus size, so page through it.
const games = [];
for (let offset = 0; ; ) {
  const page = await fetch(`${base}/api/games?limit=200&offset=${offset}`).then((r) => r.json());
  const items = page.items ?? [];
  games.push(...items);
  offset += items.length;
  if (items.length === 0 || offset >= (page.total ?? games.length)) break;
}
if (games.length === 0) throw new Error(`no games at ${base} — is the corpus still loading?`);

fs.mkdirSync(outDir, { recursive: true });

const browser = await chromium.launch();
const context = await browser.newContext({
  viewport: { width, height },
  deviceScaleFactor: scale,
  locale: locale === 'ko' ? 'ko-KR' : 'en-US',
});
// The dashboard picks its language from this key first, the browser locale
// second; setting both keeps the choice deterministic.
await context.addInitScript((loc) => {
  try { localStorage.setItem('dashboard_locale', loc); } catch { /* blocked storage */ }
}, locale);

const page = await context.newPage();
const failures = [];

// Text matchers are per-locale: the dashboard translates every narrative, so a
// shot that selects an event by its wording needs one phrase per language.
const phrase = (v) => (typeof v === 'string' ? v : v?.[locale] ?? v?.en);

/** Wait until the SPA has finished loading whatever this route asks for. */
async function settle() {
  await page.waitForLoadState('networkidle');
  await page
    .waitForFunction(() => !/Loading|불러오는/.test(document.body.innerText.slice(0, 400)), { timeout: 20_000 })
    .catch(() => {});
  // Charts and the map/hotkey PNGs animate in; give them one paint budget.
  await page.waitForTimeout(1200);
  await page.evaluate(async () => {
    await Promise.all(
      Array.from(document.images)
        .filter((img) => !img.complete)
        .map((img) => new Promise((res) => { img.onload = res; img.onerror = res; })),
    );
  });
  await page.waitForTimeout(400);
}

/**
 * Resolve a shot's framing to a Playwright screenshot call.
 *
 * - `{ selector, nth?, hasText? }`  the element's own box, captured in full
 * - `… + { trimToBottomOf }`        the same box, cut off at an inner element's
 *                                   bottom (panels are grid cells, so they can
 *                                   be much taller than the image inside them)
 * - `{ throughSelector }`           page top down to that element's bottom
 * - `{ height }`                    page top down to a fixed height
 * - omitted                         the viewport
 *
 * The last three forms exist because several surfaces are far taller than a
 * README image should be, and a raw viewport crop tends to slice a card in half.
 */
async function shoot(file, clip = {}) {
  const { selector, nth = 0, hasText, throughSelector, trimToBottomOf } = clip;
  const text = phrase(hasText);
  const at = (sel) => (text ? page.locator(sel, { hasText: text }) : page.locator(sel)).nth(nth);

  if (selector) {
    const el = at(selector);
    await el.waitFor({ state: 'visible', timeout: 15_000 });
    if (!trimToBottomOf) {
      await el.screenshot({ path: file });
      return;
    }
    await el.scrollIntoViewIfNeeded();
    const box = await el.boundingBox();
    const inner = await el.locator(trimToBottomOf).first().boundingBox();
    await page.screenshot({
      path: file,
      clip: {
        x: box.x,
        y: box.y,
        width: box.width,
        height: Math.ceil(inner.y + inner.height - box.y) + 8,
      },
    });
    return;
  }

  if (throughSelector) {
    const el = at(throughSelector);
    await el.waitFor({ state: 'visible', timeout: 15_000 });
    // Measure from the top of the document so the clip is in page coordinates.
    await page.evaluate(() => window.scrollTo(0, 0));
    const box = await el.boundingBox();
    await page.screenshot({
      path: file,
      fullPage: true,
      clip: { x: 0, y: 0, width, height: Math.ceil(box.y + box.height) + 8 },
    });
    return;
  }

  await page.screenshot({
    path: file,
    fullPage: true,
    clip: { x: 0, y: 0, width, height: clip.height ?? height },
  });
}

for (const shot of SHOTS) {
  if (only.length && !only.includes(shot.name)) continue;

  let target = null;
  if (shot.pick) {
    target = shot.pick(games);
    if (!target) {
      failures.push(`${shot.name}: no replay in the corpus matched its picker`);
      continue;
    }
  }

  await page.goto(base + shot.route(target), { waitUntil: 'domcontentloaded' });
  await settle();

  if (shot.click) {
    const { selector, hasText } = shot.click;
    const text = phrase(hasText);
    const el = text
      ? page.locator(selector, { hasText: text }).first()
      : page.locator(selector).first();
    if (await el.count()) {
      await el.click();
      await settle();
    } else {
      failures.push(`${shot.name}: nothing matched the click target ${selector}`);
      continue;
    }
  }

  const file = path.join(outDir, `${shot.name}${suffix}.png`);
  try {
    await shoot(file, typeof shot.clip === 'function' ? shot.clip(target) : shot.clip);
  } catch (err) {
    failures.push(`${shot.name}: ${err.message}`);
    continue;
  }
  const { size } = fs.statSync(file);
  console.log(`${shot.name}${suffix}.png  (${(size / 1024).toFixed(0)} KB)  ${target ? target.players_label : ''}`);
}

await browser.close();

if (failures.length) {
  console.error(`\n${failures.length} shot(s) failed:`);
  failures.forEach((f) => console.error(`  ${f}`));
  process.exit(1);
}

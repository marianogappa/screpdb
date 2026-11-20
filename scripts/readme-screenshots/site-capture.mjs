// site-capture.mjs — the landing page's Game Events loop, in English and Korean.
//
// Usage:
//   node site-capture.mjs --base http://localhost:8123 --harvest <screpharvest/harvest> \
//                         --out ../../site/images [--event "drops 4"] [--locale en] [--still]
//
// The map's glow is a CSS animation. Rather than screen-recording it, every
// animation on the page is paused and stepped to exact times, so the frames
// loop seamlessly and come out as sharp as a screenshot.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { chromium } from 'playwright';

import { aliasesFor, installAliases, loadProHandles } from './anonymize.mjs';

const args = new Map();
for (let i = 2; i < process.argv.length; i += 1) {
  const [k, v] = process.argv[i].replace(/^--/, '').split('=');
  const next = process.argv[i + 1];
  args.set(k, v ?? (next === undefined || next.startsWith('--') ? true : (i += 1, next)));
}
const base = args.get('base') ?? 'http://localhost:8123';
const harvest = args.get('harvest');
const outDir = path.resolve(args.get('out') ?? 'out');
const locales = (args.get('locale') ?? 'en,ko').split(',');
const stillOnly = args.has('still');
const PLAYERS = ['jsa_larva', 'k_bishop'];
const EVENT = { en: args.get('event') ?? 'drops 4', ko: args.get('event-ko') ?? '드랍' };
const FPS = 24;
const LOOP_MS = 2600;
const WIDTH = 1440;
const HEIGHT = 900;
const SCALE = 2;

if (!harvest) throw new Error('--harvest is required: it says which names are progamers');
const pros = loadProHandles(harvest);

const games = [];
for (let offset = 0; ; ) {
  const page = await fetch(`${base}/api/games?limit=200&offset=${offset}`).then((r) => r.json());
  const items = page.items ?? [];
  games.push(...items);
  offset += items.length;
  if (items.length === 0 || offset >= (page.total ?? games.length)) break;
}
const game = games.find((g) => PLAYERS.every((k) => g.players.some((p) => p.player_key === k)));
if (!game) throw new Error(`no game between ${PLAYERS.join(' and ')}`);

fs.mkdirSync(outDir, { recursive: true });
const browser = await chromium.launch();

for (const locale of locales) {
  const context = await browser.newContext({
    viewport: { width: WIDTH, height: HEIGHT },
    deviceScaleFactor: SCALE,
    locale: locale === 'ko' ? 'ko-KR' : 'en-US',
  });
  await context.addInitScript((loc) => {
    try { localStorage.setItem('dashboard_locale', loc); } catch { /* blocked storage */ }
  }, locale);
  await context.addInitScript(installAliases, aliasesFor([game, ...games], pros));
  const page = await context.newPage();
  await page.goto(`${base}/?view=game&replay=${game.replay_id}&gameTab=events`, { waitUntil: 'networkidle' });
  await page.waitForFunction(() => !document.querySelector('.engine-notice--running'), { timeout: 180_000 }).catch(() => {});
  await page.waitForTimeout(1500);

  const row = page.locator('.workflow-event-row', { hasText: EVENT[locale] ?? EVENT.en }).first();
  await row.click();
  await page.waitForTimeout(2500);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.evaluate(() => Promise.all(Array.from(document.images).filter((i) => !i.complete)
    .map((i) => new Promise((res) => { i.onload = res; i.onerror = res; }))));

  const layout = page.locator('.workflow-events-layout').first();
  const suffix = locale === 'en' ? '' : `.${locale}`;
  const stem = path.join(outDir, `game-events-glow${suffix}`);

  if (stillOnly) {
    await layout.screenshot({ path: `${stem}.png` });
    console.log(`${stem}.png`);
    await context.close();
    continue;
  }

  await page.evaluate(() => document.getAnimations().forEach((a) => a.pause()));
  const frames = fs.mkdtempSync(path.join(os.tmpdir(), 'screpdb-glow-'));
  const count = Math.round((LOOP_MS / 1000) * FPS);
  for (let i = 0; i < count; i += 1) {
    const t = (i * LOOP_MS) / count;
    await page.evaluate((ms) => document.getAnimations().forEach((a) => { a.currentTime = ms; }), t);
    await layout.screenshot({ path: path.join(frames, `f${String(i).padStart(3, '0')}.png`), animations: 'allow' });
  }
  const peak = Math.round(count / 2);
  execFileSync('cwebp', ['-quiet', '-q', '82', path.join(frames, `f${String(peak).padStart(3, '0')}.png`), '-o', `${stem}.webp`]);
  execFileSync('ffmpeg', [
    '-y', '-loglevel', 'error', '-framerate', String(FPS), '-i', path.join(frames, 'f%03d.png'),
    '-vf', 'scale=trunc(iw/2)*2:trunc(ih/2)*2', '-c:v', 'libx264', '-pix_fmt', 'yuv420p',
    '-crf', '24', '-preset', 'slow', '-movflags', '+faststart', '-an', `${stem}.mp4`,
  ]);
  fs.rmSync(frames, { recursive: true, force: true });
  for (const ext of ['webp', 'mp4']) {
    console.log(`${stem}.${ext}  (${(fs.statSync(`${stem}.${ext}`).size / 1024).toFixed(0)} KB)`);
  }
  await context.close();
}

await browser.close();

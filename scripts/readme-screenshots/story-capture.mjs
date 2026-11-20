// story-capture.mjs — the landing page's walkthrough: one game, followed tab by tab.
//
// Usage:
//   node story-capture.mjs --base http://localhost:8123 --harvest <screpharvest/harvest> \
//                          --out ../../site/images/story [--locale en,ko] [--only summary,events]
//
// Every step is the same game: JSA_Action (Zerg) against Light on his
// kimsabuho account, the last game of Action's last session in the harvest.
// The capture runs with Action as the user: a CSettings.json naming his
// account sits above the replay folder, and his games live in its Autosave
// folder, which is where the session view reads from.
//
// Staged on purpose: the games are about 100 days old, past the session
// view's 90 day limit, so the capture build raises that limit.
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
const locales = String(args.get('locale') ?? 'en,ko').split(',');
const only = String(args.get('only') ?? '').split(',').filter(Boolean);

const GAME_FILE = 'MM-9E3BC824';
const OPPONENT = 'kimsabuho';
const WIDTH = 1440;
const HEIGHT = 900;
const SCALE = 2;
const FPS = 24;
const LOOP_MS = 2600;

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
const game = games.find((g) => g.file_name.startsWith(GAME_FILE));
if (!game) throw new Error(`no game ${GAME_FILE} in the corpus`);
game.players.forEach((p) => {
  pros.add(p.player_key);
  pros.add(String(p.name).trim().toLowerCase());
});
const gameRoute = (tab) => `/?view=game&replay=${game.replay_id}${tab ? `&gameTab=${tab}` : ''}`;
const rowWith = (text) => ({ selector: '.workflow-event-row', hasText: text });

const STEPS = [
  {
    id: 'list',
    route: '/?view=session',
    clip: { height: 380 },
    highlight: { selector: 'tbody tr', nth: 0 },
  },
  {
    id: 'summary',
    route: gameRoute(''),
    clip: { throughSelector: '.workflow-panel' },
    highlight: { selector: '.workflow-pattern-pill', hasText: '6:34' },
  },
  {
    id: 'muta',
    route: gameRoute('mutalisk-timing'),
    clip: { height: 640 },
    highlight: { svgText: '22' },
  },
  {
    id: 'turning',
    route: gameRoute('events'),
    click: rowWith('10:01'),
    clip: { selector: '.workflow-events-layout' },
    loop: true,
  },
  {
    id: 'build-orders',
    route: gameRoute('build-orders'),
    clip: { height: 640 },
  },
  {
    id: 'skill',
    route: gameRoute('skill'),
    clip: { height: 430 },
  },
  {
    id: 'hotkeys',
    route: gameRoute('hotkeys'),
    clip: { height: 760 },
    highlight: { selector: '.workflow-production-tab-active' },
  },
  {
    id: 'player',
    route: `/?view=player&player=${encodeURIComponent(OPPONENT)}`,
    // The Battle.net panel needs StarCraft running, so offline it is an empty box.
    hide: ['.wps-top-row'],
    clip: { height: 470 },
    highlight: { text: 'Light' },
  },
];

async function settle(page) {
  await page.waitForLoadState('networkidle');
  await page.waitForFunction(() => !/Loading|불러오는/.test(document.body.innerText.slice(0, 400)), { timeout: 20_000 }).catch(() => {});
  await page.waitForFunction(() => !document.querySelector('.engine-notice--running'), { timeout: 180_000 }).catch(() => {});
  await page.waitForTimeout(1500);
  await page.evaluate(() => Promise.all(Array.from(document.images).filter((i) => !i.complete)
    .map((i) => new Promise((res) => { i.onload = res; i.onerror = res; }))));
  await page.waitForTimeout(400);
}

async function clipBox(page, clip) {
  await page.evaluate(() => window.scrollTo(0, 0));
  if (clip.selector) {
    const box = await page.locator(clip.selector).first().boundingBox();
    return { x: box.x, y: box.y, width: box.width, height: box.height };
  }
  if (clip.throughSelector) {
    const box = await page.locator(clip.throughSelector).first().boundingBox();
    return { x: 0, y: 0, width: WIDTH, height: Math.ceil(box.y + box.height) + 8 };
  }
  return { x: 0, y: 0, width: clip.width ?? WIDTH, height: clip.height ?? HEIGHT };
}

async function highlightBox(page, hl, clip) {
  if (!hl) return null;
  if (hl.svgText) {
    const found = await page.evaluate((pattern) => {
      const re = new RegExp(pattern);
      const text = [...document.querySelectorAll('svg text')].find((t) => re.test(t.textContent));
      if (!text) return null;
      const b = text.getBoundingClientRect();
      return { x: b.x, y: b.y, width: b.width, height: b.height };
    }, hl.svgText);
    return found && toFractions(found, clip);
  }
  const locator = hl.text
    ? page.getByText(hl.text, { exact: true })
    : page.locator(hl.selector, hl.hasText ? { hasText: hl.hasText } : {});
  const box = await locator.nth(hl.nth ?? 0).boundingBox();
  if (!box) return null;
  return toFractions(box, clip);
}

function toFractions(box, clip) {
  const pad = 6;
  const r = (n) => Math.round(n * 10000) / 10000;
  return {
    x: r((box.x - pad - clip.x) / clip.width),
    y: r((box.y - pad - clip.y) / clip.height),
    w: r((box.width + pad * 2) / clip.width),
    h: r((box.height + pad * 2) / clip.height),
  };
}

async function shootLoop(page, clip, stem) {
  await page.evaluate(() => document.getAnimations().forEach((a) => a.pause()));
  const frames = fs.mkdtempSync(path.join(os.tmpdir(), 'screpdb-story-'));
  const count = Math.round((LOOP_MS / 1000) * FPS);
  for (let i = 0; i < count; i += 1) {
    await page.evaluate((ms) => document.getAnimations().forEach((a) => { a.currentTime = ms; }), (i * LOOP_MS) / count);
    await page.screenshot({ path: path.join(frames, `f${String(i).padStart(3, '0')}.png`), clip, animations: 'allow' });
  }
  execFileSync('cwebp', ['-quiet', '-q', '80', path.join(frames, `f${String(Math.round(count / 2)).padStart(3, '0')}.png`), '-o', `${stem}.webp`]);
  execFileSync('ffmpeg', [
    '-y', '-loglevel', 'error', '-framerate', String(FPS), '-i', path.join(frames, 'f%03d.png'),
    '-vf', 'scale=trunc(iw/2)*2:trunc(ih/2)*2', '-c:v', 'libx264', '-pix_fmt', 'yuv420p',
    '-crf', '24', '-preset', 'slow', '-movflags', '+faststart', '-an', `${stem}.mp4`,
  ]);
  fs.rmSync(frames, { recursive: true, force: true });
}

fs.mkdirSync(outDir, { recursive: true });
const manifestPath = path.join(outDir, 'story.json');
const manifest = fs.existsSync(manifestPath) ? JSON.parse(fs.readFileSync(manifestPath, 'utf8')) : {};
const browser = await chromium.launch();

for (const locale of locales) {
  for (const step of STEPS) {
    if (only.length && !only.includes(step.id)) continue;
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
    await page.goto(base + step.route, { waitUntil: 'domcontentloaded' });
    if (step.hide) await page.addStyleTag({ content: `${step.hide.join(', ')} { display: none !important; }` });
    await settle(page);
    if (step.click) {
      await page.locator(step.click.selector, { hasText: step.click.hasText }).first().click();
      await page.waitForTimeout(2500);
    }

    const clip = await clipBox(page, step.clip);
    const suffix = locale === 'en' ? '' : `.${locale}`;
    const stem = path.join(outDir, `${step.id}${suffix}`);
    if (step.loop) {
      await shootLoop(page, clip, stem);
    } else {
      const png = `${stem}.png`;
      await page.screenshot({ path: png, clip, fullPage: true });
      execFileSync('cwebp', ['-quiet', '-q', '80', png, '-o', `${stem}.webp`]);
      fs.rmSync(png);
    }
    manifest[`${step.id}${suffix}`] = {
      width: Math.round(clip.width * SCALE),
      height: Math.round(clip.height * SCALE),
      loop: Boolean(step.loop),
      highlight: await highlightBox(page, step.highlight, clip),
    };
    console.log(`${step.id}${suffix}`, JSON.stringify(manifest[`${step.id}${suffix}`]));
    await context.close();
  }
}

fs.writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
await browser.close();

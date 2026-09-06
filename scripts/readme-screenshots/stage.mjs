// stage.mjs — build the deterministic screenshot corpus.
//
// Picks, from a screpharvest harvest, the playable ladder games of a curated
// set of progamer handles (so the dashboard's own fingerprint matcher has the
// 3+ games per handle it needs to name a barcode), tops the folder up with a
// stable spread of other playable games so the game list looks like a real
// library, and adds the fixture replays that the 1v1 ladder corpus cannot
// cover (the bundled example replays, the BGH team-stacking game).
//
// The folder never exceeds --limit, which matters: the loader reads only the
// newest DefaultMaxReplays files, so an oversized folder would silently drop
// whichever fixtures happened to sort last.
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import readline from 'node:readline';

const PLAYABLE_SECONDS = 240;

// Curated handles: progamers whose ladder name is recognisable, or who play
// enough games under one barcode for the fingerprint match to be the point.
export const FEATURED_HANDLES = [
  'JSA_Larva',        // Larva (Z)
  'BY.HERO_',         // herO (Z)
  'mae_TUBE',         // Effort (Z)
  'HM_sSak',          // sSak (T)
  'Black_Scan',       // Scan (T)
  'K_BishOp',         // BishOp (T)
  'MBU_Saber',        // Saber (Z)
  'RoyaL1123',        // RoyaL (T)
  'sAi[WHITE]',       // Sai (Z)
  'kimsabuho',        // Light (T)
  'llIIll1ll1lI',     // Jaedong (Z)
  'lllIIlIlI',        // Best (P)
  'llIlIlIIIIIlII1',  // Soma (Z)
  'lllIIllIIIIllII',  // Queen (Z)
  'IIlllllIIllIII',   // Midas (T)
  'SOMIFA',           // Hyuk (Z)
  'lllIII1llllllll',  // HyuN (Z)
  'llIIIIllIIlIlI',   // Stork (P)
  'iiiiilililli',     // Sharp (T)
  'Ne1ge',            // Rich (P)
];

const args = new Map();
for (let i = 2; i < process.argv.length; i += 1) {
  const [k, v] = process.argv[i].replace(/^--/, '').split('=');
  args.set(k, v ?? process.argv[++i]);
}
const harvest = args.get('harvest');
const out = args.get('out');
const extras = (args.get('extras') ?? '').split(',').map((s) => s.trim()).filter(Boolean);
const limit = Number(args.get('limit') ?? 500);
const perHandle = Number(args.get('per-handle') ?? 20);
if (!harvest || !out) {
  console.error('usage: stage.mjs --harvest <dir> --out <dir> [--extras <dir>,<dir>] [--limit 500] [--per-handle 20]');
  process.exit(1);
}

fs.rmSync(out, { recursive: true, force: true });
fs.mkdirSync(out, { recursive: true });

// Fixtures first: they are the reason several shots exist, so they get the
// budget before the ladder games do.
let fixtures = 0;
for (const dir of extras) {
  if (!fs.existsSync(dir)) continue;
  for (const f of fs.readdirSync(dir)) {
    if (!f.toLowerCase().endsWith('.rep')) continue;
    fs.copyFileSync(path.join(dir, f), path.join(out, f));
    fixtures += 1;
  }
}

const featured = new Set(FEATURED_HANDLES.map((h) => h.toLowerCase()));
const rows = [];
const rl = readline.createInterface({
  input: fs.createReadStream(path.join(harvest, 'replays.jsonl')),
  crlfDelay: Infinity,
});
for await (const line of rl) {
  if (!line.trim()) continue;
  const r = JSON.parse(line);
  if ((r.duration ?? 0) < PLAYABLE_SECONDS) continue;
  if (!r.file) continue;
  rows.push(r);
}

const featuredHandlesOf = (r) =>
  [r.toon, r.oppToon]
    .map((t) => String(t || '').toLowerCase())
    .filter((t) => featured.has(t));
const isFeatured = (r) => featuredHandlesOf(r).length > 0;

// Deterministic order: featured games first, then a hash-shuffled spread of the
// rest so the top-up is stable but not biased to one map or one week.
const stableKey = (r) => createHash('sha256').update(r.matchId).digest('hex');
const budget = Math.max(0, limit - fixtures);
const picked = [];
const seen = new Set();
const push = (r) => {
  if (seen.has(r.matchId)) return;
  seen.add(r.matchId);
  picked.push(r);
};

// Per-handle cap: the fingerprint matcher needs 3+ games per handle, and a cap
// keeps one prolific pro from crowding the folder out.
const perHandleTaken = new Map();
rows
  .filter(isFeatured)
  .sort((a, b) => a.matchId.localeCompare(b.matchId))
  .forEach((r) => {
    if (picked.length >= budget) return;
    const handles = featuredHandlesOf(r);
    if (handles.every((h) => (perHandleTaken.get(h) ?? 0) >= perHandle)) return;
    handles.forEach((h) => perHandleTaken.set(h, (perHandleTaken.get(h) ?? 0) + 1));
    push(r);
  });
const featuredCount = picked.length;

// Top up after dedupe, not before: a match appears in the manifest once per
// owning handle, so slicing the rows first would under-fill the folder.
for (const r of rows
  .filter((x) => !isFeatured(x))
  .sort((a, b) => stableKey(a).localeCompare(stableKey(b)))) {
  if (picked.length >= budget) break;
  push(r);
}

let copied = 0;
let missing = 0;
for (const r of picked) {
  const src = path.join(harvest, r.file);
  if (!fs.existsSync(src)) { missing += 1; continue; }
  fs.copyFileSync(src, path.join(out, path.basename(r.file)));
  copied += 1;
}

console.log(
  `staged ${fixtures} fixture replays + ${copied} harvest replays ` +
  `(${featuredCount} featured, ${missing} missing) -> ${out}`,
);

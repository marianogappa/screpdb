// anonymize.mjs — keep progamer names, alias everyone else.
//
// The harvest's own metadata says which ladder handles belong to progamers
// (cwal's proName, per match and per identity). Every other in-replay name is
// a private player's, so it is replaced on screen before the shot is taken.
import fs from 'node:fs';
import path from 'node:path';

export function loadProHandles(harvest) {
  const pros = new Set();
  const add = (handle, proName) => {
    if (handle && proName) pros.add(String(handle).toLowerCase());
  };
  const lines = (file) => {
    const p = path.join(harvest, file);
    return fs.existsSync(p) ? fs.readFileSync(p, 'utf8').split('\n').filter((l) => l.trim()) : [];
  };
  for (const l of lines('replays.jsonl')) {
    const r = JSON.parse(l);
    add(r.toon, r.proName);
    add(r.oppToon, r.oppProName);
  }
  for (const l of lines('identities.jsonl')) {
    for (const h of JSON.parse(l).handles ?? []) add(h.toon, h.proName);
  }
  return pros;
}

// Made-up ladder handles: invented syllables and decorations, so an alias reads
// like a real name without being anyone's.
const HEADS = ['Kro', 'Vel', 'Mor', 'Tas', 'Zer', 'Gol', 'Rax', 'Nex', 'Dru', 'Pyx', 'Vul', 'Sko', 'Brin', 'Qel', 'Fay', 'Hok', 'Jun', 'Lio', 'Orb', 'Wen'];
const TAILS = ['gath', 'nix', 'vor', 'dusk', 'mok', 'ril', 'zan', 'tek', 'ova', 'lim', 'quor', 'bex', 'sar', 'dun', 'iel', 'pha'];
const DECOR = [(n) => n, (n) => `${n}99`, (n) => `x${n}x`, (n) => `${n}_bw`, (n) => n.toLowerCase(), (n) => `${n}07`, (n) => `[${n.slice(0, 3).toUpperCase()}]${n}`, (n) => n.toUpperCase()];

function* fakeHandles() {
  const total = HEADS.length * TAILS.length * DECOR.length;
  // A stride coprime with the total walks every combination once, in an order
  // that does not read as alphabetical.
  const stride = 7919;
  for (let i = 0; i < total; i += 1) {
    const k = (i * stride) % total;
    const name = HEADS[k % HEADS.length] + TAILS[Math.floor(k / HEADS.length) % TAILS.length];
    yield DECOR[Math.floor(k / (HEADS.length * TAILS.length))](name);
  }
}

/** Aliases ordered by first appearance, so the shot's own players get the first handles. */
export function aliasesFor(games, pros) {
  const real = new Set();
  for (const g of games) for (const p of g.players ?? []) real.add(String(p.name ?? '').trim().toLowerCase());
  // An alias that contains a real name, or is a prefix of one, would be
  // aliased again by the page's own rewrite.
  const clashes = (alias) => {
    const a = alias.toLowerCase();
    for (const r of real) if (r && (r.startsWith(a) || (r.length >= 3 && a.includes(r)))) return true;
    return false;
  };
  const aliases = {};
  const keep = new Set();
  const handles = fakeHandles();
  for (const g of games) {
    for (const p of g.players ?? []) {
      const name = String(p.name ?? '').trim();
      if (pros.has(name.toLowerCase())) keep.add(name);
      if (!name || pros.has(name.toLowerCase()) || name in aliases) continue;
      let next = handles.next().value;
      while (next && clashes(next)) next = handles.next().value;
      if (!next) throw new Error('ran out of made-up handles');
      aliases[name] = next;
    }
  }
  return { aliases, keep: [...keep] };
}

/** Runs in the page: rewrite every text node and tooltip, now and on each render. */
export function installAliases({ aliases, keep }) {
  const kept = new Set(keep);
  const names = Object.keys(aliases).sort((a, b) => b.length - a.length);
  if (!names.length) return;
  const escape = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const re = new RegExp(`(?<![\\w])(${names.map(escape).join('|')})(?![\\w])`, 'g');
  // Narrow chips cut a long name short with CSS or a trailing ellipsis, which
  // the full-name pattern misses; a node holding only a long-enough prefix of
  // one name is that name.
  const prefixOf = (s) => {
    const t = s.trim().replace(/(…|\.\.\.)$/, '');
    if (t.length < 6 || kept.has(t)) return null;
    const hits = names.filter((n) => n.length > t.length && n.startsWith(t));
    return hits.length === 1 ? aliases[hits[0]] : null;
  };
  const swap = (s) => (s ? prefixOf(s) ?? s.replace(re, (m) => aliases[m]) : s);

  const fix = (root) => {
    if (root.nodeType === Node.TEXT_NODE) {
      const next = swap(root.nodeValue);
      if (next !== root.nodeValue) root.nodeValue = next;
      return;
    }
    if (root.nodeType !== Node.ELEMENT_NODE) return;
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT);
    for (let node = root; node; node = walker.nextNode()) {
      if (node.nodeType === Node.TEXT_NODE) {
        const next = swap(node.nodeValue);
        if (next !== node.nodeValue) node.nodeValue = next;
      } else {
        for (const attr of ['title', 'aria-label', 'alt', 'placeholder']) {
          const v = node.getAttribute(attr);
          const next = swap(v);
          if (v && next !== v) node.setAttribute(attr, next);
        }
      }
    }
  };

  const start = () => {
    fix(document.body);
    new MutationObserver((records) => {
      for (const r of records) {
        if (r.type === 'characterData') fix(r.target);
        else r.addedNodes.forEach(fix);
      }
    }).observe(document.body, { childList: true, characterData: true, subtree: true });
  };
  if (document.body) start();
  else document.addEventListener('DOMContentLoaded', start);
}

import test from 'node:test';
import assert from 'node:assert/strict';

import { computeColumnLayout } from './allianceLayout.js';

// Reference implementation: the original exhaustive search, kept verbatim so
// the fast path stays pinned to the layouts it replaced.
function referenceLayout(rows, pidList) {
  const computeRowOrder = (row, prev) => {
    const remaining = row.teams.map((_, i) => i);
    const placed = new Set();
    const newOrderActive = [];
    const placeMembers = (team) => {
      const toPlace = team.filter((pid) => !placed.has(pid));
      const retained = toPlace
        .filter((pid) => prev.includes(pid))
        .sort((a, b) => prev.indexOf(a) - prev.indexOf(b));
      const newcomers = toPlace
        .filter((pid) => !prev.includes(pid))
        .sort((a, b) => a - b);
      for (const pid of [...retained, ...newcomers]) {
        newOrderActive.push(pid);
        placed.add(pid);
      }
    };
    while (remaining.length > 0) {
      let bestIdx = 0;
      let bestKey = null;
      for (let i = 0; i < remaining.length; i += 1) {
        const ti = remaining[i];
        const team = row.teams[ti];
        const overlap = team.reduce((acc, pid) => acc + (placed.has(pid) ? 1 : 0), 0);
        const idxs = team.map((pid) => prev.indexOf(pid)).filter((v) => v >= 0);
        const centroid = idxs.length === 0
          ? Number.MAX_SAFE_INTEGER
          : idxs.reduce((a, v) => a + v, 0) / idxs.length;
        const key = [overlap, -centroid, team.length, -ti];
        if (
          bestKey === null
          || key[0] > bestKey[0]
          || (key[0] === bestKey[0] && key[1] > bestKey[1])
          || (key[0] === bestKey[0] && key[1] === bestKey[1] && key[2] > bestKey[2])
          || (key[0] === bestKey[0] && key[1] === bestKey[1] && key[2] === bestKey[2] && key[3] > bestKey[3])
        ) {
          bestKey = key;
          bestIdx = i;
        }
      }
      const ti = remaining[bestIdx];
      remaining.splice(bestIdx, 1);
      placeMembers(row.teams[ti]);
    }
    const newOrder = newOrderActive.slice();
    for (const pid of row.departures) {
      if (newOrder.includes(pid)) continue;
      const prevIdx = prev.indexOf(pid);
      if (prevIdx < 0) { newOrder.push(pid); continue; }
      newOrder.splice(Math.min(prevIdx, newOrder.length), 0, pid);
    }
    return newOrder;
  };

  const simulateOrder = (initialOrder) => {
    const cols = [];
    let movement = 0;
    let arcLen = 0;
    let prev = initialOrder.slice();
    for (let ri = 0; ri < rows.length; ri += 1) {
      const row = rows[ri];
      const newOrder = computeRowOrder(row, prev);
      cols.push(newOrder);
      for (let i = 0; i < newOrder.length; i += 1) {
        const prevIdx = prev.indexOf(newOrder[i]);
        if (prevIdx >= 0) movement += Math.abs(i - prevIdx);
      }
      for (const team of row.teams) {
        if (team.length === 2) {
          const c1 = newOrder.indexOf(team[0]);
          const c2 = newOrder.indexOf(team[1]);
          if (c1 >= 0 && c2 >= 0) arcLen += Math.abs(c1 - c2) - 1;
        }
      }
      prev = newOrder.filter((pid) => !row.departures.includes(pid));
    }
    return { cols, score: movement * 2 + arcLen };
  };

  if (rows.length === 0) return { columns: [], initialOrder: [] };
  const isPermaSolo = (pid) => {
    for (const row of rows) {
      for (const team of row.teams) {
        if (team.length >= 2 && team.includes(pid)) return false;
      }
    }
    return true;
  };
  const permaSolos = pidList.filter(isPermaSolo);
  const nonSolos = pidList.filter((pid) => !permaSolos.includes(pid));
  if (nonSolos.length > 8) {
    const fallback = [...nonSolos, ...permaSolos];
    return { columns: simulateOrder(fallback).cols, initialOrder: fallback };
  }
  const permute = (arr) => {
    if (arr.length <= 1) return [arr];
    const out = [];
    for (let i = 0; i < arr.length; i += 1) {
      const rest = arr.slice(0, i).concat(arr.slice(i + 1));
      for (const p of permute(rest)) out.push([arr[i], ...p]);
    }
    return out;
  };
  let bestOrder = [...nonSolos, ...permaSolos];
  let bestCols = null;
  let bestScore = Infinity;
  for (const perm of permute(nonSolos)) {
    const candidate = [...perm, ...permaSolos];
    const { cols, score } = simulateOrder(candidate);
    if (score < bestScore) { bestScore = score; bestOrder = candidate; bestCols = cols; }
  }
  return { columns: bestCols || [], initialOrder: bestOrder };
}

let seed = 20240607;
const rnd = () => { seed = (seed * 1664525 + 1013904223) >>> 0; return seed / 4294967296; };
const randInt = (n) => Math.floor(rnd() * n);

// Player ids mimic DB ids: not contiguous, not zero-based, not sorted in
// pidList — the layout maps them onto dense ordinals internally.
function makePids(n) {
  const pool = [];
  let v = 100 + randInt(50);
  for (let i = 0; i < n; i += 1) { pool.push(v); v += 1 + randInt(7); }
  for (let i = pool.length - 1; i > 0; i -= 1) {
    const j = randInt(i + 1);
    [pool[i], pool[j]] = [pool[j], pool[i]];
  }
  return pool;
}

function makeRows(pids, nRows, mode) {
  const rows = [];
  const leftAt = new Map();
  if (mode !== 'partition') {
    for (const p of pids) if (rnd() < 0.35) leftAt.set(p, 1 + randInt(Math.max(1, nRows - 1)));
  }
  for (let r = 0; r < nRows; r += 1) {
    const alive = pids.filter((p) => !leftAt.has(p) || leftAt.get(p) >= r);
    let teams = [];
    if (mode === 'overlapping' && rnd() < 0.5) {
      // Non-transitive alliances: a player can sit in more than one clique.
      for (let i = 0, k = 1 + randInt(3); i < k; i += 1) {
        const pool = alive.slice();
        const size = 1 + randInt(Math.min(4, Math.max(1, alive.length)));
        const team = [];
        for (let s = 0; s < size && pool.length; s += 1) team.push(pool.splice(randInt(pool.length), 1)[0]);
        if (team.length) teams.push(team);
      }
      for (const p of alive) if (!teams.some((t) => t.includes(p)) && rnd() < 0.7) teams.push([p]);
    } else {
      const pool = alive.slice();
      while (pool.length) {
        const size = Math.min(pool.length, 1 + (rnd() < 0.45 ? 1 + randInt(3) : 0));
        const team = [];
        for (let s = 0; s < size; s += 1) team.push(pool.splice(randInt(pool.length), 1)[0]);
        teams.push(team);
      }
    }
    teams = teams.filter((t) => t.length > 0);
    rows.push({ sec: r * 30, teams, departures: pids.filter((p) => leftAt.get(p) === r) });
  }
  return rows;
}

test('matches the exhaustive reference layout across generated topologies', () => {
  const modes = ['partition', 'departures', 'overlapping'];
  for (let iter = 0; iter < 1500; iter += 1) {
    const pids = makePids(2 + randInt(5));
    const rows = makeRows(pids, 1 + randInt(10), modes[randInt(modes.length)]);
    const got = computeColumnLayout(rows, pids);
    const want = referenceLayout(rows, pids);
    assert.deepEqual(got.columns, want.columns, `columns for ${JSON.stringify({ pids, rows })}`);
    assert.deepEqual(got.initialOrder, want.initialOrder, `initialOrder for ${JSON.stringify({ pids, rows })}`);
  }
});

test('matches the reference for full 8-player melee topologies', () => {
  for (let iter = 0; iter < 3; iter += 1) {
    const pids = makePids(8);
    const rows = makeRows(pids, 4 + randInt(6), 'departures');
    const got = computeColumnLayout(rows, pids);
    const want = referenceLayout(rows, pids);
    assert.deepEqual(got.columns, want.columns);
    assert.deepEqual(got.initialOrder, want.initialOrder);
  }
});

test('falls back to natural order above the search limit', () => {
  const pids = makePids(10);
  const rows = makeRows(pids, 5, 'partition');
  const got = computeColumnLayout(rows, pids);
  const want = referenceLayout(rows, pids);
  assert.deepEqual(got.columns, want.columns);
  assert.deepEqual(got.initialOrder, want.initialOrder);
});

test('returns empty layout for an empty timeline', () => {
  assert.deepEqual(computeColumnLayout([], [1, 2, 3]), { columns: [], initialOrder: [] });
});

test('keeps an allied pair adjacent rather than split across a solo', () => {
  const rows = [
    { sec: 0, teams: [[10], [20], [30]], departures: [] },
    { sec: 60, teams: [[10, 30], [20]], departures: [] },
  ];
  const { columns } = computeColumnLayout(rows, [10, 20, 30]);
  const last = columns[columns.length - 1];
  assert.equal(Math.abs(last.indexOf(10) - last.indexOf(30)), 1);
});

test('a full 8-player search stays well under a second', () => {
  const pids = makePids(8);
  const rows = makeRows(pids, 12, 'partition');
  computeColumnLayout(rows, pids);
  const started = performance.now();
  computeColumnLayout(rows, pids);
  assert.ok(performance.now() - started < 1000, 'layout search should not block the UI');
});

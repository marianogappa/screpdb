// Column layout for the alliance timeline.
//
// Each row of the timeline needs a left-to-right ordering of the player lanes.
// A layout is scored by how much it makes lanes jump between rows (movement)
// and how far apart allied pairs sit (arc length); the best-scoring initial
// ordering wins. Scoring is unchanged from the original implementation — this
// module only changes how fast the search gets to the same answer.
//
// The search space is every permutation of the non-solo players, so the naive
// version re-simulated the whole timeline 8! = 40,320 times. Two things keep
// that affordable here:
//
//   - The score from row `ri` onwards depends only on (ri, current order), so
//     suffix scores are memoised. Distinct initial orders collapse onto shared
//     states within a row or two, which makes the cost flat in row count.
//   - The inner loop works on dense player ordinals in preallocated typed
//     arrays, so a candidate ordering costs no allocations.

// placeRow is the per-row placement rule: repeatedly take the unplaced clique
// that best extends the layout so far, then append its members. Priority is
// overlap with what's placed, then the clique's previous-row centroid, then
// clique size, then original clique index. Keeping centroid ahead of size
// stops a newly-formed larger clique from hijacking column 0 from the pair
// already anchored there. Members already in the previous row keep their
// relative order; newcomers sort by player id. Departing players are then
// re-inserted at their last-known column so their terminating "x" lands where
// the lane was.
//
// The function is written against scratch buffers owned by createLayoutEngine
// rather than returning fresh arrays, because it runs in the search's hot loop.

const MAX_SAFE = Number.MAX_SAFE_INTEGER;

// Ranks two cliques when neither can overlap what is already placed: previous
// row centroid first, then clique size, then original clique index. Every
// comparison is decided by the index at the latest, so the order is total.
const betterTeam = (a, b, teams, centroid) => {
  if (centroid[a] !== centroid[b]) return centroid[a] > centroid[b];
  if (teams[a].length !== teams[b].length) return teams[a].length > teams[b].length;
  return a < b;
};

const createLayoutEngine = (oRows, n, maxTeams) => {
  const prevPos = new Int32Array(n);
  const teamCentroid = new Float64Array(maxTeams);
  const newPos = new Int32Array(n);
  const placed = new Uint8Array(n);
  const remaining = new Int32Array(maxTeams);
  const retained = new Int32Array(n);
  const newcomers = new Int32Array(n);
  const out = new Int32Array(n);

  const state = { out, score: 0, len: 0 };
  let outLen = 0;

  // Appends a clique's not-yet-placed members: those already on the previous
  // row keep their relative order, then newcomers by player id. Ordinals are
  // assigned in ascending player-id order, so sorting by ordinal is the same
  // as sorting by player id.
  const placeTeam = (team) => {
    let retCount = 0;
    let newCount = 0;
    for (let k = 0; k < team.length; k += 1) {
      const pid = team[k];
      if (placed[pid]) continue;
      if (prevPos[pid] >= 0) retained[retCount++] = pid; else newcomers[newCount++] = pid;
    }
    for (let a = 1; a < retCount; a += 1) {
      const v = retained[a];
      const kv = prevPos[v];
      let b = a - 1;
      while (b >= 0 && prevPos[retained[b]] > kv) { retained[b + 1] = retained[b]; b -= 1; }
      retained[b + 1] = v;
    }
    for (let a = 1; a < newCount; a += 1) {
      const v = newcomers[a];
      let b = a - 1;
      while (b >= 0 && newcomers[b] > v) { newcomers[b + 1] = newcomers[b]; b -= 1; }
      newcomers[b + 1] = v;
    }
    for (let a = 0; a < retCount; a += 1) { out[outLen++] = retained[a]; placed[retained[a]] = 1; }
    for (let a = 0; a < newCount; a += 1) { out[outLen++] = newcomers[a]; placed[newcomers[a]] = 1; }
  };

  state.step = (ri, prev, prevLen) => {
    const row = oRows[ri];
    const teams = row.teams;
    prevPos.fill(-1);
    for (let i = 0; i < prevLen; i += 1) prevPos[prev[i]] = i;
    placed.fill(0);
    outLen = 0;

    let remCount = teams.length;
    for (let i = 0; i < remCount; i += 1) remaining[i] = i;

    // When no player sits in two cliques, no clique can ever overlap what is
    // already placed, so the selection key is fixed up front and the whole
    // placement order is one sort instead of a rescan per pick.
    if (row.disjoint) {
      for (let i = 0; i < remCount; i += 1) {
        const team = teams[i];
        let sum = 0;
        let cnt = 0;
        for (let k = 0; k < team.length; k += 1) {
          const p = prevPos[team[k]];
          if (p >= 0) { sum += p; cnt += 1; }
        }
        teamCentroid[i] = cnt === 0 ? -MAX_SAFE : -(sum / cnt);
      }
      for (let a = 1; a < remCount; a += 1) {
        const v = remaining[a];
        let b = a - 1;
        while (b >= 0 && !betterTeam(remaining[b], v, teams, teamCentroid)) {
          remaining[b + 1] = remaining[b];
          b -= 1;
        }
        remaining[b + 1] = v;
      }
      for (let i = 0; i < remCount; i += 1) placeTeam(teams[remaining[i]]);
      remCount = 0;
    }

    while (remCount > 0) {
      let bestIdx = 0;
      let bOverlap = 0;
      let bNegCentroid = 0;
      let bSize = 0;
      let bNegTi = 0;
      let has = false;
      for (let i = 0; i < remCount; i += 1) {
        const ti = remaining[i];
        const team = teams[ti];
        let overlap = 0;
        let sum = 0;
        let cnt = 0;
        for (let k = 0; k < team.length; k += 1) {
          const pid = team[k];
          if (placed[pid]) overlap += 1;
          const p = prevPos[pid];
          if (p >= 0) { sum += p; cnt += 1; }
        }
        const negCentroid = cnt === 0 ? -MAX_SAFE : -(sum / cnt);
        const size = team.length;
        const negTi = -ti;
        if (
          !has
          || overlap > bOverlap
          || (overlap === bOverlap && negCentroid > bNegCentroid)
          || (overlap === bOverlap && negCentroid === bNegCentroid && size > bSize)
          || (overlap === bOverlap && negCentroid === bNegCentroid && size === bSize && negTi > bNegTi)
        ) {
          has = true;
          bOverlap = overlap;
          bNegCentroid = negCentroid;
          bSize = size;
          bNegTi = negTi;
          bestIdx = i;
        }
      }
      const ti = remaining[bestIdx];
      for (let i = bestIdx; i < remCount - 1; i += 1) remaining[i] = remaining[i + 1];
      remCount -= 1;

      placeTeam(teams[ti]);
    }

    const deps = row.departures;
    for (let d = 0; d < deps.length; d += 1) {
      const pid = deps[d];
      let found = false;
      for (let i = 0; i < outLen; i += 1) if (out[i] === pid) { found = true; break; }
      if (found) continue;
      const prevIdx = prevPos[pid];
      if (prevIdx < 0) { out[outLen++] = pid; continue; }
      const target = Math.min(prevIdx, outLen);
      for (let i = outLen; i > target; i -= 1) out[i] = out[i - 1];
      out[target] = pid;
      outLen += 1;
    }

    let movement = 0;
    for (let i = 0; i < outLen; i += 1) {
      const p = prevPos[out[i]];
      if (p >= 0) movement += Math.abs(i - p);
    }
    newPos.fill(-1);
    for (let i = 0; i < outLen; i += 1) newPos[out[i]] = i;
    let arcLen = 0;
    for (let t = 0; t < teams.length; t += 1) {
      const team = teams[t];
      if (team.length !== 2) continue;
      const c1 = newPos[team[0]];
      const c2 = newPos[team[1]];
      if (c1 >= 0 && c2 >= 0) arcLen += Math.abs(c1 - c2) - 1;
    }
    state.score = movement * 2 + arcLen;
    state.len = outLen;
  };

  return state;
};

// computeColumnLayout returns the per-row column orders plus the winning
// initial order.
//
//   rows:    [{ teams: [[player_id, ...], ...], departures: [player_id, ...] }]
//   pidList: player ids in display order
//
// Players who never appear in a size-≥2 clique anywhere ("permanent solos")
// are pinned to the end of the lineup: their position cannot affect the score,
// so leaving them in the search would let an arbitrary tied permutation drop
// them into the middle of an otherwise stable lineup.
export function computeColumnLayout(rows, pidList) {
  if (!Array.isArray(rows) || rows.length === 0) return { columns: [], initialOrder: [] };

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

  // Dense ordinals, assigned in ascending player-id order (see the newcomer
  // sort above, which relies on that).
  const seen = new Set(pidList);
  for (const row of rows) {
    for (const team of row.teams) for (const pid of team) seen.add(pid);
    for (const pid of row.departures) seen.add(pid);
  }
  const ordToPid = Array.from(seen).sort((a, b) => a - b);
  const n = ordToPid.length;
  const ordOf = new Map();
  ordToPid.forEach((pid, i) => ordOf.set(pid, i));

  const nRows = rows.length;
  const oRows = rows.map((row) => {
    const departSet = new Uint8Array(n);
    for (const pid of row.departures) departSet[ordOf.get(pid)] = 1;
    const teams = row.teams.map((team) => Int32Array.from(team, (pid) => ordOf.get(pid)));
    const membership = new Uint8Array(n);
    let disjoint = true;
    for (const team of teams) {
      for (let i = 0; i < team.length; i += 1) {
        if (membership[team[i]]) { disjoint = false; break; }
        membership[team[i]] = 1;
      }
      if (!disjoint) break;
    }
    return {
      teams,
      departures: Int32Array.from(row.departures, (pid) => ordOf.get(pid)),
      departSet,
      disjoint,
    };
  });
  let maxTeams = 1;
  for (const row of oRows) maxTeams = Math.max(maxTeams, row.teams.length);

  const engine = createLayoutEngine(oRows, n, maxTeams);
  const { out } = engine;

  // One reusable order buffer per row depth, plus depth 0 for the seed.
  const depthBuf = [];
  for (let i = 0; i <= nRows; i += 1) depthBuf.push(new Int32Array(n));
  const seedBuf = depthBuf[0];

  const advance = (ri, depth) => {
    const buf = depthBuf[depth + 1];
    const { departSet } = oRows[ri];
    let len = 0;
    for (let i = 0; i < engine.len; i += 1) if (!departSet[out[i]]) buf[len++] = out[i];
    return len;
  };

  // Memo keys pack the order into one number (base n+1, one digit per lane)
  // when that stays inside the safe-integer range; otherwise fall back to a
  // string key.
  const base = n + 1;
  const numericKeys = base ** n <= Number.MAX_SAFE_INTEGER;
  const keyOf = numericKeys
    ? (arr, len) => { let k = 0; for (let i = 0; i < len; i += 1) k = k * base + (arr[i] + 1); return k; }
    : (arr, len) => { let s = ''; for (let i = 0; i < len; i += 1) s += `${arr[i]},`; return s; };

  const memo = [];
  for (let i = 0; i < nRows; i += 1) memo.push(new Map());
  const pathRi = new Int32Array(nRows);
  const pathKey = new Array(nRows);
  const pathScore = new Int32Array(nRows);

  const scoreOf = (seedLen) => {
    let depth = 0;
    let cur = seedBuf;
    let curLen = seedLen;
    // Depth 0 is never memoised: every permutation seeds a distinct order, so
    // an entry there could never be hit and would only cost a map write.
    let key = 0;
    let tail = 0;
    let pathLen = 0;
    while (depth < nRows) {
      const hit = depth === 0 ? undefined : memo[depth].get(key);
      if (hit !== undefined) { tail = hit; break; }
      engine.step(depth, cur, curLen);
      pathRi[pathLen] = depth;
      pathKey[pathLen] = key;
      pathScore[pathLen] = engine.score;
      pathLen += 1;
      curLen = advance(depth, depth);
      cur = depthBuf[depth + 1];
      key = keyOf(cur, curLen);
      depth += 1;
    }
    let s = tail;
    for (let i = pathLen - 1; i >= 0; i -= 1) {
      s += pathScore[i];
      if (pathRi[i] > 0) memo[pathRi[i]].set(pathKey[i], s);
    }
    return s;
  };

  const columnsFor = (seedLen) => {
    const cols = [];
    let cur = seedBuf;
    let curLen = seedLen;
    for (let ri = 0; ri < nRows; ri += 1) {
      engine.step(ri, cur, curLen);
      const col = new Array(engine.len);
      for (let i = 0; i < engine.len; i += 1) col[i] = ordToPid[out[i]];
      cols.push(col);
      curLen = advance(ri, ri);
      cur = depthBuf[ri + 1];
    }
    return cols;
  };

  // Above this many searchable players the permutation count stops being
  // worth it; fall back to the natural order.
  if (nonSolos.length > 8) {
    const fallback = [...nonSolos, ...permaSolos];
    for (let i = 0; i < fallback.length; i += 1) seedBuf[i] = ordOf.get(fallback[i]);
    return { columns: columnsFor(fallback.length), initialOrder: fallback };
  }

  // Permutations are generated by index in factorial base, which visits them
  // in the same order the previous recursive generator did — so ties still
  // resolve to the same winner.
  const k = nonSolos.length;
  const fact = new Array(k + 1);
  fact[0] = 1;
  for (let i = 1; i <= k; i += 1) fact[i] = fact[i - 1] * i;
  const pool = new Int32Array(k);
  const permaOrd = permaSolos.map((pid) => ordOf.get(pid));
  const nonSoloOrd = nonSolos.map((pid) => ordOf.get(pid));
  const seedLen = k + permaOrd.length;

  const writePerm = (m) => {
    for (let i = 0; i < k; i += 1) pool[i] = i;
    let poolLen = k;
    let rest = m;
    for (let i = 0; i < k; i += 1) {
      const f = fact[k - 1 - i];
      const d = Math.floor(rest / f);
      rest -= d * f;
      seedBuf[i] = nonSoloOrd[pool[d]];
      for (let j = d; j < poolLen - 1; j += 1) pool[j] = pool[j + 1];
      poolLen -= 1;
    }
    for (let i = 0; i < permaOrd.length; i += 1) seedBuf[k + i] = permaOrd[i];
  };

  let bestM = -1;
  let bestScore = Infinity;
  for (let m = 0; m < fact[k]; m += 1) {
    writePerm(m);
    const score = scoreOf(seedLen);
    if (score < bestScore) { bestScore = score; bestM = m; }
  }
  if (bestM < 0) return { columns: [], initialOrder: [...nonSolos, ...permaSolos] };

  writePerm(bestM);
  const initialOrder = new Array(seedLen);
  for (let i = 0; i < seedLen; i += 1) initialOrder[i] = ordToPid[seedBuf[i]];
  return { columns: columnsFor(seedLen), initialOrder };
}

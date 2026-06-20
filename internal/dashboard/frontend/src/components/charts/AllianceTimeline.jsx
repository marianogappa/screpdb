import React, { useMemo, useRef, useState, useEffect } from 'react';

// AllianceTimeline renders alliance topology as a Sankey-style flow.
// Time runs top-to-bottom on a non-linear axis (rows = significant events
// only). Each player owns a vertical lane that terminates when they leave or
// stop playing. Team membership at each row is shown as a translucent pill
// hugging the contiguous columns of allied players. A right-hand context
// panel — chat, alliance diffs, departures, military events near departures
// — anchors each event to its time row.
//
// Inputs:
//   players: [{ player_id, name, race, team, color, left_second, leave_reason }]
//   timeline: [{ sec, teams: [[player_id, ...], ...], stacking }]
//   chat: [{ second, player_id, message }]
//   gameEvents: [{ type, second, actor, target, ... }] — the full game-event
//                stream; we filter for attack/drop/recall/nuke near departures.
//   durationSeconds: number
//   earlyEndsAt / midEndsAt: number (phase-boundary seconds)
//   stackingThresholdSeconds: number — bands ≥ this earn the stacked badge.
//   getRaceIcon: race -> url (or null)

// Fallback palette used only when a player has no recognizable BW colour name.
const TEAM_COLORS = ['#60A5FA', '#F472B6', '#34D399', '#FBBF24', '#A78BFA', '#22D3EE', '#FB7185', '#4ADE80'];

// playerHexColor returns a player's in-game BW colour via the resolver passed
// in by the host (App.jsx wraps the engine's screp-colors map). Falls back to
// the palette above when the resolver is missing or doesn't know the name —
// e.g. for synthetic player ids or pre-bootstrapped renders.
const playerHexColor = (player, getPlayerColor) => {
  if (!player) return TEAM_COLORS[0];
  if (typeof getPlayerColor === 'function') {
    const resolved = getPlayerColor(player);
    if (resolved && /^#?[0-9a-fA-F]{3,8}$/.test(String(resolved).trim())) {
      const v = String(resolved).trim();
      return v.startsWith('#') ? v : `#${v}`;
    }
  }
  return TEAM_COLORS[Math.abs(Number(player.player_id) || 0) % TEAM_COLORS.length];
};
const ROW_MIN_HEIGHT = 110;
const EVENT_ROW_HEIGHT = 30; // Each event entry's vertical footprint in the side panel.
const EVENT_ROW_TOP_OFFSET = 18; // Top padding inside a row group before the first event.
// How many rows of stable column position can pass before the player's name
// is re-rendered above their node so the reader stays oriented in long games.
const NAME_REFRESH_INTERVAL = 4;
const TOP_PAD = 60;
const BOTTOM_PAD = 32;
const COL_MIN_WIDTH = 80;
const NODE_R = 18;
const LEFT_LABEL_W = 78;
const RIGHT_PANEL_MIN_W = 320;
const RIGHT_PANEL_PAD_LEFT = 16;

// Events within this many seconds of a departure are surfaced as context.
const NEAR_DEPARTURE_WINDOW_SEC = 60;
// Two events closer than this in seconds collapse onto one row.
const ROW_MERGE_SEC = 1;
// Generic events ≥ this gap from the prior row introduce a new row even
// without an alliance change — so the right-hand event panel has a row to
// dock against. Set conservatively so we don't add too many rows.
const STANDALONE_EVENT_MIN_GAP_SEC = 0;

const formatMMSS = (sec) => {
  const v = Math.max(0, Math.floor(Number(sec) || 0));
  return `${Math.floor(v / 60)}:${String(v % 60).padStart(2, '0')}`;
};

// Team-level colour: use the BW colour of the team's lowest-pid member when
// available. This keeps a player's colour (and the lines/arcs touching them)
// in sync with the in-game replay colour they had.
const teamColor = (pids, playerByID, getPlayerColor) => {
  if (!pids || pids.length === 0) return TEAM_COLORS[0];
  const p = playerByID ? playerByID[pids[0]] : null;
  return playerHexColor(p || { player_id: pids[0] }, getPlayerColor);
};

// "4v2v1" style label — used inside team pills with ≥2 teams visible.
const teamShape = (teams) => {
  const sizes = teams.map((t) => t.length).filter((n) => n >= 1);
  if (sizes.length < 2) return '';
  return sizes.sort((a, b) => b - a).join('v');
};

const isStacked = (teams) => {
  const sizes = teams.map((t) => t.length).filter((n) => n >= 2);
  if (sizes.length < 2) return false;
  return new Set(sizes).size > 1;
};

// Phase tag for a row second based on early/mid boundary markers.
const phaseTagFor = (sec, earlyEndsAt, midEndsAt) => {
  const s = Number(sec) || 0;
  if (s <= 0) return 'START';
  if (earlyEndsAt > 0 && s < earlyEndsAt) return 'EARLY';
  if (midEndsAt > 0 && s < midEndsAt) return 'MID';
  if (midEndsAt > 0) return 'LATE';
  if (earlyEndsAt > 0) return 'MID';
  return '';
};

// Diff teams[t-1] vs teams[t] and return human-readable change pills.
// rowSec is the second of the *new* row — we use it (with playerByID's
// left_second) to suppress "× break" events that are merely the side-effect
// of a player departing. The player's line already terminates visually; a
// separate break pill would just be noise.
const diffTopology = (prevTeams, nextTeams, playerByID, rowSec, getPlayerColor) => {
  const prevPairs = new Set();
  const nextPairs = new Set();
  const addPairs = (teams, target) => {
    for (const t of teams) {
      for (let i = 0; i < t.length; i += 1) {
        for (let j = i + 1; j < t.length; j += 1) {
          const a = t[i];
          const b = t[j];
          const key = a < b ? `${a}|${b}` : `${b}|${a}`;
          target.add(key);
        }
      }
    }
  };
  addPairs(prevTeams || [], prevPairs);
  addPairs(nextTeams || [], nextPairs);
  const added = [];
  const removed = [];
  for (const k of nextPairs) if (!prevPairs.has(k)) added.push(k);
  for (const k of prevPairs) if (!nextPairs.has(k)) removed.push(k);
  const nameOf = (pid) => (playerByID[pid]?.name || `#${pid}`);
  const colorOf = (pid) => {
    const p = playerByID[pid];
    if (!p) return null;
    return playerHexColor(p, getPlayerColor);
  };
  const departedBy = (pid) => {
    const p = playerByID[pid];
    if (!p || p.left_second == null) return false;
    return Number(p.left_second) <= rowSec;
  };
  const out = [];
  for (const k of added) {
    const [a, b] = k.split('|').map(Number);
    out.push({ kind: 'ally', a: nameOf(a), b: nameOf(b), colorA: colorOf(a), colorB: colorOf(b) });
  }
  for (const k of removed) {
    const [a, b] = k.split('|').map(Number);
    // Skip pair-removals where either member has departed by this row — the
    // terminating line already explains the lost alliance.
    if (departedBy(a) || departedBy(b)) continue;
    out.push({ kind: 'break', a: nameOf(a), b: nameOf(b), colorA: colorOf(a), colorB: colorOf(b) });
  }
  const wasStacked = isStacked(prevTeams || []);
  const nowStacked = isStacked(nextTeams || []);
  if (!wasStacked && nowStacked) out.push({ kind: 'stack' });
  if (wasStacked && !nowStacked) out.push({ kind: 'unstack' });
  return out;
};

const AllianceTimeline = ({
  players = [],
  timeline = [],
  chat = [],
  gameEvents = [],
  durationSeconds = 0,
  earlyEndsAt = 0,
  midEndsAt = 0,
  stackingThresholdSeconds = 300,
  getRaceIcon,
  getPlayerColor,
}) => {
  const playerByID = useMemo(() => {
    const m = {};
    for (const p of players) {
      if (p && p.player_id != null) m[p.player_id] = p;
    }
    return m;
  }, [players]);

  // Active (non-observer) players in API order. Backend already filters
  // is_observer=0, but be defensive.
  const activePlayers = useMemo(
    () => players.filter((p) => p && p.player_id != null),
    [players],
  );

  // ── Phase rows ─────────────────────────────────────────────────────────
  // A row = a distinct time point. Each snapshot in the alliance timeline
  // emits a row; each player departure (left_second) emits a row if it
  // doesn't already coincide with an alliance event. We always anchor a
  // final row at duration_seconds so terminating lines have somewhere to
  // run to.
  const rows = useMemo(() => {
    const snaps = (Array.isArray(timeline) ? timeline : []).map((s) => ({
      sec: Math.max(0, Number(s.sec) || 0),
      teams: Array.isArray(s.teams) ? s.teams.map((t) => t.slice()) : [],
      stackingSrc: !!s.stacking,
    }));
    if (snaps.length === 0) return [];

    const secs = new Set();
    snaps.forEach((s) => secs.add(s.sec));
    for (const p of activePlayers) {
      if (p.left_second != null) {
        secs.add(Math.max(0, Number(p.left_second) || 0));
      }
    }
    if (durationSeconds > 0) secs.add(Math.max(0, Number(durationSeconds) || 0));

    // Merge near-duplicates so we don't get two rows for events 1 second apart
    // (e.g. snapshot at sec=64 + departure at sec=64).
    const sortedSecs = Array.from(secs).sort((a, b) => a - b);
    const mergedSecs = [];
    for (const s of sortedSecs) {
      if (mergedSecs.length === 0 || s - mergedSecs[mergedSecs.length - 1] > ROW_MERGE_SEC) {
        mergedSecs.push(s);
      }
    }

    const snapAtOrBefore = (sec) => {
      let best = snaps[0];
      for (const s of snaps) {
        if (s.sec <= sec) best = s;
      }
      return best;
    };

    return mergedSecs.map((sec) => {
      const snap = snapAtOrBefore(sec);
      const teams = snap.teams
        .map((t) => t.filter((pid) => {
          const p = playerByID[pid];
          if (!p) return false;
          if (p.left_second != null && Number(p.left_second) < sec) return false;
          return true;
        }))
        .filter((t) => t.length > 0);
      const departures = activePlayers
        .filter((p) => p.left_second != null && Math.abs(Number(p.left_second) - sec) <= ROW_MERGE_SEC)
        .map((p) => p.player_id);
      return { sec, teams, stacking: isStacked(teams), departures };
    });
  }, [timeline, activePlayers, playerByID, durationSeconds]);

  // ── Column ordering per row ────────────────────────────────────────────
  // Per-row layout: place the first (largest, lowest-min) clique, then
  // repeatedly pick the unplaced clique that shares the most members with
  // what's already laid down. That's the natural chain order for non-
  // transitive alliances (D-C-Y-f stays D-C-Y-f rather than being shuffled
  // into D-C-f-Y). Departed players keep their last-known column index so
  // their terminating "x" lands where the lane was.
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
        const size = team.length;
        const idxs = team.map((pid) => prev.indexOf(pid)).filter((v) => v >= 0);
        const centroid = idxs.length === 0
          ? Number.MAX_SAFE_INTEGER
          : idxs.reduce((a, v) => a + v, 0) / idxs.length;
        // Priority: extend the existing layout (overlap), then preserve the
        // prev-row position of the team's centroid (-centroid), THEN prefer
        // larger cliques as a tiebreaker. Putting -centroid before size
        // stops a newly-formed bigger clique from hijacking column 0 just
        // because it has more members than the pair currently anchored
        // there — concretely, the {D,C,Y} triangle at 11:28 stays in its
        // home columns instead of swapping with chobo+FA.
        const key = [overlap, -centroid, size, -ti];
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
      if (prevIdx < 0) {
        newOrder.push(pid);
        continue;
      }
      const target = Math.min(prevIdx, newOrder.length);
      newOrder.splice(target, 0, pid);
    }
    return newOrder;
  };

  // simulate(initialOrder) walks every row applying computeRowOrder, returns
  // both the resulting column arrays AND a quality score (lower = better).
  // The score charges 2 points for each player whose column index shifts
  // between consecutive rows (visual line crossing) and 1 point per column
  // of arc length (pair-allied players placed far apart). With both terms
  // we get layouts that are *stable* (chobo + FA never swap) AND have
  // adjacent arcs (chain D-C-Y-f naturally lined up).
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

  // Compute the BEST initial column order to minimise total movement and
  // arc length. Players who never appear in a size-≥2 clique anywhere in
  // the timeline ("permanent solos" — e.g. someone whose ally requests are
  // never reciprocated, like RememberMyName in replay 500) get pinned to
  // the end of the lineup: their position has no impact on movement or
  // arc-length scoring, so multiple permutations tie. Without this pin the
  // first tied perm encountered in iteration wins, which can drop a solo
  // randomly into the middle of an otherwise stable lineup.
  const { columns, initialOrder } = useMemo(() => {
    if (rows.length === 0) return { columns: [], initialOrder: [] };
    const pidList = activePlayers.map((p) => p.player_id);
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
      const r = simulateOrder(fallback);
      return { columns: r.cols, initialOrder: fallback };
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
      if (score < bestScore) {
        bestScore = score;
        bestOrder = candidate;
        bestCols = cols;
      }
    }
    return { columns: bestCols || [], initialOrder: bestOrder };
    // computeRowOrder + simulateOrder are stable closures over `rows` which
    // is itself a useMemo — only those identities matter for dep tracking.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, activePlayers]);

  // ── Layout dimensions ──────────────────────────────────────────────────
  const wrapRef = useRef(null);
  const [wrapWidth, setWrapWidth] = useState(960);
  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return undefined;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const w = Math.floor(entry.contentRect.width);
        if (w > 0) setWrapWidth(w);
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // ── Context panel rows (computed before layout so row heights can flex) ─
  // For each row index, gather the events that "belong" to it. Kept above
  // the early-return guard so hook order stays stable across renders.
  const eventsByRowIdx = useMemo(() => {
    const out = rows.map(() => []);
    if (rows.length === 0) return out;
    const rowSecOf = (sec) => {
      // Pick the row whose sec is closest to the event — so an attack at
      // 17:21 buckets to the 17:44 row (near the departure it correlates
      // with), not the 4:55 row (last alliance change before it).
      let chosen = 0;
      let bestDist = Infinity;
      for (let i = 0; i < rows.length; i += 1) {
        const d = Math.abs(rows[i].sec - sec);
        if (d < bestDist) {
          bestDist = d;
          chosen = i;
        }
      }
      return chosen;
    };

    for (let ri = 0; ri < rows.length; ri += 1) {
      const prev = ri === 0 ? [] : rows[ri - 1].teams;
      const diffs = diffTopology(prev, rows[ri].teams, playerByID, rows[ri].sec);
      for (const d of diffs) {
        out[ri].push({ sec: rows[ri].sec, kind: d.kind, data: d, sort: 0 });
      }
    }

    for (let ri = 0; ri < rows.length; ri += 1) {
      for (const pid of rows[ri].departures) {
        const p = playerByID[pid];
        if (!p) continue;
        // "Stopped" (inactivity-derived) gets its own visual treatment from
        // an explicit quit — the player didn't formally leave, they just
        // stopped issuing meaningful commands.
        const reason = p.leave_reason || 'Left';
        const kind = reason === 'Stopped' ? 'stopped' : 'depart';
        out[ri].push({
          sec: rows[ri].sec,
          kind,
          data: { pid, name: p.name, reason },
          sort: 1,
        });
      }
    }

    for (const c of chat || []) {
      const ri = rowSecOf(Number(c.second) || 0);
      const p = playerByID[c.player_id];
      out[ri].push({
        sec: Number(c.second) || 0,
        kind: 'chat',
        data: {
          name: p?.name || `#${c.player_id}`,
          message: c.message,
          color: p ? playerHexColor(p, getPlayerColor) : '#cbd5e1',
        },
        sort: 2,
      });
    }

    const departureSet = new Set();
    for (let ri = 0; ri < rows.length; ri += 1) {
      for (const pid of rows[ri].departures) departureSet.add(pid);
    }
    if (departureSet.size > 0) {
      for (const ev of gameEvents || []) {
        const type = String(ev.type || '');
        if (type !== 'attack' && type !== 'drop'
          && type !== 'cliff_drop'
          && type !== 'recall' && type !== 'nuke') continue;
        const actorPid = ev.actor?.player_id;
        const targetPid = ev.target?.player_id;
        const evSec = Number(ev.second) || 0;
        let nearby = false;
        let relevant = false;
        for (const pid of departureSet) {
          const p = playerByID[pid];
          if (!p || p.left_second == null) continue;
          const leftSec = Number(p.left_second) || 0;
          if (Math.abs(evSec - leftSec) > NEAR_DEPARTURE_WINDOW_SEC) continue;
          nearby = true;
          if (actorPid === pid || targetPid === pid) { relevant = true; break; }
        }
        if (!nearby || !relevant) continue;
        const ri = rowSecOf(evSec);
        const actor = playerByID[actorPid];
        const target = playerByID[targetPid];
        out[ri].push({
          sec: evSec,
          kind: type,
          data: {
            actor: actor?.name || ev.actor?.name || '',
            target: target?.name || ev.target?.name || '',
            actorColor: actor ? playerHexColor(actor, getPlayerColor) : null,
            targetColor: target ? playerHexColor(target, getPlayerColor) : null,
          },
          sort: 3,
        });
      }
    }

    for (const list of out) {
      list.sort((a, b) => {
        if (a.sec !== b.sec) return a.sec - b.sec;
        return a.sort - b.sort;
      });
    }
    return out;
  }, [rows, chat, gameEvents, playerByID]);

  if (rows.length === 0 || activePlayers.length === 0) {
    return (
      <div className="workflow-card">
        <div className="chart-empty">No alliance information available for this game.</div>
      </div>
    );
  }

  // Per-row Y coordinates — each row gets enough space to fit its events
  // without overflowing into the next row. Both SVG and the right panel
  // index into rowYs[] for vertical positioning.
  const rowYs = [];
  let cursorY = TOP_PAD;
  for (let ri = 0; ri < rows.length; ri += 1) {
    rowYs.push(cursorY);
    const eventCount = (eventsByRowIdx[ri] || []).length;
    const eventsHeight = eventCount > 0
      ? EVENT_ROW_TOP_OFFSET + eventCount * EVENT_ROW_HEIGHT + 6
      : 0;
    const spacing = Math.max(ROW_MIN_HEIGHT, eventsHeight);
    cursorY += spacing;
  }
  const rowY = (i) => rowYs[i];
  const svgHeight = (rowYs[rows.length - 1] || TOP_PAD) + BOTTOM_PAD;

  const numCols = activePlayers.length;
  // Right panel takes a fixed minimum; SVG takes the remainder. If the
  // viewport is narrow the SVG stretches horizontally with min-width per col.
  const rightPanelWidth = Math.max(RIGHT_PANEL_MIN_W, Math.min(420, Math.floor(wrapWidth * 0.36)));
  const svgWidthBudget = Math.max(360, wrapWidth - rightPanelWidth - RIGHT_PANEL_PAD_LEFT);
  const colsSpace = svgWidthBudget - LEFT_LABEL_W - 24;
  const colW = Math.max(COL_MIN_WIDTH, Math.floor(colsSpace / numCols));
  const svgWidth = LEFT_LABEL_W + colW * numCols + 24;
  const colX = (i) => LEFT_LABEL_W + colW * i + colW / 2;

  // For each player, walk through rows and collect their (rowIdx, x, y).
  // Lines terminate at the row in which they appear in the departures list.
  const playerPaths = activePlayers.map((p) => {
    const points = [];
    let terminated = false;
    for (let ri = 0; ri < rows.length; ri += 1) {
      const ord = columns[ri] || [];
      const idx = ord.indexOf(p.player_id);
      if (idx < 0) continue;
      points.push({ rowIdx: ri, x: colX(idx), y: rowY(ri) });
      if (rows[ri].departures.includes(p.player_id)) {
        terminated = true;
        break;
      }
    }
    return { player: p, points, terminated };
  });

  // Team rendering: each maximal clique becomes either an arc (size 2) or
  // a rounded pill (size ≥3). Arcs are how non-transitive alliances stay
  // honest — a chain A↔B↔C↔D draws three separate arcs, not one fake
  // 4-stack pill. A player who appears in multiple cliques contributes to
  // each of them (e.g. C draws both the A-C arc and the B-C arc) without
  // any pretense that A and B are themselves allied.
  const pills = [];
  const arcs = [];
  for (let ri = 0; ri < rows.length; ri += 1) {
    const row = rows[ri];
    const ord = columns[ri] || [];
    for (const team of row.teams) {
      if (team.length < 2) continue;
      const cols = team
        .map((pid) => ord.indexOf(pid))
        .filter((v) => v >= 0);
      if (cols.length < 2) continue;
      if (team.length === 2) {
        const [c1, c2] = cols.sort((a, b) => a - b);
        arcs.push({
          ri,
          x1: colX(c1),
          x2: colX(c2),
          y: rowY(ri),
          color: teamColor(team, playerByID, getPlayerColor),
          stacking: row.stacking,
        });
        continue;
      }
      // size ≥ 3 → render as an enclosing rounded pill (clique implies all
      // members are mutually allied, so a single enclosing rectangle is
      // faithful).
      const xs = cols.map((i) => colX(i));
      const minX = Math.min(...xs) - NODE_R - 6;
      const maxX = Math.max(...xs) + NODE_R + 6;
      pills.push({
        ri,
        x: minX,
        y: rowY(ri) - NODE_R - 6,
        w: maxX - minX,
        h: NODE_R * 2 + 12,
        color: teamColor(team, playerByID, getPlayerColor),
        stacking: row.stacking,
        label: `${team.length}-stack`,
      });
    }
  }

  // Single-glyph emoji badges per event kind. Words ("ally", "break", etc.)
  // are dropped — the colored pill + emoji conveys the type, and the body
  // text already names the actors.
  const kindBadgeLabel = (k) => {
    if (k === 'ally') return '🤝';
    if (k === 'break') return '💔';
    if (k === 'stack') return '😈';
    if (k === 'unstack') return '🕊️';
    if (k === 'depart') return '🏳️';
    if (k === 'stopped') return '💤';
    if (k === 'chat') return '💬';
    if (k === 'attack') return '⚔️';
    if (k === 'drop' || k === 'cliff_drop') return '🪂';
    if (k === 'recall') return '🌀';
    if (k === 'nuke') return '☢️';
    return k;
  };

  const kindBadgeClass = (k) => `workflow-alliance-event-badge workflow-alliance-event-badge-${k.replace('_', '-')}`;

  // ── Render ─────────────────────────────────────────────────────────────
  return (
    <div className="workflow-card workflow-alliance-timeline-v2" ref={wrapRef}>
      <div className="workflow-alliance-timeline-grid" style={{ gridTemplateColumns: `${svgWidth}px ${rightPanelWidth}px` }}>
        <div className="workflow-alliance-svg-wrap">
          <svg
            width={svgWidth}
            height={svgHeight}
            viewBox={`0 0 ${svgWidth} ${svgHeight}`}
            className="workflow-alliance-svg"
            preserveAspectRatio="xMinYMin meet"
          >
            {/* Faint row guides */}
            {rows.map((r, ri) => (
              <line
                key={`row-guide-${ri}`}
                x1={LEFT_LABEL_W - 4}
                x2={svgWidth - 8}
                y1={rowY(ri)}
                y2={rowY(ri)}
                stroke="rgba(148,163,184,0.10)"
                strokeWidth={1}
              />
            ))}

            {/* Time + phase labels on the left gutter */}
            {rows.map((r, ri) => {
              const tag = phaseTagFor(r.sec, earlyEndsAt, midEndsAt);
              return (
                <g key={`row-label-${ri}`}>
                  <text
                    x={LEFT_LABEL_W - 10}
                    y={rowY(ri) + 4}
                    textAnchor="end"
                    fontSize={12}
                    fill="#cbd5e1"
                  >
                    {ri === 0 && r.sec === 0 ? 'START' : formatMMSS(r.sec)}
                  </text>
                  {tag
                    && tag !== 'START'
                    && (ri === 0 || tag !== phaseTagFor(rows[ri - 1].sec, earlyEndsAt, midEndsAt)) ? (
                    <text
                      x={LEFT_LABEL_W - 10}
                      y={rowY(ri) - 14}
                      textAnchor="end"
                      fontSize={9}
                      fill="#64748b"
                      letterSpacing={1}
                    >
                      {tag}
                    </text>
                  ) : null}
                </g>
              );
            })}

            {/* Pair arcs (drawn under nodes/lines). Curves above the node row
                so vertical player lines stay visually unbroken. */}
            {arcs.map((a, i) => {
              const lift = Math.min(28, 8 + Math.abs(a.x2 - a.x1) * 0.18);
              const cy = a.y - lift;
              const d = `M ${a.x1} ${a.y} Q ${(a.x1 + a.x2) / 2} ${cy}, ${a.x2} ${a.y}`;
              return (
                <path
                  key={`arc-${i}`}
                  d={d}
                  fill="none"
                  stroke={a.stacking ? 'rgba(248, 113, 113, 0.7)' : 'rgba(148, 163, 184, 0.7)'}
                  strokeWidth={a.stacking ? 3 : 2}
                  strokeLinecap="round"
                  opacity={0.85}
                />
              );
            })}

            {/* Clique pills (size ≥3 — every pair within is mutually allied). */}
            {pills.map((p, i) => (
              <rect
                key={`pill-${i}`}
                x={p.x}
                y={p.y}
                width={p.w}
                height={p.h}
                rx={p.h / 2}
                fill={p.stacking ? 'rgba(248, 113, 113, 0.18)' : 'rgba(96, 165, 250, 0.14)'}
                stroke={p.stacking ? 'rgba(248, 113, 113, 0.55)' : 'rgba(148, 163, 184, 0.45)'}
                strokeWidth={1}
              />
            ))}
            {/* Stacking label per row — sized from the largest clique present.
                Sizes only count cliques of size ≥3 since pair-cliques carry no
                stacking signal under the new model. */}
            {rows.map((r, ri) => {
              if (!r.stacking) return null;
              const ord = columns[ri] || [];
              const maxIdx = Math.max(0, Math.max(...ord.map((_, i) => i)));
              const x = colX(maxIdx) + NODE_R + 8;
              const cliqueSizes = r.teams
                .map((t) => t.length)
                .filter((n) => n >= 2)
                .sort((a, b) => b - a);
              const label = cliqueSizes.length >= 2 ? `${cliqueSizes.join('v')} stacked` : 'stacked';
              return (
                <text
                  key={`stack-${ri}`}
                  x={x}
                  y={rowY(ri) + 4}
                  fontSize={11}
                  fill="#fca5a5"
                  fontWeight="600"
                >
                  {label}
                </text>
              );
            })}

            {/* Player lines (cubic-bezier between rows) */}
            {playerPaths.map(({ player, points, terminated }) => {
              if (points.length === 0) return null;
              const color = playerHexColor(player, getPlayerColor);
              const segs = [];
              for (let i = 0; i < points.length - 1; i += 1) {
                const p1 = points[i];
                const p2 = points[i + 1];
                const dy = (p2.y - p1.y) * 0.4;
                segs.push(`M ${p1.x} ${p1.y} C ${p1.x} ${p1.y + dy}, ${p2.x} ${p2.y - dy}, ${p2.x} ${p2.y}`);
              }
              return (
                <g key={`line-${player.player_id}`}>
                  {segs.map((d, si) => (
                    <path
                      key={`seg-${si}`}
                      d={d}
                      stroke={color}
                      strokeWidth={2}
                      fill="none"
                      strokeLinecap="round"
                      opacity={0.85}
                    />
                  ))}
                  {terminated ? (
                    <g transform={`translate(${points[points.length - 1].x},${points[points.length - 1].y + NODE_R + 4})`}>
                      <line x1={-5} y1={-5} x2={5} y2={5} stroke={color} strokeWidth={2} />
                      <line x1={-5} y1={5} x2={5} y2={-5} stroke={color} strokeWidth={2} />
                    </g>
                  ) : null}
                </g>
              );
            })}

            {/* Player nodes per row (drawn last so they sit on top of lines/pills).
                Names are re-rendered whenever the player swaps columns vs the
                previous row, plus every NAME_REFRESH_INTERVAL rows even without
                a swap so the reader doesn't lose track in long stable runs. */}
            {playerPaths.map(({ player, points }) => {
              const color = playerHexColor(player, getPlayerColor);
              const icon = getRaceIcon ? getRaceIcon(player.race) : null;
              const displayName = String(player.name || `#${player.player_id}`).slice(0, 14);
              // Precompute which rows show this player's name.
              const showNameAt = new Set();
              let lastNamedRow = -Infinity;
              let lastCol = -1;
              for (let pi = 0; pi < points.length; pi += 1) {
                const pt = points[pi];
                const ord = columns[pt.rowIdx] || [];
                const colIdx = ord.indexOf(player.player_id);
                const colChanged = pi > 0 && colIdx !== lastCol;
                const farFromLast = pt.rowIdx - lastNamedRow >= NAME_REFRESH_INTERVAL;
                if (pi === 0 || colChanged || farFromLast) {
                  showNameAt.add(pt.rowIdx);
                  lastNamedRow = pt.rowIdx;
                }
                lastCol = colIdx;
              }
              // Approximate text width for backdrop (SVG can't measure pre-paint).
              const nameW = Math.max(28, Math.min(120, displayName.length * 7 + 8));
              return points.map((pt) => (
                <g key={`node-${player.player_id}-${pt.rowIdx}`}>
                  <circle
                    cx={pt.x}
                    cy={pt.y}
                    r={NODE_R}
                    fill={color}
                    fillOpacity={0.25}
                    stroke={color}
                    strokeWidth={2}
                  />
                  {icon ? (
                    <image
                      href={icon}
                      x={pt.x - 11}
                      y={pt.y - 11}
                      width={22}
                      height={22}
                    />
                  ) : (
                    <text
                      x={pt.x}
                      y={pt.y + 4}
                      textAnchor="middle"
                      fontSize={11}
                      fill="#e5e7eb"
                    >
                      {String(player.race || '?').slice(0, 1).toUpperCase()}
                    </text>
                  )}
                  {showNameAt.has(pt.rowIdx) ? (
                    <g>
                      <rect
                        x={pt.x - nameW / 2}
                        y={pt.y - NODE_R - 18}
                        width={nameW}
                        height={15}
                        rx={3}
                        fill="rgba(15, 23, 42, 0.92)"
                        stroke={color}
                        strokeWidth={1}
                        strokeOpacity={0.55}
                      />
                      <text
                        x={pt.x}
                        y={pt.y - NODE_R - 7}
                        textAnchor="middle"
                        fontSize={10}
                        fill="#e5e7eb"
                        fontWeight="500"
                      >
                        {displayName}
                      </text>
                    </g>
                  ) : null}
                </g>
              ));
            })}
          </svg>
        </div>

        {/* Right context panel — events anchored to rows. */}
        <div className="workflow-alliance-context-panel" style={{ height: svgHeight, position: 'relative' }}>
          {rows.map((r, ri) => {
            const list = eventsByRowIdx[ri] || [];
            if (list.length === 0) return null;
            const top = rowY(ri) - 18;
            return (
              <div
                key={`ctx-row-${ri}`}
                className="workflow-alliance-context-row"
                style={{ position: 'absolute', top, left: 0, right: 0 }}
              >
                {list.map((ev, i) => (
                  <div
                    key={`ev-${ri}-${i}`}
                    className="workflow-alliance-event"
                    title={ev.kind === 'chat' ? `${ev.data.name}: "${ev.data.message}"` : undefined}
                  >
                    <span className="workflow-alliance-event-time">{formatMMSS(ev.sec)}</span>
                    <span className={kindBadgeClass(ev.kind)}>{kindBadgeLabel(ev.kind)}</span>
                    <span className="workflow-alliance-event-body">
                      {ev.kind === 'ally' || ev.kind === 'break' ? (
                        <>
                          <span style={{ color: ev.data.colorA || '#cbd5e1' }}>{ev.data.a}</span>
                          {' '}{ev.kind === 'ally' ? '⇌' : '⇎'}{' '}
                          <span style={{ color: ev.data.colorB || '#cbd5e1' }}>{ev.data.b}</span>
                        </>
                      ) : null}
                      {ev.kind === 'stack' ? <span>uneven non-solo teams</span> : null}
                      {ev.kind === 'unstack' ? <span>teams now even</span> : null}
                      {ev.kind === 'depart' || ev.kind === 'stopped' ? (
                        <>
                          <span style={{ color: playerHexColor(playerByID[ev.data.pid], getPlayerColor) }}>{ev.data.name}</span>
                          <span className="workflow-alliance-event-reason"> ({ev.data.reason})</span>
                        </>
                      ) : null}
                      {ev.kind === 'chat' ? (
                        <>
                          <span style={{ color: ev.data.color }}>{ev.data.name}:</span>
                          <span className="workflow-alliance-chat-message"> "{ev.data.message}"</span>
                        </>
                      ) : null}
                      {(ev.kind === 'attack' || ev.kind === 'drop'
                          || ev.kind === 'cliff_drop'
                          || ev.kind === 'recall' || ev.kind === 'nuke') ? (
                        <>
                          <span style={{ color: ev.data.actorColor || '#cbd5e1' }}>{ev.data.actor}</span>
                          {ev.data.target ? (
                            <>
                              {' → '}
                              <span style={{ color: ev.data.targetColor || '#cbd5e1' }}>{ev.data.target}</span>
                            </>
                          ) : null}
                        </>
                      ) : null}
                    </span>
                  </div>
                ))}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
};

export default AllianceTimeline;

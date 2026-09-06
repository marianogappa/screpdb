import React, { useMemo, useRef, useState, useEffect } from 'react';
import { useT } from '../../lib/i18nContext';
import { slugKey } from '../../lib/i18n';
import { computeColumnLayout } from '../../lib/allianceLayout';

// AllianceTimeline renders alliance topology as a Sankey-style flow. Time runs
// top-to-bottom on a NON-LINEAR axis: rows are significant events only. Each
// player owns a vertical lane that terminates when they leave or stop playing,
// team membership per row is a translucent pill hugging contiguous columns of
// allied players, and a right-hand context panel anchors chat, alliance diffs,
// departures and nearby military events to their time row.

// Used only when a player has no recognizable BW colour name.
const TEAM_COLORS = ['#60A5FA', '#F472B6', '#34D399', '#FBBF24', '#A78BFA', '#22D3EE', '#FB7185', '#4ADE80'];

// Resolved via the host-supplied resolver (App.jsx wraps the engine's
// screp-colors map), falling back to the palette above when it is missing or
// doesn't know the name — synthetic player ids, pre-bootstrapped renders.
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
// Rows of stable column position before a player's name is re-rendered above
// their node, so the reader stays oriented in long games.
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
// Generic events at least this far from the prior row introduce a new row even
// without an alliance change, so the event panel has a row to dock against.
// Conservative, to avoid adding too many rows.
const STANDALONE_EVENT_MIN_GAP_SEC = 0;

const formatMMSS = (sec) => {
  const v = Math.max(0, Math.floor(Number(sec) || 0));
  return `${Math.floor(v / 60)}:${String(v % 60).padStart(2, '0')}`;
};

// Team colour is the BW colour of the team's lowest-pid member, which keeps a
// player's colour — and the lines and arcs touching them — in sync with the
// replay colour they had in game.
const teamColor = (pids, playerByID, getPlayerColor) => {
  if (!pids || pids.length === 0) return TEAM_COLORS[0];
  const p = playerByID ? playerByID[pids[0]] : null;
  return playerHexColor(p || { player_id: pids[0] }, getPlayerColor);
};

// "4v2v1" style label, used inside team pills with ≥2 teams visible.
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

const phaseTagFor = (sec, earlyEndsAt, midEndsAt) => {
  const s = Number(sec) || 0;
  if (s <= 0) return 'START';
  if (earlyEndsAt > 0 && s < earlyEndsAt) return 'EARLY';
  if (midEndsAt > 0 && s < midEndsAt) return 'MID';
  if (midEndsAt > 0) return 'LATE';
  if (earlyEndsAt > 0) return 'MID';
  return '';
};

// diffTopology returns human-readable change pills. rowSec is the second of the
// NEW row, used with left_second to suppress "× break" events that are merely a
// side-effect of a departure: the player's line already terminates visually, so
// a separate break pill would be noise.
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
    // The terminating line already explains the lost alliance.
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
  const t = useT();
  const playerByID = useMemo(() => {
    const m = {};
    for (const p of players) {
      if (p && p.player_id != null) m[p.player_id] = p;
    }
    return m;
  }, [players]);

  // The backend already filters is_observer=0, but be defensive.
  const activePlayers = useMemo(
    () => players.filter((p) => p && p.player_id != null),
    [players],
  );

  // A row is a distinct time point: every alliance snapshot emits one, and each
  // player departure emits one unless it coincides with an alliance event. A final
  // row is always anchored at duration_seconds so terminating lines have somewhere
  // to run to.
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

    // Merge near-duplicates so a snapshot and a departure one second apart don't
    // produce two rows.
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

  // Which lane each player occupies on each row. See lib/allianceLayout.js for
  // the placement rule and the search over initial orderings.
  const { columns } = useMemo(
    () => computeColumnLayout(rows, activePlayers.map((p) => p.player_id)),
    [rows, activePlayers],
  );

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

  // Events belonging to each row index. Kept above the early-return guard so hook
  // order stays stable across renders.
  const eventsByRowIdx = useMemo(() => {
    const out = rows.map(() => []);
    if (rows.length === 0) return out;
    const rowSecOf = (sec) => {
      // Bucket to the row whose sec is CLOSEST, so an attack at 17:21 lands on the
      // 17:44 row near the departure it correlates with, not the 4:55 row that was
      // the last alliance change before it.
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
        // "Stopped" (inactivity-derived) gets its own treatment: the player didn't
        // formally leave, they just stopped issuing meaningful commands.
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
        <div className="chart-empty">{t('chart.alliance.empty')}</div>
      </div>
    );
  }

  // Each row gets enough space to fit its events without overflowing into the
  // next. Both the SVG and the right panel index into rowYs for positioning.
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
  // The right panel takes a fixed minimum and the SVG the remainder; on a narrow
  // viewport the SVG stretches horizontally with a min-width per column.
  const rightPanelWidth = Math.max(RIGHT_PANEL_MIN_W, Math.min(420, Math.floor(wrapWidth * 0.36)));
  const svgWidthBudget = Math.max(360, wrapWidth - rightPanelWidth - RIGHT_PANEL_PAD_LEFT);
  const colsSpace = svgWidthBudget - LEFT_LABEL_W - 24;
  const colW = Math.max(COL_MIN_WIDTH, Math.floor(colsSpace / numCols));
  const svgWidth = LEFT_LABEL_W + colW * numCols + 24;
  const colX = (i) => LEFT_LABEL_W + colW * i + colW / 2;

  // Lines terminate at the row in which the player appears in the departures list.
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

  // Each maximal clique becomes an arc (size 2) or a rounded pill (size ≥3). Arcs
  // are how non-transitive alliances stay honest: a chain A↔B↔C↔D draws three
  // separate arcs, not one fake 4-stack pill, and a player in several cliques
  // contributes to each without pretending A and B are themselves allied.
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
      // A clique implies all members are mutually allied, so one enclosing rectangle
      // is faithful.
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

  // Words ("ally", "break") are dropped: the coloured pill plus emoji conveys the
  // type, and the body text already names the actors.
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

  const phaseLabel = (tag) => t(`chart.alliance.phase.${tag.toLowerCase()}`);
  const reasonLabel = (reason) => {
    const key = `chart.alliance.reason.${slugKey(reason)}`;
    return t.has(key) ? t(key) : reason;
  };

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
                    {ri === 0 && r.sec === 0 ? t('chart.alliance.start') : formatMMSS(r.sec)}
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
                      {phaseLabel(tag)}
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
              const label = cliqueSizes.length >= 2
                ? t('chart.alliance.stackedSizes', { sizes: cliqueSizes.join('v') })
                : t('chart.alliance.stacked');
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
              // SVG can't measure text pre-paint, so approximate the backdrop width.
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
                    title={ev.kind === 'chat' ? t('chart.alliance.chatTitle', { name: ev.data.name, message: ev.data.message }) : undefined}
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
                      {ev.kind === 'stack' ? <span>{t('chart.alliance.stackEvent')}</span> : null}
                      {ev.kind === 'unstack' ? <span>{t('chart.alliance.unstackEvent')}</span> : null}
                      {ev.kind === 'depart' || ev.kind === 'stopped' ? (
                        <>
                          <span style={{ color: playerHexColor(playerByID[ev.data.pid], getPlayerColor) }}>{ev.data.name}</span>
                          <span className="workflow-alliance-event-reason"> ({reasonLabel(ev.data.reason)})</span>
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

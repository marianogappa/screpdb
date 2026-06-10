import React, { useMemo, useState } from 'react';
import { normalizeUnitName } from '../../lib/gameAssets';

// SupplyTimeline overlays each player's cumulative supply provided over the game
// as a thin step line with a dot at every supply addition. It is a descriptive
// view of supply pace — not a skill score (gap size is heavily confounded by map
// economy). Lines stop where a player left / stopped playing (left_second), not
// at game end. Built frontend-only from production_timeline.
//
// Values use the build/morph COMMAND second (commit time, not completion), and
// Overlord/Pylon counts can't be de-duped, so Protoss/Zerg lines may over-count.

// Displayed supply each provider adds. Townhalls (Command Center/Nexus/Hatchery)
// provide supply too and are included. Lair/Hive are morphs of an
// already-counted Hatchery, so they add nothing here.
const SUPPLY_PROVIDERS = {
  supplydepot: 8,
  pylon: 8,
  overlord: 8,
  commandcenter: 10,
  nexus: 9,
  hatchery: 1,
};

// Supply each race starts with (starting base + initial Overlord) — these have
// no Build/morph command, so they're seeded at t=0.
const RACE_START_SUPPLY = { Terran: 10, Protoss: 9, Zerg: 9 };

const FALLBACK_COLORS = ['#60a5fa', '#f87171', '#34d399', '#fbbf24', '#a78bfa', '#f472b6', '#22d3ee', '#fb923c'];

const W = 1000;
const H = 420;
const LABEL_GUTTER = 140;
const M = { left: 44, top: 14, bottom: 32 };
const PLOT_W = W - M.left - LABEL_GUTTER;
const PLOT_H = H - M.top - M.bottom;
const PLOT_RIGHT = M.left + PLOT_W;
const LABEL_LINE_H = 15;

// Non-linear scale params: over-represent the early/low-supply region where
// lines bunch up.
const X_EARLY_SECONDS = 600; // first 10 min...
const X_EARLY_FRAC = 0.6; // ...gets 60% of the plot width
const Y_MIN = 4; // y-axis baseline (just below the starting-supply seed)
const Y_LOW_SUPPLY = 100; // the 4-100 supply band...
const Y_LOW_FRAC = 0.55; // ...gets 55% of the plot height
const DODGE_PX = 2; // per-line vertical separation for overlapping lines

const formatTime = (seconds) => {
  const value = Math.max(0, Math.floor(Number(seconds) || 0));
  return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}`;
};

const num = (v) => Number(v) || 0;

const niceCeil = (v) => {
  if (v <= 20) return 20;
  return Math.ceil(v / 20) * 20;
};

const truncateName = (name, max = 16) => {
  const s = String(name || '');
  return s.length > max ? `${s.slice(0, max - 1)}…` : s;
};

// buildSeries folds a player's supply-provider events into cumulative points
// (seeded at t=0 by race), keeping only additions up to endSec (the player's
// departure or game end). Each point carries what was built and how much it
// added so a dot can be hovered for detail.
const buildSeries = (events, race, endSec) => {
  const adds = [{ sec: 0, delta: RACE_START_SUPPLY[race] || 0, built: 'Starting supply', count: 0 }];
  for (const ev of events || []) {
    const key = normalizeUnitName(ev?.unit_type);
    const value = SUPPLY_PROVIDERS[key];
    if (!value) continue;
    const sec = num(ev?.sec ?? ev?.second);
    if (sec > endSec) continue;
    const count = num(ev?.count) || 1;
    adds.push({ sec, delta: value * count, built: String(ev?.unit_type || ''), count });
  }
  adds.sort((a, b) => a.sec - b.sec);
  let cum = 0;
  const points = adds.map((a) => {
    cum += a.delta;
    return { sec: a.sec, cum, delta: a.delta, built: a.built, count: a.count };
  });
  return { points, total: cum };
};

function SupplyTimeline({ players, timeline, durationSeconds, playerColor }) {
  const duration = Math.max(1, Math.floor(Number(durationSeconds) || 0));
  const [hoveredId, setHoveredId] = useState(null);
  const [tip, setTip] = useState(null);

  const eventsByPlayer = useMemo(() => {
    const map = new Map();
    (timeline || []).forEach((entry) => map.set(entry.player_id, entry.events || []));
    return map;
  }, [timeline]);

  const allSeries = useMemo(() => (
    (players || []).map((player, idx) => {
      const color = playerColor ? playerColor(player.color) : (player.color || FALLBACK_COLORS[idx % FALLBACK_COLORS.length]);
      const left = player.left_second != null ? Math.floor(Number(player.left_second)) : null;
      const endSec = left != null && left > 0 ? Math.min(left, duration) : duration;
      return {
        player,
        color,
        dodgeIdx: idx,
        endSec,
        ...buildSeries(eventsByPlayer.get(player.player_id), player.race, endSec),
      };
    })
  ), [players, eventsByPlayer, playerColor, duration]);

  if (!players || players.length === 0) return null;

  const series = allSeries;
  const nSeries = series.length;

  const maxSupply = niceCeil(Math.max(20, ...series.map((s) => s.total)));

  // Non-linear scales that over-represent the dense early game (lines bunch up
  // in the first ~10 min and the 4-100 supply band). The early time window and
  // the low-supply band each get a larger share of the plot, so overlapping
  // lines spread out where it matters.
  const xBreak = Math.min(X_EARLY_SECONDS, duration);
  const xAt = (sec) => {
    const s = Math.max(0, Math.min(duration, sec));
    if (duration <= xBreak || s <= xBreak) {
      const denom = duration <= xBreak ? duration : xBreak;
      return M.left + (s / denom) * (duration <= xBreak ? 1 : X_EARLY_FRAC) * PLOT_W;
    }
    return M.left + (X_EARLY_FRAC + ((s - xBreak) / (duration - xBreak)) * (1 - X_EARLY_FRAC)) * PLOT_W;
  };

  const yMax = maxSupply;
  const yBreak = Math.min(Y_LOW_SUPPLY, yMax);
  const yFrac = (supply) => {
    const v = Math.max(Y_MIN, Math.min(yMax, supply));
    if (yMax <= yBreak || v <= yBreak) {
      const denom = yMax <= yBreak ? (yMax - Y_MIN) || 1 : (yBreak - Y_MIN) || 1;
      return ((v - Y_MIN) / denom) * (yMax <= yBreak ? 1 : Y_LOW_FRAC);
    }
    return Y_LOW_FRAC + ((v - yBreak) / ((yMax - yBreak) || 1)) * (1 - Y_LOW_FRAC);
  };
  const yAt = (supply) => M.top + PLOT_H - yFrac(supply) * PLOT_H;
  // Small deterministic vertical dodge so perfectly-overlapping lines stay
  // distinguishable (tooltip/labels still report true values).
  const yDodge = (s) => (nSeries > 1 ? (s.dodgeIdx - (nSeries - 1) / 2) * DODGE_PX : 0);

  const xStep = duration <= 600 ? 60 : 120;
  const xTicks = [];
  for (let t = 0; t <= duration; t += xStep) xTicks.push(t);
  const yTicks = [Y_MIN, 50, 100, 150, 200, 250, 300].filter((v, i) => i === 0 || v <= yMax);

  const stepPath = (points, endSec, dodge = 0) => {
    if (!points.length) return '';
    const y = (cum) => yAt(cum) + dodge;
    let d = `M ${xAt(points[0].sec)} ${y(points[0].cum)}`;
    for (let i = 1; i < points.length; i += 1) {
      d += ` L ${xAt(points[i].sec)} ${y(points[i - 1].cum)}`;
      d += ` L ${xAt(points[i].sec)} ${y(points[i].cum)}`;
    }
    d += ` L ${xAt(endSec)} ${y(points[points.length - 1].cum)}`;
    return d;
  };

  // End-of-line labels live in the right gutter, vertically de-cluttered and
  // connected back to each line's endpoint so optics stay clean with overlap.
  const labels = series
    .map((s) => ({
      id: s.player.player_id,
      name: s.player.name,
      isWinner: s.player.is_winner,
      color: s.color,
      endX: xAt(s.endSec),
      endY: yAt(s.total) + yDodge(s),
    }))
    .sort((a, b) => a.endY - b.endY);
  let lastY = -Infinity;
  labels.forEach((l) => {
    l.labelY = Math.max(l.endY, lastY + LABEL_LINE_H);
    lastY = l.labelY;
  });
  // If labels overflow the bottom, push the whole stack up.
  const overflow = labels.length ? labels[labels.length - 1].labelY - (M.top + PLOT_H) : 0;
  if (overflow > 0) labels.forEach((l) => { l.labelY -= overflow; });

  const dim = (id) => hoveredId != null && hoveredId !== id;

  return (
    <div className="workflow-card workflow-card-chat-summary">
      <div className="workflow-section-warning">
        ⚠️ Overlords &amp; Pylons cannot be de-duped effectively, expect some
        inaccuracy. The replay also doesn't track lost supply (e.g. providers
        destroyed in attacks), so totals can exceed 200.
      </div>

      <svg
        width="100%"
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="xMidYMid meet"
        style={{ display: 'block' }}
      >
        {yTicks.map((v) => (
          <g key={`y-${v}`}>
            <line x1={M.left} y1={yAt(v)} x2={PLOT_RIGHT} y2={yAt(v)} stroke="rgba(255,255,255,0.10)" strokeWidth="1" />
            <text x={M.left - 6} y={yAt(v) + 3} textAnchor="end" fill="rgba(255,255,255,0.55)" fontSize="11">{v}</text>
          </g>
        ))}
        {xTicks.map((t) => (
          <g key={`x-${t}`}>
            <line x1={xAt(t)} y1={M.top} x2={xAt(t)} y2={M.top + PLOT_H} stroke="rgba(255,255,255,0.06)" strokeWidth="1" />
            <text x={xAt(t)} y={H - 12} textAnchor="middle" fill="rgba(255,255,255,0.55)" fontSize="11">{formatTime(t)}</text>
          </g>
        ))}

        {series.map((s) => {
          const dimmed = dim(s.player.player_id);
          const isHover = hoveredId === s.player.player_id;
          const dodge = yDodge(s);
          return (
            <g
              key={`line-${s.player.player_id}`}
              opacity={dimmed ? 0.12 : 1}
              onMouseEnter={() => setHoveredId(s.player.player_id)}
              onMouseLeave={() => setHoveredId(null)}
              style={{ cursor: 'pointer' }}
            >
              {/* invisible wide hit area so the thin line is easy to hover */}
              <path d={stepPath(s.points, s.endSec, dodge)} fill="none" stroke="transparent" strokeWidth="14" />
              <path d={stepPath(s.points, s.endSec, dodge)} fill="none" stroke={s.color} strokeWidth={isHover ? 2.5 : 1.5} strokeLinejoin="round" />
              {s.points.map((p, i) => (
                <circle
                  key={`dot-${i}`}
                  cx={xAt(p.sec)}
                  cy={yAt(p.cum) + dodge}
                  r={isHover ? 3 : 2.3}
                  fill={s.color}
                  onMouseEnter={() => setTip({
                    x: xAt(p.sec),
                    y: yAt(p.cum) + dodge,
                    color: s.color,
                    lines: [
                      p.count === 0 ? p.built : `${p.built}${p.count > 1 ? ` ×${p.count}` : ''}  (+${p.delta})`,
                      `Total: ${p.cum} supply`,
                      formatTime(p.sec),
                    ],
                  })}
                  onMouseLeave={() => setTip(null)}
                />
              ))}
            </g>
          );
        })}

        {labels.map((l) => {
          const dimmed = dim(l.id);
          const isHover = hoveredId === l.id;
          return (
            <g
              key={`label-${l.id}`}
              opacity={dimmed ? 0.12 : 1}
              onMouseEnter={() => setHoveredId(l.id)}
              onMouseLeave={() => setHoveredId(null)}
              style={{ cursor: 'pointer' }}
            >
              <path
                d={`M ${l.endX} ${l.endY} L ${PLOT_RIGHT + 4} ${l.labelY}`}
                fill="none"
                stroke={l.color}
                strokeWidth="1"
                opacity="0.4"
              />
              <text
                x={PLOT_RIGHT + 8}
                y={l.labelY + 3}
                fill={l.color}
                fontSize="11"
                fontWeight={isHover ? 700 : 500}
              >
                {l.isWinner ? '👑 ' : ''}{truncateName(l.name)}
              </text>
            </g>
          );
        })}

        {tip ? (() => {
          const w = Math.max(...tip.lines.map((ln) => ln.length)) * 6.1 + 16;
          const h = tip.lines.length * 14 + 8;
          let bx = tip.x + 10;
          let by = tip.y - h - 8;
          if (bx + w > W) bx = tip.x - w - 10;
          if (by < M.top) by = tip.y + 10;
          return (
            <g pointerEvents="none">
              <rect x={bx} y={by} width={w} height={h} rx={4} fill="rgba(12,15,24,0.96)" stroke={tip.color} strokeWidth="1" />
              {tip.lines.map((ln, i) => (
                <text
                  key={i}
                  x={bx + 8}
                  y={by + 16 + i * 14}
                  fill={i === 0 ? tip.color : 'rgba(255,255,255,0.9)'}
                  fontSize="11"
                  fontWeight={i === 0 ? 700 : 400}
                >
                  {ln}
                </text>
              ))}
            </g>
          );
        })() : null}
      </svg>
    </div>
  );
}

export default SupplyTimeline;

import React, { useMemo, useState } from 'react';
import { normalizeUnitName } from '../../lib/gameAssets';

// SupplyTimeline charts each player's cumulative supply provided over the whole
// game as an overlaid step line, and annotates the flat stretches (no supply
// added) longer than a threshold with their duration in seconds — a "supply
// block / supply pace" comparison. Built frontend-only from production_timeline
// (the same per-event stream the Army timeline replays).
//
// Caveats surfaced in the UI: values use the build/morph COMMAND second (when
// supply was committed, not when it finished), and Overlord/Pylon counts are
// not de-duped (see builddedup — only Terran depots are), so Protoss/Zerg lines
// can over-count. This is a labs/experimental view.

// Displayed supply each provider adds. Lair/Hive are morphs of an
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
const THRESHOLDS = [15, 25, 40, 60];
const DEFAULT_THRESHOLD = 25;

const W = 1000;
const H = 440;
const M = { left: 48, right: 16, top: 16, bottom: 36 };
const PLOT_W = W - M.left - M.right;
const PLOT_H = H - M.top - M.bottom;

const formatTime = (seconds) => {
  const value = Math.max(0, Math.floor(Number(seconds) || 0));
  return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}`;
};

const num = (v) => Number(v) || 0;

const niceCeil = (v) => {
  if (v <= 20) return 20;
  return Math.ceil(v / 20) * 20;
};

// buildSeries folds a player's supply-provider events into a sorted list of
// cumulative points (seeded at t=0 by race), plus the gaps between additions.
const buildSeries = (events, race, duration) => {
  const adds = [{ sec: 0, delta: RACE_START_SUPPLY[race] || 0 }];
  for (const ev of events || []) {
    const key = normalizeUnitName(ev?.unit_type);
    const value = SUPPLY_PROVIDERS[key];
    if (!value) continue;
    adds.push({ sec: num(ev?.sec ?? ev?.second), delta: value * (num(ev?.count) || 1) });
  }
  adds.sort((a, b) => a.sec - b.sec);
  let cum = 0;
  const points = adds.map((a) => {
    cum += a.delta;
    return { sec: a.sec, cum };
  });
  // Gaps: stretch between one addition's second and the next (and from the last
  // addition to game end). prevCum is the supply held flat during the gap.
  const gaps = [];
  for (let i = 0; i < points.length; i += 1) {
    const start = points[i].sec;
    const end = i + 1 < points.length ? points[i + 1].sec : duration;
    if (end - start > 0) gaps.push({ start, end, durationSec: end - start, cum: points[i].cum });
  }
  return { points, gaps, total: cum };
};

const scoreTier = (score) => {
  if (score >= 70) return { label: 'disciplined', color: 'var(--color-text-success, #34d399)' };
  if (score >= 55) return { label: 'good', color: 'var(--color-text-success, #34d399)' };
  if (score >= 45) return { label: 'average', color: 'var(--color-text-secondary)' };
  if (score >= 30) return { label: 'below average', color: 'var(--color-text-warning, #fbbf24)' };
  return { label: 'leaky', color: 'var(--color-text-danger, #f87171)' };
};

function SupplyTimeline({ players, timeline, durationSeconds, hasTeamInfo, teamColorRgba, discipline = [] }) {
  const duration = Math.max(1, Math.floor(Number(durationSeconds) || 0));
  const [threshold, setThreshold] = useState(DEFAULT_THRESHOLD);

  const eventsByPlayer = useMemo(() => {
    const map = new Map();
    (timeline || []).forEach((entry) => map.set(entry.player_id, entry.events || []));
    return map;
  }, [timeline]);

  const disciplineById = useMemo(() => {
    const m = new Map();
    (discipline || []).forEach((d) => m.set(d.player_id, d));
    return m;
  }, [discipline]);

  const series = useMemo(() => (
    (players || []).map((player, idx) => {
      const color = hasTeamInfo ? teamColorRgba(player.team, 0.95) : FALLBACK_COLORS[idx % FALLBACK_COLORS.length];
      return {
        player,
        color,
        ...buildSeries(eventsByPlayer.get(player.player_id), player.race, duration),
      };
    })
  ), [players, eventsByPlayer, hasTeamInfo, teamColorRgba, duration]);

  if (!players || players.length === 0) return null;

  const maxSupply = niceCeil(Math.max(20, ...series.map((s) => s.total)));
  const xAt = (sec) => M.left + (Math.max(0, Math.min(duration, sec)) / duration) * PLOT_W;
  const yAt = (supply) => M.top + PLOT_H - (Math.max(0, supply) / maxSupply) * PLOT_H;

  const xStep = duration <= 600 ? 60 : duration <= 1800 ? 180 : 300;
  const xTicks = [];
  for (let t = 0; t <= duration; t += xStep) xTicks.push(t);
  const yTicks = [0, 0.25, 0.5, 0.75, 1].map((f) => Math.round(maxSupply * f));

  const stepPath = (points) => {
    if (!points.length) return '';
    let d = `M ${xAt(points[0].sec)} ${yAt(points[0].cum)}`;
    for (let i = 1; i < points.length; i += 1) {
      d += ` L ${xAt(points[i].sec)} ${yAt(points[i - 1].cum)}`;
      d += ` L ${xAt(points[i].sec)} ${yAt(points[i].cum)}`;
    }
    d += ` L ${xAt(duration)} ${yAt(points[points.length - 1].cum)}`;
    return d;
  };

  return (
    <div className="workflow-card workflow-card-chat-summary">
      <div className="workflow-section-info" role="note">
        🧪 Experimental. Cumulative supply provided over the game, one line per
        player. Flat stretches longer than the threshold are marked with their
        length in seconds — long gaps mean a slow supply (likely supply-blocked).
        Uses the build/morph <em>command</em> second (commit time, not completion).
      </div>

      {(discipline || []).length > 0 ? (
        <div style={{ display: 'grid', gridTemplateColumns: `repeat(auto-fit, minmax(180px, 1fr))`, gap: 12, marginBottom: 16 }}>
          {players.map((player, idx) => {
            const d = disciplineById.get(player.player_id);
            const color = hasTeamInfo ? teamColorRgba(player.team, 0.95) : FALLBACK_COLORS[idx % FALLBACK_COLORS.length];
            return (
              <div key={player.player_id} style={{ background: 'var(--color-background-secondary, rgba(255,255,255,0.04))', borderRadius: 8, padding: '10px 12px' }}>
                <div style={{ fontSize: 12, opacity: 0.75, display: 'flex', alignItems: 'center', gap: 6 }}>
                  <span style={{ width: 9, height: 9, borderRadius: 2, background: color }} />
                  {player.is_winner ? '👑 ' : ''}{player.name}
                </div>
                {d && d.eligible ? (
                  <>
                    <div style={{ fontSize: 22, fontWeight: 600, color: scoreTier(d.score).color }}>
                      {d.score}<span style={{ fontSize: 12, opacity: 0.6 }}>/100</span>
                      <span style={{ fontSize: 12, marginLeft: 6, color: scoreTier(d.score).color }}>{scoreTier(d.score).label}</span>
                    </div>
                    <div style={{ fontSize: 11, opacity: 0.7 }}>
                      avg early gap {Math.round(d.weighted_gap_sec)}s · typical {Math.round(d.typical_gap_sec)}s
                    </div>
                  </>
                ) : (
                  <div style={{ fontSize: 12, opacity: 0.5 }}>not enough supply data</div>
                )}
              </div>
            );
          })}
        </div>
      ) : null}

      <div className="workflow-production-top-row" style={{ alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <div className="workflow-radio-group" role="radiogroup" aria-label="Gap threshold">
          <span style={{ opacity: 0.7, fontSize: 12, alignSelf: 'center', marginRight: 4 }}>Mark gaps ≥</span>
          {THRESHOLDS.map((t) => (
            <label key={t} className="workflow-radio-option">
              <input
                type="radio"
                name="supply-gap-threshold"
                value={t}
                checked={threshold === t}
                onChange={() => setThreshold(t)}
              />
              <span>{t}s</span>
            </label>
          ))}
        </div>
        <div className="workflow-section-warning" style={{ marginLeft: 'auto' }}>
          ⚠️ Overlord/Pylon counts are not de-duped — Protoss/Zerg lines may over-count.
        </div>
      </div>

      <svg
        width="100%"
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="xMidYMid meet"
        style={{ display: 'block' }}
      >
        {yTicks.map((v) => (
          <g key={`y-${v}`}>
            <line x1={M.left} y1={yAt(v)} x2={W - M.right} y2={yAt(v)} stroke="rgba(255,255,255,0.10)" strokeWidth="1" />
            <text x={M.left - 6} y={yAt(v) + 3} textAnchor="end" fill="rgba(255,255,255,0.55)" fontSize="11">{v}</text>
          </g>
        ))}
        {xTicks.map((t) => (
          <g key={`x-${t}`}>
            <line x1={xAt(t)} y1={M.top} x2={xAt(t)} y2={M.top + PLOT_H} stroke="rgba(255,255,255,0.06)" strokeWidth="1" />
            <text x={xAt(t)} y={H - 12} textAnchor="middle" fill="rgba(255,255,255,0.55)" fontSize="11">{formatTime(t)}</text>
          </g>
        ))}

        {series.map((s) => (
          <g key={`line-${s.player.player_id}`}>
            <path d={stepPath(s.points)} fill="none" stroke={s.color} strokeWidth="2" strokeLinejoin="round" />
            {s.gaps.filter((g) => g.durationSec >= threshold).map((g) => {
              const midX = xAt((g.start + g.end) / 2);
              const lineY = yAt(g.cum);
              return (
                <g key={`gap-${s.player.player_id}-${g.start}`}>
                  <line x1={xAt(g.start)} y1={lineY} x2={xAt(g.end)} y2={lineY} stroke={s.color} strokeWidth="4" opacity="0.35" />
                  <text x={midX} y={lineY - 5} textAnchor="middle" fill={s.color} fontSize="10" fontWeight="700">
                    {g.durationSec}s
                  </text>
                </g>
              );
            })}
          </g>
        ))}
      </svg>

      <div className="table-container">
        <table className="data-table workflow-table">
          <thead>
            <tr>
              <th>Player</th>
              <th>Supply provided</th>
              <th>Gaps ≥ {threshold}s</th>
              <th>Largest gap</th>
            </tr>
          </thead>
          <tbody>
            {series.map((s) => {
              const flagged = s.gaps.filter((g) => g.durationSec >= threshold);
              const largest = s.gaps.reduce((mx, g) => Math.max(mx, g.durationSec), 0);
              return (
                <tr key={`stat-${s.player.player_id}`}>
                  <td>
                    <span style={{ display: 'inline-block', width: 10, height: 10, borderRadius: 2, background: s.color, marginRight: 6 }} />
                    {s.player.is_winner ? '👑 ' : ''}{s.player.name}
                  </td>
                  <td>{s.total}</td>
                  <td>{flagged.length}</td>
                  <td>{largest}s</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default SupplyTimeline;

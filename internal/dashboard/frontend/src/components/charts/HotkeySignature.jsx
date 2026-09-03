import React from 'react';
import { getUnitIcon } from '../../lib/gameAssets';
import { hotkeyBuildingIcon, hotkeyCountColor } from './HotkeyTimeline';

// Hotkey signature cards: one per race with enough games. Each key shows the
// sprite of what it held over game minutes, an assign bar (when the key is
// set) and a use bar (when it is pressed), aggregated across games.

const MAX_MINUTE = 24;

const ASSIGN_COLOR = '#c98500';
const USE_COLOR = '#4a79a8';

const CATEGORY_LABEL = {
  hall: 'town hall',
  prod: 'production',
  tech: 'tech',
  comsat: 'Comsat',
  units: 'units',
};

const CATEGORY_SPRITE = {
  hall: { Zerg: 'Hatchery', Terran: 'Command Center', Protoss: 'Nexus' },
  prod: { Zerg: 'Spire', Terran: 'Factory', Protoss: 'Gateway' },
  tech: { Zerg: 'Evolution Chamber', Terran: 'Academy', Protoss: 'Templar Archives' },
  comsat: { Zerg: 'Comsat', Terran: 'Comsat', Protoss: 'Comsat' },
  units: { Zerg: 'Zergling', Terran: 'Marine', Protoss: 'Zealot' },
};

const categoryIcon = (category, race) => {
  const name = (CATEGORY_SPRITE[category] || {})[race];
  if (!name) return null;
  return category === 'units' ? getUnitIcon(name) : hotkeyBuildingIcon(name);
};

const KEY_ORDER = [1, 2, 3, 4, 5, 6, 7, 8, 9, 0];

const pct = (minute) => (minute / (MAX_MINUTE + 1)) * 100;
const widthPct = (run) => ((run.end_min - run.start_min + 1) / (MAX_MINUTE + 1)) * 100;

function SignatureCard({ card }) {
  const keysByNumber = new Map((card.keys || []).map((k) => [k.key, k]));
  return (
    <div className="hk-sig-card">
      {KEY_ORDER.map((n) => {
        const key = keysByNumber.get(n);
        if (!key || !(key.runs || []).length) return null;
        return (
          <div key={n} className="hk-sig-row">
            <div className="hk-keycap hk-sig-keycap">{n}</div>
            <div className="hk-sig-lane">
              {key.runs.map((run, i) => {
                const icon = categoryIcon(run.category, card.race);
                if (!icon) return null;
                return (
                  <img
                    key={i}
                    src={icon}
                    alt={CATEGORY_LABEL[run.category] || run.category}
                    className={`hk-sig-sprite ${run.category === 'units' ? 'hk-silhouette' : ''}`}
                    style={{ left: `${pct(run.start_min)}%` }}
                  />
                );
              })}
              {key.runs.length && key.runs[0].category === 'units' && key.median_count > 0 ? (
                <i
                  className="hk-count hk-sig-count"
                  style={{ left: `${pct(key.runs[0].start_min)}%`, background: hotkeyCountColor(key.median_count) }}
                >
                  {key.median_count}
                </i>
              ) : null}
              {(key.assign_runs || []).map((run, i) => (
                <span
                  key={`a${i}`}
                  className="hk-sig-bar hk-sig-bar-assign"
                  data-tip="Assign"
                  style={{ left: `${pct(run.start_min)}%`, width: `${widthPct(run)}%` }}
                />
              ))}
              {key.runs.map((run, i) => (
                <span
                  key={`u${i}`}
                  className="hk-sig-bar hk-sig-bar-use"
                  data-tip="Use"
                  style={{ left: `${pct(run.start_min)}%`, width: `${widthPct(run)}%` }}
                />
              ))}
            </div>
          </div>
        );
      })}
      <div className="hk-sig-axis">
        <div />
        <div className="hk-sig-axis-in">
          {[0, 5, 10, 15, 20].map((m) => (
            <span key={m} style={{ left: `${pct(m)}%` }}>{m}m</span>
          ))}
        </div>
      </div>
    </div>
  );
}

export default function HotkeySignature({ payload, loadingNotice = '' }) {
  const cards = payload?.cards || [];
  if (!cards.length) {
    return (
      <div className="chart-empty">
        {loadingNotice || 'Not enough games for a hotkey signature. It needs at least 3 games of one race.'}
      </div>
    );
  }
  return (
    <div className="hk-sigs">
      {cards.map((card) => <SignatureCard key={card.race} card={card} />)}
    </div>
  );
}

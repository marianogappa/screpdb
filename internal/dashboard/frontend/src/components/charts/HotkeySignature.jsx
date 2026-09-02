import React from 'react';
import { getUnitIcon } from '../../lib/gameAssets';
import { hotkeyBuildingIcon, hotkeyCountColor } from './HotkeyTimeline';

// Hotkey signature cards: one per race with enough games. Each key is a
// ribbon over game minutes - a band per stretch of stable content, with a
// sprite drawn where the content first appears and at each repurpose.

const MAX_MINUTE = 24;

const CATEGORY_COLORS = {
  hall: '#3987e5',
  prod: '#d95926',
  tech: '#199e70',
  comsat: '#c98500',
  units: '#8b98a4',
};

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

function SignatureCard({ card }) {
  const keysByNumber = new Map((card.keys || []).map((k) => [k.key, k]));
  const scorePct = card.temporal_score ? Math.round(card.temporal_score * 100) : null;
  return (
    <div className="hk-sig-card">
      <div className="hk-sig-head">
        <span className={`hk-race hk-race-${(card.race || '')[0] || 'u'}`}>{card.race}</span>
        <span className="hk-sig-games">{card.games} games</span>
        {scorePct !== null ? (
          <span className="hk-sig-score" title="Share of key presses explained by the per-minute modal content of their key">
            {scorePct}%
          </span>
        ) : null}
      </div>
      <div className="hk-sig-keys">
        {KEY_ORDER.map((n) => {
          const key = keysByNumber.get(n);
          if (!key || !(key.runs || []).length) return null;
          return (
            <div key={n} className="hk-sig-row">
              <div className="hk-keycap">{n}</div>
              <div className="hk-sig-ribbon">
                {key.runs.map((run, i) => {
                  const left = (run.start_min / (MAX_MINUTE + 1)) * 100;
                  const width = ((run.end_min - run.start_min + 1) / (MAX_MINUTE + 1)) * 100;
                  const title = `min ${run.start_min}–${run.end_min}: ${CATEGORY_LABEL[run.category] || run.category} · ${Math.round(run.share * 100)}% of ${run.presses} presses`;
                  const icon = categoryIcon(run.category, card.race);
                  return (
                    <React.Fragment key={i}>
                      <span
                        className="hk-sig-band"
                        title={title}
                        style={{
                          left: `${left}%`,
                          width: `${width}%`,
                          background: CATEGORY_COLORS[run.category] || '#666',
                          opacity: (0.3 + 0.6 * run.share).toFixed(2),
                        }}
                      />
                      {icon ? (
                        <img
                          src={icon}
                          alt={CATEGORY_LABEL[run.category] || run.category}
                          title={title}
                          className={`hk-sig-sprite ${run.category === 'units' ? 'hk-silhouette' : ''}`}
                          style={{ left: `${left}%` }}
                        />
                      ) : null}
                      {run.category === 'units' && i === 0 && key.median_count > 0 ? (
                        <i
                          className="hk-count hk-sig-count"
                          style={{ left: `${left}%`, background: hotkeyCountColor(key.median_count) }}
                        >
                          {key.median_count}
                        </i>
                      ) : null}
                    </React.Fragment>
                  );
                })}
              </div>
              <div className="hk-sig-seq">
                {key.runs.length > 1
                  ? key.runs.map((r) => `${CATEGORY_LABEL[r.category]}@${r.start_min}m`).join(' → ')
                  : `${CATEGORY_LABEL[key.runs[0].category]} · ${key.uses} uses`}
              </div>
            </div>
          );
        })}
      </div>
      <div className="hk-sig-axis">
        <div />
        <div className="hk-sig-axis-in">
          {[0, 5, 10, 15, 20].map((m) => (
            <span key={m} style={{ left: `${(m / (MAX_MINUTE + 1)) * 100}%` }}>{m}m</span>
          ))}
        </div>
        <div />
      </div>
      {card.prose ? <p className="hk-sig-prose">{card.prose}</p> : null}
    </div>
  );
}

export default function HotkeySignature({ payload }) {
  const cards = payload?.cards || [];
  const gamesByRace = payload?.games_by_race || {};
  const skipped = Object.entries(gamesByRace)
    .filter(([race, games]) => games < 3 && !cards.some((c) => c.race === race))
    .map(([race, games]) => `${race} (${games} game${games === 1 ? '' : 's'})`);
  if (!cards.length) {
    return (
      <div className="chart-empty">
        Not enough games for a hotkey signature - it needs at least 3 stored games of a race
        {skipped.length ? ` (seen: ${skipped.join(', ')})` : ''}. Re-analyze old replays to backfill
        hotkey streams.
      </div>
    );
  }
  return (
    <div className="hk-sigs">
      <div className="hk-maps-note">
        Each key is a ribbon over game minutes: what presses of that key selected, aggregated across
        games. Sprites mark where the content first appears and each repurpose; band opacity is how
        unanimous the games are.
      </div>
      {cards.map((card) => <SignatureCard key={card.race} card={card} />)}
      {skipped.length ? (
        <div className="hk-maps-note">No card for {skipped.join(', ')} - needs 3+ games per race.</div>
      ) : null}
    </div>
  );
}

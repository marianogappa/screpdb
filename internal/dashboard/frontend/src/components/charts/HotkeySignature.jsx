import React from 'react';
import { getUnitIcon } from '../../lib/gameAssets';
import { hotkeyBuildingIcon, hotkeyCountColor } from './HotkeyTimeline';
import { useT } from '../../lib/i18nContext';
import { unitsName } from '../../lib/turningPoint';

// Hotkey signature cards: one per race with enough games. Each key shows the
// sprite of what it held over game minutes, an assign bar (when the key is
// set) and a use bar (when it is pressed), aggregated across games.

const MAX_MINUTE = 24;

const ASSIGN_COLOR = '#c98500';
const USE_COLOR = '#4a79a8';

const CATEGORY_LABEL_KEYS = {
  hall: 'chart.hotkey.category.hall',
  prod: 'chart.hotkey.category.prod',
  tech: 'chart.hotkey.category.tech',
  comsat: 'chart.hotkey.category.comsat',
  units: 'chart.hotkey.category.units',
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

// A units run names its main unit when the engine saw the assigns; without it, a generic silhouette.
const runIcon = (run, race) => (run.category === 'units' && run.unit ? getUnitIcon(run.unit) : categoryIcon(run.category, race));

const KEY_ORDER = [1, 2, 3, 4, 5, 6, 7, 8, 9, 0];

const pct = (minute) => (minute / (MAX_MINUTE + 1)) * 100;
const widthPct = (run) => ((run.end_min - run.start_min + 1) / (MAX_MINUTE + 1)) * 100;

const ODDITY_ICON = {
  hall_key: '🏠',
  worker_key: '⛏️',
  few_keys: '✋',
  rebinds: '🔁',
  fast_presses: '⚡',
  opening: '🎬',
  shift_adds: '➕',
  unit_key: '⚔️',
  prod_key: '🏭',
  hotkey_share: '⌨️',
  deselects: '✂️',
  tech_binds: '🔬',
  few_hall_binds: '🏠',
  first_supply: '🔌',
  comsat_keys: '📡',
  rare_unit: '👁️',
  standard_layout: '📐',
};

const shownNumber = (value, tenths) => {
  if (!tenths) return value;
  const n = value / 10;
  return Number.isInteger(n) ? n : n.toFixed(1);
};

const HALL_BUILDINGS = new Set(['Nexus', 'Command Center', 'Hatchery', 'Lair', 'Hive']);

// openingPhrase groups consecutive binds of the same thing: "the Nexus to 1 and 3, then units to 2".
function openingPhrase(t, binds, race) {
  const groups = [];
  binds.forEach((b) => {
    const what = b.building || '';
    const last = groups[groups.length - 1];
    if (last && last.what === what) last.keys.push(b.key);
    else groups.push({ what, keys: [b.key] });
  });
  return groups.map((g) => {
    const keys = g.keys.length > 1
      ? `${g.keys.slice(0, -1).join(t('chart.hotkey.odd.comma'))}${t('chart.hotkey.odd.and')}${g.keys[g.keys.length - 1]}`
      : String(g.keys[0]);
    if (!g.what) return t('chart.hotkey.odd.bindUnits', { keys });
    const building = HALL_BUILDINGS.has(g.what) ? t(`chart.hotkey.odd.noun.hall.${race}`) : g.what;
    return t('chart.hotkey.odd.bindBuilding', { building, keys });
  }).join(t('chart.hotkey.odd.then'));
}

// fill puts nodes into a translated template's {placeholders}, so pro names can be set apart from the words
// around them: a pro called Free or Shuttle must not read as part of the sentence.
const fill = (template, nodes) => template.split(/(\{\w+\})/).map((part, i) => {
  const name = part.match(/^\{(\w+)\}$/)?.[1];
  return name && name in nodes ? <React.Fragment key={i}>{nodes[name]}</React.Fragment> : part;
});

function oddityText(t, oddity, race) {
  const raceName = t(`chart.hotkey.odd.race.${race}`);
  const countKinds = new Set(['tech_binds', 'few_hall_binds']);
  let statementKey = `chart.hotkey.odd.${oddity.kind}`;
  if (countKinds.has(oddity.kind)) statementKey += { 1: '.one', 2: '.two' }[oddity.value] || '.other';
  if (oddity.kind === 'rare_unit' && oddity.from_minute) statementKey += 'From';
  const statement = t(statementKey, {
    key: oddity.key ?? oddity.value,
    count: shownNumber(oddity.value, oddity.tenths),
    rank: oddity.rank,
    total: oddity.population,
    race: t(`chart.hotkey.odd.race.${race}`),
    keys: (oddity.keys || []).length > 1
      ? `${oddity.keys.slice(0, -1).join(t('chart.hotkey.odd.comma'))}${t('chart.hotkey.odd.and')}${oddity.keys[oddity.keys.length - 1]}`
      : String((oddity.keys || [])[0] ?? ''),
    building: oddity.kind === 'first_supply' ? t(`chart.hotkey.odd.noun.supply.${race}`) : '',
    examples: t(`chart.hotkey.odd.noun.tech.${race}`),
    from: oddity.from_minute,
    hall: t(`chart.hotkey.odd.noun.hall.${race}`),
    workers: t(`chart.hotkey.odd.noun.workers.${race}`),
    units: oddity.unit ? unitsName(t, oddity.unit) : '',
    binds: oddity.binds ? openingPhrase(t, oddity.binds, race) : '',
  });
  const shared = oddity.shared_with || [];
  let context;
  if (oddity.kind === 'standard_layout') context = t('chart.hotkey.odd.standardLayoutContext');
  else if (oddity.kind === 'first_supply' || oddity.kind === 'comsat_keys') context = t('chart.hotkey.odd.fractionContext', { count: (oddity.shared_count || 0) + 1, total: oddity.population, race: raceName });
  else if (oddity.kind === 'rare_unit') context = oddity.shared_count ? t('chart.hotkey.odd.rareUnitShared', { race: raceName }) : t('chart.hotkey.odd.rareUnitUnique', { race: raceName });
  else if (oddity.rank && oddity.kind === 'few_hall_binds') context = t(`chart.hotkey.odd.${oddity.rank === 1 ? 'fewest' : 'rankLow'}${oddity.tied ? 'Tied' : ''}Context`, { rank: oddity.rank, count: oddity.population, race: raceName, median: shownNumber(oddity.baseline, oddity.tenths) });
  else if (oddity.rank && oddity.kind === 'hotkey_share') context = t(oddity.tied ? 'chart.hotkey.odd.rankPercentTiedContext' : 'chart.hotkey.odd.rankPercentContext', { rank: oddity.rank, count: oddity.population, race: raceName, median: oddity.baseline });
  else if (oddity.kind === 'opening') context = t('chart.hotkey.odd.openingContext', { count: oddity.value, race: raceName });
  else if (oddity.kind === 'shift_adds') context = t('chart.hotkey.odd.shiftAddsContext', { race: raceName });
  else if (oddity.norm_key != null) context = t(`chart.hotkey.odd.${oddity.kind}${oddity.norm_majority ? 'NormMost' : 'NormOften'}`, { race: raceName, norm: oddity.norm_key });
  else if (oddity.kind === 'few_keys') context = t('chart.hotkey.odd.fewKeysContext', { race: raceName, count: oddity.baseline });
  else if (oddity.rank) context = t(oddity.tied ? 'chart.hotkey.odd.rankTiedContext' : 'chart.hotkey.odd.rankContext', { rank: oddity.rank, count: oddity.population, race: raceName, median: shownNumber(oddity.baseline, oddity.tenths) });
  else if (shared.length === 1) context = fill(t('chart.hotkey.odd.sharedWith'), { name: <b>{shared[0]}</b> });
  else if (shared.length > 1) context = fill(t('chart.hotkey.odd.sharedWithTwo'), { a: <b>{shared[0]}</b>, b: <b>{shared[1]}</b> });
  else if (oddity.shared_count) context = t.plural('chart.hotkey.odd.sharedCount', oddity.shared_count, { race: raceName });
  else context = t('chart.hotkey.odd.unique', { race: raceName });
  return { statement, context };
}

function OddityBullet({ oddity }) {
  return <span className="hk-sig-bullet" aria-hidden="true">{ODDITY_ICON[oddity.kind] || '•'}</span>;
}

function TwinMatch({ twin, onOpenPlayer }) {
  const t = useT();
  const who = (
    <span className="hk-sig-twin-who">
      {twin.photo_url
        ? <img className="pro-photo" src={twin.photo_url} alt="" loading="lazy" />
        : <span className="pro-photo-placeholder" aria-hidden="true">★</span>}
      <span className="hk-sig-twin-name">{twin.player_name}</span>
    </span>
  );
  return (
    <div className="hk-sig-twin">
      <span className="hk-sig-note-quiet">{t('chart.hotkey.twin')}</span>
      {onOpenPlayer ? (
        <button type="button" className="hk-sig-twin-btn" onClick={() => onOpenPlayer(twin.player_key, { initialPlayerTab: 'hotkeys' })}>{who}</button>
      ) : who}
    </div>
  );
}

function SignatureHeader({ card, twin, oddities, rates, onOpenPlayer }) {
  const t = useT();
  if (!twin && !oddities?.length && !rates) return null;
  return (
    <div className="hk-sig-head">
      <div className="hk-sig-head-main">
        <SignatureStats rates={rates} />
        {oddities?.length ? (
          <ul className="hk-sig-oddities">
            {oddities.map((oddity) => {
              const { statement, context } = oddityText(t, oddity, card.race);
              return (
                <li key={`${oddity.kind}-${oddity.key ?? oddity.value}`}>
                  <OddityBullet oddity={oddity} />
                  <span>{statement} <span className="hk-sig-note-quiet">{context}</span></span>
                </li>
              );
            })}
          </ul>
        ) : null}
      </div>
      {twin ? <TwinMatch twin={twin} onOpenPlayer={onOpenPlayer} /> : null}
    </div>
  );
}

function SignatureStats({ rates }) {
  const t = useT();
  if (!rates) return null;
  const stats = [
    { value: rates.presses_per_minute, label: t('chart.hotkey.pressesPerMinute'), tip: t('chart.hotkey.pressesPerMinuteTip') },
    { value: rates.rebinds_per_minute, label: t('chart.hotkey.rebindsPerMinute'), tip: t('chart.hotkey.rebindsPerMinuteTip') },
  ];
  return (
    <div className="hk-sig-stats">
      {stats.map((s) => (
        <span key={s.label} className="hk-sig-stat" title={s.tip}>
          <b>{Math.round(s.value)}</b> {s.label}
        </span>
      ))}
    </div>
  );
}

const sharePercent = (uses, total) => {
  if (!uses || !total) return '';
  const pct = (uses / total) * 100;
  return pct < 1 ? '<1%' : `${Math.round(pct)}%`;
};

function SignatureCard({ card, twin, oddities, rates, onOpenPlayer }) {
  const t = useT();
  const keysByNumber = new Map((card.keys || []).map((k) => [k.key, k]));
  const totalUses = (card.keys || []).reduce((sum, k) => sum + (k.uses || 0), 0);
  return (
    <div className="hk-sig-card">
      <SignatureHeader card={card} twin={twin} oddities={oddities} rates={rates} onOpenPlayer={onOpenPlayer} />
      {KEY_ORDER.map((n) => {
        const key = keysByNumber.get(n);
        if (!key || !(key.runs || []).length) return null;
        return (
          <div key={n} className="hk-sig-row">
            <div className="hk-keycap hk-sig-keycap">{n}</div>
            <div className="hk-sig-lane">
              {key.runs.map((run, i) => {
                const icon = runIcon(run, card.race);
                if (!icon) return null;
                const label = run.unit || (CATEGORY_LABEL_KEYS[run.category] ? t(CATEGORY_LABEL_KEYS[run.category]) : run.category);
                return (
                  <React.Fragment key={i}>
                    <img
                      src={icon}
                      alt={label}
                      title={label}
                      className={`hk-sig-sprite ${run.category === 'units' && !run.unit ? 'hk-silhouette' : ''}`}
                      style={{ left: `${pct(run.start_min)}%` }}
                    />
                    {run.unit && run.median_count > 0 ? (
                      <i className="hk-count hk-sig-count" style={{ left: `${pct(run.start_min)}%`, background: hotkeyCountColor(run.median_count) }}>
                        {run.median_count}
                      </i>
                    ) : null}
                  </React.Fragment>
                );
              })}
              {key.runs.length && key.runs[0].category === 'units' && !key.runs[0].unit && key.median_count > 0 ? (
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
                  data-tip={t('chart.hotkey.assignTip')}
                  style={{ left: `${pct(run.start_min)}%`, width: `${widthPct(run)}%` }}
                />
              ))}
              {key.runs.map((run, i) => (
                <span
                  key={`u${i}`}
                  className="hk-sig-bar hk-sig-bar-use"
                  data-tip={t('chart.hotkey.useTip')}
                  style={{ left: `${pct(run.start_min)}%`, width: `${widthPct(run)}%` }}
                />
              ))}
            </div>
            <div className="hk-sig-share" title={t('chart.hotkey.shareTip')}>{sharePercent(key.uses, totalUses)}</div>
          </div>
        );
      })}
      <div className="hk-sig-axis">
        <div />
        <div className="hk-sig-axis-in">
          {[0, 5, 10, 15, 20].map((m) => (
            <span key={m} style={{ left: `${pct(m)}%` }}>{t('chart.hotkey.minuteTick', { minute: m })}</span>
          ))}
        </div>
      </div>
    </div>
  );
}

export default function HotkeySignature({ payload, loadingNotice = '', onOpenPlayer = null }) {
  const t = useT();
  const cards = payload?.cards || [];
  if (!cards.length) {
    return (
      <div className="chart-empty">
        {loadingNotice || t('chart.hotkey.signatureEmpty')}
      </div>
    );
  }
  return (
    <div className="hk-sigs">
      {cards.map((card) => <SignatureCard key={card.race} card={card} twin={payload?.twins?.[card.race]} oddities={payload?.oddities?.[card.race]} rates={payload?.rates?.[card.race]} onOpenPlayer={onOpenPlayer} />)}
    </div>
  );
}

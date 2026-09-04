// FilterOmnibar is the games-list filter surface: one input that reaches every
// filter, plus a browse row of axis blocks for people who would rather look
// than type.
//
// It replaces two <select> dropdowns and forty identically-styled pills. That
// bar showed 40 of the 122 options that actually exist and hid all 67 build
// orders behind three disclosure pills, so it was congested AND incomplete at
// the same time. Typing scales to 122 options; a pill bar does not.
//
// Two things worth knowing before editing:
//
//  1. Filter semantics are OR within a state key and AND across state keys
//     (see BuildWorkflowGamesListWhere). Chips are therefore grouped by STATE
//     KEY, not by display axis: build orders and tactics both live in
//     `featuring`, so they are OR-ed together and must render as one group.
//     Splitting them into two "and"-joined groups would misstate the query.
//     The menu and browse row still split them, because that is navigation and
//     carries no semantics.
//
//  2. Counts come from the API and are complete for every axis, so once the
//     replay library has finished loading a zero is information: that option
//     matches nothing in this corpus. Those options rank last and render
//     dimmed, because a filter that returns an empty list is a dead end worth
//     seeing but not worth trying first. While the library is still loading
//     (the `loading` prop) a zero only means "nothing yet", so nothing is
//     demoted or dimmed until the corpus is complete.

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { getUnitIcon } from '../lib/gameAssets';
import { t, useLocale, useT } from '../lib/i18nContext';

// Display axes, in menu order. `state` is the mainGamesFilters key the axis
// writes to; `source` is the filter_options field it reads.
const defaultAxes = () => [
  { id: 'length', label: t('omnibar.axis.length'), state: 'duration', source: 'durations' },
  { id: 'mapKind', label: t('omnibar.axis.mapKind'), state: 'mapKind', source: 'map_kinds' },
  { id: 'matchup', label: t('omnibar.axis.matchup'), state: 'matchup', source: 'matchups' },
  { id: 'map', label: t('omnibar.axis.map'), state: 'map', source: 'maps' },
  { id: 'player', label: t('omnibar.axis.player'), state: 'player', source: 'players' },
  { id: 'bo', label: t('omnibar.axis.buildOrder'), state: 'featuring', source: 'featuring', group: 'bo' },
  { id: 'tactic', label: t('omnibar.axis.tactic'), state: 'featuring', source: 'featuring', group: 'marker' },
];

// One label per state key, for the chip groups.
const defaultStateLabels = () => ({
  duration: t('omnibar.axis.length'),
  mapKind: t('omnibar.axis.mapKind'),
  matchup: t('omnibar.axis.matchup'),
  map: t('omnibar.axis.map'),
  player: t('omnibar.axis.player'),
  featuring: t('omnibar.state.featuring'),
});

const NAME_SOURCES = new Set(['players', 'maps']);

const STATE_ORDER = ['duration', 'mapKind', 'matchup', 'map', 'player', 'featuring'];

// Words players type that are not the label in the data. Data, not logic, so
// adding one is a one-line change.
const ALIASES = {
  bgh: 'big game hunters',
  cheese: 'rush',
  fe: 'expand',
  mm: 'bio',
  nyd: 'nydus',
  hydra: 'hydralisk',
  lurk: 'lurker',
  muta: 'mutalisk',
};

const RACE_INITIAL = { zerg: 'Z', terran: 'T', protoss: 'P' };

// SC:R embeds colour control codes in map names; they are not printable.
const clean = (value) => String(value || '').replace(/[\x00-\x1f]/g, '').replace(/\s+/g, ' ').trim();

const MENU_LIMIT = 60;

// Longest preview string a browse block will show. The seven blocks have to
// share one row: unbudgeted, map and player names alone pushed the row to
// 1430px against a 1400px viewport and it wrapped to two lines, which costs
// more vertical space than the pill bar it replaced.
const PREVIEW_BUDGET = 22;

const previewText = (labels) => {
  const joined = labels.join(', ');
  if (joined.length <= PREVIEW_BUDGET) return joined;
  const first = labels[0] || '';
  if (first.length <= PREVIEW_BUDGET) return first + '...';
  return first.slice(0, PREVIEW_BUDGET - 1) + '...';
};

// Splits a translated template on its {placeholders} and drops React nodes in,
// so a sentence can carry a <b> without being assembled from fragments.
const renderRich = (template, nodes) => template.split(/(\{\w+\})/).map((part, i) => {
  const match = /^\{(\w+)\}$/.exec(part);
  if (!match || nodes[match[1]] === undefined) return part;
  return <React.Fragment key={i}>{nodes[match[1]]}</React.Fragment>;
});

export default function FilterOmnibar({
  filterOptions,
  selected,
  total,
  onToggle,
  onClear,
  axes = null,
  stateLabels = null,
  stateOrder = STATE_ORDER,
  noun = 'games',
  // When set, the typed text is itself a live filter (e.g. player name):
  // { value, onChange, placeholder }. The same text still narrows the option
  // menu, so picking an option stays one keystroke away.
  textFilter = null,
  loading = false,
}) {
  const isDeadEnd = (item) => !loading && item.games === 0;
  const t = useT();
  const { locale } = useLocale();
  const axisList = useMemo(() => axes || defaultAxes(), [axes, locale]);
  const stateLabelMap = useMemo(() => stateLabels || defaultStateLabels(), [stateLabels, locale]);
  const [internalQuery, setInternalQuery] = useState('');
  const query = textFilter ? textFilter.value : internalQuery;
  const setQuery = (value) => {
    if (textFilter) textFilter.onChange(value);
    else setInternalQuery(value);
  };
  const [open, setOpen] = useState(false);
  const [cursor, setCursor] = useState(0);
  const [openAxis, setOpenAxis] = useState('');
  const inputRef = useRef(null);
  const rootRef = useRef(null);
  const menuRef = useRef(null);

  // Flatten the API's per-axis option lists into one searchable vocabulary.
  const vocab = useMemo(() => {
    const out = [];
    axisList.forEach((axis) => {
      const list = filterOptions?.[axis.source] || [];
      list.forEach((option) => {
        if (axis.group && option.group !== axis.group) return;
        const label = NAME_SOURCES.has(axis.source)
          ? clean(option.label)
          : t.buildLabel(t.server(`server.chip.${axis.source}.${option.key}.label`, clean(option.label)));
        if (!label) return;
        const iconKeys = (Array.isArray(option.icon_keys) && option.icon_keys.length)
          ? option.icon_keys
          : (option.icon_key ? [option.icon_key] : []);
        const count = option.games ?? option.count;
        out.push({
          uid: `${axis.id}:${option.key}`,
          axisId: axis.id,
          axisLabel: axis.label,
          state: axis.state,
          key: option.key,
          label,
          games: count == null ? null : Number(count) || 0,
          race: option.race || '',
          // The chips already carry unit art; keeping it in the menu means a
          // Nydus reads as a Nydus while you scan, not just as a word.
          icons: iconKeys.map((key) => getUnitIcon(key)).filter(Boolean).slice(0, 2),
          emoji: option.emoji || '',
        });
      });
    });
    return out;
  }, [filterOptions, axisList, locale]);

  const isPicked = useCallback(
    (item) => (selected?.[item.state] || []).includes(item.key),
    [selected],
  );

  // Rank: prefix beats substring beats alias beats subsequence, then by how
  // many games the option would actually match.
  const matches = useMemo(() => {
    const q = query.trim().toLowerCase();
    const scored = [];
    vocab.forEach((item) => {
      if (!q) {
        scored.push({ item, score: 1 + (item.games || 0) });
        return;
      }
      const label = item.label.toLowerCase();
      const alias = ALIASES[q];
      let score = 0;
      if (label.startsWith(q)) score = 4000;
      else if (label.includes(q)) score = 3000;
      else if (alias && label.includes(alias)) score = 2500;
      else if (item.axisLabel.toLowerCase().startsWith(q)) score = 2000;
      else {
        let qi = 0;
        for (let i = 0; i < label.length && qi < q.length; i += 1) {
          if (label[i] === q[qi]) qi += 1;
        }
        if (qi === q.length) score = 1000;
      }
      if (score) scored.push({ item, score: score + (item.games || 0) });
    });
    scored.sort((a, b) => {
      // dead ends last, however well they matched
      const aDead = isDeadEnd(a.item);
      const bDead = isDeadEnd(b.item);
      if (aDead !== bDead) return aDead ? 1 : -1;
      if (b.score !== a.score) return b.score - a.score;
      const axisDelta = axisList.findIndex((x) => x.id === a.item.axisId)
        - axisList.findIndex((x) => x.id === b.item.axisId);
      if (axisDelta) return axisDelta;
      return a.item.label.localeCompare(b.item.label);
    });
    return scored.map((entry) => entry.item);
  }, [vocab, query, axisList, loading]);

  const shown = matches.slice(0, MENU_LIMIT);

  // Bucket the visible matches by axis. Axes are ordered by their strongest
  // match, so typing "pool" leads with Build order; an axis with nothing
  // matching contributes no section at all.
  const shownGroups = useMemo(() => {
    const byAxis = new Map();
    shown.forEach((item) => {
      if (!byAxis.has(item.axisId)) {
        byAxis.set(item.axisId, { axisId: item.axisId, axisLabel: item.axisLabel, items: [] });
      }
      byAxis.get(item.axisId).items.push(item);
    });
    const totalByAxis = new Map();
    matches.forEach((item) => {
      totalByAxis.set(item.axisId, (totalByAxis.get(item.axisId) || 0) + 1);
    });
    return [...byAxis.values()].map((group) => ({
      ...group,
      total: totalByAxis.get(group.axisId) || group.items.length,
    }));
  }, [shown, matches]);

  // Flat order the cursor walks, matching the rendered order exactly.
  const flatShown = useMemo(
    () => shownGroups.flatMap((group) => group.items),
    [shownGroups],
  );

  useEffect(() => { setCursor(0); }, [query]);

  useEffect(() => {
    if (!open) return undefined;
    const onDocClick = (e) => {
      if (rootRef.current && !rootRef.current.contains(e.target)) setOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);

  useEffect(() => {
    if (!open || !menuRef.current) return;
    const active = menuRef.current.querySelector('[aria-selected="true"]');
    if (active) active.scrollIntoView({ block: 'nearest' });
  }, [open, cursor, query]);

  const pick = (item) => {
    onToggle(item.state, item.key);
    setQuery('');
    setCursor(0);
    if (inputRef.current) inputRef.current.focus();
  };

  const chipGroups = useMemo(() => {
    const byState = new Map();
    vocab.forEach((item) => {
      if (!isPicked(item)) return;
      if (!byState.has(item.state)) byState.set(item.state, []);
      // A featuring key appears once per display axis; keep the first.
      if (byState.get(item.state).some((existing) => existing.key === item.key)) return;
      byState.get(item.state).push(item);
    });
    return stateOrder.filter((state) => byState.has(state))
      .map((state) => ({ state, label: stateLabelMap[state], items: byState.get(state) }));
  }, [vocab, isPicked, stateOrder, stateLabelMap]);

  const activeCount = chipGroups.reduce((sum, group) => sum + group.items.length, 0);

  const onKeyDown = (e) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setOpen(true);
      setCursor((c) => Math.min(flatShown.length - 1, c + 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setCursor((c) => Math.max(0, c - 1));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (open && flatShown[cursor]) pick(flatShown[cursor]);
    } else if (e.key === 'Escape') {
      setOpen(false);
    } else if (e.key === 'Backspace' && !query && activeCount) {
      const lastGroup = chipGroups[chipGroups.length - 1];
      const last = lastGroup.items[lastGroup.items.length - 1];
      onToggle(last.state, last.key);
    }
  };

  const renderLabel = (item) => {
    const q = query.trim().toLowerCase();
    if (!q) return item.label;
    const idx = item.label.toLowerCase().indexOf(q);
    if (idx < 0) return item.label;
    return (
      <>
        {item.label.slice(0, idx)}
        <mark>{item.label.slice(idx, idx + q.length)}</mark>
        {item.label.slice(idx + q.length)}
      </>
    );
  };

  // One section per matching axis, so 67 build orders and 26 tactics stay
  // tellable apart and "rush" shows which axis each hit sits on.
  const menuNodes = [];
  let flatIdx = 0;
  shownGroups.forEach((group) => {
    menuNodes.push(
      <div className="wf-ob-sec" key={`sec-${group.axisId}`}>
        {group.axisLabel}
        <span>{group.total}</span>
      </div>,
    );
    group.items.forEach((item) => {
      const idx = flatIdx;
      flatIdx += 1;
      menuNodes.push(
        <button
          type="button"
          role="option"
          aria-selected={idx === cursor}
          key={item.uid}
          className={`wf-ob-opt${isDeadEnd(item) ? ' wf-ob-opt--zero' : ''}`}
          onMouseEnter={() => setCursor(idx)}
          onClick={() => pick(item)}
        >
          <span className="wf-ob-tick">{isPicked(item) ? '✓' : ''}</span>
          <span className="wf-ob-icon">
            {item.icons.length
              ? item.icons.map((src, i) => (
                <img key={`${item.uid}-icon-${i}`} src={src} alt="" />
              ))
              : (item.emoji || RACE_INITIAL[item.race] || '')}
          </span>
          <span className="wf-ob-lbl">{renderLabel(item)}</span>
          <span className="wf-ob-n">{item.games == null ? '' : item.games}</span>
        </button>,
      );
    });
  });

  const browseAxes = axisList.map((axis) => {
    const items = vocab.filter((item) => item.axisId === axis.id);
    const active = items.filter(isPicked).length;
    const ranked = items
      .slice()
      .sort((a, b) => (b.games || 0) - (a.games || 0) || a.label.localeCompare(b.label));
    const live = ranked.filter((item) => item.games > 0);
    const preview = previewText((live.length ? live : ranked).slice(0, 2).map((item) => item.label));
    // "+N" counts everything the preview did not name.
    const previewed = preview.endsWith('...') ? 1 : Math.min(2, ranked.length);
    return { axis, items, active, preview, previewed };
  }).filter((entry) => entry.items.length > 0);

  const openEntry = browseAxes.find((entry) => entry.axis.id === openAxis);

  return (
    <div className="wf-omni" ref={rootRef}>
      <div
        className="wf-ob-box"
        onClick={(e) => {
          if (e.target.closest('button')) return;
          setOpen(true);
          if (inputRef.current) inputRef.current.focus();
        }}
      >
        <span className="wf-ob-mag" aria-hidden="true">&#9906;</span>

        {chipGroups.map((group, gi) => (
          <React.Fragment key={`grp-${group.state}`}>
            {gi > 0 ? <span className="wf-ob-and">{t('omnibar.and')}</span> : null}
            <span className="wf-ob-group">
              <span className="wf-ob-group-axis">{group.label}</span>
              {group.items.map((item, ii) => (
                <React.Fragment key={item.uid}>
                  {ii > 0 ? <span className="wf-ob-or">{t('omnibar.or')}</span> : null}
                  <span className="wf-ob-chip">
                    {item.label}
                    <button
                      type="button"
                      aria-label={t('omnibar.remove', { label: item.label })}
                      onClick={() => onToggle(item.state, item.key)}
                    >
                      &times;
                    </button>
                  </span>
                </React.Fragment>
              ))}
            </span>
          </React.Fragment>
        ))}

        <input
          ref={inputRef}
          className="wf-ob-input"
          type="text"
          role="combobox"
          aria-expanded={open}
          aria-controls="wf-ob-menu"
          aria-label={t(`omnibar.filterAria.${noun}`)}
          autoComplete="off"
          spellCheck="false"
          placeholder={textFilter?.placeholder || (activeCount ? t('omnibar.addFilter') : t(`omnibar.placeholder.${noun}`))}
          value={query}
          onChange={(e) => { setQuery(e.target.value); setOpen(true); }}
          onFocus={() => setOpen(true)}
          onKeyDown={onKeyDown}
        />

        <span className="wf-ob-count">
          {activeCount
            ? renderRich(t.plural(`omnibar.summary.${noun}`, activeCount), { total: <b>{total}</b> })
            : renderRich(t(`omnibar.total.${noun}`), { total: <b>{total}</b> })}
        </span>
        {activeCount ? (
          <button type="button" className="wf-ob-clear" onClick={onClear}>{t('omnibar.clear')}</button>
        ) : null}
      </div>

      {open ? (
        <div className="wf-ob-menu" id="wf-ob-menu" role="listbox" ref={menuRef}>
          {flatShown.length === 0 ? (
            <div className="wf-ob-empty">
              {textFilter
                ? renderRich(t(`omnibar.textFilterEmpty.${noun}`), { query: <b>{query}</b> })
                : renderRich(t('omnibar.empty'), {
                  query: <b>{query}</b>,
                  buildOrder: <b>{t('omnibar.exampleBuildOrder')}</b>,
                  tactic: <b>{t('omnibar.exampleTactic')}</b>,
                })}
            </div>
          ) : (
            <>
              {menuNodes}
              {matches.length > shown.length ? (
                <div className="wf-ob-empty wf-ob-more">
                  {t('omnibar.more', { count: matches.length - shown.length })}
                </div>
              ) : null}
              <div className="wf-ob-hint">
                <span>{t('omnibar.hint.move')}</span>
                <span>{t('omnibar.hint.toggle')}</span>
                <span>{t('omnibar.hint.close')}</span>
                <span>{t('omnibar.hint.coverage', { count: vocab.length, axes: browseAxes.length })}</span>
              </div>
            </>
          )}
        </div>
      ) : null}

      <div className="wf-br">
        {browseAxes.map((entry) => (
          <button
            type="button"
            key={entry.axis.id}
            className={`wf-br-block${openAxis === entry.axis.id ? ' wf-br-block--open' : ''}${entry.active ? ' wf-br-block--has' : ''}`}
            onClick={() => setOpenAxis(openAxis === entry.axis.id ? '' : entry.axis.id)}
          >
            <span className="wf-br-axis">{entry.axis.label}</span>
            {entry.active ? (
              <b>{entry.active}</b>
            ) : (
              <>
                <span className="wf-br-ex">{entry.preview}</span>
                {entry.items.length > entry.previewed ? (
                  <span className="wf-br-more">+{entry.items.length - entry.previewed}</span>
                ) : null}
              </>
            )}
          </button>
        ))}
      </div>

      {openEntry ? (
        <div className="wf-br-panel">
          <span className="wf-br-panel-label">{openEntry.axis.label}</span>
          {openEntry.items
            .slice()
            .sort((a, b) => (b.games || 0) - (a.games || 0) || a.label.localeCompare(b.label))
            .map((item) => (
              <button
                type="button"
                key={item.uid}
                className={`wf-br-opt${isPicked(item) ? ' wf-br-opt--on' : ''}${isDeadEnd(item) ? ' wf-br-opt--zero' : ''}`}
                onClick={() => onToggle(item.state, item.key)}
              >
                {item.label}
                <span className="wf-br-opt-n">{item.games == null ? '' : item.games}</span>
              </button>
            ))}
        </div>
      ) : null}
    </div>
  );
}

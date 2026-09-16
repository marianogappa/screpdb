import React, { useState } from 'react';
import { getWorkerIconForRace } from '../lib/gameAssets';
import { slugKey } from '../lib/i18n';
import { t, useT } from '../lib/i18nContext';

const formatDuration = (seconds) => {
  const total = Math.max(0, Math.round(Number(seconds) || 0));
  const hours = Math.floor(total / 3600);
  const minutes = Math.round((total % 3600) / 60);
  if (hours > 0) return t('session.duration.hoursMinutes', { hours, minutes });
  return t('session.duration.minutes', { minutes });
};

const raceLabel = (race) => {
  const key = `session.race.${slugKey(race)}`;
  return t.has(key) ? t(key) : race;
};

const formatClock = (iso) => {
  if (!iso) return '';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
};

// RecordValue reads the session at a glance. The old "2-4" hid the undecided
// games entirely, so a ten-game night showed a record adding up to six and left
// the reader to wonder where the rest went; it also made the win rate beside it
// look wrong, since that is computed over decided games only.
function RecordValue({ stats }) {
  const t = useT();
  const parts = [
    { key: 'wins', icon: '\u{1F451}', value: stats.wins || 0, title: t('session.record.wins') },
    { key: 'losses', icon: '\u274c', value: stats.losses || 0, title: t('session.record.losses') },
    // ❓ looks lighter than the two glyphs beside it, because it is a thin
    // stroke in a box they fill. That is left alone deliberately: the fix would
    // be to size it up, and emoji metrics differ across the Apple, Segoe and
    // Noto fonts, so a correction tuned on one platform is a defect on the
    // others. No filled glyph means "we never learned the result" anyway.
    { key: 'undecided', icon: '\u2753', value: stats.undecided || 0, title: t('session.record.undecided') },
    // A drop is its own outcome, not a loss and not an unknown: the game often
    // did resolve against the user, but losing the link is not losing the game.
    { key: 'dropped', icon: '\u{1F50C}', value: stats.dropped || 0, title: t('session.record.dropped') },
  ].filter((part) => (part.key !== 'undecided' && part.key !== 'dropped') || part.value > 0);
  return (
    <span className="session-record">
      {parts.map((part) => (
        <span key={part.key} className="session-record-part" title={part.title}>
          <span className="session-record-icon" aria-hidden="true">{part.icon}</span>
          <span>{part.value}</span>
        </span>
      ))}
    </span>
  );
}

// ApmByRace replaces the EAPM line when the user switched race mid-session.
// One mean across races describes none of them, and the per-race split is the
// more useful of the two numbers exactly when it exists.
function ApmByRace({ apmByRace }) {
  const entries = Object.entries(apmByRace || {}).sort((a, b) => b[1] - a[1]);
  if (entries.length === 0) return null;
  return (
    <span className="session-apm-races">
      {entries.map(([race, apm]) => {
        const url = getWorkerIconForRace(race);
        return (
          <span key={race} className="session-apm-race" title={raceLabel(race)}>
            {url
              ? <img src={url} alt={raceLabel(race)} className="session-race-icon" />
              : <span>{raceLabel(race)}</span>}
            <span>{Math.round(apm)}</span>
          </span>
        );
      })}
    </span>
  );
}

function StatTile({ label, value, sub }) {
  return (
    <div className="session-stat-tile">
      <div className="session-stat-value">{value}</div>
      <div className="session-stat-label">{label}</div>
      {sub ? <div className="session-stat-sub">{sub}</div> : null}
    </div>
  );
}

// Alternate accounts are shown as names only. The gateway matters far less than
// the fact that the person plays under other names, and the full list is long
// enough already.
// Races render as their worker icon. A player's race is a symbol everyone in
// this game already reads instantly, and three spelled-out names in a column
// cost more width than the whole rest of the row.
function RaceIcons({ races }) {
  const list = races || [];
  if (list.length === 0) return <span className="session-cell-empty">-</span>;
  return (
    <span className="session-race-icons">
      {list.map((race) => {
        const url = getWorkerIconForRace(race);
        return url
          ? <img key={race} src={url} alt={raceLabel(race)} title={raceLabel(race)} className="session-race-icon" />
          : <span key={race}>{raceLabel(race)}</span>;
      })}
    </span>
  );
}

function OtherToons({ profile, currentName, onPlayerClick }) {
  const t = useT();
  const [expanded, setExpanded] = useState(false);
  const current = String(currentName || '').trim().toLowerCase();
  const others = (profile?.toons || [])
    .filter((toon) => String(toon?.toon || '').trim().toLowerCase() !== current);
  if (others.length === 0) return <span className="session-cell-empty">-</span>;

  const sorted = [...others].sort((a, b) => {
    const aLocal = a.local_player_key ? 0 : 1;
    const bLocal = b.local_player_key ? 0 : 1;
    return aLocal - bLocal;
  });
  const visibleLimit = 3;
  const visible = expanded ? sorted : sorted.slice(0, visibleLimit);
  const overflow = sorted.length - visibleLimit;

  return (
    <span className="session-toon-pills">
      {visible.map((toon) =>
        toon.local_player_key ? (
          <button
            key={toon.toon}
            type="button"
            className="wps-alias wps-alias-known"
            title={t('player.viewThisPlayer')}
            onClick={(e) => { e.stopPropagation(); onPlayerClick?.(toon.local_player_key); }}
          >
            {toon.toon}
          </button>
        ) : (
          <span key={toon.toon} className="wps-alias">{toon.toon}</span>
        )
      )}
      {!expanded && overflow > 0 ? (
        <button
          type="button"
          className="wps-alias session-toon-more"
          onClick={(e) => { e.stopPropagation(); setExpanded(true); }}
        >
          +{overflow}
        </button>
      ) : null}
    </span>
  );
}

function LadderCell({ profile }) {
  const t = useT();
  if (!profile?.plays_ladder) return <span className="session-cell-empty">-</span>;
  const parts = [];
  if (profile.mmr) parts.push(String(profile.mmr));
  else if (profile.highest_mmr) parts.push(t('session.ladder.peak', { mmr: profile.highest_mmr }));
  if (profile.ladder_wins || profile.ladder_losses) {
    parts.push(`${profile.ladder_wins || 0}-${profile.ladder_losses || 0}`);
  }
  return <span>{parts.length > 0 ? parts.join(' · ') : t('session.ladder.yes')}</span>;
}

// formatLastGame is deliberately relative and coarse. The underlying fact is
// "the newest game Battle.net has published for this account", which is a
// moment in the past, not a presence signal; a relative distance says that
// plainly where a clock time would invite the reader to infer more.
const formatLastGame = (iso) => {
  if (!iso) return '';
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return '';
  const minutes = Math.max(0, Math.round((Date.now() - at.getTime()) / 60000));
  if (minutes < 5) return t('session.ago.justNow');
  if (minutes < 60) return t('session.ago.minutes', { count: minutes });
  const hours = Math.round(minutes / 60);
  if (hours < 24) return t('session.ago.hours', { count: hours });
  return t('session.ago.days', { count: Math.round(hours / 24) });
};

// sessionSortValue is the one place a column's sort key is defined, so a
// column can never sort by something other than what it renders.
const sessionSortValue = {
  player: (p) => String(p.player_name || '').toLowerCase(),
  games: (p) => Number(p.games) || 0,
  wins: (p) => Number(p.wins) || 0,
  losses: (p) => Number(p.losses) || 0,
  apm: (p) => Number(p.apm) || 0,
};

// Counts open descending and names ascending, because that is the first thing
// anyone wants from each: who played the most, and where is so-and-so.
const sessionSortFirstDir = { player: 'asc' };

// nextSortState cycles a column through ascending, descending and off. The
// third state matters: it restores the order the server chose, which already
// encodes "most games first", so a reader can always get back to the default
// without knowing what the default was.
function nextSortState(current, key) {
  const firstDir = sessionSortFirstDir[key] || 'desc';
  if (current.key !== key) return { key, dir: firstDir };
  if (current.dir === firstDir) return { key, dir: firstDir === 'asc' ? 'desc' : 'asc' };
  return { key: null, dir: null };
}

function sortSessionPlayers(players, sort) {
  if (!sort.key || !sessionSortValue[sort.key]) return players;
  const read = sessionSortValue[sort.key];
  const sign = sort.dir === 'asc' ? 1 : -1;
  // Array.prototype.sort is stable, so equal rows keep the server's order and
  // a name tiebreak is only needed to keep numeric columns readable.
  return [...players].sort((a, b) => {
    const av = read(a);
    const bv = read(b);
    if (av < bv) return -1 * sign;
    if (av > bv) return 1 * sign;
    return sessionSortValue.player(a).localeCompare(sessionSortValue.player(b));
  });
}

function SortableHeader({ label, title, sortKey, sort, onSort, className }) {
  const active = sort.key === sortKey;
  return (
    <th
      className={`${className} session-col-sortable${active ? ' session-col-sorted' : ''}`}
      aria-sort={active ? (sort.dir === 'asc' ? 'ascending' : 'descending') : 'none'}
    >
      <button type="button" className="session-sort-button" onClick={() => onSort(sortKey)} title={title}>
        <span>{label}</span>
        <span className="session-sort-arrow" aria-hidden="true">{active ? (sort.dir === 'asc' ? '\u25b2' : '\u25bc') : ''}</span>
      </button>
    </th>
  );
}

function PlayerTable({ players, renderName, renderBadge, showRecord, onPlayerClick }) {
  const t = useT();
  const [sort, setSort] = useState({ key: null, dir: null });
  const onSort = (key) => setSort((current) => nextSortState(current, key));
  const rows = sortSessionPlayers(players || [], sort);
  if (!players || players.length === 0) {
    return <div className="workflow-subtle-note">{t('session.nobody')}</div>;
  }
  const header = (label, title, sortKey, className) => (
    <SortableHeader label={label} title={title} sortKey={sortKey} sort={sort} onSort={onSort} className={className} />
  );
  return (
    <table className="workflow-table session-player-table">
      <thead>
        <tr>
          {header(t('session.col.player'), t('session.col.player'), 'player', 'col-player')}
          <th className="wpl-identity-head" aria-label="" />
          {header(t('session.col.played'), t('session.col.playedTitle'), 'games', 'col-count')}
          {showRecord ? header(t('session.col.wins'), t('session.col.winsTitle'), 'wins', 'col-count') : null}
          {showRecord ? header(t('session.col.losses'), t('session.col.lossesTitle'), 'losses', 'col-count') : null}
          <th className="col-races">{t('session.col.races')}</th>
          {header(t('session.col.apm'), t('session.col.apm'), 'apm', 'col-apm')}
          <th className="col-ladder">{t('session.col.ladder')}</th>
          <th className="col-toons">{t('session.col.alsoPlaysAs')}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((player) => (
          <tr key={player.player_key}>
            <td className="col-player">{renderName ? renderName(player) : player.player_name}</td>
            <td className="wpl-identity-cell">{renderBadge ? renderBadge(player) : null}</td>
            <td className="col-count">{player.games || 0}</td>
            {showRecord ? <td className="col-count">{player.wins || 0}</td> : null}
            {showRecord ? <td className="col-count">{player.losses || 0}</td> : null}
            <td className="col-races"><RaceIcons races={player.races} /></td>
            <td className="col-apm">{player.apm ? player.apm : <span className="session-cell-empty">-</span>}</td>
            <td className="col-ladder"><LadderCell profile={player.profile} /></td>
            <td className="col-toons"><OtherToons profile={player.profile} currentName={player.player_name} onPlayerClick={onPlayerClick} /></td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function RegularsPulse({ regulars, onPlayerClick }) {
  const t = useT();
  const all = regulars || [];
  // Most recently seen first. The list this comes from is ranked by how much
  // the user plays with each person, which is the right order for a roster and
  // the wrong one here: the question this line answers is who is around, so the
  // one who just finished a game leads regardless of how often they play.
  const byRecency = (a, b) => String(b.last_seen || '').localeCompare(String(a.last_seen || ''));
  const now = all.filter((regular) => regular.freshness === 'now').sort(byRecency);
  const lately = all.filter((regular) => regular.freshness === 'lately').sort(byRecency);
  const live = now.length > 0;
  const shown = live ? now : lately;
  if (shown.length === 0) return null;
  return (
    <div className={`session-pulse${live ? ' session-pulse--live' : ''}`}>
      <span className="session-pulse-label">
        {live ? <span className="session-pulse-dot" aria-hidden="true" /> : null}
        {t(live ? 'session.pulse.now' : 'session.pulse.lately')}
      </span>
      <span className="session-pulse-names">
        {shown.map((regular) => (
          <button
            key={regular.player_key}
            type="button"
            className="session-pulse-name"
            onClick={() => onPlayerClick?.(regular.player_key)}
            title={t('player.viewThisPlayer')}
          >
            {regular.player_name}
            <span className="session-pulse-when">{formatLastGame(regular.last_seen)}</span>
          </button>
        ))}
      </span>
    </div>
  );
}

function GamingSessionPanel({ session, loading, error, renderName, renderBadge, onPlayerClick, children }) {
  const t = useT();
  const [tab, setTab] = useState('games');

  if (loading && !session) {
    return <div className="workflow-panel">{t('session.loading')}</div>;
  }
  if (error) {
    return <div className="workflow-panel"><div className="error-message">{error}</div></div>;
  }
  const regulars = session?.regulars || [];

  if (!session?.has_session) {
    return (
      <div className="workflow-panel workflow-panel--session">
        <p className="workflow-subtle-note">
          {t('session.noRecent')}
        </p>
        <RegularsPulse regulars={regulars} onPlayerClick={onPlayerClick} />
      </div>
    );
  }

  const stats = session.stats || {};
  const hasRaceSplit = Object.keys(stats.apm_by_race || {}).length > 1;
  const opponents = session.opponents || [];
  const allies = session.allies || [];

  return (
    <div className="workflow-panel workflow-panel--session">
      <div className="session-stat-row">
        <StatTile
          label={t('session.stat.games')}
          value={stats.games || 0}
          sub={t('session.stat.timeRange', { start: formatClock(stats.started_at), end: formatClock(stats.ended_at) })}
        />
        <StatTile
          label={t('session.stat.record')}
          value={<RecordValue stats={stats} />}
          sub={(stats.games || 0) > 0 ? t('session.stat.winRate', { value: Math.round((stats.win_rate || 0) * 100) }) : null}
        />
        <StatTile
          label={t('session.stat.avgApm')}
          value={(stats.average_apm || 0).toFixed(0)}
          sub={hasRaceSplit
            ? <ApmByRace apmByRace={stats.apm_by_race} />
            : t('session.stat.eapm', { value: (stats.average_eapm || 0).toFixed(0) })}
        />
        <StatTile
          label={t('session.stat.timePlayed')}
          value={formatDuration(stats.played_seconds)}
          sub={stats.duration_seconds > 0
            ? t('session.stat.inGameOfElapsed', { elapsed: formatDuration(stats.duration_seconds) })
            : t('session.stat.inGame')}
        />
      </div>

      <RegularsPulse regulars={regulars} onPlayerClick={onPlayerClick} />

      <div className="workflow-production-tabs workflow-game-main-tabs" role="tablist" aria-label={t('session.sectionsAria')}>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'games'}
          className={`workflow-production-tab ${tab === 'games' ? 'workflow-production-tab-active' : ''}`}
          onClick={() => setTab('games')}
        >
          {t('session.tab.games')}
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'players'}
          className={`workflow-production-tab ${tab === 'players' ? 'workflow-production-tab-active' : ''}`}
          onClick={() => setTab('players')}
        >
          {t('session.tab.players')}
        </button>
      </div>

      {tab === 'games' ? children : (
        <div className="session-players">
          <PlayerTable players={opponents} renderName={renderName} renderBadge={renderBadge} showRecord onPlayerClick={onPlayerClick} />
          {allies.length > 0 ? (
            <div className="session-allies">
              <div className="session-breakdown-title">{t('session.playedAlongside')}</div>
              <PlayerTable players={allies} renderName={renderName} renderBadge={renderBadge} showRecord={false} onPlayerClick={onPlayerClick} />
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}

export default GamingSessionPanel;

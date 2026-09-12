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

function PlayerTable({ players, renderName, showRecord, onPlayerClick }) {
  const t = useT();
  if (!players || players.length === 0) {
    return <div className="workflow-subtle-note">{t('session.nobody')}</div>;
  }
  return (
    <table className="workflow-table session-player-table">
      <thead>
        <tr>
          <th className="col-player">{t('session.col.player')}</th>
          {showRecord ? <th className="col-result">{t('session.col.result')}</th> : null}
          <th className="col-races">{t('session.col.races')}</th>
          <th className="col-apm">{t('session.col.apm')}</th>
          <th className="col-ladder">{t('session.col.ladder')}</th>
          <th className="col-toons">{t('session.col.alsoPlaysAs')}</th>
        </tr>
      </thead>
      <tbody>
        {players.map((player) => (
          <tr key={player.player_key}>
            <td className="col-player">{renderName ? renderName(player) : player.player_name}</td>
            {showRecord ? (
              <td className="col-result">{player.wins || 0}-{player.losses || 0}</td>
            ) : null}
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

// RegularsPulse is the "is anyone around?" line. It is the reason this page is
// worth opening before a session rather than after one, so it sits with the
// totals instead of behind a tab.
//
// Two tiers, and never both: someone who has just finished a game is worth
// interrupting your evening for, and when nobody is, naming who has been alive
// in the last few days is still better than a blank. Below that, it renders
// nothing at all: an empty shelf teaches the eye to skip the whole region.
function RegularsPulse({ regulars, onPlayerClick }) {
  const t = useT();
  const all = regulars || [];
  const now = all.filter((regular) => regular.freshness === 'now');
  const lately = all.filter((regular) => regular.freshness === 'lately');
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

function GamingSessionPanel({ session, loading, error, renderName, onPlayerClick, children }) {
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
  const decided = (stats.wins || 0) + (stats.losses || 0);
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
          value={`${stats.wins || 0}-${stats.losses || 0}`}
          sub={decided > 0 ? t('session.stat.winRate', { value: Math.round((stats.win_rate || 0) * 100) }) : null}
        />
        <StatTile
          label={t('session.stat.avgApm')}
          value={(stats.average_apm || 0).toFixed(0)}
          sub={t('session.stat.eapm', { value: (stats.average_eapm || 0).toFixed(0) })}
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
          <PlayerTable players={opponents} renderName={renderName} showRecord onPlayerClick={onPlayerClick} />
          {allies.length > 0 ? (
            <div className="session-allies">
              <div className="session-breakdown-title">{t('session.playedAlongside')}</div>
              <PlayerTable players={allies} renderName={renderName} showRecord={false} onPlayerClick={onPlayerClick} />
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}

export default GamingSessionPanel;

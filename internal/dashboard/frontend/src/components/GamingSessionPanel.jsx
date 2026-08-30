import React, { useState } from 'react';
import { getWorkerIconForRace } from '../lib/gameAssets';

const formatDuration = (seconds) => {
  const total = Math.max(0, Math.round(Number(seconds) || 0));
  const hours = Math.floor(total / 3600);
  const minutes = Math.round((total % 3600) / 60);
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
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
          ? <img key={race} src={url} alt={race} title={race} className="session-race-icon" />
          : <span key={race}>{race}</span>;
      })}
    </span>
  );
}

function OtherToons({ profile, currentName }) {
  const current = String(currentName || '').trim().toLowerCase();
  const others = (profile?.toons || [])
    .map((t) => t.toon)
    .filter((toon) => String(toon).trim().toLowerCase() !== current);
  if (others.length === 0) return <span className="session-cell-empty">-</span>;
  return <span className="session-toon-list" title={others.join(', ')}>{others.join(', ')}</span>;
}

function LadderCell({ profile }) {
  if (!profile?.plays_ladder) return <span className="session-cell-empty">-</span>;
  const parts = [];
  if (profile.mmr) parts.push(String(profile.mmr));
  else if (profile.highest_mmr) parts.push(`peak ${profile.highest_mmr}`);
  if (profile.ladder_wins || profile.ladder_losses) {
    parts.push(`${profile.ladder_wins || 0}-${profile.ladder_losses || 0}`);
  }
  return <span>{parts.length > 0 ? parts.join(' · ') : 'yes'}</span>;
}

function PlayerTable({ players, renderName, showRecord }) {
  if (!players || players.length === 0) {
    return <div className="workflow-subtle-note">Nobody here.</div>;
  }
  return (
    <table className="workflow-table session-player-table">
      <thead>
        <tr>
          <th className="col-player">Player</th>
          {showRecord ? <th className="col-result">Result</th> : null}
          <th className="col-races">Races</th>
          <th className="col-apm">APM</th>
          <th className="col-ladder">Ladder</th>
          <th className="col-tag">Battle tag</th>
          <th className="col-toons">Other toons</th>
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
            <td className="col-tag">{player.profile?.battle_tag || <span className="session-cell-empty">-</span>}</td>
            <td className="col-toons"><OtherToons profile={player.profile} currentName={player.player_name} /></td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function GamingSessionPanel({ session, loading, error, renderName, children }) {
  const [tab, setTab] = useState('games');

  if (loading && !session) {
    return <div className="workflow-panel">Loading session...</div>;
  }
  if (error) {
    return <div className="workflow-panel"><div className="error-message">{error}</div></div>;
  }
  if (!session?.active) {
    return (
      <div className="workflow-panel">
        <p className="workflow-subtle-note">
          No recent session. Play a few games and this fills in with how the run went.
        </p>
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
          label="Games"
          value={stats.games || 0}
          sub={`${formatClock(stats.started_at)} to ${formatClock(stats.ended_at)}`}
        />
        <StatTile
          label="Record"
          value={`${stats.wins || 0}-${stats.losses || 0}`}
          sub={decided > 0 ? `${Math.round((stats.win_rate || 0) * 100)}% wins` : null}
        />
        <StatTile
          label="Avg APM"
          value={(stats.average_apm || 0).toFixed(0)}
          sub={`${(stats.average_eapm || 0).toFixed(0)} EAPM`}
        />
        <StatTile label="Time played" value={formatDuration(stats.played_seconds)} sub="in game" />
      </div>

      <div className="workflow-production-tabs workflow-game-main-tabs" role="tablist" aria-label="Session sections">
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'games'}
          className={`workflow-production-tab ${tab === 'games' ? 'workflow-production-tab-active' : ''}`}
          onClick={() => setTab('games')}
        >
          Games
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'players'}
          className={`workflow-production-tab ${tab === 'players' ? 'workflow-production-tab-active' : ''}`}
          onClick={() => setTab('players')}
        >
          Players
        </button>
      </div>

      {tab === 'games' ? children : (
        <div className="session-players">
          <PlayerTable players={opponents} renderName={renderName} showRecord />
          {allies.length > 0 ? (
            <div className="session-allies">
              <div className="session-breakdown-title">Played alongside</div>
              <PlayerTable players={allies} renderName={renderName} showRecord={false} />
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}

export default GamingSessionPanel;

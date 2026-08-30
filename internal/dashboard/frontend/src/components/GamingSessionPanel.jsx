import React from 'react';

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

// Counts render as a ranked list rather than a chart: a session is a handful of
// games, so the numbers are small enough to read directly and a chart of four
// bars would be decoration.
function CountBreakdown({ title, counts, emptyLabel }) {
  const entries = Object.entries(counts || {}).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
  return (
    <div className="session-breakdown">
      <div className="session-breakdown-title">{title}</div>
      {entries.length === 0 ? (
        <div className="workflow-subtle-note">{emptyLabel}</div>
      ) : (
        <ul className="session-breakdown-list">
          {entries.map(([label, count]) => (
            <li key={label}>
              <span className="session-breakdown-label">{label}</span>
              <span className="session-breakdown-count">{count}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
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

function GamingSessionPanel({ session, loading, error, renderOpponent, children }) {
  if (loading && !session) {
    return <div className="workflow-panel">Loading session...</div>;
  }
  if (error) {
    return <div className="workflow-panel"><div className="error-message">{error}</div></div>;
  }
  if (!session?.active) {
    return (
      <div className="workflow-panel">
        <h2>Gaming Session</h2>
        <p className="workflow-subtle-note">
          No recent session. Play a few games and this fills in with how the run went.
        </p>
      </div>
    );
  }

  const stats = session.stats || {};
  const decided = (stats.wins || 0) + (stats.losses || 0);

  return (
    <div className="workflow-panel workflow-panel--session">
      <div className="workflow-title-row">
        <h2>Gaming Session</h2>
        <span className="workflow-subtle-note">
          {formatClock(stats.started_at)} to {formatClock(stats.ended_at)}
          {stats.duration_seconds ? ` · ${formatDuration(stats.duration_seconds)} at the keyboard` : ''}
        </span>
      </div>

      <div className="session-stat-row">
        <StatTile label="Games" value={stats.games || 0} />
        <StatTile
          label="Record"
          value={`${stats.wins || 0}-${stats.losses || 0}`}
          sub={decided > 0 ? `${Math.round((stats.win_rate || 0) * 100)}% wins` : null}
        />
        <StatTile label="Avg APM" value={(stats.average_apm || 0).toFixed(0)} sub={`${(stats.average_eapm || 0).toFixed(0)} EAPM`} />
        <StatTile label="Time played" value={formatDuration(stats.played_seconds)} sub="in game" />
      </div>

      <div className="session-breakdown-row">
        <CountBreakdown title="Matchups" counts={stats.matchups} emptyLabel="No matchups recorded." />
        <CountBreakdown title="Races played" counts={stats.races_played} emptyLabel="No races recorded." />
        <CountBreakdown title="Maps" counts={stats.maps} emptyLabel="No maps recorded." />
      </div>

      <div className="session-section">
        <h3>Players you faced</h3>
        {(session.opponents || []).length === 0 ? (
          <div className="workflow-subtle-note">Nobody else in these games.</div>
        ) : (
          <ul className="session-opponent-list">
            {session.opponents.map((opponent) => (
              <li key={opponent.player_key} className="session-opponent">
                {renderOpponent ? renderOpponent(opponent) : opponent.player_name}
                <span className="session-opponent-record">
                  {opponent.games} {opponent.games === 1 ? 'game' : 'games'}
                  {opponent.wins + opponent.losses > 0 ? ` · ${opponent.wins}-${opponent.losses}` : ''}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="session-section">
        <h3>Games</h3>
        {children}
      </div>
    </div>
  );
}

export default GamingSessionPanel;

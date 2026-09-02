import React, { useState } from 'react';

// Per-player minute-snapshot map crops: the backend paints the part of the map
// holding the player's hotkeyed buildings at the chosen minute, sprites at
// footprint scale, hotkey numbers badged on top. 404 means no building was
// locatable at that minute (e.g. only unit groups so far).

const hotkeyMapURL = (replayId, playerId, minute) =>
  `/api/custom/hotkeys/map?replay_id=${encodeURIComponent(replayId)}&player_id=${encodeURIComponent(playerId)}&minute=${encodeURIComponent(minute)}`;

function HotkeyMapPanel({ replayId, player, maxMinute }) {
  const [minute, setMinute] = useState(Math.min(8, maxMinute));
  const [failed, setFailed] = useState(false);
  const src = hotkeyMapURL(replayId, player.player_id, minute);
  return (
    <div className="hk-map-panel">
      <div className="hk-map-head">
        <span className="hk-player-name">{player.name}</span>
        <label className="hk-map-minute">
          minute {minute}
          <input
            type="range"
            min="1"
            max={maxMinute}
            value={minute}
            onChange={(e) => { setFailed(false); setMinute(Number(e.target.value)); }}
          />
        </label>
      </div>
      {failed ? (
        <div className="chart-empty">No hotkeyed buildings located at minute {minute}.</div>
      ) : (
        <img
          key={src}
          src={src}
          alt={`Map crop of ${player.name}'s hotkeyed buildings at minute ${minute}`}
          className="hk-map-img"
          onError={() => setFailed(true)}
        />
      )}
    </div>
  );
}

export default function HotkeyMaps({ replayId, players, durationSeconds }) {
  const maxMinute = Math.max(1, Math.floor((durationSeconds || 60) / 60));
  const eligible = (players || []).filter((p) => (p.events || []).length > 0 && !p.legacy);
  if (!eligible.length) return null;
  return (
    <div className="hk-maps">
      <div className="hk-maps-title">Where the keys live</div>
      <div className="hk-maps-note">
        The slice of the map holding each player&apos;s hotkeyed buildings at the chosen minute -
        hotkey number on the building it points at.
      </div>
      <div className="hk-maps-grid">
        {eligible.map((p) => (
          <HotkeyMapPanel key={p.player_id} replayId={replayId} player={p} maxMinute={maxMinute} />
        ))}
      </div>
    </div>
  );
}

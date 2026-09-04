import React, { useMemo, useState } from 'react';
import {
  EV_SEC, EV_TYPE, EV_GROUP, EV_TILE_X, TYPE_ASSIGN_UNITS, TYPE_ASSIGN_BUILDING,
} from './HotkeyTimeline';
import { useT } from '../../lib/i18nContext';

// Per-player map crops: the backend paints the slice of the map holding the
// player's hotkeyed buildings at a chosen moment. The slider notches at
// building-assign moments (at most one per minute); when several assigns land
// in the same minute, the moment showing the most buildings wins.

const TILE_UNKNOWN = 255;

const hotkeyMapURL = (replayId, playerId, second) =>
  `/api/custom/hotkeys/map?replay_id=${encodeURIComponent(replayId)}&player_id=${encodeURIComponent(playerId)}&second=${encodeURIComponent(second)}`;

// locatedBuildingsAt counts the distinct groups whose latest assign at or
// before the cutoff is a located building, mirroring the backend reducer.
const locatedBuildingsAt = (events, cutoffSec) => {
  const last = new Map();
  for (const e of events) {
    if (e[EV_SEC] > cutoffSec || e[EV_GROUP] > 9) continue;
    if (e[EV_TYPE] === TYPE_ASSIGN_BUILDING || e[EV_TYPE] === TYPE_ASSIGN_UNITS) {
      last.set(e[EV_GROUP], e);
    }
  }
  let n = 0;
  for (const e of last.values()) {
    if (e[EV_TYPE] === TYPE_ASSIGN_BUILDING && e[EV_TILE_X] !== TILE_UNKNOWN) n += 1;
  }
  return n;
};

// snapshotNotches picks one moment per minute: each located building assign is
// a candidate, and within a minute the candidate whose snapshot shows the most
// buildings wins (later moment breaks ties), so a quickly replaced hotkey
// still surfaces at its best moment.
export const snapshotNotches = (events) => {
  const byMinute = new Map();
  for (const e of events || []) {
    if (e[EV_TYPE] !== TYPE_ASSIGN_BUILDING || e[EV_TILE_X] === TILE_UNKNOWN || e[EV_GROUP] > 9) continue;
    const sec = e[EV_SEC];
    const minute = Math.floor(sec / 60);
    const count = locatedBuildingsAt(events, sec);
    const cur = byMinute.get(minute);
    if (!cur || count > cur.count || (count === cur.count && sec > cur.sec)) {
      byMinute.set(minute, { sec, minute, count });
    }
  }
  return [...byMinute.values()].sort((a, b) => a.sec - b.sec);
};

function HotkeyMapPanel({ replayId, player, notches }) {
  const t = useT();
  const defaultIndex = useMemo(() => {
    let best = notches.length - 1;
    for (let i = 0; i < notches.length; i += 1) {
      if (notches[i].minute <= 8) best = i;
    }
    return best;
  }, [notches]);
  const [index, setIndex] = useState(defaultIndex);
  const [failed, setFailed] = useState(false);
  const notch = notches[Math.min(index, notches.length - 1)];
  const src = hotkeyMapURL(replayId, player.player_id, notch.sec);
  return (
    <div className="hk-map-panel">
      <div className="hk-map-head">
        <span className="hk-player-name">{player.name}</span>
        {notches.length > 1 ? (
          <input
            type="range"
            min="0"
            max={notches.length - 1}
            step="1"
            value={Math.min(index, notches.length - 1)}
            onChange={(e) => { setFailed(false); setIndex(Number(e.target.value)); }}
            list={`hk-notches-${player.player_id}`}
            aria-label={t('chart.hotkey.snapshotAria', { name: player.name })}
          />
        ) : null}
      </div>
      {failed ? (
        <div className="chart-empty">{t('chart.hotkey.mapEmpty')}</div>
      ) : (
        <img
          key={src}
          src={src}
          alt={t('chart.hotkey.mapAlt', { name: player.name, minute: notch.minute })}
          className="hk-map-img"
          onError={() => setFailed(true)}
        />
      )}
    </div>
  );
}

export default function HotkeyMaps({ replayId, players }) {
  const panels = useMemo(() => (players || [])
    .filter((p) => (p.events || []).length > 0 && !p.legacy)
    .map((p) => ({ player: p, notches: snapshotNotches(p.events) }))
    .filter(({ notches }) => notches.length > 0), [players]);
  if (!panels.length) return null;
  return (
    <div className="hk-maps">
      <div className="hk-maps-grid">
        {panels.map(({ player, notches }) => (
          <HotkeyMapPanel key={player.player_id} replayId={replayId} player={player} notches={notches} />
        ))}
      </div>
    </div>
  );
}

import React from 'react';
import { useT } from '../lib/i18nContext';
import OneRowPills from './OneRowPills';

const STALE_AFTER_MS = 90 * 24 * 60 * 60 * 1000;

export default function YouAlsoPlayAs({ profiles, onOpenPlayer }) {
  const t = useT();
  if (!Array.isArray(profiles) || profiles.length === 0) return null;
  const now = Date.now();
  return (
    <div className="you-aliases">
      <span className="wps-aliases-label">{t('player.youAlsoPlayAs')}</span>
      <OneRowPills className="you-aliases-list">
        {profiles.map((p) => {
          const stale = now - Date.parse(p.last_played) > STALE_AFTER_MS;
          return (
            <button
              key={p.player_key}
              type="button"
              className={`wps-alias you-alias${stale ? ' you-alias--stale' : ''}`}
              title={t('player.viewThisPlayer')}
              onClick={() => onOpenPlayer(p.player_key)}
            >
              {p.player_name}
            </button>
          );
        })}
      </OneRowPills>
    </div>
  );
}

import React from 'react';
import { useT } from '../lib/i18nContext';
import { formatDuration } from '../lib/formatters';

// The badge keeps its width in fast mode (hidden, not absent), so the upgrade to engine data
// shows up without moving anything around it.
// enabled is false while the engine is switched off in Labs: then there is nothing to reserve room for.
export default function EngineBadge({ item, enabled }) {
  const t = useT();
  if (!enabled) return null;
  const engine = item?.mode === 'engine';
  const low = engine && item?.engine_confidence === 'low';
  let title;
  if (low) {
    title = t('engine.badge.lowTitle', { time: formatDuration(item?.engine_trusted_until_seconds || 0) });
  } else if (engine) {
    title = t('engine.badge.title');
  }
  const className = [
    'engine-badge',
    engine ? '' : 'engine-badge--reserved',
    low ? 'engine-badge--low' : '',
  ].filter(Boolean).join(' ');
  return (
    <span className={className} title={title} aria-hidden={engine ? undefined : 'true'}>
      {t('engine.badge.label')}
    </span>
  );
}

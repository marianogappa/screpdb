import React from 'react';
import { useT } from '../lib/i18nContext';
import useAnimatedNumber from '../lib/useAnimatedNumber';

// EngineCoverage is the "n of N games with engine data" line that stands in for a progress bar: the
// engine pass has no global progress indicator. Nothing shows without a StarCraft install.
export default function EngineCoverage({ coverage, className = '' }) {
  const t = useT();
  const games = useAnimatedNumber(Number(coverage?.games) || 0);
  const total = Number(coverage?.total) || 0;
  if (!coverage || coverage.state === 'off' || total === 0) return null;
  return (
    <span className={`engine-coverage ${className}`.trim()}>
      {t('engine.coverage', { games, total })}
    </span>
  );
}

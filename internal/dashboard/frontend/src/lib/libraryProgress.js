const PHASE_LABELS = {
  scanning: 'Scanning',
  recent: 'Loading recent games',
  backfill: 'Loading older games',
  ready: 'Ready',
  watching: 'Watching for new replays',
  failed: 'Failed',
};

const STATUS_LABELS = {
  idle: 'Idle',
  loading: 'Loading',
  watching: 'Watching for new replays',
  failed: 'Failed',
};

const count = (value) => {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0;
};

export const formatCount = (value) => count(value).toLocaleString('en-US');

export const formatLoaded = (loaded, total) => `Loaded ${formatCount(loaded)} of ${formatCount(total)} replays`;

export const formatLoadingShort = () => 'Loading';

export const stillLoadingCopy = () => 'Still reading your replay folder. This fills in when it finishes.';

export const phaseLabel = (phase) => PHASE_LABELS[String(phase || '').toLowerCase()] || 'Idle';

export const statusLabel = (status) => STATUS_LABELS[String(status || '').toLowerCase()] || 'Idle';

export const percent = (loaded, total) => {
  const done = count(loaded);
  const all = count(total);
  if (all === 0) return 0;
  return Math.max(0, Math.min(100, Math.round((done / all) * 100)));
};

export const isLibraryLoading = (status) => String(status || '').toLowerCase() === 'loading';

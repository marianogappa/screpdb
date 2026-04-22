export const formatDuration = (seconds) => {
  const total = Math.max(0, Math.floor(Number(seconds) || 0));
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return `${mins}:${String(secs).padStart(2, '0')}`;
};

export const formatRelativeReplayDate = (value) => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;

  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const startOfDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const diffDays = Math.floor((startOfToday.getTime() - startOfDate.getTime()) / 86400000);

  let dayLabel = '';
  if (diffDays === 0) dayLabel = 'Today';
  else if (diffDays === 1) dayLabel = 'Yesterday';
  else if (diffDays > 1) dayLabel = `${diffDays}d ago`;
  else dayLabel = date.toLocaleDateString();

  const hours = date.getHours();
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const hour12 = hours % 12 || 12;
  const ampm = hours >= 12 ? 'pm' : 'am';
  return `${dayLabel} @ ${hour12}.${minutes}${ampm}`;
};

export const formatDaysAgoCompact = (value) => {
  const days = Math.max(0, Number(value) || 0);
  if (days === 0) return 'Today';
  if (days === 1) return '1d ago';
  return `${days}d ago`;
};

export const formatPercent = (value) => `${((Number(value) || 0) * 100).toFixed(1)}%`;

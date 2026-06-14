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

  // Compact day label: drop the trailing "ago" and collapse "Yesterday" to
  // "1d" so the games-list "Played" column doesn't burn horizontal space we
  // need for the 8-player matchup pills (see workflow-games-list-table CSS).
  let dayLabel = '';
  if (diffDays === 0) dayLabel = 'Today';
  else if (diffDays >= 1) dayLabel = `${diffDays}d`;
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

// mapKindEmoji returns a single-character prefix used in the games list
// "Map" column so a player can scan-skim Money / UMS games at a glance.
//   - Money         → 💰
//   - UseMapSettings→ ⚙️ (matches the Settings nav button)
//   - Regular / "" → no emoji (returns empty string)
export const mapKindEmoji = (mapKind) => {
  // Accept both the DB column casing ("Money", "UseMapSettings") and the
  // games-list filter keys ("money", "regular").
  switch (String(mapKind || '').toLowerCase()) {
    case 'money':
      return '💰';
    case 'usemapsettings':
      return '⚙️';
    default:
      return '';
  }
};

// mapKindTooltip is the hover explanation for a map-kind emoji.
export const mapKindTooltip = (mapKind) => {
  switch (String(mapKind || '').toLowerCase()) {
    case 'money':
      return 'Money map';
    case 'usemapsettings':
      return 'Use Map Settings: custom scenario rules.';
    default:
      return '';
  }
};

// formatMapNameWithKind prefixes the map name with the kind emoji + a space
// when relevant. Regular maps render unchanged.
export const formatMapNameWithKind = (mapName, mapKind) => {
  const emoji = mapKindEmoji(mapKind);
  const name = String(mapName || '');
  return emoji ? `${emoji} ${name}` : name;
};

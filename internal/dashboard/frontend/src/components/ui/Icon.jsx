import React from 'react';

const ICON_PATHS = {
  chart: <><path d="M3 3v18h18"/><path d="M7 16l4-8 4 4 4-12"/></>,
  menu: <><circle cx="10" cy="4" r="2"/><circle cx="10" cy="10" r="2"/><circle cx="10" cy="16" r="2"/></>,
  download: <><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></>,
  grid: <><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></>,
  edit: <><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></>,
  trash: <><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></>,
  close: <><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></>,
  filter: <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>,
  chevronDown: <polyline points="6 9 12 15 18 9"/>,
  info: <><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></>,
  code: <><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></>,
  dashboard: <><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="9" y1="21" x2="9" y2="9"/></>,
  drag: <><circle cx="4" cy="3" r="1.5"/><circle cx="10" cy="3" r="1.5"/><circle cx="4" cy="7" r="1.5"/><circle cx="10" cy="7" r="1.5"/><circle cx="4" cy="11" r="1.5"/><circle cx="10" cy="11" r="1.5"/></>,
};

const FILLED_ICONS = new Set(['menu', 'drag']);

export default function Icon({ name, size = 16, className = '', style }) {
  const paths = ICON_PATHS[name];
  if (!paths) return null;

  const isFilled = FILLED_ICONS.has(name);
  const viewBox = name === 'menu' ? '0 0 20 20' : name === 'drag' ? '0 0 14 14' : '0 0 24 24';

  return (
    <svg
      width={size}
      height={size}
      viewBox={viewBox}
      fill={isFilled ? 'currentColor' : 'none'}
      stroke={isFilled ? undefined : 'currentColor'}
      strokeWidth={isFilled ? undefined : '2'}
      strokeLinecap={isFilled ? undefined : 'round'}
      strokeLinejoin={isFilled ? undefined : 'round'}
      className={className}
      style={style}
    >
      {paths}
    </svg>
  );
}

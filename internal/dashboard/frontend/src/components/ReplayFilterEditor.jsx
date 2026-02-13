import React, { useEffect, useMemo, useState } from 'react';

const BUILTIN_FILTERS = [
  {
    id: 'only_1v1_humans',
    label: 'Only 1v1 with Human players',
    sql: `SELECT *
FROM replays r
WHERE EXISTS (
  SELECT 1
  FROM players p
  WHERE p.replay_id = r.id
    AND p.type = 'Human'
  GROUP BY p.replay_id
  HAVING COUNT(*) = 2
)`,
  },
  {
    id: 'only_8_humans_melee_hunters',
    label: 'Only 8 Human players, Melee, map contains "hunters"',
    sql: `SELECT *
FROM replays r
WHERE r.game_type = 'Melee'
  AND LOWER(r.map_name) LIKE '%hunters%'
  AND r.id IN (
    SELECT p.replay_id
    FROM players p
    WHERE p.type = 'Human'
    GROUP BY p.replay_id
    HAVING COUNT(*) = 8
  )`,
  },
  {
    id: 'only_longer_than_3_minutes',
    label: 'Only longer than 3 minutes',
    sql: `SELECT *
FROM replays
WHERE duration_seconds > 180`,
  },
];

const FILTER_OPTIONS = [
  { id: 'none', label: 'No filter (all replays)' },
  ...BUILTIN_FILTERS.map((filter) => ({ id: filter.id, label: filter.label })),
  { id: 'custom', label: 'Custom SQL' },
];

const normalizeSQL = (value) => {
  if (!value) return '';
  let trimmed = value.trim();
  while (trimmed.endsWith(';')) {
    trimmed = trimmed.slice(0, -1).trim();
  }
  return trimmed.replace(/\s+/g, ' ').trim();
};

const findBuiltinMatch = (value) => {
  const normalized = normalizeSQL(value);
  if (!normalized) return null;
  return BUILTIN_FILTERS.find((filter) => normalizeSQL(filter.sql) === normalized) || null;
};

function ReplayFilterEditor({ value, onChange }) {
  const builtinMatch = useMemo(() => findBuiltinMatch(value), [value]);
  const [mode, setMode] = useState(builtinMatch ? builtinMatch.id : value ? 'custom' : 'none');
  const [customSQL, setCustomSQL] = useState(value || '');

  useEffect(() => {
    if (!value || normalizeSQL(value) === '') {
      setMode('none');
      setCustomSQL('');
      return;
    }
    const match = findBuiltinMatch(value);
    if (match) {
      setMode(match.id);
      setCustomSQL('');
      return;
    }
    setMode('custom');
    setCustomSQL(value);
  }, [value]);

  const handleModeChange = (e) => {
    const next = e.target.value;
    setMode(next);
    if (next === 'none') {
      onChange('');
      return;
    }
    if (next === 'custom') {
      onChange(customSQL);
      return;
    }
    const selected = BUILTIN_FILTERS.find((filter) => filter.id === next);
    if (selected) {
      onChange(selected.sql);
    }
  };

  const handleCustomChange = (e) => {
    const next = e.target.value;
    setCustomSQL(next);
    onChange(next);
  };

  const computedSQL = mode === 'custom'
    ? customSQL
    : (BUILTIN_FILTERS.find((filter) => filter.id === mode)?.sql || '');

  return (
    <>
      <div className="form-group">
        <label>Replay Filter</label>
        <select className="form-input" value={mode} onChange={handleModeChange}>
          {FILTER_OPTIONS.map((opt) => (
            <option key={opt.id} value={opt.id}>
              {opt.label}
            </option>
          ))}
        </select>
        <div className="form-hint">
          Use a query like <code>SELECT * FROM replays WHERE ...</code> to scope all widgets in this dashboard.
        </div>
      </div>

      {mode === 'custom' ? (
        <div className="form-group">
          <label>Custom SQL</label>
          <textarea
            className="form-textarea"
            rows={8}
            value={customSQL}
            onChange={handleCustomChange}
            placeholder="SELECT * FROM replays WHERE ..."
          />
        </div>
      ) : mode === 'none' ? null : (
        <div className="form-group">
          <label>Computed SQL</label>
          <textarea className="form-textarea" rows={8} value={computedSQL} readOnly />
        </div>
      )}
    </>
  );
}

export default ReplayFilterEditor;
export { BUILTIN_FILTERS };

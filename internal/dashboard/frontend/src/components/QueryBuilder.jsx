import React, { useState, useMemo, useCallback } from 'react';
import { useSchema } from '../hooks/useSchema';
import { QUERY_TEMPLATES } from '../constants/chartTypes';

const OPERATORS = {
  text: [
    { value: '=', label: 'equals' },
    { value: '!=', label: 'not equals' },
    { value: 'LIKE', label: 'contains' },
    { value: 'NOT LIKE', label: 'does not contain' },
    { value: 'IS NULL', label: 'is empty', noValue: true },
    { value: 'IS NOT NULL', label: 'is not empty', noValue: true },
  ],
  number: [
    { value: '=', label: '=' },
    { value: '!=', label: '!=' },
    { value: '>', label: '>' },
    { value: '<', label: '<' },
    { value: '>=', label: '>=' },
    { value: '<=', label: '<=' },
  ],
  boolean: [
    { value: '= 1', label: 'is true', noValue: true },
    { value: '= 0', label: 'is false', noValue: true },
  ],
};

const AGGREGATES = [
  { value: '', label: 'None' },
  { value: 'COUNT', label: 'Count' },
  { value: 'SUM', label: 'Sum' },
  { value: 'AVG', label: 'Average' },
  { value: 'MIN', label: 'Min' },
  { value: 'MAX', label: 'Max' },
];

function getColumnType(colType) {
  const t = (colType || '').toUpperCase();
  if (t.includes('INT') || t.includes('REAL') || t.includes('FLOAT') || t.includes('NUMERIC') || t.includes('BIGINT')) return 'number';
  if (t.includes('BOOL')) return 'boolean';
  return 'text';
}

function getOperatorsForType(colType) {
  return OPERATORS[getColumnType(colType)] || OPERATORS.text;
}

export default function QueryBuilder({ onQueryGenerated, initialMode = 'visual' }) {
  const { schema, loading: schemaLoading } = useSchema();
  const [mode, setMode] = useState(initialMode);
  const [selectedTable, setSelectedTable] = useState('replays');
  const [selectedColumns, setSelectedColumns] = useState([]);
  const [filters, setFilters] = useState([]);
  const [joins, setJoins] = useState({ players: false, commands: false });
  const [groupBy, setGroupBy] = useState([]);
  const [aggregates, setAggregates] = useState({});
  const [orderBy, setOrderBy] = useState([]);
  const [limit, setLimit] = useState(100);
  const [showTemplates, setShowTemplates] = useState(false);

  const tables = useMemo(() => {
    if (!schema?.tables) return [];
    return Object.keys(schema.tables).filter(t =>
      ['replays', 'players', 'commands', 'detected_patterns_replay', 'detected_patterns_replay_team', 'detected_patterns_replay_player'].includes(t)
    );
  }, [schema]);

  const availableColumns = useMemo(() => {
    if (!schema?.tables) return [];
    const cols = [];
    const addTable = (name) => {
      const table = schema.tables[name];
      if (!table) return;
      for (const [colName, info] of Object.entries(table.columns)) {
        cols.push({ table: name, column: colName, type: info.type, qualified: `${name}.${colName}` });
      }
    };
    addTable(selectedTable);
    if (joins.players && selectedTable !== 'players') addTable('players');
    if (joins.commands && selectedTable !== 'commands') addTable('commands');
    return cols;
  }, [schema, selectedTable, joins]);

  const generateSQL = useCallback(() => {
    if (selectedColumns.length === 0 && Object.keys(aggregates).length === 0) {
      return `SELECT *\nFROM ${selectedTable}\nLIMIT ${limit}`;
    }

    const selectParts = [];
    selectedColumns.forEach(col => {
      const agg = aggregates[col];
      if (agg) {
        const alias = `${agg.toLowerCase()}_${col.split('.').pop()}`;
        selectParts.push(`${agg}(${col}) AS ${alias}`);
      } else {
        selectParts.push(col);
      }
    });

    let sql = `SELECT ${selectParts.length > 0 ? selectParts.join(',\n  ') : '*'}`;
    sql += `\nFROM ${selectedTable}`;

    if (joins.players && selectedTable === 'replays') {
      sql += `\nJOIN players ON players.replay_id = replays.id`;
    }
    if (joins.commands && selectedTable === 'replays') {
      sql += `\nJOIN commands ON commands.replay_id = replays.id`;
    }
    if (joins.players && selectedTable === 'commands') {
      sql += `\nJOIN players ON players.id = commands.player_id`;
    }

    const whereClauses = filters.filter(f => f.column && f.operator).map(f => {
      const op = getOperatorsForType(f.colType).find(o => o.value === f.operator);
      if (op?.noValue) return `${f.column} ${f.operator}`;
      if (f.operator === 'LIKE' || f.operator === 'NOT LIKE') {
        return `${f.column} ${f.operator} '%${(f.value || '').replace(/'/g, "''")}%'`;
      }
      const val = getColumnType(f.colType) === 'number' ? f.value : `'${(f.value || '').replace(/'/g, "''")}'`;
      return `${f.column} ${f.operator} ${val}`;
    });
    if (whereClauses.length > 0) {
      sql += `\nWHERE ${whereClauses.join('\n  AND ')}`;
    }

    if (groupBy.length > 0) {
      sql += `\nGROUP BY ${groupBy.join(', ')}`;
    }

    if (orderBy.length > 0) {
      sql += `\nORDER BY ${orderBy.map(o => `${o.column} ${o.dir}`).join(', ')}`;
    }

    sql += `\nLIMIT ${limit}`;
    return sql;
  }, [selectedTable, selectedColumns, filters, joins, groupBy, aggregates, orderBy, limit]);

  const handleApply = () => {
    onQueryGenerated(generateSQL());
  };

  const handleTemplateSelect = (template) => {
    onQueryGenerated(template.query);
    setShowTemplates(false);
  };

  const toggleColumn = (qualified) => {
    setSelectedColumns(prev =>
      prev.includes(qualified) ? prev.filter(c => c !== qualified) : [...prev, qualified]
    );
  };

  const addFilter = () => {
    setFilters(prev => [...prev, { column: '', operator: '=', value: '', colType: 'TEXT' }]);
  };

  const updateFilter = (idx, field, value) => {
    setFilters(prev => prev.map((f, i) => {
      if (i !== idx) return f;
      const updated = { ...f, [field]: value };
      if (field === 'column') {
        const col = availableColumns.find(c => c.qualified === value);
        updated.colType = col?.type || 'TEXT';
        updated.operator = getOperatorsForType(updated.colType)[0].value;
        updated.value = '';
      }
      return updated;
    }));
  };

  const removeFilter = (idx) => {
    setFilters(prev => prev.filter((_, i) => i !== idx));
  };

  const toggleOrderBy = (col) => {
    setOrderBy(prev => {
      const existing = prev.find(o => o.column === col);
      if (!existing) return [...prev, { column: col, dir: 'ASC' }];
      if (existing.dir === 'ASC') return prev.map(o => o.column === col ? { ...o, dir: 'DESC' } : o);
      return prev.filter(o => o.column !== col);
    });
  };

  if (schemaLoading) {
    return <div className="qb-loading">Loading schema...</div>;
  }

  return (
    <div className="query-builder">
      <div className="qb-header">
        <div className="qb-mode-toggle">
          <button
            className={`qb-mode-btn ${mode === 'visual' ? 'active' : ''}`}
            onClick={() => setMode('visual')}
          >
            Visual Builder
          </button>
          <button
            className={`qb-mode-btn ${mode === 'templates' ? 'active' : ''}`}
            onClick={() => setMode('templates')}
          >
            Templates
          </button>
        </div>
      </div>

      {mode === 'templates' && (
        <div className="qb-templates">
          {QUERY_TEMPLATES.map(t => (
            <button
              key={t.id}
              className="qb-template-card"
              onClick={() => handleTemplateSelect(t)}
            >
              <span className="qb-template-name">{t.name}</span>
              <span className="qb-template-desc">{t.description}</span>
            </button>
          ))}
        </div>
      )}

      {mode === 'visual' && (
        <div className="qb-visual">
          <div className="qb-step">
            <div className="qb-step-header">
              <span className="qb-step-num">1</span>
              <span className="qb-step-title">From Table</span>
            </div>
            <select
              className="qb-select"
              value={selectedTable}
              onChange={(e) => {
                setSelectedTable(e.target.value);
                setSelectedColumns([]);
                setFilters([]);
                setGroupBy([]);
                setOrderBy([]);
                setJoins({ players: false, commands: false });
              }}
            >
              {tables.map(t => (
                <option key={t} value={t}>{t}</option>
              ))}
            </select>
          </div>

          <div className="qb-step">
            <div className="qb-step-header">
              <span className="qb-step-num">2</span>
              <span className="qb-step-title">Select Columns</span>
              <div className="qb-step-actions">
                <button className="qb-link-btn" onClick={() => setSelectedColumns(availableColumns.map(c => c.qualified))}>All</button>
                <button className="qb-link-btn" onClick={() => setSelectedColumns([])}>None</button>
              </div>
            </div>
            <div className="qb-columns-grid">
              {availableColumns.map(col => (
                <label key={col.qualified} className="qb-column-check">
                  <input
                    type="checkbox"
                    checked={selectedColumns.includes(col.qualified)}
                    onChange={() => toggleColumn(col.qualified)}
                  />
                  <span className="qb-col-name">{col.column}</span>
                  <span className="qb-col-table">{col.table}</span>
                </label>
              ))}
            </div>
          </div>

          {selectedTable === 'replays' && (
            <div className="qb-step">
              <div className="qb-step-header">
                <span className="qb-step-num">3</span>
                <span className="qb-step-title">Include Related Data</span>
              </div>
              <div className="qb-joins">
                <label className="qb-join-check">
                  <input
                    type="checkbox"
                    checked={joins.players}
                    onChange={(e) => setJoins(prev => ({ ...prev, players: e.target.checked }))}
                  />
                  <span>Player details</span>
                </label>
                <label className="qb-join-check">
                  <input
                    type="checkbox"
                    checked={joins.commands}
                    onChange={(e) => setJoins(prev => ({ ...prev, commands: e.target.checked }))}
                  />
                  <span>Game commands</span>
                </label>
              </div>
            </div>
          )}

          <div className="qb-step">
            <div className="qb-step-header">
              <span className="qb-step-num">{selectedTable === 'replays' ? '4' : '3'}</span>
              <span className="qb-step-title">Filters</span>
              <button className="qb-link-btn" onClick={addFilter}>+ Add Filter</button>
            </div>
            {filters.map((filter, idx) => (
              <div key={idx} className="qb-filter-row">
                <select
                  className="qb-select qb-filter-col"
                  value={filter.column}
                  onChange={(e) => updateFilter(idx, 'column', e.target.value)}
                >
                  <option value="">Column...</option>
                  {availableColumns.map(c => (
                    <option key={c.qualified} value={c.qualified}>{c.column} ({c.table})</option>
                  ))}
                </select>
                <select
                  className="qb-select qb-filter-op"
                  value={filter.operator}
                  onChange={(e) => updateFilter(idx, 'operator', e.target.value)}
                >
                  {getOperatorsForType(filter.colType).map(op => (
                    <option key={op.value} value={op.value}>{op.label}</option>
                  ))}
                </select>
                {!getOperatorsForType(filter.colType).find(o => o.value === filter.operator)?.noValue && (
                  <input
                    className="qb-input qb-filter-val"
                    type={getColumnType(filter.colType) === 'number' ? 'number' : 'text'}
                    value={filter.value}
                    onChange={(e) => updateFilter(idx, 'value', e.target.value)}
                    placeholder="Value..."
                  />
                )}
                <button className="qb-remove-btn" onClick={() => removeFilter(idx)}>x</button>
              </div>
            ))}
          </div>

          <div className="qb-step">
            <div className="qb-step-header">
              <span className="qb-step-num">{selectedTable === 'replays' ? '5' : '4'}</span>
              <span className="qb-step-title">Group & Aggregate</span>
            </div>
            {selectedColumns.length > 0 && (
              <div className="qb-agg-grid">
                {selectedColumns.map(col => (
                  <div key={col} className="qb-agg-row">
                    <span className="qb-agg-col">{col.split('.').pop()}</span>
                    <select
                      className="qb-select qb-agg-select"
                      value={aggregates[col] || ''}
                      onChange={(e) => {
                        const val = e.target.value;
                        setAggregates(prev => {
                          const next = { ...prev };
                          if (val) next[col] = val;
                          else delete next[col];
                          return next;
                        });
                        if (val && !groupBy.includes(col)) {
                          const nonAggCols = selectedColumns.filter(c => c !== col && !aggregates[c]);
                          setGroupBy(nonAggCols);
                        }
                      }}
                    >
                      {AGGREGATES.map(a => (
                        <option key={a.value} value={a.value}>{a.label}</option>
                      ))}
                    </select>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="qb-step">
            <div className="qb-step-header">
              <span className="qb-step-num">{selectedTable === 'replays' ? '6' : '5'}</span>
              <span className="qb-step-title">Sort & Limit</span>
            </div>
            <div className="qb-sort-chips">
              {selectedColumns.map(col => {
                const order = orderBy.find(o => o.column === col);
                return (
                  <button
                    key={col}
                    className={`qb-sort-chip ${order ? 'active' : ''}`}
                    onClick={() => toggleOrderBy(col)}
                  >
                    {col.split('.').pop()} {order ? (order.dir === 'ASC' ? '\u2191' : '\u2193') : ''}
                  </button>
                );
              })}
            </div>
            <div className="qb-limit-row">
              <label>Limit results:</label>
              <select className="qb-select" value={limit} onChange={(e) => setLimit(Number(e.target.value))}>
                {[10, 50, 100, 500, 1000].map(n => (
                  <option key={n} value={n}>{n}</option>
                ))}
              </select>
            </div>
          </div>

          <div className="qb-preview-sql">
            <div className="qb-preview-header">Generated SQL</div>
            <pre className="qb-preview-code">{generateSQL()}</pre>
          </div>

          <button className="qb-apply-btn" onClick={handleApply}>
            Apply Query
          </button>
        </div>
      )}
    </div>
  );
}

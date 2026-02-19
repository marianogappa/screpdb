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

const JOIN_MAP = {
  replays: [
    { table: 'players', label: 'Player details', on: 'players.replay_id = replays.id' },
    { table: 'commands', label: 'Game commands', on: 'commands.replay_id = replays.id' },
  ],
  players: [
    { table: 'replays', label: 'Replay info', on: 'replays.id = players.replay_id' },
  ],
  commands: [
    { table: 'replays', label: 'Replay info', on: 'replays.id = commands.replay_id' },
    { table: 'players', label: 'Player details', on: 'players.id = commands.player_id' },
  ],
};

function getColumnType(colType) {
  const t = (colType || '').toUpperCase();
  if (t.includes('INT') || t.includes('REAL') || t.includes('FLOAT') || t.includes('NUMERIC') || t.includes('BIGINT')) return 'number';
  if (t.includes('BOOL')) return 'boolean';
  return 'text';
}

function getOperatorsForType(colType) {
  return OPERATORS[getColumnType(colType)] || OPERATORS.text;
}

function escapeSqlString(val) {
  return (val || '').replace(/'/g, "''");
}

function escapeLikeValue(val) {
  return escapeSqlString(val).replace(/%/g, '\\%').replace(/_/g, '\\_');
}

function highlightSQL(sql) {
  if (!sql) return '';
  const keywords = /\b(SELECT|FROM|WHERE|JOIN|INNER|LEFT|RIGHT|ON|AS|AND|OR|NOT|IN|LIKE|BETWEEN|IS|NULL|GROUP|BY|ORDER|HAVING|LIMIT|OFFSET|DISTINCT|COUNT|SUM|AVG|MAX|MIN|CASE|WHEN|THEN|ELSE|END|DESC|ASC)\b/gi;
  let result = sql
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
  result = result.replace(keywords, '<span class="sql-keyword">$1</span>');
  result = result.replace(/'([^']*)'/g, '<span class="sql-string">\'$1\'</span>');
  result = result.replace(/\b(\d+\.?\d*)\b/g, '<span class="sql-number">$1</span>');
  return result;
}

export default function QueryBuilder({ onQueryGenerated }) {
  const { schema, loading: schemaLoading } = useSchema();
  const [mode, setMode] = useState('visual');
  const [selectedTable, setSelectedTable] = useState('replays');
  const [selectedColumns, setSelectedColumns] = useState([]);
  const [filters, setFilters] = useState([]);
  const [joins, setJoins] = useState({});
  const [groupBy, setGroupBy] = useState([]);
  const [aggregates, setAggregates] = useState({});
  const [orderBy, setOrderBy] = useState([]);
  const [limit, setLimit] = useState(100);
  const [previewTemplate, setPreviewTemplate] = useState(null);

  const tables = useMemo(() => {
    if (!schema?.tables) return [];
    return Object.keys(schema.tables).filter(t =>
      ['replays', 'players', 'commands', 'detected_patterns_replay', 'detected_patterns_replay_team', 'detected_patterns_replay_player'].includes(t)
    );
  }, [schema]);

  const availableJoins = useMemo(() => {
    return JOIN_MAP[selectedTable] || [];
  }, [selectedTable]);

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
    availableJoins.forEach(j => {
      if (joins[j.table]) addTable(j.table);
    });
    return cols;
  }, [schema, selectedTable, joins, availableJoins]);

  const incompleteFilters = useMemo(() => {
    return filters.map((f, i) => {
      if (!f.column || !f.operator) return i;
      const op = getOperatorsForType(f.colType).find(o => o.value === f.operator);
      if (!op?.noValue && !f.value && f.value !== 0) return i;
      return null;
    }).filter(i => i !== null);
  }, [filters]);

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

    availableJoins.forEach(j => {
      if (joins[j.table]) {
        sql += `\nJOIN ${j.table} ON ${j.on}`;
      }
    });

    const validFilters = filters.filter(f => {
      if (!f.column || !f.operator) return false;
      const op = getOperatorsForType(f.colType).find(o => o.value === f.operator);
      if (op?.noValue) return true;
      return f.value !== '' && f.value !== undefined;
    });

    const whereClauses = validFilters.map(f => {
      const op = getOperatorsForType(f.colType).find(o => o.value === f.operator);
      if (op?.noValue) return `${f.column} ${f.operator}`;
      if (f.operator === 'LIKE' || f.operator === 'NOT LIKE') {
        return `${f.column} ${f.operator} '%${escapeLikeValue(f.value)}%'`;
      }
      if (getColumnType(f.colType) === 'number') {
        const num = Number(f.value);
        if (isNaN(num)) return null;
        return `${f.column} ${f.operator} ${num}`;
      }
      return `${f.column} ${f.operator} '${escapeSqlString(f.value)}'`;
    }).filter(Boolean);

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
  }, [selectedTable, selectedColumns, filters, joins, groupBy, aggregates, orderBy, limit, availableJoins]);

  const handleApply = () => {
    onQueryGenerated(generateSQL());
  };

  const handleTemplateSelect = (template) => {
    if (previewTemplate?.id === template.id) {
      setPreviewTemplate(null);
    } else {
      setPreviewTemplate(template);
    }
  };

  const handleTemplateUse = (template) => {
    onQueryGenerated(template.query);
    setPreviewTemplate(null);
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

  const handleAggregateChange = (col, val) => {
    const newAggs = { ...aggregates };
    if (val) newAggs[col] = val;
    else delete newAggs[col];
    setAggregates(newAggs);

    const hasAnyAggregate = Object.values(newAggs).some(Boolean);
    if (hasAnyAggregate) {
      setGroupBy(selectedColumns.filter(c => !newAggs[c]));
    } else {
      setGroupBy([]);
    }
  };

  const toggleOrderBy = (col) => {
    setOrderBy(prev => {
      const existing = prev.find(o => o.column === col);
      if (!existing) return [...prev, { column: col, dir: 'ASC' }];
      if (existing.dir === 'ASC') return prev.map(o => o.column === col ? { ...o, dir: 'DESC' } : o);
      return prev.filter(o => o.column !== col);
    });
  };

  const resetBuilder = () => {
    setSelectedColumns([]);
    setFilters([]);
    setJoins({});
    setGroupBy([]);
    setAggregates({});
    setOrderBy([]);
    setLimit(100);
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
            <div key={t.id}>
              <button
                className={`qb-template-card ${previewTemplate?.id === t.id ? 'active' : ''}`}
                onClick={() => handleTemplateSelect(t)}
              >
                <span className="qb-template-name">{t.name}</span>
                <span className="qb-template-desc">{t.description}</span>
              </button>
              {previewTemplate?.id === t.id && (
                <>
                  <pre
                    className="qb-template-preview"
                    dangerouslySetInnerHTML={{ __html: highlightSQL(t.query) }}
                  />
                  <div className="qb-template-actions">
                    <button className="btn-primary" onClick={() => handleTemplateUse(t)}>
                      Use This Query
                    </button>
                  </div>
                </>
              )}
            </div>
          ))}
        </div>
      )}

      {mode === 'visual' && (
        <div className="qb-visual">
          <div className="qb-step">
            <div className="qb-step-header">
              <span className="qb-step-num">1</span>
              <span className="qb-step-title">From Table</span>
              <button className="qb-reset-btn" onClick={resetBuilder}>Clear All</button>
            </div>
            <select
              className="qb-select"
              value={selectedTable}
              onChange={(e) => {
                setSelectedTable(e.target.value);
                setSelectedColumns([]);
                setFilters([]);
                setGroupBy([]);
                setAggregates({});
                setOrderBy([]);
                setJoins({});
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
            {selectedColumns.length === 0 && (
              <div className="qb-select-star-note">All columns will be returned (SELECT *)</div>
            )}
          </div>

          <div className="qb-step">
            <div className="qb-step-header">
              <span className="qb-step-num">3</span>
              <span className="qb-step-title">Include Related Data</span>
            </div>
            {availableJoins.length > 0 ? (
              <div className="qb-joins">
                {availableJoins.map(j => (
                  <label key={j.table} className="qb-join-check">
                    <input
                      type="checkbox"
                      checked={!!joins[j.table]}
                      onChange={(e) => setJoins(prev => ({ ...prev, [j.table]: e.target.checked }))}
                    />
                    <span>{j.label}</span>
                  </label>
                ))}
              </div>
            ) : (
              <div className="qb-no-joins">No related tables available for {selectedTable}</div>
            )}
          </div>

          <div className="qb-step">
            <div className="qb-step-header">
              <span className="qb-step-num">4</span>
              <span className="qb-step-title">Filters</span>
              <button className="qb-link-btn" onClick={addFilter}>+ Add Filter</button>
            </div>
            {filters.map((filter, idx) => {
              const isIncomplete = incompleteFilters.includes(idx);
              return (
                <div key={idx}>
                  <div className={`qb-filter-row${isIncomplete ? ' qb-filter-incomplete' : ''}`}>
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
                  {isIncomplete && (
                    <div className="qb-filter-warning">Incomplete filter -- will be ignored</div>
                  )}
                </div>
              );
            })}
          </div>

          <div className="qb-step">
            <div className="qb-step-header">
              <span className="qb-step-num">5</span>
              <span className="qb-step-title">Group & Aggregate</span>
            </div>
            {selectedColumns.length > 0 ? (
              <div className="qb-agg-grid">
                {selectedColumns.map(col => (
                  <div key={col} className="qb-agg-row">
                    <span className="qb-agg-col">{col.split('.').pop()}</span>
                    <select
                      className="qb-select qb-agg-select"
                      value={aggregates[col] || ''}
                      onChange={(e) => handleAggregateChange(col, e.target.value)}
                    >
                      {AGGREGATES.map(a => (
                        <option key={a.value} value={a.value}>{a.label}</option>
                      ))}
                    </select>
                  </div>
                ))}
              </div>
            ) : (
              <div className="qb-select-star-note">Select columns first to configure aggregates</div>
            )}
          </div>

          <div className="qb-step">
            <div className="qb-step-header">
              <span className="qb-step-num">6</span>
              <span className="qb-step-title">Sort & Limit</span>
            </div>
            {selectedColumns.length > 0 && (
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
            )}
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
            <pre
              className="qb-preview-code"
              dangerouslySetInnerHTML={{ __html: highlightSQL(generateSQL()) }}
            />
          </div>

          <button className="qb-apply-btn" onClick={handleApply}>
            Apply Query
          </button>
        </div>
      )}
    </div>
  );
}

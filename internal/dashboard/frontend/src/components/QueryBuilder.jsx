import React, { useState, useMemo, useCallback } from 'react';
import { useSchema } from '../hooks/useSchema';
import { QUERY_TEMPLATES, TABLE_LABELS } from '../constants/chartTypes';
import { highlightSQL } from '../utils/sqlHighlight';

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
    { table: 'detected_patterns_replay', label: 'Replay patterns', on: 'detected_patterns_replay.replay_id = replays.id' },
    { table: 'detected_patterns_replay_team', label: 'Team patterns', on: 'detected_patterns_replay_team.replay_id = replays.id' },
    { table: 'detected_patterns_replay_player', label: 'Player patterns', on: 'detected_patterns_replay_player.replay_id = replays.id' },
  ],
  players: [
    { table: 'replays', label: 'Replay info', on: 'replays.id = players.replay_id' },
    { table: 'detected_patterns_replay_player', label: 'Player patterns', on: 'detected_patterns_replay_player.player_id = players.id' },
  ],
  commands: [
    { table: 'replays', label: 'Replay info', on: 'replays.id = commands.replay_id' },
    { table: 'players', label: 'Player details', on: 'players.id = commands.player_id' },
  ],
  detected_patterns_replay: [
    { table: 'replays', label: 'Replay info', on: 'replays.id = detected_patterns_replay.replay_id' },
  ],
  detected_patterns_replay_team: [
    { table: 'replays', label: 'Replay info', on: 'replays.id = detected_patterns_replay_team.replay_id' },
  ],
  detected_patterns_replay_player: [
    { table: 'replays', label: 'Replay info', on: 'replays.id = detected_patterns_replay_player.replay_id' },
    { table: 'players', label: 'Player details', on: 'players.id = detected_patterns_replay_player.player_id' },
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

function FilterRow({
  filter,
  columnOptions,
  operators,
  valueType,
  isIncomplete,
  incompleteWarning,
  onColumnChange,
  onOperatorChange,
  onValueChange,
  onRemove,
}) {
  const op = operators.find(o => o.value === filter.operator);
  const showValue = !op?.noValue;
  return (
    <div>
      <div className={`qb-filter-row${isIncomplete ? ' qb-filter-incomplete' : ''}`}>
        <select
          className="qb-select qb-filter-col"
          value={filter.column}
          onChange={(e) => onColumnChange(e.target.value)}
        >
          <option value="">{columnOptions.placeholder}</option>
          {columnOptions.options.map(opt => (
            <option key={opt.value} value={opt.value}>{opt.label}</option>
          ))}
        </select>
        <select
          className="qb-select qb-filter-op"
          value={filter.operator}
          onChange={(e) => onOperatorChange(e.target.value)}
        >
          {operators.map(o => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
        {showValue && (
          <input
            className="qb-input qb-filter-val"
            type={valueType}
            value={filter.value}
            onChange={(e) => onValueChange(e.target.value)}
            placeholder="Value..."
          />
        )}
        <button type="button" className="qb-remove-btn" onClick={onRemove}>x</button>
      </div>
      {isIncomplete && <div className="qb-filter-warning">{incompleteWarning}</div>}
    </div>
  );
}

const TEMPLATE_CATEGORY_LABELS = {
  overview: 'Overview',
  win_rates: 'Win rates & races',
  apm_skill: 'APM & skill',
  bgh_teams: 'BGH & teams',
  carriers_build: 'Carriers & build order',
  suspicious: 'Suspicious / review',
};

export default function QueryBuilder({ onQueryGenerated }) {
  const { schema, loading: schemaLoading } = useSchema();
  const [builderMode, setBuilderMode] = useState('easy');
  const [mode, setMode] = useState('visual');
  const [selectedTable, setSelectedTable] = useState('replays');
  const [easyFilterMap, setEasyFilterMap] = useState('');
  const [easyFilterRace, setEasyFilterRace] = useState('');
  const [selectedColumns, setSelectedColumns] = useState([]);
  const [filters, setFilters] = useState([]);
  const [joins, setJoins] = useState({});
  const [groupBy, setGroupBy] = useState([]);
  const [aggregates, setAggregates] = useState({});
  const [orderBy, setOrderBy] = useState([]);
  const [havingFilters, setHavingFilters] = useState([]);
  const [limit, setLimit] = useState(100);
  const [previewTemplate, setPreviewTemplate] = useState(null);
  const [selectedMilestonePattern, setSelectedMilestonePattern] = useState('');

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

  const availableHavingColumns = useMemo(() => {
    return Object.entries(aggregates)
      .filter(([, agg]) => agg)
      .map(([col]) => `${aggregates[col].toLowerCase()}_${col.split('.').pop()}`);
  }, [aggregates]);

  const incompleteHavingFilters = useMemo(() => {
    return havingFilters.map((f, i) => {
      if (!f.column || !f.operator) return i;
      const op = OPERATORS.number.find(o => o.value === f.operator);
      if (op?.noValue) return null;
      const num = Number(f.value);
      if (f.value !== '' && f.value !== undefined && !isNaN(num)) return null;
      return i;
    }).filter(i => i !== null);
  }, [havingFilters]);

  const templatesByCategory = useMemo(() => {
    const map = {};
    QUERY_TEMPLATES.forEach(t => {
      const cat = t.category || 'overview';
      if (!map[cat]) map[cat] = [];
      map[cat].push(t);
    });
    const order = ['overview', 'win_rates', 'apm_skill', 'bgh_teams', 'carriers_build', 'suspicious'];
    return order.filter(c => map[c]).map(c => ({ id: c, label: TEMPLATE_CATEGORY_LABELS[c] || c, templates: map[c] }));
  }, []);

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

    const validHavingFilters = havingFilters.filter(f => {
      if (!f.column || !f.operator) return false;
      const op = OPERATORS.number.find(o => o.value === f.operator);
      if (op?.noValue) return true;
      const num = Number(f.value);
      return f.value !== '' && f.value !== undefined && !isNaN(num);
    });
    const havingClauses = validHavingFilters.map(f => {
      const op = OPERATORS.number.find(o => o.value === f.operator);
      if (op?.noValue) return `${f.column} ${f.operator}`;
      return `${f.column} ${f.operator} ${Number(f.value)}`;
    });
    if (havingClauses.length > 0) {
      sql += `\nHAVING ${havingClauses.join('\n  AND ')}`;
    }

    if (orderBy.length > 0) {
      sql += `\nORDER BY ${orderBy.map(o => `${o.column} ${o.dir}`).join(', ')}`;
    }

    sql += `\nLIMIT ${limit}`;
    return sql;
  }, [selectedTable, selectedColumns, filters, joins, groupBy, aggregates, havingFilters, orderBy, limit, availableJoins]);

  const handleApply = () => {
    onQueryGenerated(generateSQL());
  };

  const handleTemplateSelect = (template) => {
    if (previewTemplate?.id === template.id) {
      setPreviewTemplate(null);
      setSelectedMilestonePattern('');
    } else {
      setPreviewTemplate(template);
      setSelectedMilestonePattern(template.milestoneOptions?.[0]?.value ?? '');
    }
  };

  const getEffectiveMilestone = (template) => {
    if (!template?.milestoneOptions?.length) return '';
    const valid = template.milestoneOptions.some(o => o.value === selectedMilestonePattern);
    return valid ? selectedMilestonePattern : (template.milestoneOptions[0]?.value ?? '');
  };

  const getTemplateQueryForPreview = (template) => {
    if (template.query) return template.query;
    if (template.queryTemplate && template.milestoneOptions?.length) {
      const milestone = getEffectiveMilestone(template);
      return milestone
        ? template.queryTemplate.replace('__MILESTONE__', `'${escapeSqlString(milestone)}'`)
        : template.queryTemplate.replace('__MILESTONE__', "'(choose milestone)'");
    }
    return '';
  };

  const handleTemplateUse = (template) => {
    let q = template.query ?? template.queryTemplate;
    if (template.queryTemplate && template.milestoneOptions?.length) {
      const milestone = getEffectiveMilestone(template);
      q = template.queryTemplate.replace('__MILESTONE__', `'${escapeSqlString(milestone)}'`);
    }
    const clauses = [];
    if (builderMode === 'easy' && (easyFilterMap?.trim() || easyFilterRace)) {
      if (easyFilterMap?.trim() && /\br\.\b/.test(q)) {
        clauses.push(`r.map_name LIKE '%${escapeLikeValue(easyFilterMap.trim())}%'`);
      }
      if (easyFilterRace && /\bp\.\b/.test(q)) {
        clauses.push(`p.race = '${escapeSqlString(easyFilterRace)}'`);
      }
      if (clauses.length > 0) {
        const addition = (/ WHERE /i.test(q) ? ' AND ' : ' WHERE ') + clauses.join(' AND ');
        q = q.replace(/( ORDER BY | LIMIT \d+)/i, addition + ' $1');
      }
    }
    onQueryGenerated(q, {
      chartType: template.chartType,
      config: template.config,
    });
    setPreviewTemplate(null);
    setSelectedMilestonePattern('');
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
      setHavingFilters([]);
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

  const addHavingFilter = () => {
    setHavingFilters(prev => [...prev, { column: '', operator: '>=', value: '', colType: 'number' }]);
  };

  const updateHavingFilter = (idx, field, value) => {
    setHavingFilters(prev => prev.map((f, i) => (i !== idx) ? f : { ...f, [field]: value }));
  };

  const removeHavingFilter = (idx) => {
    setHavingFilters(prev => prev.filter((_, i) => i !== idx));
  };

  const resetBuilder = () => {
    setSelectedColumns([]);
    setFilters([]);
    setJoins({});
    setGroupBy([]);
    setAggregates({});
    setHavingFilters([]);
    setOrderBy([]);
    setLimit(100);
  };

  if (schemaLoading) {
    return <div className="qb-loading">Loading schema...</div>;
  }

  return (
    <div className="query-builder">
      <div className="qb-header">
        <div className="qb-header-row">
          <div className="qb-mode-toggle segment-control">
            <button
              type="button"
              className={`qb-mode-btn segment-control-btn ${builderMode === 'easy' ? 'active' : ''}`}
              onClick={() => setBuilderMode('easy')}
            >
              Easy
            </button>
            <button
              type="button"
              className={`qb-mode-btn segment-control-btn ${builderMode === 'advanced' ? 'active' : ''}`}
              onClick={() => setBuilderMode('advanced')}
            >
              Advanced
            </button>
          </div>
        </div>
        {builderMode === 'advanced' && (
          <div className="qb-header-row">
            <div className="qb-inner-toggle segment-control">
              <button
                type="button"
                className={`segment-control-btn ${mode === 'visual' ? 'active' : ''}`}
                onClick={() => setMode('visual')}
              >
                Visual Builder
              </button>
              <button
                type="button"
                className={`segment-control-btn ${mode === 'templates' ? 'active' : ''}`}
                onClick={() => setMode('templates')}
              >
                Templates
              </button>
            </div>
          </div>
        )}
      </div>

      {builderMode === 'easy' && (
        <div className="qb-easy">
          <p className="qb-easy-intro">What do you want to see? Pick a template, then add to dashboard.</p>
          <div className="qb-easy-filters">
            <label className="qb-easy-filter">
              <span>Only map containing:</span>
              <input
                type="text"
                className="qb-input input-base"
                value={easyFilterMap}
                onChange={(e) => setEasyFilterMap(e.target.value)}
                placeholder="e.g. BGH"
              />
            </label>
            <label className="qb-easy-filter">
              <span>Only race:</span>
              <select
                className="qb-select input-base"
                value={easyFilterRace}
                onChange={(e) => setEasyFilterRace(e.target.value)}
              >
                <option value="">Any</option>
                <option value="Terran">Terran</option>
                <option value="Protoss">Protoss</option>
                <option value="Zerg">Zerg</option>
              </select>
            </label>
          </div>
          {previewTemplate && (
            <div className="qb-templates-selected-panel">
              <div className="qb-use-selected-bar">
                <span className="qb-use-selected-label">Selected: {previewTemplate.name}</span>
                {previewTemplate.milestoneOptions?.length > 0 && (
                  <label className="qb-milestone-pick">
                    Reach:{' '}
                    <select
                      className="qb-select input-base"
                      value={getEffectiveMilestone(previewTemplate)}
                      onChange={(e) => setSelectedMilestonePattern(e.target.value)}
                    >
                      {previewTemplate.milestoneOptions.map(opt => (
                        <option key={opt.value} value={opt.value}>{opt.label}</option>
                      ))}
                    </select>
                  </label>
                )}
                <button type="button" className="btn-primary" onClick={() => handleTemplateUse(previewTemplate)}>
                  Use this template
                </button>
              </div>
              <pre
                className="qb-template-preview qb-template-preview-above"
                dangerouslySetInnerHTML={{ __html: highlightSQL(getTemplateQueryForPreview(previewTemplate)) }}
              />
            </div>
          )}
          {templatesByCategory.map(({ id, label, templates }) => (
            <div key={id} className="qb-templates-group">
              <h3 className="qb-templates-group-title">{label}</h3>
              <div className="qb-templates">
                {templates.map(t => (
                  <button
                    key={t.id}
                    type="button"
                    className={`qb-template-card ${previewTemplate?.id === t.id ? 'active' : ''}`}
                    onClick={() => handleTemplateSelect(t)}
                  >
                    <span className="qb-template-name">{t.name}</span>
                    <span className="qb-template-desc">{t.description}</span>
                  </button>
                ))}
              </div>
            </div>
          ))}
          <div className="qb-easy-advanced-link">
            <button type="button" className="qb-link-btn" onClick={() => setBuilderMode('advanced')}>
              Build custom query (Advanced)
            </button>
          </div>
        </div>
      )}

      {builderMode === 'advanced' && mode === 'templates' && (
        <>
          {previewTemplate && (
            <div className="qb-templates-selected-panel">
              <div className="qb-use-selected-bar">
                <span className="qb-use-selected-label">Selected: {previewTemplate.name}</span>
                {previewTemplate.milestoneOptions?.length > 0 && (
                  <label className="qb-milestone-pick">
                    Reach:{' '}
                    <select
                      className="qb-select input-base"
                      value={getEffectiveMilestone(previewTemplate)}
                      onChange={(e) => setSelectedMilestonePattern(e.target.value)}
                    >
                      {previewTemplate.milestoneOptions.map(opt => (
                        <option key={opt.value} value={opt.value}>{opt.label}</option>
                      ))}
                    </select>
                  </label>
                )}
                <button type="button" className="btn-primary" onClick={() => handleTemplateUse(previewTemplate)}>
                  Use this template
                </button>
              </div>
              <pre
                className="qb-template-preview qb-template-preview-above"
                dangerouslySetInnerHTML={{ __html: highlightSQL(getTemplateQueryForPreview(previewTemplate)) }}
              />
            </div>
          )}
          <div className="qb-templates">
            {QUERY_TEMPLATES.map(t => (
              <button
                key={t.id}
                type="button"
                className={`qb-template-card ${previewTemplate?.id === t.id ? 'active' : ''}`}
                onClick={() => handleTemplateSelect(t)}
              >
                <span className="qb-template-name">{t.name}</span>
                <span className="qb-template-desc">{t.description}</span>
              </button>
            ))}
          </div>
        </>
      )}

      {builderMode === 'advanced' && mode === 'visual' && (
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
                <option key={t} value={t}>
                  {TABLE_LABELS[t] ? `${TABLE_LABELS[t]} (${t})` : t}
                </option>
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
            {filters.map((filter, idx) => (
              <FilterRow
                key={idx}
                filter={filter}
                columnOptions={{
                  placeholder: 'Column...',
                  options: availableColumns.map(c => ({ value: c.qualified, label: `${c.column} (${c.table})` })),
                }}
                operators={getOperatorsForType(filter.colType)}
                valueType={getColumnType(filter.colType) === 'number' ? 'number' : 'text'}
                isIncomplete={incompleteFilters.includes(idx)}
                incompleteWarning="Incomplete filter -- will be ignored"
                onColumnChange={(v) => updateFilter(idx, 'column', v)}
                onOperatorChange={(v) => updateFilter(idx, 'operator', v)}
                onValueChange={(v) => updateFilter(idx, 'value', v)}
                onRemove={() => removeFilter(idx)}
              />
            ))}
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

          {availableHavingColumns.length > 0 && (
            <div className="qb-step">
              <div className="qb-step-header">
                <span className="qb-step-num">6</span>
                <span className="qb-step-title">Having</span>
                <button className="qb-link-btn" onClick={addHavingFilter}>+ Add condition</button>
              </div>
              {havingFilters.map((filter, idx) => (
                <FilterRow
                  key={idx}
                  filter={filter}
                  columnOptions={{
                    placeholder: 'Aggregate...',
                    options: availableHavingColumns.map(alias => ({ value: alias, label: alias })),
                  }}
                  operators={OPERATORS.number}
                  valueType="number"
                  isIncomplete={incompleteHavingFilters.includes(idx)}
                  incompleteWarning="Incomplete HAVING condition -- will be ignored"
                  onColumnChange={(v) => updateHavingFilter(idx, 'column', v)}
                  onOperatorChange={(v) => updateHavingFilter(idx, 'operator', v)}
                  onValueChange={(v) => updateHavingFilter(idx, 'value', v)}
                  onRemove={() => removeHavingFilter(idx)}
                />
              ))}
            </div>
          )}

          <div className="qb-step">
            <div className="qb-step-header">
              <span className="qb-step-num">{availableHavingColumns.length > 0 ? '7' : '6'}</span>
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

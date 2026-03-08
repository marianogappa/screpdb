const SQL_KEYWORDS = [
  'SELECT', 'FROM', 'WHERE', 'JOIN', 'INNER', 'LEFT', 'RIGHT', 'FULL', 'OUTER',
  'ON', 'AS', 'AND', 'OR', 'NOT', 'IN', 'EXISTS', 'LIKE', 'ILIKE', 'BETWEEN',
  'IS', 'NULL', 'TRUE', 'FALSE', 'GROUP', 'BY', 'ORDER', 'HAVING', 'LIMIT',
  'OFFSET', 'DISTINCT', 'UNION', 'ALL', 'INTERSECT', 'EXCEPT', 'CASE', 'WHEN',
  'THEN', 'ELSE', 'END', 'CAST', 'COUNT', 'SUM', 'AVG', 'MAX', 'MIN', 'COALESCE',
  'NULLIF', 'GREATEST', 'LEAST', 'EXTRACT', 'DATE_PART', 'NOW', 'CURRENT_DATE',
  'CURRENT_TIME', 'INTERVAL', 'OVER', 'PARTITION', 'WINDOW', 'ROW_NUMBER',
  'RANK', 'DENSE_RANK', 'LAG', 'LEAD', 'FIRST_VALUE', 'LAST_VALUE',
];

export function highlightSQL(sql) {
  if (!sql) return '';

  let highlighted = sql;

  // Escape HTML
  highlighted = highlighted
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');

  // Highlight SQL keywords (case-insensitive)
  const keywordPattern = new RegExp(`\\b(${SQL_KEYWORDS.join('|')})\\b`, 'gi');
  highlighted = highlighted.replace(keywordPattern, '<span class="sql-keyword">$1</span>');

  // Highlight strings (single and double quoted)
  highlighted = highlighted.replace(/'([^']*)'/g, '<span class="sql-string">\'$1\'</span>');
  highlighted = highlighted.replace(/"([^"]*)"/g, '<span class="sql-string">"$1"</span>');

  // Highlight numbers
  highlighted = highlighted.replace(/\b(\d+\.?\d*)\b/g, '<span class="sql-number">$1</span>');

  // Highlight comments (-- and /* */)
  highlighted = highlighted.replace(/--.*$/gm, '<span class="sql-comment">$&</span>');
  highlighted = highlighted.replace(/\/\*[\s\S]*?\*\//g, '<span class="sql-comment">$&</span>');

  return highlighted;
}


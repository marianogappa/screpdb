import React, { useRef, useEffect } from 'react';

const SQL_KEYWORDS = [
  'SELECT', 'FROM', 'WHERE', 'JOIN', 'INNER', 'LEFT', 'RIGHT', 'FULL', 'OUTER',
  'ON', 'AS', 'AND', 'OR', 'NOT', 'IN', 'EXISTS', 'LIKE', 'ILIKE', 'BETWEEN',
  'IS', 'NULL', 'TRUE', 'FALSE', 'GROUP', 'BY', 'ORDER', 'HAVING', 'LIMIT',
  'OFFSET', 'DISTINCT', 'UNION', 'ALL', 'INTERSECT', 'EXCEPT', 'CASE', 'WHEN',
  'THEN', 'ELSE', 'END', 'CAST', 'COUNT', 'SUM', 'AVG', 'MAX', 'MIN', 'COALESCE',
  'NULLIF', 'GREATEST', 'LEAST', 'EXTRACT', 'DATE_PART', 'NOW', 'CURRENT_DATE',
  'CURRENT_TIME', 'INTERVAL', 'OVER', 'PARTITION', 'WINDOW', 'ROW_NUMBER',
  'RANK', 'DENSE_RANK', 'LAG', 'LEAD', 'FIRST_VALUE', 'LAST_VALUE'
];

function SqlEditor({ value, onChange, placeholder, className = '' }) {
  const textareaRef = useRef(null);
  const highlightRef = useRef(null);

  useEffect(() => {
    const textarea = textareaRef.current;
    const highlight = highlightRef.current;
    
    if (!textarea || !highlight) return;

    const scrollHandler = () => {
      highlight.scrollTop = textarea.scrollTop;
      highlight.scrollLeft = textarea.scrollLeft;
    };

    textarea.addEventListener('scroll', scrollHandler);
    return () => textarea.removeEventListener('scroll', scrollHandler);
  }, []);

  const highlightSQL = (sql) => {
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
  };

  const handleChange = (e) => {
    onChange(e.target.value);
  };

  const handleKeyDown = (e) => {
    // Handle tab insertion
    if (e.key === 'Tab') {
      e.preventDefault();
      const textarea = textareaRef.current;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const value = textarea.value;
      const newValue = value.substring(0, start) + '  ' + value.substring(end);
      textarea.value = newValue;
      textarea.selectionStart = textarea.selectionEnd = start + 2;
      onChange(newValue);
    }
  };

  return (
    <div className={`sql-editor-wrapper ${className}`}>
      <div 
        className="sql-editor-highlight" 
        ref={highlightRef} 
        dangerouslySetInnerHTML={{ __html: highlightSQL(value) || '&nbsp;' }} 
      />
      <textarea
        ref={textareaRef}
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        className="sql-editor-input"
        placeholder={placeholder}
        spellCheck={false}
      />
    </div>
  );
}

export default SqlEditor;


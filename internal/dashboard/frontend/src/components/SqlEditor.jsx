import React, { useRef, useEffect } from 'react';
import { highlightSQL } from '../utils/sqlHighlight';

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

  const handleChange = (e) => {
    onChange(e.target.value);
  };

  const handleKeyDown = (e) => {
    // Handle tab insertion
    if (e.key === 'Tab') {
      e.preventDefault();
      const textarea = textareaRef.current;
      if (!textarea) return;
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


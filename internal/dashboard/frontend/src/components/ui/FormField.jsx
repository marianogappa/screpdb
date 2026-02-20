import React from 'react';

export default function FormField({
  label,
  required,
  type = 'text',
  value,
  onChange,
  options,
  placeholder,
  className = '',
  readOnly,
  rows,
  children,
  ...rest
}) {
  if (type === 'checkbox') {
    return (
      <label className="form-checkbox-label">
        <input
          type="checkbox"
          checked={value || false}
          onChange={(e) => onChange(e.target.checked)}
          {...rest}
        />
        <span>{label}</span>
      </label>
    );
  }

  const renderInput = () => {
    if (children) return children;

    if (type === 'select') {
      return (
        <select
          value={value ?? ''}
          onChange={(e) => onChange(e.target.value)}
          className="form-input"
          {...rest}
        >
          {options?.map((opt) => (
            <option key={opt.value ?? opt} value={opt.value ?? opt}>
              {opt.label ?? opt}
            </option>
          ))}
        </select>
      );
    }

    if (type === 'textarea') {
      return (
        <textarea
          value={value ?? ''}
          onChange={(e) => onChange(e.target.value)}
          className={`form-textarea ${className}`}
          placeholder={placeholder}
          readOnly={readOnly}
          rows={rows || 8}
          {...rest}
        />
      );
    }

    return (
      <input
        type={type}
        value={value ?? ''}
        onChange={(e) => {
          if (type === 'number') {
            onChange(e.target.value ? parseFloat(e.target.value) : undefined);
          } else {
            onChange(e.target.value);
          }
        }}
        className={`form-input ${className}`}
        placeholder={placeholder}
        readOnly={readOnly}
        {...rest}
      />
    );
  };

  return (
    <div className="form-group">
      {label && (
        <label>
          {label}
          {required && <span className="required-mark">*</span>}
        </label>
      )}
      {renderInput()}
    </div>
  );
}

import React from 'react';

const VARIANT_CLASSES = {
  primary: 'btn-primary',
  secondary: 'btn-secondary',
  icon: 'btn-widget-action',
  'icon-close': 'btn-icon-close',
  'icon-delete': 'btn-widget-action btn-widget-delete',
};

export default function Button({ variant = 'primary', className = '', children, ...props }) {
  const base = VARIANT_CLASSES[variant] || VARIANT_CLASSES.primary;
  return (
    <button className={`${base} ${className}`.trim()} {...props}>
      {children}
    </button>
  );
}

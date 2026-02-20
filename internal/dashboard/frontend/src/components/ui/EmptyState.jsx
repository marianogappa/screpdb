import React from 'react';
import Icon from './Icon';

export default function EmptyState({ icon, title, description, actions, hint }) {
  return (
    <div className="empty-state">
      {icon && (
        <div className="empty-state-icon">
          {typeof icon === 'string' ? <Icon name={icon} size={64} /> : icon}
        </div>
      )}
      {title && <h2>{title}</h2>}
      {description && <p>{description}</p>}
      {actions && <div className="empty-state-actions">{actions}</div>}
      {hint && (
        <div className="empty-state-hint">
          <Icon name="info" size={16} />
          {hint}
        </div>
      )}
    </div>
  );
}

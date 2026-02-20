import React, { useEffect } from 'react';

export default function Modal({ title, onClose, className = '', children }) {
  useEffect(() => {
    const handler = (e) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [onClose]);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className={`modal-content ${className}`.trim()} onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>{title}</h2>
          <button onClick={onClose} className="btn-close">&times;</button>
        </div>
        {children}
      </div>
    </div>
  );
}

import React, { useState, useEffect } from 'react';

export function LoadingScreen({ message = 'Loading...' }) {
  return (
    <div className="loading">
      <div className="loading-spinner" />
      <span>{message}</span>
    </div>
  );
}

const DEFAULT_MESSAGES = [
  'Analyzing your request...',
  'Querying the database...',
  'Generating SQL query...',
  'Creating visualization...',
  'Rendering chart...',
  'Almost done...',
];

export function OverlaySpinner({ messages = DEFAULT_MESSAGES, subtitle = 'This may take 10-30 seconds' }) {
  const [idx, setIdx] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setIdx((prev) => (prev + 1) % messages.length);
    }, 2000);
    return () => clearInterval(interval);
  }, [messages.length]);

  return (
    <div className="widget-creation-spinner-overlay">
      <div className="widget-creation-spinner-content">
        <div className="spinner" />
        <div className="spinner-message">{messages[idx]}</div>
        <div className="spinner-subtitle">{subtitle}</div>
      </div>
    </div>
  );
}

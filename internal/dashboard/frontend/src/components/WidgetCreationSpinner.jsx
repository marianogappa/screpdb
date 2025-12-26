import React, { useState, useEffect } from 'react';

const messages = [
  'Analyzing your request...',
  'Querying the database...',
  'Generating SQL query...',
  'Creating visualization...',
  'Rendering chart...',
  'Almost done...',
];

function WidgetCreationSpinner() {
  const [currentMessageIndex, setCurrentMessageIndex] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setCurrentMessageIndex((prev) => (prev + 1) % messages.length);
    }, 2000); // Change message every 2 seconds

    return () => clearInterval(interval);
  }, []);

  return (
    <div className="widget-creation-spinner-overlay">
      <div className="widget-creation-spinner-content">
        <div className="spinner"></div>
        <div className="spinner-message">{messages[currentMessageIndex]}</div>
        <div className="spinner-subtitle">This may take 10-30 seconds</div>
      </div>
    </div>
  );
}

export default WidgetCreationSpinner;


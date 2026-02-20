import { useEffect, useRef, useState } from 'react';

export function useChartDimensions({ minWidth = 300, minHeight = 300, deps = [] } = {}) {
  const containerRef = useRef(null);
  const svgRef = useRef(null);
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });

  useEffect(() => {
    const updateDimensions = () => {
      if (!containerRef.current) return;
      const { width, height } = containerRef.current.getBoundingClientRect();
      const newWidth = Math.max(minWidth, width);
      const newHeight = Math.max(minHeight, height);
      setDimensions((prev) => {
        if (Math.abs(prev.width - newWidth) > 1 || Math.abs(prev.height - newHeight) > 1) {
          return { width: newWidth, height: newHeight };
        }
        return prev;
      });
    };

    updateDimensions();
    const resizeObserver = new ResizeObserver(updateDimensions);
    if (containerRef.current) {
      resizeObserver.observe(containerRef.current);
    }
    return () => resizeObserver.disconnect();
  }, deps);

  return { containerRef, svgRef, dimensions };
}

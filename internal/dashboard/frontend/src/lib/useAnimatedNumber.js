import { useEffect, useRef, useState } from 'react';

const DURATION_MS = 600;

const prefersReducedMotion = () => (
  typeof window !== 'undefined'
  && typeof window.matchMedia === 'function'
  && window.matchMedia('(prefers-reduced-motion: reduce)').matches
);

// A hidden tab gets no animation frames, so an animation started there would hold the old value.
const pageHidden = () => typeof document !== 'undefined' && document.visibilityState === 'hidden';

// useAnimatedNumber eases the displayed integer from its last value to the new one, so an aggregate
// that is recomputed as engine data arrives counts to its new value instead of jumping.
export default function useAnimatedNumber(value) {
  const target = Number.isFinite(value) ? value : 0;
  const [shown, setShown] = useState(target);
  const shownRef = useRef(target);

  useEffect(() => {
    const from = shownRef.current;
    if (from === target || prefersReducedMotion() || pageHidden() || typeof window.requestAnimationFrame !== 'function') {
      shownRef.current = target;
      setShown(target);
      return undefined;
    }
    const start = performance.now();
    let frame;
    const step = (now) => {
      const progress = Math.min(1, (now - start) / DURATION_MS);
      const eased = 1 - (1 - progress) ** 3;
      const next = Math.round(from + (target - from) * eased);
      shownRef.current = next;
      setShown(next);
      if (progress < 1) frame = window.requestAnimationFrame(step);
    };
    frame = window.requestAnimationFrame(step);
    return () => window.cancelAnimationFrame(frame);
  }, [target]);

  return shown;
}

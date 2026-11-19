import React, { useLayoutEffect, useRef } from 'react';

// OneRowPills shows only the pills that fit on the first row. Children come in priority order, so what
// wraps to a second row is the least important; hiding it never moves the pills before it.
export default function OneRowPills({ className, children }) {
  const ref = useRef(null);
  const width = useRef(-1);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return undefined;
    const fit = () => {
      const kids = [...el.children];
      kids.forEach((k) => { k.style.display = ''; });
      if (kids.length === 0) return;
      const firstBottom = kids[0].offsetTop + kids[0].offsetHeight;
      const rowBottom = Math.max(...kids.filter((k) => k.offsetTop < firstBottom).map((k) => k.offsetTop + k.offsetHeight));
      kids.forEach((k) => {
        if (k.offsetTop >= rowBottom) k.style.display = 'none';
      });
    };
    fit();
    width.current = el.clientWidth;
    const ro = new ResizeObserver(() => {
      if (el.clientWidth === width.current) return;
      width.current = el.clientWidth;
      fit();
    });
    ro.observe(el);
    const mo = new MutationObserver(fit);
    mo.observe(el, { childList: true, subtree: true });
    return () => {
      ro.disconnect();
      mo.disconnect();
    };
  });
  return <div ref={ref} className={className}>{children}</div>;
}

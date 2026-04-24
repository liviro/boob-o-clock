import { useLayoutEffect, useRef, useState } from 'preact/hooks';

// Returns a ref plus the live-measured width (in CSS pixels) of whatever
// element it's attached to. Drives dynamic SVG viewBox sizing so user
// coordinates map 1:1 to CSS pixels — otherwise font-size and circle
// radius get scaled by the viewBox transform when the SVG stretches to
// fill a wider container (e.g. landscape).
export function useMeasuredWidth<T extends Element>(fallback: number) {
  const ref = useRef<T>(null);
  const [width, setWidth] = useState(fallback);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;

    const initial = el.getBoundingClientRect().width;
    if (initial > 0) setWidth(Math.round(initial));

    const observer = new ResizeObserver(entries => {
      const w = entries[0].contentRect.width;
      if (w > 0) setWidth(Math.round(w));
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return [ref, width] as const;
}

import { useState, useEffect } from 'preact/hooks';

const QUERY = '(orientation: landscape)';

export function useIsLandscape(): boolean {
  const [isLandscape, setIsLandscape] = useState<boolean>(() =>
    typeof window !== 'undefined' && window.matchMedia(QUERY).matches
  );

  useEffect(() => {
    const mql = window.matchMedia(QUERY);
    const handler = (e: MediaQueryListEvent) => setIsLandscape(e.matches);
    mql.addEventListener('change', handler);
    setIsLandscape(mql.matches);
    return () => mql.removeEventListener('change', handler);
  }, []);

  return isLandscape;
}

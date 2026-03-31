import { useRef, useEffect } from 'preact/hooks';

/**
 * Mobile browsers synthesize a click event ~300ms after touchend.
 * When a touch opens a modal, that ghost click lands on whatever is
 * now under the finger (overlay, buttons) and can dismiss/activate
 * the modal before the user interacts. This hook returns a wrapper
 * that suppresses callbacks fired within the guard window.
 */
const GHOST_CLICK_THRESHOLD_MS = 350;

export function useGhostClickGuard(open: boolean) {
  const openedAt = useRef(0);

  useEffect(() => {
    if (open) openedAt.current = Date.now();
  }, [open]);

  return <T extends (...args: never[]) => void>(fn: T): T =>
    ((...args: Parameters<T>) => {
      if (Date.now() - openedAt.current > GHOST_CLICK_THRESHOLD_MS) fn(...args);
    }) as unknown as T;
}

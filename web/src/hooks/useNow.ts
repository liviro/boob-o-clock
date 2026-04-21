import { useState, useEffect } from 'preact/hooks';

// Single shared 1 Hz interval across all useNow() consumers.
// Prevents per-component drift and keeps exactly one timer running.
let sharedInterval: ReturnType<typeof setInterval> | null = null;
const listeners = new Set<(n: number) => void>();

function startTicker() {
  if (sharedInterval !== null) return;
  sharedInterval = setInterval(() => {
    const n = Date.now();
    listeners.forEach(l => l(n));
  }, 1000);
}

function stopTicker() {
  if (sharedInterval === null) return;
  clearInterval(sharedInterval);
  sharedInterval = null;
}

export function useNow(): number {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    listeners.add(setNow);
    startTicker();
    return () => {
      listeners.delete(setNow);
      if (listeners.size === 0) stopTicker();
    };
  }, []);
  return now;
}

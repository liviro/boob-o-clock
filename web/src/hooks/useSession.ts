import { useState, useEffect, useCallback, useRef } from 'preact/hooks';
import { getCurrentSession, postEvent, postStartNight, postUndo, SessionResponse, StartNightConfig } from '../api';

const STALE_MS = 15 * 60 * 1000; // 15 minutes

export function useSession() {
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const lastFetchRef = useRef(0);

  function applySession(data: SessionResponse) {
    setSession(data);
    setError(null);
    lastFetchRef.current = Date.now();
  }

  const load = useCallback(async () => {
    try {
      applySession(await getCurrentSession());
    } catch {
      setError('Failed to connect to server');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  // Auto-refresh: on visibility change or every 15 min while visible
  useEffect(() => {
    function refreshIfStale() {
      if (!document.hidden && Date.now() - lastFetchRef.current >= STALE_MS) {
        load();
      }
    }
    document.addEventListener('visibilitychange', refreshIfStale);
    const id = setInterval(refreshIfStale, STALE_MS);
    return () => {
      document.removeEventListener('visibilitychange', refreshIfStale);
      clearInterval(id);
    };
  }, [load]);

  const dispatch = useCallback(async (
    action: string,
    metadata?: Record<string, string>,
    timestamp?: Date,
  ) => {
    try {
      applySession(await postEvent(action, metadata, timestamp));
      if (navigator.vibrate) navigator.vibrate(10);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Request failed');
    }
  }, []);

  const startNight = useCallback(async (config: StartNightConfig) => {
    try {
      applySession(await postStartNight(config));
      if (navigator.vibrate) navigator.vibrate(10);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Start night failed');
    }
  }, []);

  const undo = useCallback(async () => {
    try {
      applySession(await postUndo());
      if (navigator.vibrate) navigator.vibrate([10, 50, 10]);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Undo failed');
    }
  }, []);

  const clearError = useCallback(() => setError(null), []);

  return { session, loading, error, dispatch, startNight, undo, clearError };
}

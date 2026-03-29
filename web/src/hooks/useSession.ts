import { useState, useEffect, useCallback } from 'preact/hooks';
import { getCurrentSession, postEvent, postUndo, SessionResponse } from '../api';

export function useSession() {
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const data = await getCurrentSession();
      setSession(data);
      setError(null);
    } catch {
      setError('Failed to connect to server');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const dispatch = useCallback(async (
    action: string,
    metadata?: Record<string, string>,
    timestamp?: Date,
  ) => {
    try {
      const data = await postEvent(action, metadata, timestamp);
      setSession(data);
      setError(null);
      if (navigator.vibrate) navigator.vibrate(10);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Request failed');
    }
  }, []);

  const undo = useCallback(async () => {
    try {
      const data = await postUndo();
      setSession(data);
      setError(null);
      if (navigator.vibrate) navigator.vibrate([10, 50, 10]);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Undo failed');
    }
  }, []);

  const clearError = useCallback(() => setError(null), []);

  return { session, loading, error, dispatch, undo, clearError };
}

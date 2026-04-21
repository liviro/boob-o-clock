import { useState } from 'preact/hooks';
import { useSession } from './hooks/useSession';
import { Tracker } from './pages/Tracker';
import { History } from './pages/History';
import { ErrorToast } from './components/ErrorToast';

type Page = 'tracker' | 'history';

export function App() {
  const [page, setPage] = useState<Page>('tracker');
  const { session, loading, error, dispatch, startNight, undo, clearError } = useSession();

  if (loading) {
    return <div class="no-data">Loading...</div>;
  }

  return (
    <>
      <div class="tabs">
        <button
          class={`tab ${page === 'tracker' ? 'active' : ''}`}
          onClick={() => setPage('tracker')}
        >
          Tracker
        </button>
        <button
          class={`tab ${page === 'history' ? 'active' : ''}`}
          onClick={() => setPage('history')}
        >
          History
        </button>
      </div>

      {page === 'tracker' && session && (
        <Tracker session={session} onDispatch={dispatch} onStartNight={startNight} onUndo={undo} />
      )}

      {page === 'history' && <History />}

      <ErrorToast message={error} onDismiss={clearError} />
    </>
  );
}

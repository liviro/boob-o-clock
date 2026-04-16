import { useState, useCallback, useRef, useEffect } from 'preact/hooks';
import { SessionResponse } from '../api';
import { ACTION_INFO, actionLabel } from '../constants';
import { StateDisplay } from '../components/StateDisplay';
import { ActionGrid } from '../components/ActionGrid';
import { BreastPicker } from '../components/BreastPicker';
import { TimestampPicker } from '../components/TimestampPicker';
import { ConfirmModal } from '../components/ConfirmModal';
import { UndoButton } from '../components/UndoButton';

interface Props {
  session: SessionResponse;
  onDispatch: (action: string, metadata?: Record<string, string>, timestamp?: Date) => void;
  onUndo: () => void;
}

type ModalState =
  | { type: 'none' }
  | { type: 'breast'; action: string; wantsTimePicker: boolean }
  | { type: 'timestamp'; action: string; metadata?: Record<string, string>; title: string }
  | { type: 'confirm'; action: string; wantsTimePicker: boolean };

export function Tracker({ session, onDispatch, onUndo }: Props) {
  const [modal, setModal] = useState<ModalState>({ type: 'none' });
  const longPressTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const longPressFired = useRef(false);

  // Cleanup timer on unmount
  useEffect(() => {
    return () => { if (longPressTimer.current) clearTimeout(longPressTimer.current); };
  }, []);

  function fireAction(action: string, metadata?: Record<string, string>, timestamp?: Date) {
    onDispatch(action, metadata, timestamp ?? new Date());
  }

  function openTimePicker(action: string, metadata?: Record<string, string>) {
    const label = actionLabel(action);
    const breastSuffix = metadata?.breast ? ` (${metadata.breast})` : '';
    setModal({ type: 'timestamp', action, metadata, title: `${label}${breastSuffix} — When?` });
  }

  // Resolve metadata for switch_breast using the API-provided suggestion
  function switchBreastMeta(): Record<string, string> | undefined {
    return session.suggestBreast ? { breast: session.suggestBreast } : undefined;
  }

  // Shared action resolution: resolves modals/metadata, then calls terminal fn
  function resolveAction(action: string, wantsTimePicker: boolean) {
    const ai = ACTION_INFO[action];

    if (ai?.confirm) {
      setModal({ type: 'confirm', action, wantsTimePicker });
      return;
    }

    if (action === 'switch_breast') {
      const meta = switchBreastMeta();
      if (meta) {
        wantsTimePicker ? openTimePicker(action, meta) : fireAction(action, meta);
        return;
      }
    }

    if (ai?.needsBreast) {
      setModal({ type: 'breast', action, wantsTimePicker });
      return;
    }

    wantsTimePicker ? openTimePicker(action) : fireAction(action);
  }

  const handleAction = useCallback((action: string) => resolveAction(action, false),
    [session.suggestBreast, onDispatch]);

  const handleLongPress = useCallback((action: string) => resolveAction(action, true),
    [session.suggestBreast]);

  // Touch/mouse handlers for long-press detection
  const handlePointerDown = useCallback((action: string) => {
    longPressFired.current = false;
    longPressTimer.current = setTimeout(() => {
      longPressFired.current = true;
      handleLongPress(action);
      if (navigator.vibrate) navigator.vibrate(20);
    }, 400);
  }, [handleLongPress]);

  const handlePointerUp = useCallback((action: string) => {
    if (longPressTimer.current) {
      clearTimeout(longPressTimer.current);
      longPressTimer.current = null;
    }
    if (!longPressFired.current) {
      handleAction(action);
    }
  }, [handleAction]);

  const handlePointerCancel = useCallback(() => {
    if (longPressTimer.current) {
      clearTimeout(longPressTimer.current);
      longPressTimer.current = null;
    }
  }, []);

  const handleBreastPick = useCallback((side: 'L' | 'R') => {
    if (modal.type !== 'breast') return;
    const meta = { breast: side };
    setModal({ type: 'none' });
    modal.wantsTimePicker ? openTimePicker(modal.action, meta) : fireAction(modal.action, meta);
  }, [modal, onDispatch]);

  const handleTimestampPick = useCallback((minutesAgo: number) => {
    if (modal.type !== 'timestamp') return;
    const ts = new Date(Date.now() + minutesAgo * 60000);
    onDispatch(modal.action, modal.metadata, ts);
    setModal({ type: 'none' });
  }, [modal, onDispatch]);

  const handleConfirm = useCallback(() => {
    if (modal.type !== 'confirm') return;
    setModal({ type: 'none' });
    modal.wantsTimePicker ? openTimePicker(modal.action) : fireAction(modal.action);
  }, [modal, onDispatch]);

  const closeModal = useCallback(() => setModal({ type: 'none' }), []);

  return (
    <div class="page-tracker">
      <StateDisplay
        state={session.state}
        lastEventTimestamp={session.lastEvent?.timestamp}
        currentBreast={session.currentBreast}
        lastFeedStartedAt={session.lastFeedStartedAt}
      />

      <ActionGrid
        actions={session.validActions}
        onPointerDown={handlePointerDown}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerCancel}
      />

      <div class="bottom-bar">
        <UndoButton
          lastAction={session.lastEvent?.action}
          onUndo={onUndo}
        />
      </div>

      <div class="hint">hold for time adjust</div>

      <BreastPicker
        open={modal.type === 'breast'}
        suggestSide={session.suggestBreast}
        onPick={handleBreastPick}
        onClose={closeModal}
      />

      <TimestampPicker
        open={modal.type === 'timestamp'}
        title={modal.type === 'timestamp' ? modal.title : ''}
        onPick={handleTimestampPick}
        onClose={closeModal}
      />

      <ConfirmModal
        open={modal.type === 'confirm'}
        title={modal.type === 'confirm'
          ? `${ACTION_INFO[modal.action]?.icon || ''} ${actionLabel(modal.action)} — are you sure?`
          : ''}
        onConfirm={handleConfirm}
        onCancel={closeModal}
      />
    </div>
  );
}

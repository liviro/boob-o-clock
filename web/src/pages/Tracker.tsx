import { useState, useCallback, useRef } from 'preact/hooks';
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
  | { type: 'breast'; action: string }
  | { type: 'timestamp'; action: string; metadata?: Record<string, string>; title: string }
  | { type: 'confirm'; action: string };

export function Tracker({ session, onDispatch, onUndo }: Props) {
  const [modal, setModal] = useState<ModalState>({ type: 'none' });
  const longPressTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const longPressFired = useRef(false);

  // Resolve an action: fire with "now" or stash for time picker
  function fireAction(action: string, metadata?: Record<string, string>, timestamp?: Date) {
    onDispatch(action, metadata, timestamp ?? new Date());
  }

  function openTimePicker(action: string, metadata?: Record<string, string>) {
    const label = actionLabel(action);
    const breastSuffix = metadata?.breast ? ` (${metadata.breast})` : '';
    setModal({
      type: 'timestamp',
      action,
      metadata,
      title: `${label}${breastSuffix} — When?`,
    });
  }

  // Action button tap: resolves immediately (with breast picker or confirm if needed)
  const handleAction = useCallback((action: string) => {
    const ai = ACTION_INFO[action];

    if (ai?.confirm) {
      setModal({ type: 'confirm', action });
      return;
    }

    // Switch breast: auto-fill opposite of current, fire immediately
    if (action === 'switch_breast' && session.currentBreast) {
      const newSide = session.currentBreast === 'L' ? 'R' : 'L';
      fireAction(action, { breast: newSide });
      return;
    }

    if (ai?.needsBreast) {
      setModal({ type: 'breast', action });
      return;
    }

    fireAction(action);
  }, [session.currentBreast, onDispatch]);

  // Long-press: open time picker instead of firing immediately
  const handleLongPress = useCallback((action: string) => {
    const ai = ACTION_INFO[action];

    // For confirm actions, long-press also needs confirm first
    if (ai?.confirm) {
      setModal({ type: 'confirm', action });
      return;
    }

    // Switch breast with time picker
    if (action === 'switch_breast' && session.currentBreast) {
      const newSide = session.currentBreast === 'L' ? 'R' : 'L';
      openTimePicker(action, { breast: newSide });
      return;
    }

    if (ai?.needsBreast) {
      // Need breast first, then time picker will follow
      setModal({ type: 'breast', action });
      return;
    }

    openTimePicker(action);
  }, [session.currentBreast]);

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
    setModal({ type: 'none' });
    fireAction(modal.action, { breast: side });
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
    fireAction(modal.action);
  }, [modal, onDispatch]);

  const closeModal = useCallback(() => setModal({ type: 'none' }), []);

  return (
    <div class="page-tracker">
      <StateDisplay
        state={session.state}
        lastEventTimestamp={session.lastEvent?.timestamp}
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

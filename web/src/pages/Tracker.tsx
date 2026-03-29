import { useState, useCallback } from 'preact/hooks';
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

  const handleAction = useCallback((action: string) => {
    const ai = ACTION_INFO[action];

    if (ai?.confirm) {
      setModal({ type: 'confirm', action });
      return;
    }

    if (ai?.needsBreast) {
      setModal({ type: 'breast', action });
      return;
    }

    setModal({ type: 'timestamp', action, title: `${actionLabel(action)} — When?` });
  }, []);

  const handleBreastPick = useCallback((side: 'L' | 'R') => {
    if (modal.type !== 'breast') return;
    setModal({
      type: 'timestamp',
      action: modal.action,
      metadata: { breast: side },
      title: `${actionLabel(modal.action)} (${side}) — When?`,
    });
  }, [modal]);

  const handleTimestampPick = useCallback((minutesAgo: number) => {
    if (modal.type !== 'timestamp') return;
    const ts = new Date(Date.now() + minutesAgo * 60000);
    onDispatch(modal.action, modal.metadata, ts);
    setModal({ type: 'none' });
  }, [modal, onDispatch]);

  const handleConfirm = useCallback(() => {
    if (modal.type !== 'confirm') return;
    setModal({
      type: 'timestamp',
      action: modal.action,
      title: `${actionLabel(modal.action)} — When?`,
    });
  }, [modal]);

  const closeModal = useCallback(() => setModal({ type: 'none' }), []);

  return (
    <div class="page-tracker">
      <StateDisplay
        state={session.state}
        lastEventTimestamp={session.lastEvent?.timestamp}
      />

      <ActionGrid
        actions={session.validActions}
        onAction={handleAction}
      />

      <div class="bottom-bar">
        <UndoButton
          lastAction={session.lastEvent?.action}
          onUndo={onUndo}
        />
      </div>

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

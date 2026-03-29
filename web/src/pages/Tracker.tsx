import { useState, useCallback } from 'preact/hooks';
import { SessionResponse } from '../api';
import { ACTION_INFO } from '../constants';
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

    const label = ai ? ai.label.replace(/\n/g, ' ') : action;
    setModal({ type: 'timestamp', action, title: `${label} — When?` });
  }, []);

  const handleBreastPick = useCallback((side: 'L' | 'R') => {
    if (modal.type !== 'breast') return;
    const ai = ACTION_INFO[modal.action];
    const label = ai ? ai.label.replace(/\n/g, ' ') : modal.action;
    setModal({
      type: 'timestamp',
      action: modal.action,
      metadata: { breast: side },
      title: `${label} (${side}) — When?`,
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
    const ai = ACTION_INFO[modal.action];
    const label = ai ? ai.label.replace(/\n/g, ' ') : modal.action;
    setModal({
      type: 'timestamp',
      action: modal.action,
      title: `${label} — When?`,
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
          ? `${ACTION_INFO[modal.action]?.icon || ''} ${ACTION_INFO[modal.action]?.label.replace(/\n/g, ' ') || modal.action} — are you sure?`
          : ''}
        onConfirm={handleConfirm}
        onCancel={closeModal}
      />
    </div>
  );
}

import { Modal } from './Modal';
import { useGhostClickGuard } from '../hooks/useGhostClickGuard';

interface Props {
  open: boolean;
  title: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmModal({ open, title, onConfirm, onCancel }: Props) {
  const guard = useGhostClickGuard(open);

  return (
    <Modal open={open} onClose={onCancel} title={title}>
      <div class="confirm-grid">
        <button class="confirm-btn no" onClick={guard(onCancel)}>No</button>
        <button class="confirm-btn yes" onClick={guard(onConfirm)}>Yes</button>
      </div>
    </Modal>
  );
}

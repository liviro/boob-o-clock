import { Modal } from './Modal';

interface Props {
  open: boolean;
  title: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmModal({ open, title, onConfirm, onCancel }: Props) {
  return (
    <Modal open={open} onClose={onCancel} title={title}>
      <div class="confirm-grid">
        <button class="confirm-btn no" onClick={onCancel}>No</button>
        <button class="confirm-btn yes" onClick={onConfirm}>Yes</button>
      </div>
    </Modal>
  );
}

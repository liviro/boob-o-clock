interface Props {
  open: boolean;
  title: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmModal({ open, title, onConfirm, onCancel }: Props) {
  if (!open) return null;

  return (
    <div class="modal-overlay" onClick={onCancel}>
      <div class="modal-content" onClick={e => e.stopPropagation()}>
        <h3 class="modal-title">{title}</h3>
        <div class="confirm-grid">
          <button class="confirm-btn no" onClick={onCancel}>No</button>
          <button class="confirm-btn yes" onClick={onConfirm}>Yes</button>
        </div>
      </div>
    </div>
  );
}

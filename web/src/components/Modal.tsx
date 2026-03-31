import { ComponentChildren } from 'preact';
import { useGhostClickGuard } from '../hooks/useGhostClickGuard';

interface Props {
  open: boolean;
  onClose: () => void;
  title?: string;
  children: ComponentChildren;
}

export function Modal({ open, onClose, title, children }: Props) {
  const guard = useGhostClickGuard(open);

  if (!open) return null;

  return (
    <div class="modal-overlay" onClick={guard(onClose)}>
      <div class="modal-content" onClick={e => e.stopPropagation()}>
        {title && <h3 class="modal-title">{title}</h3>}
        {children}
        <button class="modal-cancel" onClick={onClose}>Cancel</button>
      </div>
    </div>
  );
}

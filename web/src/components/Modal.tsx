import { ComponentChildren } from 'preact';
import { useRef, useEffect } from 'preact/hooks';

interface Props {
  open: boolean;
  onClose: () => void;
  title?: string;
  children: ComponentChildren;
}

export function Modal({ open, onClose, title, children }: Props) {
  const openedAt = useRef(0);

  useEffect(() => {
    if (open) openedAt.current = Date.now();
  }, [open]);

  if (!open) return null;

  // Ignore clicks within 350ms of opening to prevent ghost-click dismissals
  const guardedClose = () => {
    if (Date.now() - openedAt.current > 350) onClose();
  };

  return (
    <div class="modal-overlay" onClick={guardedClose}>
      <div class="modal-content" onClick={e => e.stopPropagation()}>
        {title && <h3 class="modal-title">{title}</h3>}
        {children}
        <button class="modal-cancel" onClick={onClose}>Cancel</button>
      </div>
    </div>
  );
}

import { useRef, useEffect } from 'preact/hooks';
import { Modal } from './Modal';

interface Props {
  open: boolean;
  suggestSide?: string;
  onPick: (side: 'L' | 'R') => void;
  onClose: () => void;
}

export function BreastPicker({ open, suggestSide, onPick, onClose }: Props) {
  const openedAt = useRef(0);

  useEffect(() => {
    if (open) openedAt.current = Date.now();
  }, [open]);

  // Ignore picks within 350ms of opening to prevent ghost-click auto-selection
  const guardedPick = (side: 'L' | 'R') => {
    if (Date.now() - openedAt.current > 350) onPick(side);
  };

  return (
    <Modal open={open} onClose={onClose} title="Which side?">
      <div class="breast-grid">
        <button
          class={`breast-btn ${suggestSide === 'L' ? 'suggested' : ''}`}
          onClick={() => guardedPick('L')}
        >
          L{suggestSide === 'L' ? ' ★' : ''}
        </button>
        <button
          class={`breast-btn ${suggestSide === 'R' ? 'suggested' : ''}`}
          onClick={() => guardedPick('R')}
        >
          R{suggestSide === 'R' ? ' ★' : ''}
        </button>
      </div>
    </Modal>
  );
}

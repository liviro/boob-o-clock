import { Modal } from './Modal';
import { useGhostClickGuard } from '../hooks/useGhostClickGuard';

interface Props {
  open: boolean;
  suggestSide?: string;
  onPick: (side: 'L' | 'R') => void;
  onClose: () => void;
}

export function BreastPicker({ open, suggestSide, onPick, onClose }: Props) {
  const guard = useGhostClickGuard(open);

  return (
    <Modal open={open} onClose={onClose} title="Which side?">
      <div class="breast-grid">
        <button
          class={`breast-btn ${suggestSide === 'L' ? 'suggested' : ''}`}
          onClick={guard(() => onPick('L'))}
        >
          L{suggestSide === 'L' ? ' ★' : ''}
        </button>
        <button
          class={`breast-btn ${suggestSide === 'R' ? 'suggested' : ''}`}
          onClick={guard(() => onPick('R'))}
        >
          R{suggestSide === 'R' ? ' ★' : ''}
        </button>
      </div>
    </Modal>
  );
}

import { Modal } from './Modal';

interface Props {
  open: boolean;
  suggestSide?: string;
  onPick: (side: 'L' | 'R') => void;
  onClose: () => void;
}

export function BreastPicker({ open, suggestSide, onPick, onClose }: Props) {
  return (
    <Modal open={open} onClose={onClose} title="Which side?">
      <div class="breast-grid">
        <button
          class={`breast-btn ${suggestSide === 'L' ? 'suggested' : ''}`}
          onClick={() => onPick('L')}
        >
          L{suggestSide === 'L' ? ' ★' : ''}
        </button>
        <button
          class={`breast-btn ${suggestSide === 'R' ? 'suggested' : ''}`}
          onClick={() => onPick('R')}
        >
          R{suggestSide === 'R' ? ' ★' : ''}
        </button>
      </div>
    </Modal>
  );
}

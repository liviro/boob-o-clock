import { Modal } from './Modal';

interface Props {
  open: boolean;
  onPick: (side: 'L' | 'R') => void;
  onClose: () => void;
}

export function BreastPicker({ open, onPick, onClose }: Props) {
  return (
    <Modal open={open} onClose={onClose} title="Which side?">
      <div class="breast-grid">
        <button class="breast-btn" onClick={() => onPick('L')}>L</button>
        <button class="breast-btn" onClick={() => onPick('R')}>R</button>
      </div>
    </Modal>
  );
}

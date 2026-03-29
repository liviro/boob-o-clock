import { Modal } from './Modal';

interface Props {
  open: boolean;
  title: string;
  onPick: (minutesAgo: number) => void;
  onClose: () => void;
}

export function TimestampPicker({ open, title, onPick, onClose }: Props) {
  return (
    <Modal open={open} onClose={onClose} title={title}>
      <div class="ts-grid">
        <button class="ts-btn" onClick={() => onPick(-1)}>1 min ago</button>
        <button class="ts-btn" onClick={() => onPick(-3)}>3 min ago</button>
        <button class="ts-btn" onClick={() => onPick(-5)}>5 min ago</button>
        <button class="ts-btn" onClick={() => onPick(-10)}>10 min ago</button>
      </div>
    </Modal>
  );
}

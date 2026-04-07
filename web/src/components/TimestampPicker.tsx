import { Modal } from './Modal';
import { useGhostClickGuard } from '../hooks/useGhostClickGuard';

interface Props {
  open: boolean;
  title: string;
  /** Negative value = minutes in the past (e.g. -3 means "3 min ago"). */
  onPick: (minutesAgo: number) => void;
  onClose: () => void;
}

export function TimestampPicker({ open, title, onPick, onClose }: Props) {
  const guard = useGhostClickGuard(open);

  return (
    <Modal open={open} onClose={onClose} title={title}>
      <div class="ts-grid">
        <button class="ts-btn" onClick={guard(() => onPick(-1))}>1 min ago</button>
        <button class="ts-btn" onClick={guard(() => onPick(-3))}>3 min ago</button>
        <button class="ts-btn" onClick={guard(() => onPick(-5))}>5 min ago</button>
        <button class="ts-btn" onClick={guard(() => onPick(-10))}>10 min ago</button>
      </div>
    </Modal>
  );
}

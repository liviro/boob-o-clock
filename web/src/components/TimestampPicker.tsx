import { useEffect, useState } from 'preact/hooks';
import { Modal } from './Modal';
import { useGhostClickGuard } from '../hooks/useGhostClickGuard';
import { fmtLocalYMDHM } from '../constants';

interface Props {
  open: boolean;
  title: string;
  onPick: (ts: Date) => void;
  onClose: () => void;
}

export function TimestampPicker({ open, title, onPick, onClose }: Props) {
  const guard = useGhostClickGuard(open);
  const [customValue, setCustomValue] = useState('');

  useEffect(() => {
    if (open) setCustomValue(fmtLocalYMDHM(new Date()));
  }, [open]);

  const minutesAgo = (n: number) => new Date(Date.now() + n * 60000);

  function submitCustom() {
    if (!customValue) return;
    const picked = new Date(customValue);
    if (Number.isNaN(picked.getTime())) return;
    const now = new Date();
    // Defense in depth: browsers sometimes honor `max`, sometimes not; clamp
    // on submit so the server never sees a future timestamp.
    onPick(picked > now ? now : picked);
  }

  return (
    <Modal open={open} onClose={onClose} title={title}>
      <div class="ts-grid">
        <button class="ts-btn" onClick={guard(() => onPick(minutesAgo(-1)))}>1 min ago</button>
        <button class="ts-btn" onClick={guard(() => onPick(minutesAgo(-3)))}>3 min ago</button>
        <button class="ts-btn" onClick={guard(() => onPick(minutesAgo(-5)))}>5 min ago</button>
        <button class="ts-btn" onClick={guard(() => onPick(minutesAgo(-10)))}>10 min ago</button>
      </div>
      <div class="ts-custom">
        <input
          class="ts-input"
          type="datetime-local"
          value={customValue}
          max={fmtLocalYMDHM(new Date())}
          onChange={(e) => setCustomValue((e.currentTarget as HTMLInputElement).value)}
        />
        <button class="ts-btn" onClick={guard(submitCustom)}>Set time</button>
      </div>
    </Modal>
  );
}

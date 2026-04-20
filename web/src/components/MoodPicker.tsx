import { Modal } from './Modal';
import { useGhostClickGuard } from '../hooks/useGhostClickGuard';
import { MOODS, Mood } from '../ferber';

interface Props {
  open: boolean;
  onPick: (mood: Mood) => void;
  onClose: () => void;
  title?: string;
}

const LABELS: Record<Mood, string> = {
  quiet: '🙂 Quiet',
  fussy: '😣 Fussy',
  crying: '😭 Crying',
};

export function MoodPicker({ open, onPick, onClose, title }: Props) {
  const guard = useGhostClickGuard(open);
  return (
    <Modal open={open} onClose={onClose} title={title ?? "How is baby?"}>
      <div class="mood-grid">
        {MOODS.map(mood => (
          <button
            key={mood}
            class="mood-btn"
            onClick={guard(() => onPick(mood))}
          >
            {LABELS[mood]}
          </button>
        ))}
      </div>
    </Modal>
  );
}

import { Modal } from './Modal';
import { useGhostClickGuard } from '../hooks/useGhostClickGuard';
import { MOODS, Mood } from '../ferber';

interface Props {
  open: boolean;
  onPick: (mood: Mood) => void;
  onClose: () => void;
  title?: string;
}

const LABELS: Record<Mood, { emoji: string; word: string }> = {
  quiet:  { emoji: '🙂', word: 'Quiet' },
  fussy:  { emoji: '😣', word: 'Fussing' },
  crying: { emoji: '😭', word: 'Crying' },
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
            <span class="mood-btn-emoji">{LABELS[mood].emoji}</span>
            <span class="mood-btn-word">{LABELS[mood].word}</span>
          </button>
        ))}
      </div>
    </Modal>
  );
}

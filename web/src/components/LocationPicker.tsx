import { Modal } from './Modal';
import { useGhostClickGuard } from '../hooks/useGhostClickGuard';
import { Location } from '../api';
import { LOCATION_LABELS } from '../constants';

interface Props {
  open: boolean;
  onPick: (location: Location) => void;
  onClose: () => void;
}

const LOCATIONS: Location[] = ['crib', 'stroller', 'on_me', 'car'];

export function LocationPicker({ open, onPick, onClose }: Props) {
  const guard = useGhostClickGuard(open);

  return (
    <Modal open={open} onClose={onClose} title="Where?">
      <div class="location-grid">
        {LOCATIONS.map(loc => (
          <button
            key={loc}
            class="location-btn"
            onClick={guard(() => onPick(loc))}
          >
            <span class="location-icon">{LOCATION_LABELS[loc].icon}</span>
            <span>{LOCATION_LABELS[loc].label}</span>
          </button>
        ))}
      </div>
    </Modal>
  );
}

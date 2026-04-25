import { useState, useEffect } from 'preact/hooks';
import { Modal } from './Modal';
import { useGhostClickGuard } from '../hooks/useGhostClickGuard';
import { useConfig } from '../hooks/useConfig';

interface Props {
  open: boolean;
  // When present, seed the modal's Ferber toggle ON with this night number.
  // Absent → Ferber toggle starts OFF.
  suggestNightNumber?: number | null;
  // ferberConfig is null when the user confirms without Ferber.
  onConfirm: (ferberConfig: { nightNumber: number } | null) => void;
  onClose: () => void;
}

// StartNightPicker asks the user to optionally configure Ferber mode before
// creating the night session. Replaces the always-visible Ferber toggle that
// sat next to the Start Night button — keeps the main grid cleaner during
// the day, and lets the Ferber prompt be easily hidden later by wrapping the
// entire toggle block in a runtime flag.
export function StartNightPicker({ open, suggestNightNumber, onConfirm, onClose }: Props) {
  const ferberVisible = useConfig().features.ferber;
  const [ferberOn, setFerberOn] = useState(false);
  const [ferberNight, setFerberNight] = useState(1);
  const guard = useGhostClickGuard(open);

  // Re-seed whenever the modal opens (or the server's suggestion changes).
  // When Ferber is hidden, ignore any stale suggestion and keep the toggle off.
  useEffect(() => {
    if (!open) return;
    if (ferberVisible && suggestNightNumber != null) {
      setFerberOn(true);
      setFerberNight(suggestNightNumber);
    } else {
      setFerberOn(false);
      setFerberNight(1);
    }
  }, [open, suggestNightNumber, ferberVisible]);

  function handleConfirm() {
    onConfirm(ferberOn ? { nightNumber: ferberNight } : null);
  }

  return (
    <Modal open={open} onClose={onClose} title="Start night">
      <div class="start-night-picker">
        {ferberVisible && (
          <>
            <label class="ferber-toggle">
              <input
                type="checkbox"
                checked={ferberOn}
                onChange={e => setFerberOn((e.target as HTMLInputElement).checked)}
              />
              <span class="ferber-toggle-switch" aria-hidden="true"></span>
              <span class="ferber-toggle-label">Ferber mode</span>
            </label>
            <div class={`ferber-night-stepper ${ferberOn ? '' : 'is-hidden'}`} aria-hidden={!ferberOn}>
              Night
              <button
                type="button"
                aria-label="decrease"
                class={ferberNight === 1 ? 'is-hidden' : ''}
                onClick={() => setFerberNight(n => Math.max(1, n - 1))}
              >−</button>
              <span>{ferberNight}</span>
              <button
                type="button"
                aria-label="increase"
                onClick={() => setFerberNight(n => n + 1)}
              >+</button>
            </div>
          </>
        )}
        <button class="action-btn primary full-width" onClick={guard(handleConfirm)}>
          <span class="action-icon">🌙</span>
          <span>{ferberOn ? `Start Ferber night ${ferberNight}` : 'Start night'}</span>
        </button>
      </div>
    </Modal>
  );
}

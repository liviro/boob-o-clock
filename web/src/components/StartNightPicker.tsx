import { useState, useEffect } from 'preact/hooks';
import { Modal } from './Modal';
import { useGhostClickGuard } from '../hooks/useGhostClickGuard';
import { useConfig } from '../hooks/useConfig';
import { NightModeChoice } from '../api';

interface Props {
  open: boolean;
  // When present, seed as Ferber with this night number.
  suggestNightNumber?: number | null;
  // When true, seed as Chair.
  suggestChair?: boolean;
  onConfirm: (choice: NightModeChoice) => void;
  onClose: () => void;
}

// Picker state combines mode + ferber's per-night number. Modeling mode as a
// single field (not two booleans) makes "both modes set" unrepresentable.
type PickerState =
  | { mode: 'plain'; ferberNight: number }
  | { mode: 'ferber'; ferberNight: number }
  | { mode: 'chair'; ferberNight: number };

function seedFromProps(
  features: { ferber: boolean; chair: boolean },
  suggestNightNumber: number | null | undefined,
  suggestChair: boolean | undefined,
): PickerState {
  if (suggestNightNumber != null && features.ferber) {
    return { mode: 'ferber', ferberNight: suggestNightNumber };
  }
  if (suggestChair && features.chair) {
    return { mode: 'chair', ferberNight: 1 };
  }
  return { mode: 'plain', ferberNight: 1 };
}

export function StartNightPicker({ open, suggestNightNumber, suggestChair, onConfirm, onClose }: Props) {
  const { features } = useConfig();
  const [state, setState] = useState<PickerState>(() => seedFromProps(features, suggestNightNumber, suggestChair));
  const guard = useGhostClickGuard(open);

  useEffect(() => {
    if (!open) return;
    setState(seedFromProps(features, suggestNightNumber, suggestChair));
  }, [open, suggestNightNumber, suggestChair, features]);

  function setMode(mode: PickerState['mode']) {
    setState(s => ({ ...s, mode }));
  }

  function handleConfirm() {
    if (state.mode === 'ferber') {
      onConfirm({ mode: 'ferber', nightNumber: state.ferberNight });
    } else {
      onConfirm({ mode: state.mode });
    }
  }

  const buttonLabel =
    state.mode === 'ferber' ? `Start Ferber night ${state.ferberNight}` :
    state.mode === 'chair'  ? 'Start chair night' :
    'Start night';

  return (
    <Modal open={open} onClose={onClose} title="Start night">
      <div class="start-night-picker">
        {features.ferber && (
          <>
            <label class="ferber-toggle">
              <input
                type="checkbox"
                checked={state.mode === 'ferber'}
                onChange={e => setMode((e.target as HTMLInputElement).checked ? 'ferber' : 'plain')}
              />
              <span class="ferber-toggle-switch" aria-hidden="true"></span>
              <span class="ferber-toggle-label">Ferber mode</span>
            </label>
            <div class={`ferber-night-stepper ${state.mode === 'ferber' ? '' : 'is-hidden'}`} aria-hidden={state.mode !== 'ferber'}>
              Night
              <button
                type="button"
                aria-label="decrease"
                class={state.ferberNight === 1 ? 'is-hidden' : ''}
                onClick={() => setState(s => ({ ...s, ferberNight: Math.max(1, s.ferberNight - 1) }))}
              >−</button>
              <span>{state.ferberNight}</span>
              <button
                type="button"
                aria-label="increase"
                onClick={() => setState(s => ({ ...s, ferberNight: s.ferberNight + 1 }))}
              >+</button>
            </div>
          </>
        )}
        {features.chair && (
          <label class="ferber-toggle">
            <input
              type="checkbox"
              checked={state.mode === 'chair'}
              onChange={e => setMode((e.target as HTMLInputElement).checked ? 'chair' : 'plain')}
            />
            <span class="ferber-toggle-switch" aria-hidden="true"></span>
            <span class="ferber-toggle-label">Chair mode</span>
          </label>
        )}
        <button class="action-btn primary full-width" onClick={guard(handleConfirm)}>
          <span class="action-icon">🌙</span>
          <span>{buttonLabel}</span>
        </button>
      </div>
    </Modal>
  );
}

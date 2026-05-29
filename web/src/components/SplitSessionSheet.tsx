import { useState, useEffect } from 'preact/hooks';
import { Modal } from './Modal';
import { SessionMeta, splitSession } from '../api';
import { fmtLocalYMDHM, nextOccurrence, NIGHT_TRANSITION_HOUR, DAY_TRANSITION_HOUR } from '../constants';

interface Props {
  // The session being split (the >18h half). Drives copy + the default time.
  session: SessionMeta;
  onClose: () => void;
  // Called after a successful split so the parent can refetch in place.
  onSplit: () => void;
}

export function SplitSessionSheet({ session, onClose, onSplit }: Props) {
  const isNight = session.kind === 'night';
  // Night → split at the morning the day should have started (7am).
  // Day   → split at the evening the night should have started (8pm).
  const defaultHour = isNight ? NIGHT_TRANSITION_HOUR : DAY_TRANSITION_HOUR;
  const startedAt = new Date(session.startedAt);
  const defaultTime = nextOccurrence(defaultHour, startedAt);

  const [value, setValue] = useState(fmtLocalYMDHM(defaultTime));
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    // Reset to the computed default whenever the target session changes.
    // Keyed on session.id only — adding defaultTime (a fresh Date each render)
    // would clobber the user's edits on every render.
    setValue(fmtLocalYMDHM(defaultTime));
    setError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session.id]);

  const kindWord = isNight ? 'day' : 'night';
  const title = isNight ? 'Split this night' : 'Split this day';
  const prompt = `Pick the moment when the ${kindWord} should have started:`;

  async function submit() {
    if (!value || submitting) return;
    const picked = new Date(value);
    if (Number.isNaN(picked.getTime())) {
      setError('Pick a valid date and time.');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await splitSession(session.id, picked);
      onSplit();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Split failed. Try a different time.');
      setSubmitting(false);
    }
  }

  return (
    <Modal open onClose={onClose} title={title}>
      <p class="split-sheet-prompt">{prompt}</p>
      <div class="ts-custom">
        <input
          class="ts-input"
          type="datetime-local"
          value={value}
          max={fmtLocalYMDHM(new Date())}
          onChange={(e) => setValue((e.currentTarget as HTMLInputElement).value)}
        />
      </div>
      {error && <p class="split-sheet-error">{error}</p>}
      <p class="split-sheet-note">Splits can't be undone — pick carefully.</p>
      <button class="ts-btn split-sheet-confirm" onClick={submit} disabled={submitting}>
        {submitting ? 'Splitting…' : 'Split'}
      </button>
    </Modal>
  );
}

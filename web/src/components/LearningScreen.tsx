import { useState } from 'preact/hooks';
import { SessionResponse } from '../api';
import { otherMoods, Mood, MOOD_LABELS } from '../ferber';
import { fmtTimer } from '../constants';
import { useNow } from '../hooks/useNow';
import { ConfirmModal } from './ConfirmModal';

interface Props {
  session: SessionResponse;
  dispatch: (action: string, metadata?: Record<string, string>) => Promise<void>;
}

export function LearningScreen({ session, dispatch }: Props) {
  const now = useNow();
  const [confirmExit, setConfirmExit] = useState(false);

  // Tracker only renders LearningScreen when state is 'learning' and the
  // server populates ferber.current.checkInAvailableAt in that state.
  // Treat absence as a server/client invariant violation, not as benign.
  const current = session.ferber?.current;
  if (!current?.checkInAvailableAt) {
    throw new Error('LearningScreen: session.ferber.current.checkInAvailableAt is required in Learning state');
  }

  const availableAtMs = new Date(current.checkInAvailableAt).getTime();
  const countdownSec = Math.max(0, Math.floor((availableAtMs - now) / 1000));
  const readyToCheck = countdownSec === 0;

  const [mA, mB] = otherMoods(current.mood as Mood);

  const moodLabel = (m: Mood) => `${MOOD_LABELS[m].emoji} ${MOOD_LABELS[m].word}`;

  return (
    <div class="learning-screen">
      <div class="learning-buttons">
        <button
          class={`btn btn-checkin ${readyToCheck ? 'ready' : 'locked'}`}
          disabled={!readyToCheck}
          onClick={() => dispatch('check_in')}
        >
          {readyToCheck ? '👣 Check in' : `Wait ${fmtTimer(countdownSec)} until checking in`}
        </button>

        <button class="btn btn-mood" onClick={() => dispatch('mood_change', { mood: mA })}>
          {moodLabel(mA)}
        </button>
        <button class="btn btn-mood" onClick={() => dispatch('mood_change', { mood: mB })}>
          {moodLabel(mB)}
        </button>

        <button class="btn btn-settled" onClick={() => dispatch('settled')}>
          😴 Settled!
        </button>

        <button class="btn btn-danger" onClick={() => setConfirmExit(true)}>
          🏳️ Give up
        </button>
      </div>

      <ConfirmModal
        open={confirmExit}
        title="Give up for now? This ends the Ferber session."
        onConfirm={async () => { setConfirmExit(false); await dispatch('exit_ferber'); }}
        onCancel={() => setConfirmExit(false)}
      />
    </div>
  );
}

import { useState } from 'preact/hooks';
import { SessionResponse } from '../api';
import { intervalMinutes, otherMoods, Mood, MOOD_LABELS } from '../ferber';
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

  const nightNumber = session.ferberNightNumber ?? 1;
  const checkInCount = session.ferberCheckInCount ?? 0;
  const lastTickMs = session.ferberLastTick ? new Date(session.ferberLastTick).getTime() : now;
  const intervalSec = intervalMinutes(nightNumber, checkInCount + 1) * 60;
  const sinceTickSec = Math.max(0, Math.floor((now - lastTickMs) / 1000));
  const countdownSec = Math.max(0, intervalSec - sinceTickSec);
  const readyToCheck = countdownSec === 0;

  const currentMood = (session.ferberCurrentMood ?? 'quiet') as Mood;
  const [mA, mB] = otherMoods(currentMood);

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

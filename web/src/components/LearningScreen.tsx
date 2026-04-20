import { useState, useEffect } from 'preact/hooks';
import { SessionResponse } from '../api';
import { intervalMinutes, otherMoods, Mood } from '../ferber';
import { fmtTimer } from '../constants';
import { ConfirmModal } from './ConfirmModal';

interface Props {
  session: SessionResponse;
  dispatch: (action: string, metadata?: Record<string, string>) => Promise<void>;
}

export function LearningScreen({ session, dispatch }: Props) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  const [confirmExit, setConfirmExit] = useState(false);

  const nightNumber = session.ferberNightNumber ?? 1;
  const checkInCount = session.ferberCheckInCount ?? 0;
  const lastTickMs = session.ferberLastTick ? new Date(session.ferberLastTick).getTime() : now;
  const sessionStartMs = session.ferberSessionStart ? new Date(session.ferberSessionStart).getTime() : now;
  const intervalSec = intervalMinutes(nightNumber, checkInCount + 1) * 60;
  const sinceTickSec = Math.max(0, Math.floor((now - lastTickMs) / 1000));
  const countdownSec = Math.max(0, intervalSec - sinceTickSec);
  const readyToCheck = countdownSec === 0;
  const inCribSec = Math.max(0, Math.floor((now - sessionStartMs) / 1000));

  const currentMood = (session.ferberCurrentMood ?? 'quiet') as Mood;
  const [mA, mB] = otherMoods(currentMood);

  const moodLabel = (m: Mood) => m === 'quiet' ? '🙂 Quiet' : m === 'fussy' ? '😣 Fussy' : '😭 Crying';

  return (
    <div class="learning-screen">
      <div class="learning-timers">
        <div class={`learning-countdown ${readyToCheck ? 'ready' : ''}`}>
          {readyToCheck ? 'Check In' : `Check in: ${fmtTimer(countdownSec)}`}
        </div>
        <div class="learning-elapsed">In crib: {fmtTimer(inCribSec)}</div>
        <div class="learning-mood">Mood: {moodLabel(currentMood)}</div>
      </div>

      <div class="learning-buttons">
        <button
          class={`btn btn-checkin ${readyToCheck ? 'ready' : 'locked'}`}
          disabled={!readyToCheck}
          onClick={() => dispatch('check_in')}
        >
          {readyToCheck ? '👣 Check In' : `Wait ${fmtTimer(countdownSec)}`}
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
          🚪 Exit Learning
        </button>
      </div>

      <ConfirmModal
        open={confirmExit}
        title="Exit Learning? This ends the Ferber session."
        onConfirm={async () => { setConfirmExit(false); await dispatch('exit_ferber'); }}
        onCancel={() => setConfirmExit(false)}
      />
    </div>
  );
}

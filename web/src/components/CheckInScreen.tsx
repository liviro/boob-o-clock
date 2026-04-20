import { useState, useEffect } from 'preact/hooks';
import { SessionResponse } from '../api';
import { fmtTimer } from '../constants';
import { Mood } from '../ferber';
import { MoodPicker } from './MoodPicker';
import { ConfirmModal } from './ConfirmModal';

interface Props {
  session: SessionResponse;
  dispatch: (action: string, metadata?: Record<string, string>) => Promise<void>;
}

export function CheckInScreen({ session, dispatch }: Props) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  const [picker, setPicker] = useState(false);
  const [confirmExit, setConfirmExit] = useState(false);

  const startMs = session.ferberCheckInStart
    ? new Date(session.ferberCheckInStart).getTime()
    : now;
  const elapsedSec = Math.max(0, Math.floor((now - startMs) / 1000));

  return (
    <div class="checkin-screen">
      <div class="checkin-timer">Check-in: {fmtTimer(elapsedSec)}</div>

      <div class="checkin-buttons">
        <button class="btn btn-return" onClick={() => setPicker(true)}>
          🌱 Back to Learning
        </button>
        <button class="btn btn-settled" onClick={() => dispatch('settled')}>
          😴 Settled!
        </button>
        <button class="btn btn-danger" onClick={() => setConfirmExit(true)}>
          🚪 Exit Learning
        </button>
      </div>

      <MoodPicker
        open={picker}
        title="How is baby now?"
        onPick={async (mood: Mood) => {
          setPicker(false);
          await dispatch('end_check_in', { mood });
        }}
        onClose={() => setPicker(false)}
      />

      <ConfirmModal
        open={confirmExit}
        title="Exit Learning? This ends the Ferber session."
        onConfirm={async () => { setConfirmExit(false); await dispatch('exit_ferber'); }}
        onCancel={() => setConfirmExit(false)}
      />
    </div>
  );
}

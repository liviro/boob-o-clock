import { useState } from 'preact/hooks';
import { Mood } from '../ferber';
import { MoodPicker } from './MoodPicker';
import { ConfirmModal } from './ConfirmModal';

interface Props {
  dispatch: (action: string, metadata?: Record<string, string>) => Promise<void>;
}

export function CheckInScreen({ dispatch }: Props) {
  const [picker, setPicker] = useState(false);
  const [confirmExit, setConfirmExit] = useState(false);

  return (
    <div class="checkin-screen">
      <div class="checkin-buttons">
        <button class="btn btn-return" onClick={() => setPicker(true)}>
          🌱 Resume learning
        </button>
        <button class="btn btn-settled" onClick={() => dispatch('settled')}>
          😴 Settled!
        </button>
        <button class="btn btn-danger" onClick={() => setConfirmExit(true)}>
          🏳️ Give up
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
        title="Give up for now? This ends the Ferber session."
        onConfirm={async () => { setConfirmExit(false); await dispatch('exit_ferber'); }}
        onCancel={() => setConfirmExit(false)}
      />
    </div>
  );
}

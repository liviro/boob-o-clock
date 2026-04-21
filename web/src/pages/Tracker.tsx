import { useState, useRef, useEffect } from 'preact/hooks';
import { SessionResponse, StartNightConfig } from '../api';
import { ACTION_INFO, actionLabel } from '../constants';
import { Mood, moodWord } from '../ferber';
import { StateDisplay } from '../components/StateDisplay';
import { ActionGrid } from '../components/ActionGrid';
import { BreastPicker } from '../components/BreastPicker';
import { MoodPicker } from '../components/MoodPicker';
import { TimestampPicker } from '../components/TimestampPicker';
import { ConfirmModal } from '../components/ConfirmModal';
import { UndoButton } from '../components/UndoButton';
import { LearningScreen } from '../components/LearningScreen';
import { CheckInScreen } from '../components/CheckInScreen';

interface Props {
  session: SessionResponse;
  onDispatch: (action: string, metadata?: Record<string, string>, timestamp?: Date) => void;
  onStartNight: (config: StartNightConfig) => void;
  onUndo: () => void;
}

type ModalState =
  | { type: 'none' }
  | { type: 'breast'; action: string; wantsTimePicker: boolean }
  | { type: 'mood'; action: string; wantsTimePicker: boolean }
  | { type: 'timestamp'; action: string; metadata?: Record<string, string>; title: string }
  | { type: 'confirm'; action: string; wantsTimePicker: boolean };

export function Tracker({ session, onDispatch, onStartNight, onUndo }: Props) {
  const [modal, setModal] = useState<ModalState>({ type: 'none' });
  const longPressTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const longPressFired = useRef(false);

  const [ferberOn, setFerberOn] = useState(false);
  const [ferberNight, setFerberNight] = useState(1);

  useEffect(() => {
    const suggested = session.suggestFerberNight;
    if (suggested != null) {
      setFerberOn(true);
      setFerberNight(suggested);
    } else {
      setFerberOn(false);
      setFerberNight(1);
    }
  }, [session.suggestFerberNight]);

  useEffect(() => {
    return () => { if (longPressTimer.current) clearTimeout(longPressTimer.current); };
  }, []);

  function fireAction(action: string, metadata?: Record<string, string>, timestamp?: Date) {
    onDispatch(action, metadata, timestamp ?? new Date());
  }

  function handleStartNight() {
    onStartNight(ferberOn ? { ferber: { nightNumber: ferberNight } } : {});
  }

  function openTimePicker(action: string, metadata?: Record<string, string>) {
    const label = actionLabel(action);
    const breastSuffix = metadata?.breast ? ` (${metadata.breast})` : '';
    setModal({ type: 'timestamp', action, metadata, title: `${label}${breastSuffix} — When?` });
  }

  function switchBreastMeta(): Record<string, string> | undefined {
    return session.suggestBreast ? { breast: session.suggestBreast } : undefined;
  }

  function resolveAction(action: string, wantsTimePicker: boolean) {
    const ai = ACTION_INFO[action];

    if (ai?.confirm) {
      setModal({ type: 'confirm', action, wantsTimePicker });
      return;
    }

    if (action === 'switch_breast') {
      const meta = switchBreastMeta();
      if (meta) {
        wantsTimePicker ? openTimePicker(action, meta) : fireAction(action, meta);
        return;
      }
    }

    if (ai?.needsBreast) {
      setModal({ type: 'breast', action, wantsTimePicker });
      return;
    }

    if (ai?.needsMood) {
      setModal({ type: 'mood', action, wantsTimePicker });
      return;
    }

    wantsTimePicker ? openTimePicker(action) : fireAction(action);
  }

  const handleAction = (action: string) => resolveAction(action, false);
  const handleLongPress = (action: string) => resolveAction(action, true);

  function handlePointerDown(action: string) {
    longPressFired.current = false;
    longPressTimer.current = setTimeout(() => {
      longPressFired.current = true;
      handleLongPress(action);
      if (navigator.vibrate) navigator.vibrate(20);
    }, 400);
  }

  function handlePointerUp(action: string) {
    if (longPressTimer.current) {
      clearTimeout(longPressTimer.current);
      longPressTimer.current = null;
    }
    if (!longPressFired.current) {
      handleAction(action);
    }
  }

  function handlePointerCancel() {
    if (longPressTimer.current) {
      clearTimeout(longPressTimer.current);
      longPressTimer.current = null;
    }
  }

  function handleBreastPick(side: 'L' | 'R') {
    if (modal.type !== 'breast') return;
    const meta = { breast: side };
    setModal({ type: 'none' });
    modal.wantsTimePicker ? openTimePicker(modal.action, meta) : fireAction(modal.action, meta);
  }

  function handleMoodPick(mood: Mood) {
    if (modal.type !== 'mood') return;
    const meta = { mood };
    setModal({ type: 'none' });
    modal.wantsTimePicker ? openTimePicker(modal.action, meta) : fireAction(modal.action, meta);
  }

  function handleTimestampPick(minutesAgo: number) {
    if (modal.type !== 'timestamp') return;
    const ts = new Date(Date.now() + minutesAgo * 60000);
    onDispatch(modal.action, modal.metadata, ts);
    setModal({ type: 'none' });
  }

  function handleConfirm() {
    if (modal.type !== 'confirm') return;
    setModal({ type: 'none' });
    modal.wantsTimePicker ? openTimePicker(modal.action) : fireAction(modal.action);
  }

  function closeModal() {
    setModal({ type: 'none' });
  }

  return (
    <div class={`page-tracker ${session.state === 'night_off' ? 'is-night-off' : ''}`}>
      <StateDisplay
        state={session.state}
        lastEventTimestamp={session.lastEvent?.timestamp}
        currentBreast={session.currentBreast}
        lastFeedStartedAt={session.lastFeedStartedAt}
        sessionStartIso={session.state === 'learning' ? session.ferber?.current?.startedAt : undefined}
        moodLabel={session.state === 'learning' ? moodWord(session.ferber?.current?.mood) : undefined}
      />

      {session.state === 'night_off'
        ? (
          <>
            <div class="action-grid">
              <button class="action-btn primary full-width" onClick={handleStartNight}>
                <span class="action-icon">🌙</span>
                <span>Start night</span>
              </button>
            </div>
            <div class="ferber-start-row">
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
                <button type="button" aria-label="decrease"
                        class={ferberNight === 1 ? 'is-hidden' : ''}
                        onClick={() => setFerberNight(n => Math.max(1, n - 1))}>−</button>
                <span>{ferberNight}</span>
                <button type="button" aria-label="increase"
                        onClick={() => setFerberNight(n => n + 1)}>+</button>
              </div>
            </div>
          </>
        )
        : session.state === 'learning' && session.ferber
        ? (
          <LearningScreen
            session={session}
            dispatch={async (action, metadata) => { onDispatch(action, metadata); }}
          />
        )
        : session.state === 'check_in'
        ? (
          <CheckInScreen
            dispatch={async (action, metadata) => { onDispatch(action, metadata); }}
          />
        )
        : (
          <ActionGrid
            actions={session.validActions}
            onPointerDown={handlePointerDown}
            onPointerUp={handlePointerUp}
            onPointerCancel={handlePointerCancel}
          />
        )
      }

      <div class="bottom-bar">
        <UndoButton
          lastAction={session.lastEvent?.action}
          onUndo={onUndo}
        />
      </div>

      <div class="hint">hold for time adjust</div>

      <BreastPicker
        open={modal.type === 'breast'}
        suggestSide={session.suggestBreast}
        onPick={handleBreastPick}
        onClose={closeModal}
      />

      <MoodPicker
        open={modal.type === 'mood'}
        onPick={handleMoodPick}
        onClose={closeModal}
      />

      <TimestampPicker
        open={modal.type === 'timestamp'}
        title={modal.type === 'timestamp' ? modal.title : ''}
        onPick={handleTimestampPick}
        onClose={closeModal}
      />

      <ConfirmModal
        open={modal.type === 'confirm'}
        title={modal.type === 'confirm'
          ? `${ACTION_INFO[modal.action]?.icon || ''} ${actionLabel(modal.action)} — are you sure?`
          : ''}
        onConfirm={handleConfirm}
        onCancel={closeModal}
      />
    </div>
  );
}

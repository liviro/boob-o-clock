import { useState, useRef, useEffect } from 'preact/hooks';
import { SessionResponse, StartSessionConfig, Location } from '../api';
import { ACTION_INFO, actionLabel } from '../constants';
import { Mood, moodWord } from '../ferber';
import { StateDisplay } from '../components/StateDisplay';
import { ActionGrid } from '../components/ActionGrid';
import { BreastPicker } from '../components/BreastPicker';
import { MoodPicker } from '../components/MoodPicker';
import { LocationPicker } from '../components/LocationPicker';
import { StartNightPicker } from '../components/StartNightPicker';
import { TimestampPicker } from '../components/TimestampPicker';
import { ConfirmModal } from '../components/ConfirmModal';
import { UndoButton } from '../components/UndoButton';
import { LearningScreen } from '../components/LearningScreen';
import { CheckInScreen } from '../components/CheckInScreen';

interface Props {
  session: SessionResponse;
  onDispatch: (action: string, metadata?: Record<string, string>, timestamp?: Date) => void;
  onStartSession: (config: StartSessionConfig) => void;
  onUndo: () => void;
}

type FerberConfig = { nightNumber: number } | null;

type ModalState =
  | { type: 'none' }
  | { type: 'breast'; action: string; wantsTimePicker: boolean }
  | { type: 'mood'; action: string; wantsTimePicker: boolean }
  | { type: 'location'; action: string; wantsTimePicker: boolean }
  | { type: 'startNight'; wantsTimePicker: boolean }
  // The timestamp picker can carry a ferber config when the source was a
  // long-press on Start Night (StartNightPicker → TimestampPicker → fire).
  | { type: 'timestamp'; action: string; metadata?: Record<string, string>; ferberConfig?: FerberConfig; title: string }
  | { type: 'confirm'; action: string; wantsTimePicker: boolean };

// Chain-advance actions create a new session via POST /api/session/start rather
// than appending an event. Tracker intercepts these and routes to onStartSession.
const CHAIN_ACTIONS = new Set(['start_day', 'start_night']);

export function Tracker({ session, onDispatch, onStartSession, onUndo }: Props) {
  const [modal, setModal] = useState<ModalState>({ type: 'none' });
  const longPressTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const longPressFired = useRef(false);

  useEffect(() => {
    return () => { if (longPressTimer.current) clearTimeout(longPressTimer.current); };
  }, []);

  function fireAction(action: string, metadata?: Record<string, string>, timestamp?: Date) {
    if (CHAIN_ACTIONS.has(action)) {
      // Reaches here only for start_day (direct fire) — start_night always
      // routes through the Ferber modal in resolveAction.
      fireChainAdvance(action, timestamp);
      return;
    }
    onDispatch(action, metadata, timestamp ?? new Date());
  }

  function fireChainAdvance(action: string, timestamp?: Date, ferber?: FerberConfig) {
    const kind = action === 'start_night' ? 'night' : 'day';
    const config: StartSessionConfig = { kind };
    if (kind === 'night' && ferber) {
      config.ferber = ferber;
    }
    if (timestamp) {
      config.timestamp = timestamp;
    }
    onStartSession(config);
  }

  function openTimePicker(action: string, metadata?: Record<string, string>, ferberConfig?: FerberConfig) {
    const label = actionLabel(action);
    const breastSuffix = metadata?.breast ? ` (${metadata.breast})` : '';
    setModal({ type: 'timestamp', action, metadata, ferberConfig, title: `${label}${breastSuffix} — When?` });
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

    // Starting a night always goes through the Ferber-config modal first,
    // regardless of tap-vs-long-press (long-press just chains into the
    // timestamp picker afterward). start_day has no config, so it fires
    // directly (or through the timestamp picker on long-press).
    if (action === 'start_night') {
      setModal({ type: 'startNight', wantsTimePicker });
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

    if (ai?.needsLocation) {
      setModal({ type: 'location', action, wantsTimePicker });
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

  // finishPick closes a metadata-picker modal and either fires the action
  // directly or chains into the timestamp picker (for long-press flows).
  // Shared across breast/mood/location handlers.
  function finishPick(expectedType: 'breast' | 'mood' | 'location', meta: Record<string, string>) {
    if (modal.type !== expectedType) return;
    const wantsTimePicker = modal.wantsTimePicker;
    const action = modal.action;
    setModal({ type: 'none' });
    if (wantsTimePicker) openTimePicker(action, meta);
    else fireAction(action, meta);
  }

  const handleBreastPick = (side: 'L' | 'R') => finishPick('breast', { breast: side });
  const handleMoodPick = (mood: Mood) => finishPick('mood', { mood });
  const handleLocationPick = (location: Location) => finishPick('location', { location });

  function handleStartNightConfirm(ferberConfig: FerberConfig) {
    if (modal.type !== 'startNight') return;
    const wantsTimePicker = modal.wantsTimePicker;
    setModal({ type: 'none' });
    if (wantsTimePicker) {
      openTimePicker('start_night', undefined, ferberConfig);
    } else {
      fireChainAdvance('start_night', undefined, ferberConfig);
    }
  }

  function handleTimestampPick(minutesAgo: number) {
    if (modal.type !== 'timestamp') return;
    const ts = new Date(Date.now() + minutesAgo * 60000);
    if (CHAIN_ACTIONS.has(modal.action)) {
      fireChainAdvance(modal.action, ts, modal.ferberConfig);
    } else {
      onDispatch(modal.action, modal.metadata, ts);
    }
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

      {session.state === 'learning' && session.ferber
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

      <LocationPicker
        open={modal.type === 'location'}
        onPick={handleLocationPick}
        onClose={closeModal}
      />

      <StartNightPicker
        open={modal.type === 'startNight'}
        suggestNightNumber={session.suggestFerberNight}
        onConfirm={handleStartNightConfirm}
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

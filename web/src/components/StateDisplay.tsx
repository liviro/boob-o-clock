import { STATE_INFO, fmtTimer, fmtAgo } from '../constants';
import { useNow } from '../hooks/useNow';

interface Props {
  state: string;
  lastEventTimestamp?: string;
  currentBreast?: string;
  lastFeedStartedAt?: string;
  lastSleepStartedAt?: string;
  sessionStartIso?: string;
  moodLabel?: string;
}

export function StateDisplay({ state, lastEventTimestamp, currentBreast, lastFeedStartedAt, lastSleepStartedAt, sessionStartIso, moodLabel }: Props) {
  const info = STATE_INFO[state] || { icon: '?', label: state };
  const now = useNow();
  const ageMs = (iso?: string) => iso ? Math.max(0, now - new Date(iso).getTime()) : 0;

  // During day_awake the "how long have I been awake" ticker tells us nothing
  // we can't see already — replace it with "when did the baby last nap/sleep."
  const showStateElapsed = state !== 'night_off' && state !== 'learning' && state !== 'day_awake' && !!lastEventTimestamp;
  const showFeedAgo = !!lastFeedStartedAt && state !== 'feeding' && state !== 'day_feeding' && state !== 'night_off';
  const showSleepAgo = state === 'day_awake' && !!lastSleepStartedAt;

  const isFeeding = state === 'feeding' && currentBreast;
  const flipIcon = isFeeding && currentBreast === 'R';
  const subLabel = isFeeding
    ? (currentBreast === 'L' ? 'Left' : 'Right')
    : moodLabel;

  return (
    <div class="state-display">
      <span class={`state-icon${flipIcon ? ' flip' : ''}`}>{info.icon}</span>
      <span class="state-label">
        {subLabel ? `${info.label} — ${subLabel}` : info.label}
      </span>
      {showStateElapsed && (
        <div class="state-timer">{fmtTimer(Math.floor(ageMs(lastEventTimestamp) / 1000))} in this state</div>
      )}
      {sessionStartIso && (
        <div class="state-timer">{fmtTimer(Math.floor(ageMs(sessionStartIso) / 1000))} in this session</div>
      )}
      {showSleepAgo && (
        <div class="state-timer"><span class="state-timer-emoji">💤</span>{fmtAgo(ageMs(lastSleepStartedAt))}</div>
      )}
      {showFeedAgo && (
        <div class="state-timer"><span class="state-timer-emoji">🍼</span>{fmtAgo(ageMs(lastFeedStartedAt))}</div>
      )}
    </div>
  );
}

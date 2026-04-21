import { STATE_INFO, fmtTimer, fmtAgo } from '../constants';
import { useNow } from '../hooks/useNow';

interface Props {
  state: string;
  lastEventTimestamp?: string;
  currentBreast?: string;
  lastFeedStartedAt?: string;
  sessionStartIso?: string;
  moodLabel?: string;
}

export function StateDisplay({ state, lastEventTimestamp, currentBreast, lastFeedStartedAt, sessionStartIso, moodLabel }: Props) {
  const info = STATE_INFO[state] || { icon: '?', label: state };
  const now = useNow();

  const elapsed = lastEventTimestamp && state !== 'night_off'
    ? Math.max(0, Math.floor((now - new Date(lastEventTimestamp).getTime()) / 1000))
    : 0;

  const sessionSec = sessionStartIso
    ? Math.max(0, Math.floor((now - new Date(sessionStartIso).getTime()) / 1000))
    : 0;

  const showFeedAgo = !!lastFeedStartedAt && state !== 'feeding' && state !== 'night_off';
  const feedAgoMs = showFeedAgo
    ? Math.max(0, now - new Date(lastFeedStartedAt!).getTime())
    : 0;

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
      {state !== 'night_off' && state !== 'learning' && lastEventTimestamp && (
        <div class="state-timer">{fmtTimer(elapsed)} in this state</div>
      )}
      {sessionStartIso && (
        <div class="state-timer">{fmtTimer(sessionSec)} in this session</div>
      )}
      {showFeedAgo && (
        <div class="state-timer">🍼 {fmtAgo(feedAgoMs)}</div>
      )}
    </div>
  );
}

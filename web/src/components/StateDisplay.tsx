import { useState, useEffect } from 'preact/hooks';
import { STATE_INFO, fmtTimer, fmtAgo } from '../constants';

interface Props {
  state: string;
  lastEventTimestamp?: string;
  currentBreast?: string;
  lastFeedStartedAt?: string;
}

export function StateDisplay({ state, lastEventTimestamp, currentBreast, lastFeedStartedAt }: Props) {
  const info = STATE_INFO[state] || { icon: '?', label: state };
  const [elapsed, setElapsed] = useState(0);
  const [feedAgoMs, setFeedAgoMs] = useState(0);

  useEffect(() => {
    if (!lastEventTimestamp || state === 'night_off') {
      setElapsed(0);
      return;
    }

    const start = new Date(lastEventTimestamp).getTime();
    const update = () => setElapsed(Math.floor((Date.now() - start) / 1000));
    update();
    const id = setInterval(update, 1000);
    return () => clearInterval(id);
  }, [lastEventTimestamp, state]);

  const showFeedAgo = !!lastFeedStartedAt && state !== 'feeding' && state !== 'night_off';

  useEffect(() => {
    if (!showFeedAgo) return;
    const start = new Date(lastFeedStartedAt!).getTime();
    const update = () => setFeedAgoMs(Date.now() - start);
    update();
    const id = setInterval(update, 60_000);
    return () => clearInterval(id);
  }, [lastFeedStartedAt, showFeedAgo]);

  const isFeeding = state === 'feeding' && currentBreast;
  const sideLabel = currentBreast === 'L' ? 'Left' : 'Right';
  const flipIcon = isFeeding && currentBreast === 'R';

  return (
    <div class="state-display">
      <span class={`state-icon${flipIcon ? ' flip' : ''}`}>{info.icon}</span>
      <span class="state-label">
        {isFeeding ? `${info.label} — ${sideLabel}` : info.label}
      </span>
      {state !== 'night_off' && lastEventTimestamp && (
        <div class="state-timer">{fmtTimer(elapsed)} in this state</div>
      )}
      {showFeedAgo && (
        <div class="state-timer">🍼 {fmtAgo(feedAgoMs)}</div>
      )}
    </div>
  );
}

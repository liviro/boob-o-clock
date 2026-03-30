import { useState, useEffect } from 'preact/hooks';
import { STATE_INFO, fmtTimer } from '../constants';

interface Props {
  state: string;
  lastEventTimestamp?: string;
  currentBreast?: string;
}

export function StateDisplay({ state, lastEventTimestamp, currentBreast }: Props) {
  const info = STATE_INFO[state] || { icon: '?', label: state };
  const [elapsed, setElapsed] = useState(0);

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

  const isFeeding = state === 'feeding' && currentBreast;
  const sideLabel = currentBreast === 'L' ? 'Left' : 'Right';
  const flipIcon = currentBreast === 'R';

  return (
    <div class="state-display">
      <span class={`state-icon${flipIcon ? ' flip' : ''}`}>{info.icon}</span>
      <span class="state-label">
        {isFeeding ? `${info.label} — ${sideLabel}` : info.label}
      </span>
      {state !== 'night_off' && lastEventTimestamp && (
        <div class="state-timer">{fmtTimer(elapsed)} in this state</div>
      )}
    </div>
  );
}

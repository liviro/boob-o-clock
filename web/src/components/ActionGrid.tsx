import { ACTION_INFO } from '../constants';

interface Props {
  actions: string[];
  onPointerDown: (action: string) => void;
  onPointerUp: (action: string) => void;
  onPointerCancel: () => void;
  // When set, the named action's button glows and shows the cue line — the
  // "you're in the wrong mode for the time of day" Start-day/night nudge.
  nudge?: { action: string; cue: string } | null;
}

// States with <= this many actions render every button full-width.
// Day-awake has 5 (Feed, Nap, Solids, Poop, Start night) and its lone
// half-width action (Poop) has nothing to pair with, so it reads best all
// full-width. Denser states (Awake has 6) stay below the threshold so their
// two half-width actions (Stroller/Poop) can pair into one row.
const FULL_WIDTH_THRESHOLD = 5;

export function ActionGrid({ actions, onPointerDown, onPointerUp, onPointerCancel, nudge }: Props) {
  const allFull = actions.length <= FULL_WIDTH_THRESHOLD;
  return (
    <div class="action-grid">
      {actions.map(action => {
        const ai = ACTION_INFO[action] || { icon: '?', label: action, cls: '' };
        let cls = allFull && !ai.cls.includes('full-width') ? `${ai.cls} full-width` : ai.cls;
        const nudged = nudge?.action === action;
        if (nudged) cls += ' nudge';
        return (
          <button
            key={action}
            class={`action-btn ${cls}`}
            onTouchStart={(e) => { e.preventDefault(); onPointerDown(action); }}
            onTouchEnd={() => onPointerUp(action)}
            onTouchCancel={onPointerCancel}
            onMouseDown={() => onPointerDown(action)}
            onMouseUp={() => onPointerUp(action)}
            onMouseLeave={onPointerCancel}
          >
            <span class="action-icon">{ai.icon}</span>
            <span>{ai.label.split('\n').map((line, i) =>
              i > 0 ? [<br />, line] : line
            )}</span>
            {nudged && <span class="action-cue">{nudge!.cue}</span>}
          </button>
        );
      })}
    </div>
  );
}
